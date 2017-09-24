#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Olla Sticky Session Test Script

Validates that sticky session (KV cache affinity) routing works correctly.
Sends repeated requests with identical content or explicit session IDs and
verifies that the same backend is selected on subsequent turns.

Auto-discovers available backends and models, then runs a test matrix covering
session ID pinning, prefix-hash affinity, session independence, and header
validation.
"""

import sys
import json
import time
import argparse
import requests
import os
from typing import Dict, List, Optional, Any

# Fix Windows console encoding for Unicode
if sys.platform == 'win32':
    import io
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')
    os.environ['PYTHONIOENCODING'] = 'utf-8'

# ANSI colour codes
RED = '\033[0;31m'
GREEN = '\033[0;32m'
YELLOW = '\033[1;33m'
BLUE = '\033[0;34m'
PURPLE = '\033[0;35m'
CYAN = '\033[0;36m'
WHITE = '\033[1;37m'
GREY = '\033[0;37m'
RESET = '\033[0m'
BOLD = '\033[1m'

# Configuration
TARGET_URL = "http://localhost:40114"
DEFAULT_TIMEOUT = 30

# Valid values for sticky session response headers
VALID_STICKY_RESULTS = {"hit", "miss", "repin", "disabled"}
VALID_KEY_SOURCES = {"session_header", "prefix_hash", "auth_header", "ip", "none"}

# Long system prompt used for prefix-hash tests — must be > 512 bytes so the
# hash captures real content rather than colliding on short strings.
LONG_SYSTEM_PROMPT = (
    "You are a highly knowledgeable Go programming expert. "
    "You specialise in concurrent systems, high-performance proxies, and load "
    "balancing algorithms. You write clean, idiomatic Go following effective Go "
    "conventions. You always explain the why behind design decisions, not just "
    "the what. You are familiar with the Olla project and its hexagonal "
    "architecture. When reviewing code you consider correctness, performance, "
    "and maintainability equally."
)

ALTERNATIVE_SYSTEM_PROMPT = (
    "You are a Python data science expert specialising in pandas, NumPy, and "
    "scikit-learn. You focus on efficient data pipelines, reproducible research, "
    "and clear visualisations. Your answers are concise and include runnable "
    "code examples."
)


class BackendInfo:
    """Discovered backend with its health, type, and selected model."""
    __slots__ = ("name", "backend_type", "status", "models", "selected_model")

    def __init__(self, name: str, backend_type: str, status: str):
        self.name = name
        self.backend_type = backend_type
        self.status = status
        self.models: List[str] = []
        self.selected_model: Optional[str] = None


class TestResult:
    """Outcome of a single test case."""
    __slots__ = ("name", "passed", "detail")

    def __init__(self, name: str, passed: bool, detail: str = ""):
        self.name = name
        self.passed = passed
        self.detail = detail


class StickySessionTester:
    def __init__(self, base_url: str, timeout: int, verbose: bool, model: Optional[str]):
        self.base_url = base_url
        self.timeout = timeout
        self.verbose = verbose
        self.forced_model = model
        self.backends: List[BackendInfo] = []
        self.results: List[TestResult] = []
        self.sticky_enabled: Optional[bool] = None  # detected from first request

    # ── Helpers ─────────────────────────────────────────────────────────

    def pcolor(self, color: str, msg: str, end: str = '\n'):
        print(f"{color}{msg}{RESET}", end=end)
        sys.stdout.flush()

    def print_header(self):
        self.pcolor(PURPLE, "=" * 72)
        self.pcolor(PURPLE, f"  {CYAN}Olla Sticky Session Test{RESET}")
        self.pcolor(PURPLE, f"  {GREY}Validates KV cache affinity routing behaviour{RESET}")
        self.pcolor(PURPLE, "=" * 72)
        print()

    def record(self, name: str, passed: bool, detail: str = "") -> bool:
        self.results.append(TestResult(name, passed, detail))
        return passed

    def _chat_headers(self, session_id: Optional[str] = None) -> Dict[str, str]:
        h = {"Content-Type": "application/json"}
        if session_id:
            h["X-Olla-Session-ID"] = session_id
        return h

    def _chat_body(self, model: str, system: Optional[str] = None,
                   user_turn: str = "Reply with one word: yes") -> Dict[str, Any]:
        messages: List[Dict[str, str]] = []
        if system:
            messages.append({"role": "system", "content": system})
        messages.append({"role": "user", "content": user_turn})
        return {
            "model": model,
            "messages": messages,
            "max_tokens": 5,
        }

    def _post(self, body: Dict[str, Any],
              headers: Optional[Dict[str, str]] = None) -> Optional[requests.Response]:
        """POST to the OpenAI-compatible proxy endpoint. Returns None on network error."""
        try:
            return requests.post(
                f"{self.base_url}/olla/proxy/v1/chat/completions",
                headers=headers or {"Content-Type": "application/json"},
                json=body,
                timeout=self.timeout,
            )
        except Exception as e:
            self.pcolor(RED, f"  [ERROR] Request failed: {e}")
            return None

    # ── Discovery ───────────────────────────────────────────────────────

    def check_health(self) -> bool:
        self.pcolor(YELLOW, "Checking Olla availability...")
        try:
            r = requests.get(f"{self.base_url}/internal/health", timeout=5)
            if r.status_code == 200:
                self.pcolor(GREEN, "[OK] Olla is reachable")
                return True
        except Exception:
            pass
        self.pcolor(RED, f"[FAIL] Cannot reach Olla at {self.base_url}")
        return False

    def discover(self) -> bool:
        """Discover endpoints and pick a model for testing."""
        self.pcolor(YELLOW, "Discovering endpoints...")
        try:
            r = requests.get(f"{self.base_url}/internal/status/endpoints", timeout=self.timeout)
            r.raise_for_status()
            endpoints = r.json().get("endpoints", [])
        except Exception as e:
            self.pcolor(RED, f"[FAIL] Endpoint discovery failed: {e}")
            return False

        ep_map: Dict[str, BackendInfo] = {}
        for ep in endpoints:
            name = ep.get("name", "unknown")
            btype = ep.get("type", "unknown")
            status = ep.get("status", "unknown")
            bi = BackendInfo(name, btype, status)
            ep_map[name] = bi
            self.backends.append(bi)

        if not self.backends:
            self.pcolor(RED, "[FAIL] No endpoints discovered")
            return False

        self.pcolor(GREEN, f"[OK] Found {len(self.backends)} endpoint(s)")

        # Fetch models
        self.pcolor(YELLOW, "Discovering models...")
        try:
            r = requests.get(f"{self.base_url}/internal/status/models", timeout=self.timeout)
            r.raise_for_status()
            data = r.json()
            recent = data.get("recent_models", [])
        except Exception as e:
            self.pcolor(RED, f"[FAIL] Model discovery failed: {e}")
            return False

        # Build URL → endpoint name reverse map
        url_to_name: Dict[str, str] = {}
        for ep in endpoints:
            url = ep.get("url", "")
            name = ep.get("name", "")
            if url and name:
                url_to_name[url.rstrip("/")] = name

        type_to_backends: Dict[str, List[str]] = {}
        for ep in endpoints:
            btype = ep.get("type", "")
            name = ep.get("name", "")
            if btype and name:
                type_to_backends.setdefault(btype, []).append(name)

        for m in recent:
            model_name = m.get("name", "")
            model_endpoints = m.get("endpoints", [])
            model_type = m.get("type", "")
            matched = False

            for ep_url in model_endpoints:
                ep_name = url_to_name.get(ep_url.rstrip("/"))
                if ep_name and ep_name in ep_map:
                    ep_map[ep_name].models.append(model_name)
                    matched = True

            if not matched and model_type and model_type in type_to_backends:
                for ep_name in type_to_backends[model_type]:
                    if ep_name in ep_map:
                        ep_map[ep_name].models.append(model_name)

        # Select one non-embedding model per backend
        for bi in self.backends:
            if self.forced_model:
                bi.selected_model = self.forced_model
            else:
                candidates = [m for m in bi.models if "embed" not in m.lower()]
                if candidates:
                    bi.selected_model = candidates[0]

        self._print_discovery_summary()
        return True

    def _print_discovery_summary(self):
        print()
        self.pcolor(WHITE, "Configuration Summary")
        self.pcolor(PURPLE, "-" * 72)
        header = f"  {'Backend':<20} {'Type':<18} {'Status':<10} {'Model'}"
        self.pcolor(GREY, header)
        self.pcolor(GREY, f"  {'-'*20} {'-'*18} {'-'*10} {'-'*24}")

        for bi in self.backends:
            status_c = GREEN if bi.status == "healthy" else RED
            status_str = f"{status_c}{bi.status:<10}{RESET}"
            model_str = bi.selected_model or "(none)"
            print(f"  {bi.name:<20} {bi.backend_type:<18} {status_str} {model_str}")

    def _testable(self) -> List[BackendInfo]:
        return [b for b in self.backends if b.status == "healthy" and b.selected_model]

    # ── Feature detection ────────────────────────────────────────────────

    def detect_sticky_enabled(self, model: str) -> bool:
        """
        Make one probe request and inspect X-Olla-Sticky-Session.
        Returns True if sticky sessions are active, False if disabled.
        """
        self.pcolor(YELLOW, "\nDetecting sticky session status...")
        r = self._post(self._chat_body(model))
        if r is None or r.status_code != 200:
            self.pcolor(RED, "  [FAIL] Probe request failed — cannot detect feature status")
            return False

        val = r.headers.get("X-Olla-Sticky-Session", "")
        if val == "disabled":
            self.pcolor(YELLOW, (
                "  [WARN] Sticky sessions are disabled in Olla config.\n"
                "         Affinity tests will be skipped.\n"
                "         To enable, add to config.yaml under proxy:\n"
                "           sticky_sessions:\n"
                "             enabled: true"
            ))
            self.sticky_enabled = False
            return False

        if val in VALID_STICKY_RESULTS:
            self.pcolor(GREEN, f"  [OK] Sticky sessions active (probe returned: {val})")
            self.sticky_enabled = True
            return True

        # Header absent — old binary or feature not built
        self.pcolor(YELLOW, f"  [WARN] X-Olla-Sticky-Session header absent (value: '{val}')")
        self.sticky_enabled = False
        return False

    # ── Test Sections ────────────────────────────────────────────────────

    def run_header_validation(self, model: str):
        """
        Basic header presence and value validation — runs regardless of whether
        sticky sessions are enabled, because the 'disabled' value is also valid.
        """
        print()
        self.pcolor(WHITE, "Header Validation")
        self.pcolor(PURPLE, "=" * 72)

        self._test_sticky_header_present(model)
        self._test_key_source_valid(model)

    def run_affinity_tests(self, model: str):
        """Core affinity tests — only meaningful when sticky sessions are enabled."""
        print()
        self.pcolor(WHITE, "Affinity Tests")
        self.pcolor(PURPLE, "=" * 72)

        if not self.sticky_enabled:
            self.pcolor(GREY, "  Skipped — sticky sessions are disabled")
            return

        self._test_session_id_miss_then_hit(model)
        self._test_session_id_echoed(model)
        self._test_prefix_hash_hit(model)
        self._test_sessions_independent(model)

    def run_multi_backend_tests(self):
        """
        Tests that only make sense when multiple healthy backends are available,
        e.g. verifying that different session IDs can land on different backends.
        Skipped silently when only one backend is present.
        """
        testable = self._testable()
        if len(testable) < 2:
            return

        print()
        self.pcolor(WHITE, "Multi-Backend Tests")
        self.pcolor(PURPLE, "=" * 72)

        if not self.sticky_enabled:
            self.pcolor(GREY, "  Skipped — sticky sessions are disabled")
            return

        self._test_different_sessions_may_use_different_backends(testable)

    # ── Individual Tests ─────────────────────────────────────────────────

    def _test_sticky_header_present(self, model: str):
        label = "header/sticky-session-present"
        self.pcolor(YELLOW, "  X-Olla-Sticky-Session present:  ", end="")

        r = self._post(self._chat_body(model))
        if r is None:
            print(f"{RED}[FAIL]{RESET}")
            self.record(label, False, "request failed")
            return
