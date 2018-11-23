#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Olla Comprehensive Integration Test Script

Validates all major API endpoints and functionality against a running Olla
instance. Covers health checks, internal monitoring, unified model endpoints,
proxy (OpenAI format), Anthropic translator, provider-specific routes, response
header validation, and error handling.

Auto-discovers available backends and models to dynamically build the test
matrix, giving confidence there are no major regressions before release.
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

# Backend types that natively support the Anthropic Messages API (passthrough)
PASSTHROUGH_TYPES = {"vllm", "vllm-mlx", "lm-studio", "ollama", "llamacpp", "lemonade"}
# Backend types that require translation (Anthropic -> OpenAI -> backend -> OpenAI -> Anthropic)
TRANSLATION_TYPES = {"openai-compatible", "litellm"}

# Anthropic SSE event types expected in well-formed streaming responses
ANTHROPIC_FULL_EVENTS = {
    "message_start", "content_block_start", "content_block_delta",
    "content_block_stop", "message_delta", "message_stop",
}
ANTHROPIC_MIN_EVENTS = {"message_start", "message_delta", "message_stop"}


class TestResult:
    """Outcome of a single test case."""
    __slots__ = ("name", "passed", "detail", "phase")

    def __init__(self, name: str, passed: bool, detail: str = "", phase: str = ""):
        self.name = name
        self.passed = passed
        self.detail = detail
        self.phase = phase


class IntegrationTester:
    def __init__(self, base_url: str, timeout: int, verbose: bool,
                 skip_streaming: bool, skip_anthropic: bool, skip_providers: bool,
                 skip_sticky: bool = False):
        self.base_url = base_url
        self.timeout = timeout
        self.verbose = verbose
        self.skip_streaming = skip_streaming
        self.skip_anthropic = skip_anthropic
        self.skip_providers = skip_providers
        self.skip_sticky = skip_sticky
        self.results: List[TestResult] = []
        self.endpoints: List[Dict] = []
        self.models: List[Dict] = []
        self.selected_model: Optional[str] = None
        self.passthrough_model: Optional[str] = None
        self.passthrough_backend: Optional[str] = None
        self.translation_model: Optional[str] = None
        self.translation_backend: Optional[str] = None
        self.discovered_providers: List[str] = []
        self.endpoint_types: Dict[str, str] = {}  # endpoint name -> backend type

    # -- Helpers --------------------------------------------------------------

    def pcolor(self, color: str, msg: str, end: str = '\n'):
        print(f"{color}{msg}{RESET}", end=end)
        sys.stdout.flush()

    def print_header(self):
        self.pcolor(PURPLE, "=" * 72)
        self.pcolor(PURPLE, f"  {CYAN}Olla Comprehensive Integration Test{RESET}")
        self.pcolor(PURPLE, f"  {GREY}Validates all major API endpoints and functionality{RESET}")
        self.pcolor(PURPLE, "=" * 72)
        print()

    def record(self, name: str, passed: bool, detail: str = "", phase: str = "") -> bool:
        self.results.append(TestResult(name, passed, detail, phase))
        return passed

    def _print_result(self, label: str, ok: bool, detail: str = ""):
        status = f"{GREEN}[PASS]{RESET}" if ok else f"{RED}[FAIL]{RESET}"
        extra = f"  {GREY}{detail}{RESET}" if detail else ""
        print(f"  {status} {label}{extra}")

    def _get(self, path: str, **kwargs) -> Optional[requests.Response]:
        """GET helper with error handling."""
        try:
            return requests.get(f"{self.base_url}{path}", timeout=self.timeout, **kwargs)
        except Exception as e:
            self._last_error = e
            if self.verbose:
                self.pcolor(GREY, f"    [{type(e).__name__}] {e}")
            return None

    def _post(self, path: str, json_body: Dict, **kwargs) -> Optional[requests.Response]:
        """POST helper with error handling."""
        try:
            return requests.post(
                f"{self.base_url}{path}",
                json=json_body,
                timeout=self.timeout,
                **kwargs,
            )
        except Exception as e:
            self._last_error = e
            if self.verbose:
                self.pcolor(GREY, f"    [{type(e).__name__}] {e}")
            return None

    def _error_detail(self, r: Optional[requests.Response]) -> str:
        """Build a descriptive error string from a failed response or last exception."""
        if r is not None:
            return f"HTTP {r.status_code}"
        err = getattr(self, "_last_error", None)
        if err:
            return f"{type(err).__name__}: {err}"
        return "connection error"

    def _anthropic_headers(self) -> Dict[str, str]:
        return {
            "Content-Type": "application/json",
            "x-api-key": "test-key",
            "anthropic-version": "2023-06-01",
        }

    def _anthropic_body(self, model: str, stream: bool = False,
                        max_tokens: int = 10) -> Dict[str, Any]:
        body: Dict[str, Any] = {
            "model": model,
            "messages": [{"role": "user", "content": "Say hello briefly"}],
            "max_tokens": max_tokens,
        }
        if stream:
            body["stream"] = True
        return body

    def _phase_header(self, num: int, title: str):
        print()
        self.pcolor(WHITE, f"Phase {num}: {title}")
        self.pcolor(PURPLE, "-" * 72)

    # -- Phase 1: Health Check ------------------------------------------------

    def check_health(self) -> bool:
        self.pcolor(YELLOW, "Phase 1: Health Check")
        self.pcolor(PURPLE, "-" * 72)
        try:
            r = requests.get(f"{self.base_url}/internal/health", timeout=5)
            if r.status_code == 200:
                self.pcolor(GREEN, "  [OK] Olla is reachable")
                return True
        except Exception:
            pass
        self.pcolor(RED, f"  [FAIL] Cannot reach Olla at {self.base_url}")
        return False

    # -- Discovery (not a test phase, used to inform later phases) ------------

    def discover(self) -> bool:
        """Discover endpoints and models for use in later phases."""
        self.pcolor(YELLOW, "\nDiscovering endpoints and models...")

        # Endpoints
        r = self._get("/internal/status/endpoints")
        if r and r.status_code == 200:
            self.endpoints = r.json().get("endpoints", [])
            self.pcolor(GREEN, f"  [OK] Found {len(self.endpoints)} endpoint(s)")
        else:
            self.pcolor(RED, "  [FAIL] Could not discover endpoints")
            return False

        # Build provider list and endpoint type map
        provider_types = set()
        for ep in self.endpoints:
            ep_type = ep.get("type", "")
            ep_name = ep.get("name", "")
            if ep_type:
                provider_types.add(ep_type)
            if ep_name and ep_type:
                self.endpoint_types[ep_name] = ep_type
        self.discovered_providers = sorted(provider_types)

        # Unified models (includes availability with endpoint names)
        r = self._get("/olla/models")
        if r and r.status_code == 200:
            data = r.json()
            self.models = data.get("data", data.get("models", []))
            self.pcolor(GREEN, f"  [OK] Found {len(self.models)} model(s)")
        else:
            self.pcolor(YELLOW, "  [WARN] Could not discover models")

        # Build candidate lists for each mode using olla.availability from unified models
        import random
        all_candidates = []
        passthrough_candidates: List[Tuple[str, str]] = []  # (model, endpoint_name)
        translation_candidates: List[Tuple[str, str]] = []

        # Build healthy endpoint set for quick lookup
        healthy_endpoints = {ep.get("name") for ep in self.endpoints if ep.get("status") == "healthy"}

        for m in self.models:
            mid = m.get("id", m.get("name", ""))
            if not mid or "embed" in mid.lower():
                continue
            all_candidates.append(mid)

            # Use olla.availability to find the endpoint name for this model
            olla_meta = m.get("olla", {})
            availability = olla_meta.get("availability", [])
            for avail in availability:
                ep_name = avail.get("endpoint", "")
                if not ep_name or ep_name not in healthy_endpoints:
                    continue
                ep_type = self.endpoint_types.get(ep_name, "")

                if ep_type in PASSTHROUGH_TYPES:
                    passthrough_candidates.append((mid, ep_name))
                elif ep_type in TRANSLATION_TYPES:
                    translation_candidates.append((mid, ep_name))
                break  # Use the first healthy endpoint for this model

        # Select models: random general, plus specific for passthrough/translation
        if all_candidates:
            self.selected_model = random.choice(all_candidates)
        if passthrough_candidates:
            pt = random.choice(passthrough_candidates)
            self.passthrough_model, self.passthrough_backend = pt
        if translation_candidates:
            tr = random.choice(translation_candidates)
            self.translation_model, self.translation_backend = tr

        if self.selected_model:
            self.pcolor(GREY, f"  Selected model for general tests: {self.selected_model}")
        else:
            self.pcolor(YELLOW, "  [WARN] No suitable model found for inference tests")
        if self.passthrough_model:
            pt_type = self.endpoint_types.get(self.passthrough_backend, "?")
            self.pcolor(CYAN, f"  Passthrough model: {self.passthrough_model} "
                               f"({self.passthrough_backend}, {pt_type})")
        if self.translation_model:
            tr_type = self.endpoint_types.get(self.translation_backend, "?")
            self.pcolor(YELLOW, f"  Translation model: {self.translation_model} "
                                 f"({self.translation_backend}, {tr_type})")

        # Print discovery summary
        print()
        self.pcolor(WHITE, "  Discovered Backends")
        self.pcolor(GREY, f"  {'Name':<20} {'Type':<18} {'Status':<10} {'Models'}")
        self.pcolor(GREY, f"  {'-'*20} {'-'*18} {'-'*10} {'-'*6}")
        for ep in self.endpoints:
            name = ep.get("name", "?")
            etype = ep.get("type", "?")
            status = ep.get("status", "?")
            mcount = ep.get("model_count", 0)
            sc = GREEN if status == "healthy" else RED
            print(f"  {name:<20} {etype:<18} {sc}{status:<10}{RESET} {mcount}")

        return True

    # -- Phase 2: Internal/Monitoring Endpoints --------------------------------

    def phase_internal_endpoints(self):
        self._phase_header(2, "Internal/Monitoring Endpoints")
        phase = "internal"

        tests = [
            ("/internal/health", "Health endpoint", ["status"]),
            ("/internal/status", "Status endpoint", []),
            ("/internal/status/endpoints", "Endpoints status", ["endpoints"]),
            ("/internal/status/models", "Models status", []),
            ("/internal/stats/models", "Model statistics", []),
            ("/internal/stats/translators", "Translator statistics", []),
            ("/internal/process", "Process statistics", []),
            ("/version", "Version info", []),
        ]

        for path, label, required_keys in tests:
            r = self._get(path)
            ok = False
            detail = ""

            if r is None:
                detail = self._error_detail(r)
            elif r.status_code != 200:
                detail = f"HTTP {r.status_code}"
            else:
                try:
                    data = r.json()
                    # Check required keys if specified
                    missing = [k for k in required_keys if k not in data]
                    if missing:
                        detail = f"missing keys: {missing}"
                    else:
                        ok = True
                        if self.verbose:
                            detail = json.dumps(data, indent=2)[:200]
                except Exception:
                    detail = "invalid JSON"

            self._print_result(f"{label} ({path})", ok, detail)
            self.record(f"{phase}/{label}", ok, detail, phase)

    # -- Phase 3: Unified Model Endpoints -------------------------------------

    def phase_unified_models(self):
        self._phase_header(3, "Unified Model Endpoints")
        phase = "models"

        # GET /olla/models
        r = self._get("/olla/models")
        ok = False
        detail = ""
        if r and r.status_code == 200:
            data = r.json()
            model_list = data.get("data", data.get("models", []))
            if isinstance(model_list, list):
                ok = True
                detail = f"{len(model_list)} models"
            else:
                detail = "unexpected response structure"
        else:
            detail = self._error_detail(r)

        self._print_result("Unified model list (/olla/models)", ok, detail)
        self.record(f"{phase}/list", ok, detail, phase)

        # GET /olla/models/{id} using discovered model
        if self.selected_model:
            r = self._get(f"/olla/models/{self.selected_model}")
            ok = False
            detail = ""
            if r and r.status_code == 200:
                ok = True
                detail = f"found {self.selected_model}"
            else:
                detail = self._error_detail(r)

            self._print_result(f"Model lookup (/olla/models/{{id}})", ok, detail)
            self.record(f"{phase}/lookup", ok, detail, phase)
        else:
            self.pcolor(GREY, "  [SKIP] No model available for lookup test")

    # -- Phase 4: Proxy Endpoints (OpenAI format) -----------------------------

    def phase_proxy_endpoints(self):
        self._phase_header(4, "Proxy Endpoints (OpenAI format)")
        phase = "proxy"

        # GET /olla/proxy/v1/models
        r = self._get("/olla/proxy/v1/models")
        ok = False
        detail = ""
        if r and r.status_code == 200:
            try:
                data = r.json()
                ok = True
                model_list = data.get("data", [])
                detail = f"{len(model_list)} models"
            except Exception:
                detail = "invalid JSON"
        else:
            detail = self._error_detail(r)

        self._print_result("OpenAI model list (/olla/proxy/v1/models)", ok, detail)
        self.record(f"{phase}/models", ok, detail, phase)

        if not self.selected_model:
            self.pcolor(GREY, "  [SKIP] No model for inference tests")
            return

        # POST non-streaming chat completion via proxy
        body = {
            "model": self.selected_model,
            "messages": [{"role": "user", "content": "Say hello briefly"}],
            "max_tokens": 10,
            "stream": False,
        }
        r = self._post("/olla/proxy/v1/chat/completions", body)
        ok = False
        detail = ""
        if r and r.status_code == 200:
            try:
                data = r.json()
                ok = True
                # Show token usage if present
                usage = data.get("usage", {})
                if usage:
                    detail = (f"prompt={usage.get('prompt_tokens', '?')}, "
                              f"completion={usage.get('completion_tokens', '?')}, "
                              f"total={usage.get('total_tokens', '?')}")
                else:
                    detail = "no usage block in response"
            except Exception:
                detail = "invalid JSON response"
        else:
            detail = self._error_detail(r)

        self._print_result("Non-streaming chat completion", ok, detail)
        self.record(f"{phase}/chat-nonstream", ok, detail, phase)

        # Validate token usage is present in response
        if r and r.status_code == 200:
            try:
                data = r.json()
                usage = data.get("usage", {})
                has_prompt = isinstance(usage.get("prompt_tokens"), int) and usage["prompt_tokens"] > 0
                has_completion = isinstance(usage.get("completion_tokens"), int) and usage["completion_tokens"] >= 0
                has_total = isinstance(usage.get("total_tokens"), int) and usage["total_tokens"] > 0
                ok = has_prompt and has_completion and has_total
                if ok:
                    detail = (f"prompt_tokens={usage['prompt_tokens']}, "
                              f"completion_tokens={usage['completion_tokens']}, "
                              f"total_tokens={usage['total_tokens']}")
                else:
                    missing = []
                    if not has_prompt:
                        missing.append("prompt_tokens")
                    if not has_completion:
                        missing.append("completion_tokens")
                    if not has_total:
                        missing.append("total_tokens")
                    detail = f"missing or invalid: {missing}, usage={usage}"
            except Exception:
                ok = False
                detail = "could not parse usage from response"

            self._print_result("Token usage in non-streaming response", ok, detail)
            self.record(f"{phase}/token-usage-nonstream", ok, detail, phase)

        # POST streaming chat completion via proxy
        if not self.skip_streaming:
            body_stream = {
                "model": self.selected_model,
                "messages": [{"role": "user", "content": "Say hello briefly"}],
                "max_tokens": 10,
                "stream": True,
                "stream_options": {"include_usage": True},
            }
            r = self._post("/olla/proxy/v1/chat/completions", body_stream, stream=True)
            ok = False
            detail = ""
            stream_usage = {}
            if r and r.status_code == 200:
                has_data = False
                has_done = False
                for line in r.iter_lines(decode_unicode=True):
                    if not line:
                        continue
                    if line.startswith("data: "):
                        payload = line[6:].strip()
                        has_data = True
                        if payload == "[DONE]":
                            has_done = True
                        else:
                            try:
                                chunk = json.loads(payload)
                                chunk_usage = chunk.get("usage")
                                if chunk_usage:
                                    stream_usage = chunk_usage
                            except (json.JSONDecodeError, ValueError):
                                pass
                ok = has_data
                if not has_data:
                    detail = "no data: lines in SSE stream"
                elif not has_done:
                    detail = "stream completed (no [DONE] marker)"
            else:
                detail = self._error_detail(r)

            self._print_result("Streaming chat completion (SSE)", ok, detail)
            self.record(f"{phase}/chat-stream", ok, detail, phase)

            # Validate token usage in streaming response
            if r and r.status_code == 200:
                if stream_usage:
                    has_prompt = isinstance(stream_usage.get("prompt_tokens"), int) and stream_usage["prompt_tokens"] > 0
                    has_completion = isinstance(stream_usage.get("completion_tokens"), int) and stream_usage["completion_tokens"] >= 0
                    has_total = isinstance(stream_usage.get("total_tokens"), int) and stream_usage["total_tokens"] > 0
                    ok = has_prompt and has_completion and has_total
                    if ok:
                        detail = (f"prompt_tokens={stream_usage['prompt_tokens']}, "
                                  f"completion_tokens={stream_usage['completion_tokens']}, "
                                  f"total_tokens={stream_usage['total_tokens']}")
                    else:
                        missing = []
                        if not has_prompt:
                            missing.append("prompt_tokens")
                        if not has_completion:
                            missing.append("completion_tokens")
                        if not has_total:
                            missing.append("total_tokens")
                        detail = f"missing or invalid: {missing}, usage={stream_usage}"
                else:
                    ok = False
                    detail = "no usage block in streaming response (stream_options.include_usage may not be supported)"

                self._print_result("Token usage in streaming response", ok, detail)
                self.record(f"{phase}/token-usage-stream", ok, detail, phase)

        # Response header validation on a proxy request
        r = self._post("/olla/proxy/v1/chat/completions", {
            "model": self.selected_model,
            "messages": [{"role": "user", "content": "Hi"}],
            "max_tokens": 5,
            "stream": False,
        })
        ok = False
        detail = ""
        if r and r.status_code == 200:
            expected_headers = ["X-Olla-Endpoint", "X-Olla-Model",
                                "X-Olla-Backend-Type", "X-Olla-Request-ID"]
            present = [h for h in expected_headers if r.headers.get(h)]
