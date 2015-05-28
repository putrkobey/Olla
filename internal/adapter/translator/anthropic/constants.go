package anthropic

const (
	contentTypeText       = "text"
	contentTypeToolUse    = "tool_use"
	contentTypeToolResult = "tool_result"
	contentTypeImage      = "image"
	contentTypeThinking   = "thinking"
)

const (
	toolChoiceAuto = "auto"
	toolChoiceAny  = "any"
	toolChoiceNone = "none"
	toolChoiceTool = "tool"
)

const (
	openAIToolChoiceRequired = "required"
	openAIToolChoiceAuto     = "auto"
	openAIToolChoiceNone     = "none"
