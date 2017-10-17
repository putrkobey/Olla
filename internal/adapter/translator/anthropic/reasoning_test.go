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

