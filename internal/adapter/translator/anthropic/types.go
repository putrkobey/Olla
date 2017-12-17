package anthropic

import (
	"errors"
	"fmt"
)

// AnthropicRequest represents an Anthropic API request
// Maps to the Anthropic Messages API format
type AnthropicRequest struct {
	ToolChoice    interface{}            `json:"tool_choice,omitempty"`   // string or object
	System        interface{}            `json:"system,omitempty"`        // string or []ContentBlock
	Thinking      interface{}            `json:"thinking,omitempty"`      // Extended thinking configuration
	OutputConfig  interface{}            `json:"output_config,omitempty"` // Output configuration (effort, structured output format)
	Temperature   *float64               `json:"temperature,omitempty"`
	TopP          *float64               `json:"top_p,omitempty"`
	TopK          *int                   `json:"top_k,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Model         string                 `json:"model"`
	Messages      []AnthropicMessage     `json:"messages"`
	StopSequences []string               `json:"stop_sequences,omitempty"`
	Tools         []AnthropicTool        `json:"tools,omitempty"`
	MaxTokens     int                    `json:"max_tokens"`
	Stream        bool                   `json:"stream,omitempty"`
}

// Validate checks that required fields are present and all fields have valid values
// Returns an error if validation fails with a descriptive message
func (r *AnthropicRequest) Validate() error {
	// Validate required fields
	if r.Model == "" {
		return errors.New("model field is required")
	}
	if len(r.Messages) == 0 {
		return errors.New("at least one message is required")
	}

	// Validate optional fields have acceptable values
	if r.MaxTokens < 1 {
		return fmt.Errorf("max_tokens must be at least 1, got %d", r.MaxTokens)
