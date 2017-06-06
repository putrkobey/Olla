package unifier

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// fallbackConfig caches the error-path default so a run of LoadModelConfig
// errors (extraction is called per-request) doesn't rebuild the whole
// ModelUnificationConfig struct literal on every call. Deliberately a
// separate cache from LoadModelConfig's own sync.Once/configOnce: this one
// only ever holds the fixed embedded default, so it has nothing for
// resetConfigSingleton to need to invalidate.
var (
	fallbackConfigOnce sync.Once
	fallbackConfig     *ModelUnificationConfig
)

// getConfig returns the model unification config. LoadModelConfig already
// caches via its own sync.Once, so this used to duplicate that with a second,
// independent cache - which meant resetConfigSingleton (used by tests to force
// a fresh reload) didn't actually reach extraction, since this package's own
// Once stayed latched to whatever it saw first.
func getConfig() *ModelUnificationConfig {
	config, err := LoadModelConfig()
	if err != nil {
		fallbackConfigOnce.Do(func() {
			fallbackConfig = getDefaultConfig()
			// Uncompiled patterns make regex-driven extraction a silent no-op,
			// so compile here as LoadModelConfig does on its own recovery path.
			_ = fallbackConfig.compilePatterns()
		})
		return fallbackConfig
	}
	return config
}

// normalizeQuantization converts various quantization formats to a canonical form
func normalizeQuantization(quant string) string {
	if quant == "" {
		return ""
	}

	config := getConfig()
	quant = strings.ToUpper(quant)

	// Use mappings from configuration
	if normalized, exists := config.Quantization.Mappings[quant]; exists {
		return normalized
	}

	// Try to extract quantization from model name patterns
	// e.g., "llama-3.2-1b-instruct-q4_k_m.gguf" -> "q4km"
	lowercaseQuant := strings.ToLower(quant)
	for _, p := range quantNamePatterns {
		if matches := p.regex.FindStringSubmatch(lowercaseQuant); matches != nil {
			return p.regex.ReplaceAllString(matches[0], p.replacement)
		}
	}

	return strings.ToLower(quant)
}

// Compiled once: these used to be rebuilt on every call, which was a quarter of the time
// spent unifying a model list.
var (
	quantNamePatterns = []struct {
		regex       *regexp.Regexp
		replacement string
	}{
		{regexp.MustCompile(`(?i)q(\d+)_k_([msl])`), "q$1k$2"},
		{regexp.MustCompile(`(?i)q(\d+)_(\d+)`), "q$1_$2"},
		{regexp.MustCompile(`(?i)q(\d+)`), "q$1"},
		{regexp.MustCompile(`(?i)f(\d+)`), "f$1"},
		{regexp.MustCompile(`(?i)fp(\d+)`), "f$1"},
		{regexp.MustCompile(`(?i)int(\d+)`), "int$1"},
	}
	paramSizePattern     = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([BM]?)$`)
	nameDelimiterPattern = regexp.MustCompile(`[-_:/]`)
)

// extractParameterSize extracts and normalizes parameter size from various formats
func extractParameterSize(sizeStr string) (string, int64) {
	if sizeStr == "" {
		return "", 0
	}

	originalStr := sizeStr
	sizeStr = strings.TrimSpace(strings.ToUpper(sizeStr))

	matches := paramSizePattern.FindStringSubmatch(sizeStr)

	if len(matches) != 3 {
		return originalStr, 0
	}

	numStr := matches[1]
	unit := matches[2]

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return originalStr, 0
	}

	var paramCount int64
	var normalizedSize string

	switch unit {
	case "M":
		paramCount = int64(num * 1_000_000)
		billionValue := num / 1000.0
		if billionValue >= 1.0 {
			normalizedSize = fmt.Sprintf("%.1fb", billionValue)
		} else {
			normalizedSize = fmt.Sprintf("%.0fm", num)
		}
	case "B", "":
		paramCount = int64(num * 1_000_000_000)
		normalizedSize = fmt.Sprintf("%.1fb", num)
	}

	// Remove redundant decimal zeros for cleaner representation
	if strings.HasSuffix(normalizedSize, ".0b") {
		normalizedSize = strings.TrimSuffix(normalizedSize, ".0b") + "b"
	}
	if strings.HasSuffix(normalizedSize, ".0m") {
		normalizedSize = strings.TrimSuffix(normalizedSize, ".0m") + "m"
	}

	return normalizedSize, paramCount
}

// nonGenerativeModelTypes are model types that never produce text, so they must
// not inherit the text-generation default.
var nonGenerativeModelTypes = map[string]struct{}{
	"embeddings": {},
	"embedding":  {},
	"reranker":   {},
	"classifier": {},
}

var embedderCapabilities = []string{"embeddings", "similarity", "vector-search"}

// nonGenerativeCapabilities mark a model that scores or encodes text. Name
// patterns can add them to a model whose backend doesn't report a kind (a vLLM
// reranker, an OpenAI-style BGE embedder), and such a model must not keep the
// text-generation default any more than a typed one does.
var nonGenerativeCapabilities = []string{"embeddings", "rerank", "classification"}

// dropGenerativeForNonGenerative removes the generative defaults from a model
// that name patterns tagged as an embedder, reranker or classifier.
func dropGenerativeForNonGenerative(capabilities map[string]bool, modelType string) {
	// no model is both a reranker and an embedder, and "rerank" is the stronger
	// signal (granite-embedding-reranker-*), so the embedder tags go. A model the
	// backend typed as an embedder keeps them.
	if capabilities["rerank"] {
		if _, typedNonGenerative := nonGenerativeModelTypes[modelType]; !typedNonGenerative || modelType == "reranker" {
			for _, c := range embedderCapabilities {
				delete(capabilities, c)
			}
		}
	}
	for _, cap := range nonGenerativeCapabilities {
		if capabilities[cap] {
			delete(capabilities, "text-generation")
