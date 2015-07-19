#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Olla Streaming Detection Test Script
Detects whether Olla is streaming or buffering responses by analyzing chunk timing patterns
"""

import sys
import json
import time
import argparse
import requests
import os
from typing import Dict, List, Tuple, Optional, Any
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
BOLD = '\033[1m'

# Configuration
TARGET_URL = "http://localhost:40114"
DEFAULT_TIMEOUT = 30

class StreamingDetector:
    def __init__(self, base_url: str):
        self.base_url = base_url
        self.chunk_times: List[Tuple[float, str]] = []
        
    def print_color(self, color: str, message: str, end='\n'):
        print(f"{color}{message}{RESET}", end=end)
        
    def print_header(self):
        self.print_color(PURPLE, "=" * 60)
        self.print_color(PURPLE, f"  {CYAN}Olla Streaming Detection Test{RESET}")
        self.print_color(PURPLE, f"  {GREY}Analyzing proxy streaming vs buffering behavior{RESET}")
        self.print_color(PURPLE, "=" * 60)
        print()
        
    def check_health(self) -> bool:
        """Check if Olla is available"""
        self.print_color(YELLOW, "Checking Olla availability...")
        try:
            response = requests.get(f"{self.base_url}/internal/health", timeout=5)
            if response.status_code == 200:
                self.print_color(GREEN, "[OK] Olla is reachable")
                return True
        except Exception as e:
            pass
        self.print_color(RED, f"[FAIL] Cannot reach Olla at {self.base_url}")
        return False
        
    def get_available_models(self) -> List[str]:
        """Fetch available models from Olla"""
        try:
            response = requests.get(f"{self.base_url}/olla/models", timeout=10)
            if response.status_code != 200:
                return []
                
            data = response.json()
            models = data.get('data', data.get('models', []))
            
            model_names = []
            for model in models:
                model_id = model.get('id', model.get('name', ''))
                if model_id:
                    model_names.append(model_id)
                    
            return sorted(model_names)
            
        except Exception as e:
            self.print_color(RED, f"Error fetching models: {e}")
            return []
            
    def select_model(self, preferred_model: Optional[str] = None, manual_select: bool = False) -> Optional[str]:
        """Select a model from CLI arg, auto-select phi models, or user input"""
        models = self.get_available_models()
        if not models:
            self.print_color(RED, "No models available")
            return None
            
        # Filter out embedding models
        text_models = [m for m in models if 'embed' not in m.lower()]
        if not text_models:
            text_models = models  # Fallback to all models if no text models
            
        # Check if preferred model exists
        if preferred_model:
            if preferred_model in text_models:
                self.print_color(GREEN, f"Using specified model: {preferred_model}")
                return preferred_model
            else:
                self.print_color(RED, f"Model '{preferred_model}' not found in available models")
                return None
        
        # Auto-select phi models if available and not manual selection
        if not manual_select:
            preferred_models = ['phi4:latest', 'phi3.5:latest', 'phi3:latest']
            for model in preferred_models:
                if model in text_models:
                    self.print_color(GREEN, f"Auto-selected model: {model}")
                    return model
            
            # No phi models found
            self.print_color(YELLOW, "No phi models found (phi4:latest, phi3.5:latest, or phi3:latest)")
            self.print_color(YELLOW, "Please select a model manually:")
            
        # Prompt user
        self.print_color(WHITE, "\nAvailable models:")
        for i, model in enumerate(text_models[:10], 1):  # Show max 10 models
            self.print_color(GREY, f"  {i:2d}. {model}")
            
        if len(text_models) > 10:
            self.print_color(GREY, f"  ... and {len(text_models) - 10} more")
            
        try:
            choice = input(f"{CYAN}Select model number (1-{min(10, len(text_models))}): {RESET}")
            model_index = int(choice.strip()) - 1
            
            if 0 <= model_index < len(text_models):
                selected = text_models[model_index]
                self.print_color(GREEN, f"Selected model: {selected}")
                return selected
            else:
                self.print_color(RED, "Invalid selection")
                return None
                
        except (ValueError, KeyboardInterrupt):
            self.print_color(RED, "Invalid input or cancelled")
            return None
        
    def test_streaming(self, model: str = "phi4:latest", prompt: str = "Count from 1 to 10 slowly", endpoint_type: str = "openai") -> Dict[str, Any]:
        """Test if the endpoint is streaming or buffering."""
        # Different endpoint formats
        if endpoint_type == "openai":
            url = f"{self.base_url}/olla/openai/v1/chat/completions"
            payload = {
                "model": model,
                "messages": [{"role": "user", "content": prompt}],
                "stream": True
            }
        else:  # ollama
            url = f"{self.base_url}/olla/ollama/api/generate"
            payload = {
                "model": model,
                "prompt": prompt,
                "stream": True
            }
        
        headers = {
            "Content-Type": "application/json"
        }
        
        self.print_color(WHITE, f"\nTesting {endpoint_type.upper()} streaming behavior:")
        self.print_color(GREY, f"  URL: {url}")
        self.print_color(GREY, f"  Model: {model}")
        self.print_color(GREY, f"  Prompt: {prompt[:50]}{'...' if len(prompt) > 50 else ''}")
        print()
        
        self.chunk_times = []
        start_time = time.time()
        first_chunk_time = None
        chunk_count = 0
        total_tokens = []
        endpoint_used = ''
        
        # Visual streaming indicator
        self.print_color(YELLOW, "  Streaming: ", end='')
        sys.stdout.flush()
        
        try:
            response = requests.post(url, json=payload, headers=headers, stream=True, timeout=DEFAULT_TIMEOUT)
            response.raise_for_status()
            
            # Extract response headers
            endpoint_used = response.headers.get('X-Olla-Endpoint', 'unknown')
            
            for line in response.iter_lines(decode_unicode=True, chunk_size=1):
                if line:
                    current_time = time.time()
                    elapsed = current_time - start_time
                    
                    if endpoint_type == "openai" and line.startswith('data: '):
                        chunk_count += 1
                        data_str = line[6:]
                        
                        if data_str == '[DONE]':
                            break
                            
                        try:
                            data = json.loads(data_str)
                            content = data.get('choices', [{}])[0].get('delta', {}).get('content', '')
                            
                            if content:
                                total_tokens.append(content)
                                self.chunk_times.append((elapsed, content))
                                
                                if first_chunk_time is None:
                                    first_chunk_time = elapsed
                                    self.print_color(GREEN, "●", end='')
                                else:
                                    self.print_color(CYAN, "●", end='')
                                sys.stdout.flush()
                                
                        except json.JSONDecodeError:
                            pass
                    
                    elif endpoint_type == "ollama":
                        # Ollama format - newline-delimited JSON
                        try:
                            data = json.loads(line)
                            response_text = data.get('response', '')
                            
                            if response_text:
                                chunk_count += 1
                                total_tokens.append(response_text)
                                self.chunk_times.append((elapsed, response_text))
                                
                                if first_chunk_time is None:
                                    first_chunk_time = elapsed
                                    self.print_color(GREEN, "●", end='')
                                else:
                                    self.print_color(CYAN, "●", end='')
                                sys.stdout.flush()
                                
                            if data.get('done', False):
                                break
                                
                        except json.JSONDecodeError:
                            pass
            
            self.print_color(GREEN, " [OK]")
            self.print_color(GREY, f"  Endpoint: {endpoint_used}")
            return self._analyze_results(start_time, first_chunk_time, chunk_count, endpoint_type)
            
        except requests.exceptions.RequestException as e:
            self.print_color(RED, " [FAIL]")
            self.print_color(RED, f"  Error: {e}")
            return {"error": str(e)}
    
    def _analyze_results(self, start_time: float, first_chunk_time: Optional[float], chunk_count: int, endpoint_type: str) -> Dict[str, Any]:
        """Analyze the chunk timing to determine streaming behavior."""
        if not self.chunk_times:
            return {"error": "No chunks received", "chunk_count": 0}
        
        total_time = time.time() - start_time
        
        # Calculate inter-chunk delays
        inter_chunk_delays = []
        for i in range(1, len(self.chunk_times)):
            delay = self.chunk_times[i][0] - self.chunk_times[i-1][0]
            inter_chunk_delays.append(delay)
        
        # Analyze the pattern
        avg_delay = sum(inter_chunk_delays) / len(inter_chunk_delays) if inter_chunk_delays else 0
        max_delay = max(inter_chunk_delays) if inter_chunk_delays else 0
        min_delay = min(inter_chunk_delays) if inter_chunk_delays else 0
        
        # Determine if it's streaming or buffering
        is_streaming = self._determine_streaming_mode(
            first_chunk_time or 0, avg_delay, max_delay, len(self.chunk_times)
        )
        
        # Print analysis
        self.print_color(GREY, f"  Total time: {total_time:.3f}s")
        self.print_color(GREY, f"  First chunk: {first_chunk_time:.3f}s" if first_chunk_time else "  First chunk: N/A")
        self.print_color(GREY, f"  Total chunks: {len(self.chunk_times)}")
        if inter_chunk_delays:
            self.print_color(GREY, f"  Avg inter-chunk delay: {avg_delay*1000:.1f}ms")
            self.print_color(GREY, f"  Min/Max delays: {min_delay*1000:.1f}ms / {max_delay*1000:.1f}ms")
        
        # Mode determination with color
        mode_color = GREEN if is_streaming else RED
        mode_text = "STREAMING" if is_streaming else "STANDARD"
        self.print_color(mode_color, f"  Mode: {mode_text}")
        
        return {
            "mode": "streaming" if is_streaming else "standard",
            "total_time": total_time,
            "first_chunk_time": first_chunk_time,
            "chunk_count": len(self.chunk_times),
            "avg_inter_chunk_delay": avg_delay,
            "max_inter_chunk_delay": max_delay,
            "min_inter_chunk_delay": min_delay,
            "is_streaming": is_streaming,
            "endpoint_type": endpoint_type
        }
    
    def _determine_streaming_mode(self, first_chunk_time: float, avg_delay: float, 
                                  max_delay: float, chunk_count: int) -> bool:
        """Determine if the response pattern indicates streaming or buffering."""
        # Criteria for streaming:
        # 1. Multiple chunks received (not just one big response)
        # 2. Regular chunks with reasonable delays between them
        # 3. No extremely large gaps that would indicate buffering
        
        # If we only get 1-2 chunks, it's likely standard mode (no streaming)
        if chunk_count < 3:
            return False
        
        # For many chunks with consistent timing, it's streaming
        # Even if first chunk takes time (model loading), subsequent chunks should be regular
        if chunk_count > 10:
            # Many chunks with reasonable average delay = streaming
            if avg_delay < 0.2:  # 200ms average is fine for streaming
                return True
            # If we have many chunks but long delays, might be choppy streaming
            if avg_delay < 0.5:  # Up to 500ms average could still be streaming
                return True
        
        # Check first chunk time - be more lenient as models need to load
        if first_chunk_time > 5.0:  # 5 seconds is definitely too long
            return False
        
        # If we see very large gaps between chunks, it's likely buffering
        if max_delay > 1.0:  # 1 second gap suggests buffering
            return False
        
        # Default: if we have multiple chunks, assume streaming
        return chunk_count >= 3
    
    def test_stream_false(self, model: str) -> Dict[str, Any]:
        """Test with stream:false to verify buffering behavior."""
        self.print_color(WHITE, "\nTest 4: Text with stream:false (should buffer)")
        
        url = f"{self.base_url}/olla/openai/v1/chat/completions"
        payload = {
            "model": model,
            "messages": [{"role": "user", "content": "Count from 1 to 5"}],
            "stream": False  # Explicitly request non-streaming
        }
        
        headers = {
            "Content-Type": "application/json"
        }
        
        self.print_color(YELLOW, "  Testing non-streaming request: ", end='')
        sys.stdout.flush()
        
        start_time = time.time()
        
        try:
            response = requests.post(url, json=payload, headers=headers, timeout=DEFAULT_TIMEOUT)
            response_time = time.time() - start_time
            
            if response.status_code == 200:
                self.print_color(GREEN, "[OK]")
                endpoint_used = response.headers.get('X-Olla-Endpoint', 'unknown')
                self.print_color(GREY, f"  Endpoint: {endpoint_used}")
                self.print_color(GREY, f"  Response time: {response_time:.3f}s")
                self.print_color(GREEN, f"  Mode: STANDARD (as expected with stream:false)")
                
                return {
                    "mode": "standard",
                    "total_time": response_time,
                    "first_chunk_time": response_time,  # All data arrives at once
                    "chunk_count": 1,
                    "is_standard": True,
                    "endpoint_type": "text_no_stream"
                }
            else:
                self.print_color(RED, " [FAIL]")
                self.print_color(RED, f"  HTTP {response.status_code}")
                return {"error": f"HTTP {response.status_code}"}
                
        except requests.exceptions.RequestException as e:
            self.print_color(RED, " [FAIL]")
            self.print_color(RED, f"  Error: {e}")
            return {"error": str(e)}
    
    def test_image_generation(self, model: str) -> Dict[str, Any]:
        """Test image generation to verify buffering behavior."""
        self.print_color(WHITE, "\nTest 5: Image generation (should buffer)")
        
        # First, try to find an image generation model
        models = self.get_available_models()
        image_models = [m for m in models if any(keyword in m.lower() for keyword in ['image', 'dall', 'stable', 'diffusion', 'sdxl'])]
        
        if not image_models:
            # Try a generic image endpoint
            url = f"{self.base_url}/olla/openai/v1/images/generations"
            payload = {
                "prompt": "A simple test image",
                "n": 1,
                "size": "256x256"
            }
        else:
            # Use the first image model found
            image_model = image_models[0]
            self.print_color(GREY, f"  Using image model: {image_model}")
            url = f"{self.base_url}/olla/openai/v1/chat/completions"
            payload = {
                "model": image_model,
                "messages": [{"role": "user", "content": "Generate a simple test image"}],
                "stream": True  # Test if it properly buffers even with stream:true
            }
        
        headers = {
            "Content-Type": "application/json"
        }
        
        self.print_color(YELLOW, "  Testing image endpoint: ", end='')
        sys.stdout.flush()
        
        start_time = time.time()
        chunk_count = 0
        first_chunk_time = None
        
        try:
            response = requests.post(url, json=payload, headers=headers, stream=True, timeout=DEFAULT_TIMEOUT)
            
            if response.status_code != 200:
                self.print_color(YELLOW, " [SKIP]")
                self.print_color(GREY, f"  Image generation not available (HTTP {response.status_code})")
                return {"skipped": True, "reason": "Image generation not available"}
            
            # Check content type
            content_type = response.headers.get('Content-Type', '')
            endpoint_used = response.headers.get('X-Olla-Endpoint', 'unknown')
            
            # Read response
            for chunk in response.iter_content(chunk_size=8192):
                if chunk:
                    current_time = time.time()
                    elapsed = current_time - start_time
                    
                    if first_chunk_time is None:
                        first_chunk_time = elapsed
                        self.print_color(GREEN, "●", end='')
                    else:
                        self.print_color(CYAN, "●", end='')
                    sys.stdout.flush()
                    
                    chunk_count += 1
            
            self.print_color(GREEN, " [OK]")
            self.print_color(GREY, f"  Endpoint: {endpoint_used}")
            
            # Analyze timing
            total_time = time.time() - start_time
            
            # For images, we expect standard mode (no streaming)
            is_standard = chunk_count == 1 or (chunk_count > 1 and first_chunk_time > 0.5)
            
            self.print_color(GREY, f"  Total time: {total_time:.3f}s")
