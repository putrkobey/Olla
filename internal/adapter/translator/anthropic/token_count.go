package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/putrkobey/Olla/internal/adapter/translator"
)

// countTokensWireResponse is the Anthropic-conformant wire shape for
// POST /v1/messages/count_tokens. The spec returns {"input_tokens":N} only;
// output_tokens and total_tokens are not part of this endpoint's contract.
// Confirmed against vLLM, lmdeploy, bifrost, and litellm reference impls.
type countTokensWireResponse struct {
	InputTokens int `json:"input_tokens"`
}

// EstimateInputTokens implements translator.InputTokenEstimator.
// Called by the streaming path to seed input_tokens in message_start before the
// upstream usage chunk arrives (which only comes at the end of the stream).
// vLLM and lmdeploy both populate real input_tokens in message_start; this brings
// Olla's translation path in line with that behaviour.
func (t *Translator) EstimateInputTokens(originalBodyBytes []byte) int {
	var req AnthropicRequest
	if err := json.Unmarshal(originalBodyBytes, &req); err != nil {
		return 0
	}
	return estimateTokensFromRequest(&req)
}

// token estimation for claude code compatibility
func (t *Translator) CountTokens(ctx context.Context, r *http.Request) (*translator.TokenCountResponse, error) {
	// bounded read to prevent OOM attacks
	// read up to maxMessageSize + 1 to detect oversized requests
	limitedReader := io.LimitReader(r.Body, t.maxMessageSize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		// close the original body even on error
		_ = r.Body.Close()
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// detect oversized requests
	if int64(len(body)) > t.maxMessageSize {
		_ = r.Body.Close()
		return nil, fmt.Errorf("request body exceeds maximum size of %d bytes", t.maxMessageSize)
	}

	// reset body for downstream handlers to re-read
	r.Body = io.NopCloser(bytes.NewReader(body))

	var req AnthropicRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	// token counting using character estimation
	// simple algorithm: chars / 4
	tokenCount := estimateTokensFromRequest(&req)

	return &translator.TokenCountResponse{
		InputTokens: tokenCount,
	}, nil
}

// SerialiseCountTokens implements translator.TokenCountSerializer.
// Returns only {"input_tokens":N} — the Anthropic spec for this endpoint does not
// include output_tokens or total_tokens. Confirmed against vLLM, lmdeploy, bifrost,
// and litellm reference implementations.
func (t *Translator) SerialiseCountTokens(resp *translator.TokenCountResponse) ([]byte, error) {
	return json.Marshal(countTokensWireResponse{InputTokens: resp.InputTokens})
}

// character-based token estimation
// simple algorithm from python reference: chars / 4
func estimateTokensFromRequest(req *AnthropicRequest) int {
	totalChars := 0

	// system prompt char counting
	// handles string and content block formats
	totalChars += countSystemChars(req.System)

	// count all message content
	for _, msg := range req.Messages {
		totalChars += countMessageChars(&msg)
	}

	// Claude Code sends 10-20k tokens of tool schemas per request; omitting tools
	// causes a large systematic undercount that misseeds message_start input_tokens.
	for i := range req.Tools {
		totalChars += countToolDefinitionChars(&req.Tools[i])
	}

	tokenCount := totalChars / 4
	if tokenCount < 1 {
		tokenCount = 1
	}

	return tokenCount
}

// countToolDefinitionChars estimates character cost of a single tool definition.
// Counts name + description + JSON-marshalled input schema, matching the chars/4 heuristic.
func countToolDefinitionChars(tool *AnthropicTool) int {
	if tool == nil {
		return 0
	}
	n := len(tool.Name) + len(tool.Description)
	if tool.InputSchema != nil {
		if schemaJSON, err := json.Marshal(tool.InputSchema); err == nil {
			n += len(schemaJSON)
		}
	}
	return n
}
