package constants

// Routing reason constants used across all routing strategies
// these define the reasons for routing decisions that are returned
// in the X-Olla-Routing-Reason header and determine HTTP status codes
const (
	// Model found scenarios (200 OK)
	RoutingReasonModelFound          = "model_found"
	RoutingReasonModelFoundNoRefresh = "model_found_no_refresh"

