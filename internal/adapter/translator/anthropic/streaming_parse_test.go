package anthropic

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProcessStreamLine_NullContentDelta verifies that a delta with an explicit
// "content":null value does not emit any content_block_delta event. JSON null
// unmarshals to nil for a *string, so the nil-check on the typed path must hold.
func TestProcessStreamLine_NullContentDelta(t *testing.T) {
	t.Parallel()
	tr := newTestTranslator()

	// content is explicitly null — not absent, not empty string
	stream := mockOpenAIStream([]string{
		`data: {"id":"chatcmpl-null","model":"claude-3-5-sonnet-20241022","choices":[{"delta":{"content":null},"index":0}]}` + "\n\n",
		finishChunk("chatcmpl-null", "stop"),
		doneChunk(),
	})

	recorder := httptest.NewRecorder()
	require.NoError(t, tr.TransformStreamingResponse(context.Background(), stream, recorder, nil))

	events := parseAnthropicEvents(t, recorder.Body.String())

	// null content must not produce any content_block_delta
	assertEventCount(t, events, "content_block_delta", 0)
	// but the stream must still complete
	assertHasEventType(t, events, "message_start")
	assertHasEventType(t, events, "message_stop")
}

// TestProcessStreamLine_RoleOnlyDelta verifies that a delta carrying only a
// "role":"assistant" field emits no content blocks at all. This is the first
// chunk pattern emitted by some backends to announce the role before content
// begins streaming.
func TestProcessStreamLine_RoleOnlyDelta(t *testing.T) {
	t.Parallel()
	tr := newTestTranslator()

	stream := mockOpenAIStream([]string{
		`data: {"id":"chatcmpl-role","model":"claude-3-5-sonnet-20241022","choices":[{"delta":{"role":"assistant"},"index":0}]}` + "\n\n",
		finishChunk("chatcmpl-role", "stop"),
		doneChunk(),
	})

	recorder := httptest.NewRecorder()
	require.NoError(t, tr.TransformStreamingResponse(context.Background(), stream, recorder, nil))

	events := parseAnthropicEvents(t, recorder.Body.String())

	// role-only delta must produce zero content blocks and zero deltas
	assertEventCount(t, events, "content_block_start", 0)
	assertEventCount(t, events, "content_block_delta", 0)
	assertEventCount(t, events, "content_block_stop", 0)
}

// TestProcessStreamLine_ReasoningBeatsContent verifies that when a single delta
// carries both "reasoning" and "content", BOTH are emitted rather than dropping
// the content. Some OpenAI-compatible adapters coalesce reasoning + content into
// one chunk; the old early-return behaviour silently lost the content.
//
// Correct output: thinking block (opened and closed) followed by text block, with
// the thinking block stop emitted before the text block start.
func TestProcessStreamLine_ReasoningBeatsContent(t *testing.T) {
	t.Parallel()
	tr := newTestTranslator()

	// Single chunk with both reasoning and content populated.
	stream := mockOpenAIStream([]string{
		`data: {"id":"chatcmpl-rc","model":"deepseek-r1","choices":[{"delta":{"reasoning":"think","content":"text"},"index":0}]}` + "\n\n",
		finishChunk("chatcmpl-rc", "stop"),
		doneChunk(),
	})

	recorder := httptest.NewRecorder()
	require.NoError(t, tr.TransformStreamingResponse(context.Background(), stream, recorder, nil))

	body := recorder.Body.String()
	events := parseAnthropicEvents(t, body)

	// Both the thinking and the text must be present.
	assert.Contains(t, body, `"thinking":"think"`)
	assert.Contains(t, body, `"text":"text"`)

	// Two content_block_deltas: one thinking_delta, one text_delta.
	assertEventCount(t, events, "content_block_delta", 2)

	deltas := findEventsByType(events, "content_block_delta")
	require.Len(t, deltas, 2)

	firstDelta, ok := deltas[0]["delta"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "thinking_delta", firstDelta["type"], "first delta must be thinking_delta")

	secondDelta, ok := deltas[1]["delta"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "text_delta", secondDelta["type"], "second delta must be text_delta")

	// Two blocks (thinking + text), both properly closed.
	assertContentBlockCount(t, events, 2)
	assertBlocksClosed(t, events)

	// Thinking block must be fully stopped before the text block opens.
	assertThinkingBlockTransitionOrder(t, events)
}

// TestProcessStreamLine_ReasoningContentFieldName verifies the alternative field
// name "reasoning_content" (vLLM / SGLang / DeepSeek convention) is treated
// identically to "reasoning" and produces a thinking_delta.
func TestProcessStreamLine_ReasoningContentFieldName(t *testing.T) {
	t.Parallel()
	tr := newTestTranslator()

	stream := mockOpenAIStream([]string{
		`data: {"id":"chatcmpl-rcf","model":"qwq-32b","choices":[{"delta":{"reasoning_content":"deep think"},"index":0}]}` + "\n\n",
		finishChunk("chatcmpl-rcf", "stop"),
		doneChunk(),
	})

	recorder := httptest.NewRecorder()
	require.NoError(t, tr.TransformStreamingResponse(context.Background(), stream, recorder, nil))

	body := recorder.Body.String()
	events := parseAnthropicEvents(t, body)

	assert.Contains(t, body, `"thinking":"deep think"`)

	// exactly one thinking block opened and closed
	assertContentBlockCount(t, events, 1)
	assertBlocksClosed(t, events)

	starts := findEventsByType(events, "content_block_start")
	bt, _ := getContentBlockType(starts[0])
	assert.Equal(t, contentTypeThinking, bt)
}

// TestProcessStreamLine_ToolCallContinuationNoID verifies the two-chunk tool
// call sequence: first chunk carries id+name (index=0), second carries only
// index+partial-args (no id, no name). The assembled args must appear in
// input_json_delta events and both deltas must reference block index 0.
//
// The args string must use the \\\" escaping convention (same as other streaming
// tests) because toolArgsChunk embeds the args into a JSON string value: raw
// braces would break the outer JSON and cause the chunk to be skipped.
func TestProcessStreamLine_ToolCallContinuationNoID(t *testing.T) {
	t.Parallel()
	tr := newTestTranslator()

	stream := mockOpenAIStream([]string{
		// first chunk: id + name establish the block
		toolStartChunk("chatcmpl-cont", 0, "call_cont", "cont_tool"),
