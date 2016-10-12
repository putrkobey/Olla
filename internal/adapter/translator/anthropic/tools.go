package anthropic

import "errors"

// convertTools transforms Anthropic tool definitions to OpenAI function format
// Maps input_schema directly to parameters (both use JSON Schema)
// Returns only the converted tools since this conversion cannot fail
func (t *Translator) convertTools(anthropicTools []AnthropicTool) []map[string]interface{} {
	openaiTools := make([]map[string]interface{}, 0, len(anthropicTools))

	for _, tool := range anthropicTools {
		openaiTool := map[string]interface{}{
			"type": openAITypeFunction,
			"function": map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.InputSchema, // Direct mapping - both use JSON Schema
			},
		}
		openaiTools = append(openaiTools, openaiTool)
	}

	return openaiTools
}

// convertToolChoice transforms Anthropic tool_choice to OpenAI format
// Handles both string and object forms with semantic mapping:
// - "auto" -> "auto" (let model decide)
// - "any" -> "required" (force tool use)
// - {"type": "tool", "name": "X"} -> {"type": "function", "function": {"name": "X"}}
func (t *Translator) convertToolChoice(toolChoice interface{}) (interface{}, error) {
