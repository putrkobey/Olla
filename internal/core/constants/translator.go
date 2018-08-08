package constants

// TranslatorMode represents how the request was handled by the translator
type TranslatorMode string

const (
	// TranslatorModePassthrough indicates request was passed through natively (no translation)
	TranslatorModePassthrough TranslatorMode = "passthrough"

	// TranslatorModeTranslation indicates request was translated between formats
	TranslatorModeTranslation TranslatorMode = "translation"
)

// TranslatorFallbackReason explains why passthrough wasn't used
type TranslatorFallbackReason string
