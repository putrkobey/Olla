package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/logger"
)

// Setup Helpers
// -------------

// newTestTranslator creates a configured translator for testing with error-level logging.
func newTestTranslator() *Translator {
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	styledLog := logger.NewPlainStyledLogger(log)
	tr, err := NewTranslator(styledLog, createStreamingTestConfig())
	if err != nil {
		panic("newTestTranslator: " + err.Error())
	}
	return tr
}

// createStreamingTestConfig creates a minimal config for streaming tests.
// Kept separate from newTestTranslator to allow config customization if needed.
func createStreamingTestConfig() config.AnthropicTranslatorConfig {
	return config.AnthropicTranslatorConfig{
		Enabled:        true,
		MaxMessageSize: 10 << 20, // 10MB
	}
}

// Mock Data Helpers
// -----------------

// mockOpenAIStream creates a mock OpenAI SSE stream from string chunks.
// Each chunk should be a complete SSE line (e.g., "data: {...}\n\n").
func mockOpenAIStream(chunks []string) io.Reader {
	var buf bytes.Buffer
	for _, chunk := range chunks {
		buf.WriteString(chunk)
	}
	return &buf
}

// textChunk creates an OpenAI SSE chunk containing text content.
// First chunk should include model name, subsequent chunks can omit it.
func textChunk(messageID, model, content string) string {
	if model != "" {
		return fmt.Sprintf("data: {\"id\":\"%s\",\"model\":\"%s\",\"choices\":[{\"delta\":{\"content\":\"%s\"},\"index\":0}]}\n\n",
			messageID, model, content)
	}
	return fmt.Sprintf("data: {\"id\":\"%s\",\"choices\":[{\"delta\":{\"content\":\"%s\"},\"index\":0}]}\n\n",
		messageID, content)
}

// toolStartChunk creates an OpenAI SSE chunk that starts a new tool call.
// This includes the tool ID, name, and initial (usually empty) arguments.
func toolStartChunk(messageID string, toolIndex int, toolID, toolName string) string {
	return fmt.Sprintf("data: {\"id\":\"%s\",\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":%d,\"id\":\"%s\",\"type\":\"function\",\"function\":{\"name\":\"%s\",\"arguments\":\"\"}}]},\"index\":0}]}\n\n",
		messageID, toolIndex, toolID, toolName)
}

// toolArgsChunk creates an OpenAI SSE chunk with tool argument data.
// Arguments are typically streamed in multiple chunks and concatenated.
func toolArgsChunk(messageID string, toolIndex int, args string) string {
	return fmt.Sprintf("data: {\"id\":\"%s\",\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":%d,\"function\":{\"arguments\":\"%s\"}}]},\"index\":0}]}\n\n",
		messageID, toolIndex, args)
}

// finishChunk creates an OpenAI SSE chunk indicating stream completion.
// Common finish reasons: "stop", "tool_calls", "length".
func finishChunk(messageID, finishReason string) string {
	return fmt.Sprintf("data: {\"id\":\"%s\",\"choices\":[{\"delta\":{},\"index\":0,\"finish_reason\":\"%s\"}]}\n\n",
		messageID, finishReason)
}

// finishChunkWithUsage creates an OpenAI SSE chunk with finish reason and token usage.
func finishChunkWithUsage(messageID, finishReason string, promptTokens, completionTokens int) string {
	return fmt.Sprintf("data: {\"id\":\"%s\",\"choices\":[{\"delta\":{},\"index\":0,\"finish_reason\":\"%s\"}],\"usage\":{\"prompt_tokens\":%d,\"completion_tokens\":%d,\"total_tokens\":%d}}\n\n",
		messageID, finishReason, promptTokens, completionTokens, promptTokens+completionTokens)
}

// doneChunk returns the standard OpenAI stream termination marker.
func doneChunk() string {
	return "data: [DONE]\n\n"
}

// usageOnlyChunk creates an OpenAI SSE chunk with choices:[] and a populated usage
// object. This is the format vLLM (and other backends that respect include_usage=true)
// use to deliver final token counts as a separate terminal chunk rather than attaching
// them to the finish chunk. The message ID is included for completeness; clients
// correlate by stream position rather than by ID.
func usageOnlyChunk(messageID string, promptTokens, completionTokens int) string {
	return fmt.Sprintf(
		"data: {\"id\":\"%s\",\"choices\":[],\"usage\":{\"prompt_tokens\":%d,\"completion_tokens\":%d,\"total_tokens\":%d}}\n\n",
		messageID, promptTokens, completionTokens, promptTokens+completionTokens,
	)
}

// reasoningChunk creates an OpenAI SSE chunk carrying reasoning text using the
// "reasoning" field name (Ollama / LM Studio / Lemonade convention).
func reasoningChunk(messageID, model, reasoning string) string {
	if model != "" {
		return fmt.Sprintf("data: {\"id\":\"%s\",\"model\":\"%s\",\"choices\":[{\"delta\":{\"reasoning\":\"%s\"},\"index\":0}]}\n\n",
			messageID, model, reasoning)
	}
	return fmt.Sprintf("data: {\"id\":\"%s\",\"choices\":[{\"delta\":{\"reasoning\":\"%s\"},\"index\":0}]}\n\n",
		messageID, reasoning)
}

// reasoningContentChunk creates an OpenAI SSE chunk carrying reasoning text using the
// "reasoning_content" field name (vLLM / SGLang / DeepSeek convention).
func reasoningContentChunk(messageID, model, reasoning string) string {
	if model != "" {
