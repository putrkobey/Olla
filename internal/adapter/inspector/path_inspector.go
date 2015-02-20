package inspector

import (
	"context"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

type PathInspector struct {
	profileFactory *profile.Factory
	logger         logger.StyledLogger
	pathToProfiles map[string][]string
	// profilePaths keeps each profile's paths in declaration order, which pathToProfiles
	// cannot: ResolveTargetPath needs "first declared" to be stable.
	profilePaths map[string][]string
}

const (
	PathInspectorName = "path"
)

// pathCapability sets the capability a request path implies when its body
// cannot: TEI's native /rerank, /predict and /embed bodies carry no model field,
// so without this a model-less request lands on whichever instance the balancer
// picks, and TEI answers 424 when that instance serves the wrong kind.
type pathCapability struct {
	apply  func(*domain.ModelCapabilities)
	suffix string
}

// Matching rules (see pathMatches): the single-segment names TEI serves
// (/rerank, /predict, /embed...) only mean those things as the whole path, so
// they must not claim /api/rerank or Ollama's /api/embed. The versioned
// vLLM-style variants are listed in full so they keep matching behind a prefix.
var pathCapabilities = []pathCapability{
	{suffix: "/rerank", apply: func(c *domain.ModelCapabilities) { c.Rerank = true }},
	{suffix: "/v1/rerank", apply: func(c *domain.ModelCapabilities) { c.Rerank = true }},
	{suffix: "/v2/rerank", apply: func(c *domain.ModelCapabilities) { c.Rerank = true }},
	// OpenAI-style embeddings. The body's `input` field is the primary signal, but it is only
	// readable when the body is; the path keeps embeddings requests off chat-only hosts either way.
	{suffix: "/v1/embeddings", apply: func(c *domain.ModelCapabilities) { c.Embeddings = true }},
	{suffix: "/predict", apply: func(c *domain.ModelCapabilities) { c.Classification = true }},
	{suffix: "/embed", apply: func(c *domain.ModelCapabilities) { c.Embeddings = true }},
	{suffix: "/embed_all", apply: func(c *domain.ModelCapabilities) { c.Embeddings = true }},
	{suffix: "/embed_sparse", apply: func(c *domain.ModelCapabilities) { c.Embeddings = true }},
	{suffix: "/similarity", apply: func(c *domain.ModelCapabilities) { c.Embeddings = true }},
}

// pathMatches reports whether a request path belongs to a profile path. They
// match when equal, or when the request path carries extra leading segments (a
// mount prefix the route did not strip, such as the translators' /olla/anthropic)
// and the profile path has two or more segments. A single segment such as
// /embed, /health or /version is too common a name to claim every deeper path
// that happens to end with it. A suffix always starts at a '/', so it can only
// begin on a segment boundary.
func pathMatches(requestPath, profilePath string) bool {
	if requestPath == profilePath {
		return true
	}
	return strings.Count(profilePath, "/") >= 2 && strings.HasPrefix(profilePath, "/") && strings.HasSuffix(requestPath, profilePath)
}

// capabilitiesForPath returns the capabilities a path implies, or nil.
func capabilitiesForPath(path string) *domain.ModelCapabilities {
	var caps *domain.ModelCapabilities
	for _, pc := range pathCapabilities {
		if pathMatches(path, pc.suffix) {
			if caps == nil {
				caps = &domain.ModelCapabilities{}
			}
			pc.apply(caps)
		}
	}
	return caps
}

func NewPathInspector(profileFactory *profile.Factory, logger logger.StyledLogger) *PathInspector {
	inspector := &PathInspector{
		profileFactory: profileFactory,
		logger:         logger,
		pathToProfiles: make(map[string][]string),
		profilePaths:   make(map[string][]string),
	}

	inspector.buildPathMap()
	return inspector
}

// buildPathMap builds a map of paths to profiles for fast lookups.
func (pi *PathInspector) buildPathMap() {
	if pi.profileFactory == nil {
