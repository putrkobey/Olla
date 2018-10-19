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
