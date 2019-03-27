package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJSONParsingWithThinkingAndSystemArray verifies that JSON with thinking field
// and system array can be parsed without "unknown field" errors
func TestJSONParsingWithThinkingAndSystemArray(t *testing.T) {
	jsonData := `{
		"model": "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"thinking": {
			"type": "enabled",
			"budget_tokens": 2000
		},
		"system": [
			{
				"type": "text",
				"text": "You are a helpful AI assistant."
			}
		],
		"messages": [
			{
				"role": "user",
				"content": "What is 2+2?"
			}
		]
	}`

	var req AnthropicRequest

	// This should NOT error with "unknown field 'thinking'"
	err := json.Unmarshal([]byte(jsonData), &req)
	require.NoError(t, err, "Should parse JSON with thinking and system array without error")

