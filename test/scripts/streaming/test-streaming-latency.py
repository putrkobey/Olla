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
                
            self.print_color(GREEN, f"Loaded {len(self.questions)} questions from {question_file}")
            return True
            
        except Exception as e:
            self.print_color(RED, f"Error loading questions: {e}")
            return False
            
    def create_default_questions(self, question_file: str):
        """Create a default questions file"""
        default_questions = [
            "# Questions for testing streaming latency",
            "# One question per line, lines starting with # are ignored",
            "",
            "Tell me a short story about a robot learning to paint.",
            "Explain how photosynthesis works in simple terms.",
            "What are the main differences between machine learning and artificial intelligence?",
            "Describe the process of making bread from scratch.",
            "Write a haiku about the ocean.",
            "Explain the concept of recursion in programming with an example.",
            "What would happen if gravity suddenly became twice as strong?",
            "Tell me about the history of the internet in three paragraphs.",
            "How do solar panels convert sunlight into electricity?",
            "Write a dialogue between two characters meeting for the first time.",
            "Explain why the sky appears blue during the day.",
            "What are the benefits and drawbacks of renewable energy?",
            "Describe how a computer processor works at a basic level.",
            "Tell me about an interesting historical event from the 1960s.",
            "How do vaccines help prevent diseases?",
            "Write a short poem about autumn leaves.",
            "Explain the water cycle and its importance to life on Earth.",
            "What makes a good leader, and can leadership skills be learned?",
            "Describe the process of evolution by natural selection.",
            "How do search engines like Google find and rank web pages?"
        ]
        
        try:
            with open(question_file, 'w', encoding='utf-8') as f:
                f.write('\n'.join(default_questions))
            self.print_color(GREEN, f"Created default questions file: {question_file}")
        except Exception as e:
            self.print_color(RED, f"Error creating questions file: {e}")
            
    def select_model(self, model_arg: Optional[str], manual_select: bool = False) -> Optional[str]:
        """Select model from CLI arg, auto-select phi4:latest, or user input"""
        if model_arg:
            if model_arg in self.available_models:
                self.print_color(GREEN, f"Using specified model: {model_arg}")
                return model_arg
            else:
                self.print_color(RED, f"Model '{model_arg}' not found in available models")
                return None
                
        # Auto-select phi models if available and not manual selection
        if not manual_select:
            preferred_models = ['phi4:latest', 'phi3.5:latest', 'phi3:latest']
            for model in preferred_models:
                if model in self.available_models:
                    self.print_color(GREEN, f"Auto-selecting {model}")
                    return model
            
            # No phi models found
            self.print_color(YELLOW, "No phi models found (phi4:latest, phi3.5:latest, or phi3:latest)")
            self.print_color(YELLOW, "Please select a model manually:")
                
        print()
        self.print_color(WHITE, "Select a model to test:")
        
        try:
            choice = input(f"{CYAN}Enter model number (1-{len(self.available_models)}): {RESET}")
            model_index = int(choice.strip()) - 1
            
            if 0 <= model_index < len(self.available_models):
                selected = self.available_models[model_index]
                self.print_color(GREEN, f"Selected model: {selected}")
                return selected
            else:
                self.print_color(RED, "Invalid selection")
                return None
                
        except (ValueError, KeyboardInterrupt):
            self.print_color(RED, "Invalid input or cancelled")
            return None
            
    def get_random_question(self) -> str:
        """Get a random question from the loaded questions"""
        return random.choice(self.questions)
        
    def test_streaming_latency(self, model: str, question: str) -> Dict[str, Any]:
        """Test streaming latency for a specific model and question"""
        result = {
            'model': model,
            'question': question[:50] + "..." if len(question) > 50 else question,
            'success': False,
            'chunks_received': 0,
            'total_chars': 0,
            'time_to_first_token': 0.0,
            'total_time': 0.0,
            'endpoint_used': '',
            'chunk_intervals': [],
            'error': None,
            'streaming_quality': 'unknown'
        }
        
        # Use OpenAI-compatible endpoint (most common)
        url = f"{self.base_url}/olla/openai/v1/chat/completions"
        payload = {
            "model": model,
            "messages": [
                {"role": "user", "content": question}
            ],
            "stream": True,
            "max_tokens": 200,
            "temperature": 0.7
        }
        
        try:
            start_time = time.time()
            last_chunk_time = start_time
            
            self.print_color(GREY, f"    Question: {question[:60]}{'...' if len(question) > 60 else ''}")
            self.print_color(YELLOW, f"    Streaming: ", end='')
