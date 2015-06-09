package version

import (
	"fmt"
	"log"
	"runtime"
	"strings"

	"github.com/putrkobey/Olla/internal/util"
	"github.com/putrkobey/Olla/theme"
)

var (
	Name         = "Olla"
	ShortName    = "olla"
	Edition      = "Community"
	Authors      = "Thushan Fernando"
	Description  = "The AI Proxy for your LLMs"
	Version      = "v0.0.x"
	Commit       = "none"
	Date         = "nowish"
	User         = "local"
	Tool         = "make"
	Runtime      = "Go 1.2x.0"
	Capabilities = []string{
		"load_balancing",
		"health_checking",
		"rate_limiting",
		"model_unification",
		"endpoint_discovery",
		"message_translation",
		"translation_anthropic",
		"passthrough_anthropic",
	}
	SupportedBackends = []string{
		"ollama",
		"lemonade",
		"litellm",
		"llamacpp",
		"lmdeploy",
		"lm_studio",
		"omlx",
		"sglang",
