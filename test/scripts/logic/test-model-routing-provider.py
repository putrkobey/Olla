#!/usr/bin/env python3
"""
Olla Provider-Specific Model Routing Test Script
Tests model routing with different provider formats (OpenAI, Ollama, LM Studio, vLLM)
"""

import sys
import json
import time
import argparse
import requests
from typing import Dict, List, Tuple
from collections import defaultdict

# ANSI color codes
RED = '\033[0;31m'
GREEN = '\033[0;32m'
YELLOW = '\033[1;33m'
BLUE = '\033[0;34m'
PURPLE = '\033[0;35m'
CYAN = '\033[0;36m'
WHITE = '\033[1;37m'
GREY = '\033[0;37m'
RESET = '\033[0m'

# Configuration
TARGET_URL = "http://localhost:40114"
TIMEOUT = 180

class ProviderTester:
    def __init__(self, base_url: str):
        self.base_url = base_url
        self.all_models = []
        self.available_endpoints = []
        self.provider_models = {
            'openai': [],
            'ollama': [],
            'lmstudio': [],
            'vllm': []
        }
        self.endpoint_usage = defaultdict(int)
        self.endpoint_success = defaultdict(int)
        self.endpoint_failure = defaultdict(int)
        self.total_tests = 0
        self.successful_tests = 0
        self.failed_tests = 0
        
    def print_color(self, color: str, message: str):
        print(f"{color}{message}{RESET}")
        
    def print_header(self):
        self.print_color(PURPLE, "=" * 58)
        self.print_color(PURPLE, f"  {CYAN}Olla Provider-Specific Model Routing Test{RESET}")
        self.print_color(PURPLE, "=" * 58)
        print()
        
    def check_health(self) -> bool:
        self.print_color(YELLOW, "Checking Olla availability...")
        try:
            response = requests.get(f"{self.base_url}/internal/health", timeout=5)
            if response.status_code == 200:
                self.print_color(GREEN, "[OK] Olla is reachable")
                return True
        except Exception:
            pass
        self.print_color(RED, f"[FAIL] Cannot reach Olla at {self.base_url}")
        return False
        
    def fetch_endpoints(self) -> bool:
        self.print_color(YELLOW, "Fetching available endpoints...")
        try:
            response = requests.get(f"{self.base_url}/internal/status/endpoints", timeout=TIMEOUT)
            if response.status_code == 200:
                data = response.json()
                self.available_endpoints = data.get('endpoints', [])
                if self.available_endpoints:
                    self.print_color(GREEN, f"[OK] Found {len(self.available_endpoints)} endpoints")
                    self.print_color(GREY, "Available endpoints:")
                    for ep in self.available_endpoints:
                        is_healthy = ep.get('status', '').lower() == 'healthy'
                        status = "[HEALTHY]" if is_healthy else "[UNHEALTHY]"
                        success_rate = ep.get('success_rate', 'N/A')
                        model_count = ep.get('model_count', 0)
                        print(f"  - {CYAN}{ep.get('name', 'unknown')}{RESET} {GREEN if is_healthy else RED}{status}{RESET} - {model_count} models, {success_rate} success")
                    return True
        except Exception as e:
            print(f"Error fetching endpoints: {e}")
        self.print_color(RED, "[FAIL] Failed to fetch endpoints")
        return False
        
    def fetch_models(self) -> bool:
        # First fetch all models
        self.print_color(YELLOW, "Fetching all available models...")
        try:
            response = requests.get(f"{self.base_url}/olla/models", timeout=TIMEOUT)
            if response.status_code == 200:
                data = response.json()
                # Try different JSON structures
                models = data.get('data', [])
                if models:
                    self.all_models = [m.get('id', m.get('name', '')) for m in models]
                else:
                    models = data.get('models', [])
                    if models:
                        self.all_models = [m.get('name', m.get('id', '')) for m in models]
                        
                if self.all_models:
                    self.print_color(GREEN, f"[OK] Found {len(self.all_models)} total models")
                    print()
        except Exception as e:
            print(f"Error fetching all models: {e}")
            self.print_color(RED, "[FAIL] Failed to fetch models")
            return False
            
        # Now fetch provider-specific models
        self.print_color(YELLOW, "Fetching provider-specific models...")
        for provider in ['openai', 'ollama', 'lmstudio', 'vllm']:
            try:
                response = requests.get(f"{self.base_url}/olla/models?format={provider}", timeout=TIMEOUT)
                if response.status_code == 200:
                    data = response.json()
                    models = data.get('data', [])
                    if models:
                        self.provider_models[provider] = [m.get('id', m.get('name', '')) for m in models]
                    else:
                        models = data.get('models', [])
                        if models:
                            self.provider_models[provider] = [m.get('name', m.get('id', '')) for m in models]
                    
                    if self.provider_models[provider]:
                        self.print_color(GREEN, f"  {provider}: {len(self.provider_models[provider])} models")
                    else:
                        self.print_color(YELLOW, f"  {provider}: No models available")
            except Exception as e:
                print(f"Error fetching {provider} models: {e}")
                self.print_color(RED, f"  {provider}: Failed to fetch")
                
        return len(self.all_models) > 0
        
    def display_model_summary(self):
        self.print_color(WHITE, "\nModel Summary:")
        self.print_color(PURPLE, "-" * 58)
        
        self.print_color(CYAN, f"Total models available: {len(self.all_models)}")
        
        # Show models per provider
        for provider in ['openai', 'ollama', 'lmstudio', 'vllm']:
            if self.provider_models[provider]:
                print()
                self.print_color(YELLOW, f"{provider.upper()} models ({len(self.provider_models[provider])}):")
                for i, model in enumerate(self.provider_models[provider][:5]):
                    print(f"  - {model}")
                if len(self.provider_models[provider]) > 5:
                    self.print_color(GREY, f"  ... and {len(self.provider_models[provider]) - 5} more")
        
    def test_endpoint(self, desc: str, url: str, data: dict = None, method: str = "POST") -> Tuple[bool, str]:
        print(f"{desc}: ", end='', flush=True)
        self.total_tests += 1
        
        try:
            start_time = time.time()
            
            if method == "GET":
                response = requests.get(url, timeout=TIMEOUT)
            else:
                response = requests.post(url, json=data, timeout=TIMEOUT)
                
            duration = time.time() - start_time
            
            # Extract endpoint header
            endpoint_header = response.headers.get('X-Olla-Endpoint', '')
            
            if response.status_code == 200:
                self.print_color(GREEN, f"[OK] Success ({duration:.3f}s)")
                if endpoint_header:
                    print(f"  -> Routed to: {endpoint_header}")
                    self.endpoint_usage[endpoint_header] += 1
                    self.endpoint_success[endpoint_header] += 1
                self.successful_tests += 1
                return True, endpoint_header
            else:
                self.print_color(RED, f"[FAIL] Failed (HTTP {response.status_code})")
                if endpoint_header:
                    self.endpoint_usage[endpoint_header] += 1
                    self.endpoint_failure[endpoint_header] += 1
                self.failed_tests += 1
                return False, ""
                
        except requests.exceptions.Timeout:
            self.print_color(RED, "[FAIL] Failed (Timeout)")
            self.failed_tests += 1
            return False, ""
        except Exception as e:
            self.print_color(RED, f"[FAIL] Failed ({type(e).__name__})")
            self.failed_tests += 1
            return False, ""
            
    def test_models_endpoints(self, providers: List[str]):
        self.print_color(WHITE, "\nTesting Models Discovery Endpoints:")
        self.print_color(PURPLE, "-" * 58)
        
        if 'openai' in providers:
            self.test_endpoint(
                "OpenAI Models (/olla/openai/v1/models)",
                f"{self.base_url}/olla/openai/v1/models",
                method="GET"
            )
            
        if 'ollama' in providers:
            self.test_endpoint(
                "Ollama Models (/olla/ollama/api/tags)",
                f"{self.base_url}/olla/ollama/api/tags",
                method="GET"
            )
            self.test_endpoint(
                "Ollama Models OpenAI Format (/olla/ollama/v1/models)",
                f"{self.base_url}/olla/ollama/v1/models",
                method="GET"
            )
            
        if 'lmstudio' in providers:
            self.test_endpoint(
                "LM Studio Models (/olla/lm-studio/v1/models)",
                f"{self.base_url}/olla/lm-studio/v1/models",
                method="GET"
            )
            self.test_endpoint(
                "LM Studio Enhanced Models (/olla/lm-studio/api/v0/models)",
                f"{self.base_url}/olla/lm-studio/api/v0/models",
                method="GET"
            )
            
        if 'vllm' in providers:
            self.test_endpoint(
