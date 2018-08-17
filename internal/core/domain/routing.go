package domain

import "github.com/puzpuzpuz/xsync/v4"

const (
	InspectionMetaPathSupport = "path_support"
)

// RequestType identifies what kind of LLM request this is
type RequestType int

const (
	RequestTypeUnknown RequestType = iota
	RequestTypeChat
	RequestTypeCompletion
	RequestTypeEmbedding
	RequestTypeImage
)

type RequestProfile struct {
	InspectionMeta *xsync.Map[string, interface{}]

	// Rich request metadata for intelligent routing
	ModelCapabilities    *ModelCapabilities    // What the request needs
	ResourceRequirements *ResourceRequirements // Resources needed
	RoutingDecision      *ModelRoutingDecision // Routing strategy decision
	Path                 string
	ModelName            string
	SupportedBy          []string
	// ProviderScope is the endpoint types a provider route (/olla/<provider>/) is confined to,
	// empty on /olla/proxy/. SupportedBy alone can't express it: a path shared with another
	// provider (/v1/embeddings) lists both, but provider filtering has already dropped one.
	ProviderScope []string

	RequestType     RequestType // Chat, completion, embedding, etc.
	EstimatedTokens int         // For capacity planning

	RequiresFunctionCall bool
	RequiresVision       bool
