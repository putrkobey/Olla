#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Olla Streaming Response Test Script
Tests that LLM responses actually stream data incrementally
"""

import sys
import json
import time
import argparse
import requests
import os
from typing import Dict, List, Tuple, Optional, Any
from collections import defaultdict
from datetime import datetime

# Fix Windows console encoding for Unicode
if sys.platform == 'win32':
    import io
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')
    os.environ['PYTHONIOENCODING'] = 'utf-8'

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
DEFAULT_MAX_STREAM_TIME = 30  # seconds
DEFAULT_MAX_TOKENS = 100
CHUNK_REPORT_INTERVAL = 0.5  # Report streaming progress every N seconds

class StreamingTester:
    def __init__(self, base_url: str, max_stream_time: int):
        self.base_url = base_url
        self.max_stream_time = max_stream_time
        self.all_models = []
        self.provider_models = {
            'openai': [],
            'ollama': [],
            'lmstudio': [],
            'vllm': []
        }
        self.test_results = []
        self.total_tests = 0
        self.successful_tests = 0
        self.failed_tests = 0
        
    def print_color(self, color: str, message: str, end='\n'):
        print(f"{color}{message}{RESET}", end=end)
        
    def print_header(self):
        self.print_color(PURPLE, "=" * 58)
        self.print_color(PURPLE, f"  {CYAN}Olla Streaming Response Test{RESET}")
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
        
    def fetch_models(self) -> bool:
        self.print_color(YELLOW, "Fetching available models...")
        
        # Fetch provider-specific models
        for provider in ['openai', 'ollama', 'lmstudio', 'vllm']:
            try:
                response = requests.get(f"{self.base_url}/olla/models?format={provider}", timeout=10)
                if response.status_code == 200:
                    data = response.json()
                    models = data.get('data', data.get('models', []))
                    
                    for model in models:
                        model_id = model.get('id', model.get('name', ''))
                        if model_id and 'embed' not in model_id.lower():
                            self.provider_models[provider].append(model_id)
                    
                    if self.provider_models[provider]:
                        self.print_color(GREEN, f"  {provider}: {len(self.provider_models[provider])} models")
                    else:
                        self.print_color(YELLOW, f"  {provider}: No models available")
            except Exception as e:
                self.print_color(RED, f"  {provider}: Failed to fetch - {str(e)}")
                
        # Collect all unique models
        all_models_set = set()
        for models in self.provider_models.values():
            all_models_set.update(models)
        self.all_models = sorted(list(all_models_set))
        
        return len(self.all_models) > 0
        
    def parse_sse_chunk(self, line: str) -> Optional[Dict[str, Any]]:
        """Parse Server-Sent Events format chunk"""
        if not line.startswith('data: '):
            return None
            
        data_str = line[6:]  # Remove 'data: ' prefix
        if data_str == '[DONE]':
            return {'done': True}
            
        try:
            return json.loads(data_str)
        except:
            return None
            
    def test_openai_streaming(self, model: str) -> Dict[str, Any]:
        """Test OpenAI-format streaming endpoint"""
        url = f"{self.base_url}/olla/openai/v1/chat/completions"
        data = {
            "model": model,
            "messages": [
                {"role": "user", "content": "Tell me a story about a robot. Be creative and descriptive."}
            ],
            "stream": True,
            "max_tokens": DEFAULT_MAX_TOKENS
        }
        
        return self._test_streaming(url, data, 'openai', model)
        
    def test_ollama_streaming(self, model: str) -> Dict[str, Any]:
        """Test Ollama-format streaming endpoint"""
        url = f"{self.base_url}/olla/ollama/api/generate"
        data = {
            "model": model,
            "prompt": "Tell me a story about a robot. Be creative and descriptive.",
            "stream": True,
            "options": {
                "num_predict": DEFAULT_MAX_TOKENS
            }
        }
        
        return self._test_streaming(url, data, 'ollama', model)
        
    def test_lmstudio_streaming(self, model: str) -> Dict[str, Any]:
        """Test LM Studio-format streaming endpoint"""
        url = f"{self.base_url}/olla/lm-studio/v1/chat/completions"
        data = {
            "model": model,
            "messages": [
                {"role": "user", "content": "Tell me a story about a robot. Be creative and descriptive."}
            ],
            "stream": True,
            "max_tokens": DEFAULT_MAX_TOKENS
        }
        
        return self._test_streaming(url, data, 'lmstudio', model)
        
    def test_vllm_streaming(self, model: str) -> Dict[str, Any]:
        """Test vLLM-format streaming endpoint"""
        url = f"{self.base_url}/olla/vllm/v1/chat/completions"
        data = {
            "model": model,
            "messages": [
                {"role": "user", "content": "Tell me a story about a robot. Be creative and descriptive."}
            ],
            "stream": True,
            "max_tokens": DEFAULT_MAX_TOKENS
        }
        
        return self._test_streaming(url, data, 'vllm', model)
        
    def _test_streaming(self, url: str, data: Dict, provider: str, model: str) -> Dict[str, Any]:
        """Generic streaming test implementation"""
        result = {
            'provider': provider,
            'model': model,
            'success': False,
            'chunks_received': 0,
            'time_to_first_token': None,
            'total_time': 0,
            'tokens_per_second': 0,
            'content_length': 0,
            'endpoint_used': None,
            'error': None,
            'chunk_times': []
