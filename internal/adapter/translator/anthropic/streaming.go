package anthropic

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/util"
)

// openAIChunk is the typed representation of an incoming OpenAI SSE data line.
// Using a typed struct instead of map[string]interface{} avoids per-chunk map
// allocations and interface boxing - the dominant allocator on the streaming hot path.
type openAIChunk struct {
	Usage   *openAIUsage   `json:"usage"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
}

type openAIChoice struct {
	FinishReason *string     `json:"finish_reason"`
	Delta        openAIDelta `json:"delta"`
	Index        int         `json:"index"`
}

type openAIDelta struct {
	Role             string           `json:"role"`
	Content          *string          `json:"content"`
	Reasoning        string           `json:"reasoning"`
	ReasoningContent string           `json:"reasoning_content"`
	ToolCalls        []openAIToolCall `json:"tool_calls"`
}

type openAIToolCall struct {
	Index    *int           `json:"index"`
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// openAIUsage holds token counts from the final SSE chunk.
// Fields are int rather than float64 because json.Unmarshal into a typed struct
// uses the declared field type directly, avoiding the float64-then-int cast
// that the map[string]interface{} path required.
type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// sseDeltaEvent is the hot-path content_block_delta envelope.
// Field order is chosen for struct packing (interface{} + string + int fits without
// padding on amd64). JSON tag order (delta, index, type) differs from field
// declaration order; clients parse JSON by key, not position, so this is safe.
type sseDeltaEvent struct {
	Delta interface{} `json:"delta"` // sseTextDelta | sseThinkingDelta | sseInputJSONDelta
	Type  string      `json:"type"`
	Index int         `json:"index"`
}

// sseTextDelta is the delta payload for text_delta events.
// Field order matches alphabetical JSON tag order (text < type).
type sseTextDelta struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

// sseThinkingDelta is the delta payload for thinking_delta events.
// Field order matches alphabetical JSON tag order (thinking < type).
type sseThinkingDelta struct {
	Thinking string `json:"thinking"`
	Type     string `json:"type"`
}

// sseInputJSONDelta is the delta payload for input_json_delta events.
// Field order matches alphabetical JSON tag order (partial_json < type).
type sseInputJSONDelta struct {
	PartialJSON string `json:"partial_json"`
	Type        string `json:"type"`
}

// errInterleavedToolArguments signals that args arrived for a tool block that was
// already stopped and closed on the wire. Corrupt tool input is worse than a failed
// stream; the client can retry a clean failure, but cannot recover silent data loss.
var errInterleavedToolArguments = errors.New("tool arguments received for already-stopped block")

// tracks state while streaming - buffers partial data, blocks in progress
type StreamingState struct {
	currentBlock     *ContentBlock
	toolCallBuffers  map[int]*strings.Builder // keyed by tool index, avoids string formatting overhead
	toolIndexToBlock map[int]int              // maps tool index to content block index for finalisation
	messageID        string
	model            string
	lastFinishReason string
	contentBlocks    []ContentBlock
	currentIndex     int
	inputTokens      int
	outputTokens     int
	messageStartSent bool
	// reasoningOpen is true while a thinking block has been started but not yet stopped.
	// The first non-reasoning delta closes the thinking block before opening its own.
	reasoningOpen bool
}

// convert openai sse stream to anthropic format
func (t *Translator) TransformStreamingResponse(ctx context.Context, openaiStream io.Reader, w http.ResponseWriter, original *http.Request) error {
	// text/event-stream, no caching
	w.Header().Set(constants.HeaderContentType, "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	rc := http.NewResponseController(w)

	state := &StreamingState{
		messageID:        t.generateMessageID(),
		contentBlocks:    make([]ContentBlock, 0, 4),
		toolCallBuffers:  make(map[int]*strings.Builder),
		toolIndexToBlock: make(map[int]int),
	}

	// Seed input_tokens from the pre-computed estimate injected by the handler.
	// vLLM and lmdeploy both emit real input_tokens in message_start rather than 0;
	// this brings the translation path in line with that behaviour. The value is
	// overwritten by actual upstream usage when (and if) the backend sends it.
	if estimate, ok := ctx.Value(constants.ContextInputTokensKey).(int); ok && estimate > 0 {
		state.inputTokens = estimate
	}

	// sync streaming for now (async needs more work for agent workflows)
	streamErr := t.transformStreamingSync(ctx, openaiStream, w, rc, state)

	if streamErr != nil {
		// Return immediately for all stream errors. For errInterleavedToolArguments the
		// SSE error event is already on the wire; emitting message_delta/message_stop after
		// an error event is invalid per the Anthropic spec.
		return streamErr
	}

	// send message_start even if stream was empty
	if !state.messageStartSent {
		if err := t.writeEvent(w, "message_start", t.createMessageStart(state)); err != nil {
			return err
		}
		if err := rc.Flush(); err != nil {
			return fmt.Errorf("flush failed: %w", err)
		}
		state.messageStartSent = true
	}

	// send final events (stop reason + token counts)
	if err := t.finalizeStream(state, w, rc, original); err != nil {
		return err
	}

	return nil
}

// process stream using blocking scanner, safer and simpler
func (t *Translator) transformStreamingSync(ctx context.Context, openaiStream io.Reader, w http.ResponseWriter, rc *http.ResponseController, state *StreamingState) error {
	scanner := bufio.NewScanner(openaiStream)
	// allow large deltas and tool arg chunks, prevents "token too long" errors
	// initial buffer 64 KiB, max 1 MiB per SSE line (handles large tool arguments)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1<<20)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if err := t.processStreamLine(line, state, w, rc); err != nil {
			// Interleaved tool args corrupt the client's tool input; abort the stream.
			if errors.Is(err, errInterleavedToolArguments) {
				return err
			}
			t.logger.Error("Error processing stream line", "error", err)
			continue // keep going, don't fail entire stream on one bad line
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	return nil
}

// process single sse line from openai, route to content or tool handlers
func (t *Translator) processStreamLine(line string, state *StreamingState, w http.ResponseWriter, rc *http.ResponseController) error {
	if !strings.HasPrefix(line, "data: ") {
		return nil
	}

	data := strings.TrimPrefix(line, "data: ")
	if strings.TrimSpace(data) == "[DONE]" {
		return nil
	}

	var chunk openAIChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		// log bad chunks but keep going, partial responses better than nothing
		t.logger.Warn("Malformed chunk encountered, skipping", "error", err,
			"data", util.TruncateString(data, util.DefaultTruncateLengthPII), "data_len", len(data))
		return nil
	}

	// grab model name for message_start event
	if state.model == "" && chunk.Model != "" {
		state.model = chunk.Model
	}

	// Capture usage before the empty-choices guard. vLLM (and others that honour
	// include_usage=true) emit the final usage as a standalone chunk with
	// choices:[] and a populated usage object. Processing choices first and
	// returning early would silently discard those real token counts, leaving
	// message_delta with the pre-stream estimate or output_tokens:0.
	if u := chunk.Usage; u != nil {
		state.inputTokens = u.PromptTokens
		state.outputTokens = u.CompletionTokens
	}

	if len(chunk.Choices) == 0 {
		return nil
	}

	choice := chunk.Choices[0]

	// capture finish_reason for later stop_reason mapping; nil or empty means absent
	if fr := choice.FinishReason; fr != nil && *fr != "" {
		state.lastFinishReason = *fr
	}

	delta := choice.Delta

	// reasoning / reasoning_content are the per-backend field names for chain-of-thought.
	// Ollama, LM Studio, and Lemonade use "reasoning"; vLLM, SGLang, and DeepSeek use
	// "reasoning_content". Treat them as equivalent and prefer whichever is non-empty.
	//
	// Some OpenAI-compatible adapters coalesce reasoning + content (or + tool_calls) into
	// a single chunk. We must not return early after handling reasoning: fall through so
	// content and tool_calls in the same chunk are also processed. handleContentDelta and
	// initializeToolBlock both call closeCurrentBlock before opening a new one, so the
	// thinking block is correctly stopped before the text or tool block opens.
	reasoning := extractReasoningFieldTyped(delta)
	if reasoning != "" {
		if err := t.handleReasoningDelta(reasoning, state, w, rc); err != nil {
			return err
		}
	}

