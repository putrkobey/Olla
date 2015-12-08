package anthropic

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Streaming tests
// ---------------------------------------------------------------------------

// TestStreaming_Reasoning_ReasoningField verifies that "reasoning" deltas produce
// the full thinking block sequence before the text block.
func TestStreaming_Reasoning_ReasoningField(t *testing.T) {
	t.Parallel()
	tr := newTestTranslator()

	stream := mockOpenAIStream([]string{
		reasoningChunk("chatcmpl-r1", "deepseek-r1", "Let me think"),
		reasoningChunk("chatcmpl-r1", "", " about this"),
		textChunk("chatcmpl-r1", "", "The answer is 42"),
		finishChunk("chatcmpl-r1", "stop"),
		doneChunk(),
	})

	recorder := httptest.NewRecorder()
	require.NoError(t, tr.TransformStreamingResponse(context.Background(), stream, recorder, nil))

	body := recorder.Body.String()
	events := parseAnthropicEvents(t, body)

	// thinking block must start before text block
	assertThinkingBlockTransitionOrder(t, events)

	// thinking block opened with correct type
	starts := findEventsByType(events, "content_block_start")
	require.GreaterOrEqual(t, len(starts), 2)
	thinkingType, _ := getContentBlockType(starts[0])
	assert.Equal(t, contentTypeThinking, thinkingType, "first block must be thinking")

	textType, _ := getContentBlockType(starts[1])
	assert.Equal(t, contentTypeText, textType, "second block must be text")

	// both blocks must be closed
	assertBlocksClosed(t, events)
	assertContentBlockCount(t, events, 2)

	// thinking text must appear in thinking_delta events
	assert.Contains(t, body, `"type":"thinking_delta"`)
	assert.Contains(t, body, `"thinking":"Let me think"`)
	assert.Contains(t, body, `"thinking":" about this"`)

	// text must appear in text_delta
	assertTextContent(t, body, "The answer is 42")
}

// TestStreaming_Reasoning_ReasoningContentField verifies that "reasoning_content"
// (vLLM / SGLang / DeepSeek naming) produces identical output to "reasoning".
func TestStreaming_Reasoning_ReasoningContentField(t *testing.T) {
	t.Parallel()
	tr := newTestTranslator()

	stream := mockOpenAIStream([]string{
		reasoningContentChunk("chatcmpl-rc1", "qwq-32b", "Step 1: analyse"),
		reasoningContentChunk("chatcmpl-rc1", "", " the problem"),
		textChunk("chatcmpl-rc1", "", "Done."),
		finishChunk("chatcmpl-rc1", "stop"),
		doneChunk(),
	})

	recorder := httptest.NewRecorder()
	require.NoError(t, tr.TransformStreamingResponse(context.Background(), stream, recorder, nil))

	body := recorder.Body.String()
	events := parseAnthropicEvents(t, body)

	assertThinkingBlockTransitionOrder(t, events)
	assertBlocksClosed(t, events)
	assertContentBlockCount(t, events, 2)

	assert.Contains(t, body, `"type":"thinking_delta"`)
	assertThinkingContent(t, body, "Step 1: analyse")
	assertThinkingContent(t, body, " the problem")
	assertTextContent(t, body, "Done.")
}

// TestStreaming_Reasoning_OnlyReasoningNoContent verifies that a reasoning-only
// response (no text content) still produces a complete, closed thinking block
// plus the standard message_delta / message_stop tail.
func TestStreaming_Reasoning_OnlyReasoningNoContent(t *testing.T) {
	t.Parallel()
	tr := newTestTranslator()

	stream := mockOpenAIStream([]string{
		reasoningChunk("chatcmpl-ro1", "deepseek-r1", "Internal monologue"),
		finishChunk("chatcmpl-ro1", "stop"),
		doneChunk(),
	})

	recorder := httptest.NewRecorder()
	require.NoError(t, tr.TransformStreamingResponse(context.Background(), stream, recorder, nil))

	body := recorder.Body.String()
	events := parseAnthropicEvents(t, body)

	// exactly one content block: the thinking block
	assertContentBlockCount(t, events, 1)
	assertBlocksClosed(t, events)

	starts := findEventsByType(events, "content_block_start")
	require.Len(t, starts, 1)
	bt, _ := getContentBlockType(starts[0])
	assert.Equal(t, contentTypeThinking, bt)

	// full sequence must be present
	assertRequiredEvents(t, events)
	assert.Contains(t, body, `"thinking":"Internal monologue"`)
	assertStopReason(t, events, "end_turn")
}

// TestStreaming_Reasoning_NoReasoningUnchanged is a regression guard: when there is
// no reasoning field the output must be byte-identical to the pre-reasoning behaviour.
func TestStreaming_Reasoning_NoReasoningUnchanged(t *testing.T) {
	t.Parallel()
	tr := newTestTranslator()

	stream := mockOpenAIStream([]string{
		textChunk("chatcmpl-nr1", "some-model", "Hello world"),
		finishChunk("chatcmpl-nr1", "stop"),
		doneChunk(),
	})

	recorder := executeTransform(t, tr, stream)
	body := recorder.Body.String()
	events := parseAnthropicEvents(t, body)

	// no thinking blocks at all
	assert.NotContains(t, body, contentTypeThinking)
	assert.NotContains(t, body, "thinking_delta")

	assertContentBlockCount(t, events, 1)
	starts := findEventsByType(events, "content_block_start")
	bt, _ := getContentBlockType(starts[0])
	assert.Equal(t, contentTypeText, bt)

	assertTextContent(t, body, "Hello world")
	assertBlocksClosed(t, events)
}

// TestStreaming_Reasoning_BlockOrderExactSequence pins the full SSE event sequence
