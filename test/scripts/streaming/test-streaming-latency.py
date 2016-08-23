#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Olla Streaming Latency Test Script
Tests streaming latency and choppiness by simulating OpenWebUI behavior
"""

import sys
import json
import time
import argparse
import requests
import random
import os
from typing import Dict, List, Tuple, Optional, Any
from collections import deque
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
DEFAULT_QUESTION_FILE = "questions.txt"
DEFAULT_TIMEOUT = 60

class StreamingLatencyTester:
    def __init__(self, base_url: str, timeout: int):
        self.base_url = base_url
        self.timeout = timeout
        self.available_models = []
        self.questions = []
        
    def print_color(self, color: str, message: str, end='\n'):
        print(f"{color}{message}{RESET}", end=end)
        
    def print_header(self):
        self.print_color(PURPLE, "=" * 60)
        self.print_color(PURPLE, f"  {CYAN}Olla Streaming Latency Test{RESET}")
        self.print_color(PURPLE, f"  {GREY}Simulating OpenWebUI streaming behavior{RESET}")
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
        
    def fetch_models(self) -> List[str]:
        """Fetch available models from Olla"""
        self.print_color(YELLOW, "Fetching available models...")
        
        try:
            response = requests.get(f"{self.base_url}/olla/models", timeout=10)
            if response.status_code != 200:
                self.print_color(RED, f"Failed to fetch models: HTTP {response.status_code}")
                return []
                
            data = response.json()
            models = data.get('data', data.get('models', []))
            
            model_names = []
            for model in models:
                model_id = model.get('id', model.get('name', ''))
                # Filter out embedding models
                if model_id and 'embed' not in model_id.lower():
                    model_names.append(model_id)
                    
            self.available_models = sorted(model_names)
            self.print_color(GREEN, f"Found {len(self.available_models)} available models")
            
            # Display models with numbers for selection
            for i, model in enumerate(self.available_models, 1):
                self.print_color(GREY, f"  {i:2d}. {model}")
                
            return self.available_models
            
        except Exception as e:
            self.print_color(RED, f"Error fetching models: {e}")
            return []
            
    def load_questions(self, question_file: str) -> bool:
        """Load questions from file"""
        if not os.path.exists(question_file):
            self.print_color(YELLOW, f"Questions file '{question_file}' not found. Creating default questions...")
            self.create_default_questions(question_file)
            
        try:
            with open(question_file, 'r', encoding='utf-8') as f:
                lines = f.readlines()
                self.questions = [line.strip() for line in lines if line.strip() and not line.strip().startswith('#')]
                
            if not self.questions:
                self.print_color(RED, f"No valid questions found in {question_file}")
                return False
                
