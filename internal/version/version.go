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
		"vllm",
		"vllm-mlx",
		"openai_compatible",
		"docker-model-runner",
		"text-embeddings-inference",
	}
	ExperimentalCapabilities = []string{
		"translation_anthropic",
		"passthrough_anthropic",
	}
)

const (
	GithubHomeText  = "github.com/putrkobey/Olla"
	GithubHomeUri   = "https://github.com/putrkobey/Olla"
	GithubLatestUri = "https://github.com/putrkobey/Olla/releases/latest"
	BoxWidth        = 70
)

func PrintVersionInfo(extendedInfo bool, vlog *log.Logger) {

	var b strings.Builder

	if util.ShouldUseColors() {
		b.WriteString(formatAsciiBanner())
	} else {
		b.WriteString(formatPlainBanner())
	}

	if extendedInfo {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf(" Commit: %s\n", Commit))
		b.WriteString(fmt.Sprintf("  Built: %s\n", Date))
		b.WriteString(fmt.Sprintf("  Using: %s\n", Tool))
		b.WriteString(fmt.Sprintf("   With: %s\n", Runtime))
		b.WriteString(fmt.Sprintf("    For: %s\n", runtime.GOOS))
		b.WriteString(fmt.Sprintf("     On: %s\n", runtime.GOARCH))
		b.WriteString(fmt.Sprintf("     By: %s\n", User))
	}

	vlog.Println(b.String())
}
func formatAsciiBanner() string {
