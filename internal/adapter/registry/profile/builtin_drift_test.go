package profile

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// shippedProfilesDir is config/profiles, relative to this package
// (internal/adapter/registry/profile -> repo root is four levels up).
const shippedProfilesDir = "../../../../config/profiles/"

// builtinShippedPairs maps each hardcoded Go fallback in loadBuiltInProfilesInto
// to the shipped YAML file it must match.
var builtinShippedPairs = map[string]string{
	domain.ProfileOllama:           "ollama.yaml",
	domain.ProfileLmStudio:         "lmstudio.yaml",
	domain.ProfileOpenAICompatible: "openai-compatible.yaml",
	domain.ProfileLlamaCpp:         "llamacpp.yaml",
}

// TestBuiltinProfilesMatchShippedYAML is a drift guard between the hardcoded
// Go fallbacks in loadBuiltInProfilesInto (used when config/profiles/ is
