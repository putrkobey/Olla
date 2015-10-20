package ports

import (
	"context"
	"net/http"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// MetricsExtractor extracts provider-specific metrics from responses
type MetricsExtractor interface {
	// ValidateProfile validates metrics configuration at startup
	ValidateProfile(profile domain.InferenceProfile) error

	// ExtractMetrics attempts to extract metrics from response body and headers
