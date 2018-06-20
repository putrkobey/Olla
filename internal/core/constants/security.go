package constants

const (
	ViolationRateLimit = "rate_limit"
	ViolationSizeLimit = "size_limit"

	// Rejection reasons carried on ports.SecurityResult. Callers and tests compare
	// these, so they are constants rather than literals repeated at each return
	// site: a reason that drifts out of sync fails silently, since a comparison
	// that never matches looks the same as a rejection that never happened.
	ReasonRateLimitExceeded       = "Rate limit exceeded"
	ReasonGlobalRateLimitExceeded = "Global rate limit exceeded"
