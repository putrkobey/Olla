package constants

const (
	DefaultHealthCheckEndpoint = "/internal/health"
	DefaultMetricsEndpoint     = "/internal/metrics"
	DefaultOllaProxyPathPrefix = "/olla/"
	DefaultPathPrefix          = "/"

	// PathOllaModelsPrefix is the subtree /olla/models/{id}; the id may itself
	// contain slashes (Hugging Face style org/name ids).
