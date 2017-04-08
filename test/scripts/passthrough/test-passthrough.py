#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Olla Anthropic Passthrough Test Script

Validates that the Anthropic Messages API translator correctly uses passthrough
mode for backends with native support (vllm, vllm-mlx, lm-studio, ollama,
llamacpp) and falls back to translation for others (openai-compatible, litellm).

Auto-discovers available backends and models, then runs a test matrix covering
non-streaming, streaming, OpenAI baseline, edge cases, and translator stats.
"""

import sys
import json
import time
import argparse
import requests
import os
from typing import Dict, List, Optional, Any, Tuple

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

# Backend types that natively support the Anthropic Messages API
PASSTHROUGH_TYPES = {"vllm", "vllm-mlx", "lm-studio", "ollama", "llamacpp", "lemonade"}
# Backend types that require translation (Anthropic -> OpenAI -> backend -> OpenAI -> Anthropic)
TRANSLATION_TYPES = {"openai-compatible", "litellm"}

# Anthropic SSE event types expected in a well-formed streaming response
PASSTHROUGH_EVENTS = {
    "message_start", "content_block_start", "content_block_delta",
    "content_block_stop", "message_delta", "message_stop",
}
TRANSLATION_MIN_EVENTS = {"message_start", "message_delta", "message_stop"}


class BackendInfo:
    """Discovered backend with its health, type, and selected model."""
    __slots__ = ("name", "backend_type", "status", "models", "selected_model")

    def __init__(self, name: str, backend_type: str, status: str):
        self.name = name
        self.backend_type = backend_type
        self.status = status
        self.models: List[str] = []
        self.selected_model: Optional[str] = None

    @property
    def expects_passthrough(self) -> bool:
        return self.backend_type in PASSTHROUGH_TYPES

    @property
    def mode_label(self) -> str:
        return "passthrough" if self.expects_passthrough else "translation"


class TestResult:
    """Outcome of a single test case."""
    __slots__ = ("name", "passed", "detail")

    def __init__(self, name: str, passed: bool, detail: str = ""):
        self.name = name
        self.passed = passed
        self.detail = detail


class PassthroughTester:
    def __init__(self, base_url: str, timeout: int, verbose: bool):
        self.base_url = base_url
        self.timeout = timeout
        self.verbose = verbose
        self.backends: List[BackendInfo] = []
        self.results: List[TestResult] = []

    # ── Helpers ─────────────────────────────────────────────────────────

    def pcolor(self, color: str, msg: str, end: str = '\n'):
        print(f"{color}{msg}{RESET}", end=end)
        sys.stdout.flush()

    def print_header(self):
        self.pcolor(PURPLE, "=" * 72)
        self.pcolor(PURPLE, f"  {CYAN}Olla Anthropic Passthrough Test{RESET}")
        self.pcolor(PURPLE, f"  {GREY}Validates passthrough vs translation mode selection{RESET}")
        self.pcolor(PURPLE, "=" * 72)
        print()

    def record(self, name: str, passed: bool, detail: str = "") -> bool:
        self.results.append(TestResult(name, passed, detail))
        return passed

    def _anthropic_headers(self) -> Dict[str, str]:
        return {
            "Content-Type": "application/json",
            "x-api-key": "test-key",
            "anthropic-version": "2023-06-01",
        }

    def _anthropic_body(self, model: str, stream: bool = False,
                        system: Optional[str] = None,
                        messages: Optional[List[Dict]] = None) -> Dict[str, Any]:
        body: Dict[str, Any] = {
            "model": model,
            "messages": messages or [{"role": "user", "content": "Say hello briefly"}],
            "max_tokens": 10,
        }
        if stream:
            body["stream"] = True
        if system:
            body["system"] = system
        return body

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
        """Discover endpoints, map models to backends, pick one model each."""
        # Fetch endpoints
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

        # Fetch models and map to endpoints
        self.pcolor(YELLOW, "Discovering models...")
        try:
            r = requests.get(f"{self.base_url}/internal/status/models", timeout=self.timeout)
            r.raise_for_status()
            data = r.json()
            recent = data.get("recent_models", [])
        except Exception as e:
            self.pcolor(RED, f"[FAIL] Model discovery failed: {e}")
            return False

        # Build URL -> endpoint name reverse map since models reference
        # endpoints by URL, not by name.
        url_to_name: Dict[str, str] = {}
        for ep in endpoints:
            url = ep.get("url", "")
            name = ep.get("name", "")
            if url and name:
                url_to_name[url.rstrip("/")] = name

        # Also index backends by type for fallback matching
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

            # Primary: match by URL using the reverse map
            for ep_url in model_endpoints:
                normalised = ep_url.rstrip("/")
                ep_name = url_to_name.get(normalised)
                if ep_name and ep_name in ep_map:
                    ep_map[ep_name].models.append(model_name)
                    matched = True

            # Fallback: match by backend type if URL lookup missed
            if not matched and model_type and model_type in type_to_backends:
                for ep_name in type_to_backends[model_type]:
                    if ep_name in ep_map:
                        ep_map[ep_name].models.append(model_name)
                        matched = True

        # Select one non-embedding model per backend
        for bi in self.backends:
            candidates = [m for m in bi.models if "embed" not in m.lower()]
            if candidates:
                bi.selected_model = candidates[0]

        self._print_discovery_summary()
        return True

    def _print_discovery_summary(self):
        print()
        self.pcolor(WHITE, "Configuration Summary")
        self.pcolor(PURPLE, "-" * 72)
        header = f"  {'Backend':<20} {'Type':<18} {'Status':<10} {'Models':<6} {'Mode'}"
        self.pcolor(GREY, header)
        self.pcolor(GREY, f"  {'-'*20} {'-'*18} {'-'*10} {'-'*6} {'-'*14}")

        for bi in self.backends:
