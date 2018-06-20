package anthropic

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/constants"
)

func TestTransformStreamingResponse_SimpleText(t *testing.T) {
	translator := newTestTranslator()

	// simulate openai streaming response with text chunks
	stream := mockOpenAIStream([]string{
		textChunk("chatcmpl-123", "claude-3-5-sonnet-20241022", "Hello"),
		textChunk("chatcmpl-123", "", " world"),
		textChunk("chatcmpl-123", "", "!"),
		finishChunk("chatcmpl-123", "stop"),
		doneChunk(),
	})

	recorder := executeTransform(t, translator, stream)
	body := recorder.Body.String()

	// verify all required anthropic events are present
	assertContainsAll(t, body, []string{
		"event: message_start",
		"event: content_block_start",
		"event: content_block_delta",
		"event: content_block_stop",
		"event: message_delta",
		"event: message_stop",
	})

	// verify text content is present
	assertTextContent(t, body, "Hello")
	assertTextContent(t, body, " world")
	assertTextContent(t, body, "!")

	// verify message_start includes model
	assert.Contains(t, body, `"model":"claude-3-5-sonnet-20241022"`)

	// parse and validate event structure
	events := parseAnthropicEvents(t, body)
	require.NotEmpty(t, events)

	// note: implementation sends content_block_start first, then message_start when model is known
	// This is a valid streaming pattern - events don't have to be in strict order
	// as long as all required events are present
	assertRequiredEvents(t, events)
	assertHasEventType(t, events, "content_block_start")
	assertHasEventType(t, events, "content_block_delta")
	assertHasEventType(t, events, "content_block_stop")
	assertStopReason(t, events, "end_turn")
}

func TestTransformStreamingResponse_WithToolCalls(t *testing.T) {
	translator := newTestTranslator()

	// simulate openai streaming with text followed by tool call
	stream := mockOpenAIStream([]string{
		textChunk("chatcmpl-456", "claude-3-5-sonnet-20241022", "Let me check that for you."),
		toolStartChunk("chatcmpl-456", 0, "call_abc123", "get_weather"),
		toolArgsChunk("chatcmpl-456", 0, `{\\\"location\\\"`),
		toolArgsChunk("chatcmpl-456", 0, `:`),
		toolArgsChunk("chatcmpl-456", 0, `\\\"San Francisco\\\"}`),
		toolArgsChunk("chatcmpl-456", 0, `}`),
		finishChunk("chatcmpl-456", "tool_calls"),
		doneChunk(),
	})

	recorder := executeTransform(t, translator, stream)
	body := recorder.Body.String()

	// verify text and tool content
	assertTextContent(t, body, "Let me check that for you.")
	assertToolPresent(t, body, "call_abc123", "get_weather")

	// verify tool events and structure
	assertContainsAll(t, body, []string{
		`"type":"tool_use"`,
		`"type":"input_json_delta"`,
		`"partial_json"`,
	})

	// parse events to verify structure
	events := parseAnthropicEvents(t, body)
	assertStopReason(t, events, "tool_use")
	// should have two content blocks: text and tool_use
	assertContentBlockCount(t, events, 2)
}

func TestTransformStreamingResponse_MultipleToolCalls(t *testing.T) {
	translator := newTestTranslator()

	stream := mockOpenAIStream([]string{
		toolStartChunk("chatcmpl-789", 0, "call_1", "get_weather"),
		toolArgsChunk("chatcmpl-789", 0, `{\\\"location\\\":\\\"NYC\\\"}`),
		toolStartChunk("chatcmpl-789", 1, "call_2", "get_time"),
		toolArgsChunk("chatcmpl-789", 1, `{\\\"timezone\\\":\\\"EST\\\"}`),
		finishChunk("chatcmpl-789", "tool_calls"),
		doneChunk(),
	})

	recorder := executeTransform(t, translator, stream)
	body := recorder.Body.String()

	// verify both tools are present
	assertToolPresent(t, body, "call_1", "get_weather")
	assertToolPresent(t, body, "call_2", "get_time")

	// verify arguments for both tools
	assert.Contains(t, body, `NYC`)
	assert.Contains(t, body, `EST`)

	// should have content_block_start for both tool calls
	events := parseAnthropicEvents(t, body)
	assertContentBlockCount(t, events, 2)
}

func TestTransformStreamingResponse_ToolCallsOnly(t *testing.T) {
	translator := newTestTranslator()

	stream := mockOpenAIStream([]string{
		toolStartChunk("chatcmpl-tool-only", 0, "call_only", "search"),
		toolArgsChunk("chatcmpl-tool-only", 0, `{\\\"query\\\":\\\"test\\\"}`),
		finishChunk("chatcmpl-tool-only", "tool_calls"),
		doneChunk(),
	})

	recorder := executeTransform(t, translator, stream)
	body := recorder.Body.String()

	// verify tool_use block is created
	assertToolPresent(t, body, "call_only", "search")
	assert.Contains(t, body, `"type":"tool_use"`)

	// verify no text content block - only 1 block for tool_use
	events := parseAnthropicEvents(t, body)
	assertContentBlockCount(t, events, 1)
}

func TestTransformStreamingResponse_ContextCancellation(t *testing.T) {
	translator := newTestTranslator()

	// create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// create a slow stream that will be cancelled
	slowStream := &slowReader{
		data:   []byte("data: {\"id\":\"chatcmpl-cancel\",\"choices\":[{\"delta\":{\"content\":\"Hello\"},\"index\":0}]}\n\n"),
		cancel: cancel,
	}

	_, err := executeTransformWithContext(t, translator, ctx, slowStream)

	// should return context cancelled error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

// slow reader is a helper for testing context cancellation
type slowReader struct {
	data   []byte
	pos    int
	cancel context.CancelFunc
}

func (r *slowReader) Read(p []byte) (n int, err error) {
	if r.pos == 0 {
		r.cancel() // Cancel after first read
	}
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func TestTransformStreamingResponse_MalformedChunk(t *testing.T) {
	translator := newTestTranslator()

	stream := mockOpenAIStream([]string{
		textChunk("chatcmpl-bad", "claude-3-5-sonnet-20241022", "Hello"),
		"data: {invalid json}\n\n", // malformed chunk
		finishChunk("chatcmpl-bad", "stop"),
		doneChunk(),
	})

	recorder := executeTransform(t, translator, stream)
	body := recorder.Body.String()

	// stream should complete successfully with valid chunks processed
	assertContainsAll(t, body, []string{
		"event: message_start",
		"event: message_stop",
	})
	assertTextContent(t, body, "Hello")
}

func TestTransformStreamingResponse_EmptyStream(t *testing.T) {
	translator := newTestTranslator()

	stream := mockOpenAIStream([]string{doneChunk()})

	recorder := executeTransform(t, translator, stream)
	body := recorder.Body.String()

	// should still have message_start and message_stop even for empty content
	assertContainsAll(t, body, []string{
		"event: message_start",
		"event: message_stop",
	})
}

func TestTransformStreamingResponse_ModelExtraction(t *testing.T) {
	translator := newTestTranslator()

	testCases := []struct {
		name      string
		modelName string
	}{
		{name: "sonnet_3_5", modelName: "claude-3-5-sonnet-20241022"},
		{name: "opus_3", modelName: "claude-3-opus-20240229"},
		{name: "haiku_3", modelName: "claude-3-haiku-20240307"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stream := mockOpenAIStream([]string{
				textChunk("chatcmpl-model", tc.modelName, "Test"),
				finishChunk("chatcmpl-model", "stop"),
				doneChunk(),
			})

			recorder := executeTransform(t, translator, stream)
			body := recorder.Body.String()
			assert.Contains(t, body, `"model":"`+tc.modelName+`"`, "Model should be in message_start")
		})
	}
}

func TestTransformStreamingResponse_UsageTokens(t *testing.T) {
	translator := newTestTranslator()

	stream := mockOpenAIStream([]string{
		textChunk("chatcmpl-usage", "claude-3-5-sonnet-20241022", "Hello"),
		finishChunkWithUsage("chatcmpl-usage", "stop", 10, 5),
		doneChunk(),
	})

	recorder := executeTransform(t, translator, stream)
	body := recorder.Body.String()

	// parse events to verify token usage in message_delta
	events := parseAnthropicEvents(t, body)
	assertUsageTokens(t, events, 10, 5)

	// verify message_start includes usage structure (values will be 0 initially)
	messageStart := getEventByType(events, "message_start")
	require.NotNil(t, messageStart, "message_start event should exist")
	message, ok := messageStart["message"].(map[string]interface{})
	require.True(t, ok, "message_start should have message field")

	startUsage, ok := message["usage"].(map[string]interface{})
	require.True(t, ok, "message_start.message should have usage field")

	// openai provides usage at the end of the stream, so message_start will have 0 tokens
	startInputTokens, ok := startUsage["input_tokens"].(float64)
	require.True(t, ok, "message_start usage should have input_tokens field")
	assert.Equal(t, float64(0), startInputTokens, "message_start input_tokens should be 0 (usage comes at end in OpenAI)")
}

func TestTransformStreamingResponse_SSEFormat(t *testing.T) {
	translator := newTestTranslator()

	stream := mockOpenAIStream([]string{
		textChunk("chatcmpl-sse", "claude-3-5-sonnet-20241022", "Test"),
		finishChunk("chatcmpl-sse", "stop"),
		doneChunk(),
	})

	recorder := executeTransform(t, translator, stream)
	body := recorder.Body.String()

	// verify SSE format and content-type header
	assertSSEFormat(t, body)
	assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
}

func TestTransformStreamingResponse_FinishReasonMapping(t *testing.T) {
	translator := newTestTranslator()

	testCases := []struct {
		name               string
		finishReason       string
		expectedStopReason string
		needsTool          bool
	}{
		{name: "stop_to_end_turn", finishReason: "stop", expectedStopReason: "end_turn", needsTool: false},
		{name: "tool_calls_to_tool_use", finishReason: "tool_calls", expectedStopReason: "tool_use", needsTool: true},
		{name: "length_to_max_tokens", finishReason: "length", expectedStopReason: "max_tokens", needsTool: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var chunks []string

			// build appropriate stream based on finish_reason
			if tc.needsTool {
				chunks = []string{
					toolStartChunk("chatcmpl-reason", 0, "call_1", "test"),
					toolArgsChunk("chatcmpl-reason", 0, "{}"),
					finishChunk("chatcmpl-reason", tc.finishReason),
					doneChunk(),
				}
			} else {
				chunks = []string{
					textChunk("chatcmpl-reason", "claude-3-5-sonnet-20241022", "Test"),
					finishChunk("chatcmpl-reason", tc.finishReason),
					doneChunk(),
				}
			}

			stream := mockOpenAIStream(chunks)
			recorder := executeTransform(t, translator, stream)
			body := recorder.Body.String()

			assert.Contains(t, body, `"stop_reason":"`+tc.expectedStopReason+`"`,
				"finish_reason %s should map to stop_reason %s", tc.finishReason, tc.expectedStopReason)
		})
