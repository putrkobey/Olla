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
	}
	if r.Temperature != nil && (*r.Temperature < 0 || *r.Temperature > 2) {
		return fmt.Errorf("temperature must be between 0 and 2, got %f", *r.Temperature)
	}
	if r.TopP != nil && (*r.TopP < 0 || *r.TopP > 1) {
		return fmt.Errorf("top_p must be between 0 and 1, got %f", *r.TopP)
	}
	if r.TopK != nil && *r.TopK < 0 {
		return fmt.Errorf("top_k must be non-negative, got %d", *r.TopK)
	}

	return nil
}

// AnthropicMessage represents a message in the conversation
// Content can be either a simple string or an array of content blocks
type AnthropicMessage struct {
	Content interface{} `json:"content"` // string or []ContentBlock
	Role    string      `json:"role"`    // "user" or "assistant"
}

// ContentBlock represents different types of content in messages
// Anthropic uses a block-based content model for text, images, tool use, tool results, and thinking.
type ContentBlock struct {
	Content   interface{}            `json:"content,omitempty"` // for tool_result
	Source    *ImageSource           `json:"source,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	Type      string                 `json:"type"` // "text", "image", "tool_use", "tool_result", "thinking"
	Text      string                 `json:"text,omitempty"`
	Thinking  string                 `json:"thinking,omitempty"` // populated for type="thinking" response blocks
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
}

// ImageSource represents image data in content blocks
// Supports both base64-encoded data and URLs
type ImageSource struct {
	Type      string `json:"type"` // "base64" or "url"
	MediaType string `json:"media_type,omitempty"`
