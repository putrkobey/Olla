// ollamock is a multi-protocol mock LLM backend for end-to-end validation of
// Olla's proxy and routing logic without requiring real inference infrastructure.
//
// It speaks Ollama, LM Studio, OpenAI-compatible, Lemonade, and Anthropic
// wire formats and supports controllable failure injection via a control plane.
//
// Usage:
//
//	go run ./test/cmd/ollamock \
//	    --addr 127.0.0.1:19431 \
//	    --name mock-a \
//	    --models llama3.2,phi4 \
//	    --ttft-ms 50 \
//	    --tps 20 \
//	    --stream-chunks 5
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:19431", "listen address")
	name := flag.String("name", "mock-a", "instance marker embedded in every response")
	models := flag.String("models", "test-model", "comma-separated list of model names to serve")
	ttftMS := flag.Int("ttft-ms", 0, "delay in milliseconds before first streamed byte")
	tps := flag.Int("tps", 0, "tokens per second pacing (0 = instant)")
	streamChunks := flag.Int("stream-chunks", 5, "number of content chunks per streamed response")
	tagsCapabilities := flag.Bool("tags-capabilities", false, "emit per-model capabilities in /api/tags, as Ollama 0.30+ does; off by default so the main fleet's listing is unchanged")
	loaded := flag.String("loaded", "", "comma-separated models /api/ps reports as resident (empty = none)")
