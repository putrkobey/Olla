package discovery

import "github.com/putrkobey/Olla/internal/core/domain"

// TestStaticEndpointRepository provides test-specific repository extensions
type TestStaticEndpointRepository struct {
	*StaticEndpointRepository
}

// NewTestStaticEndpointRepository creates a repository with test helpers
func NewTestStaticEndpointRepository() *TestStaticEndpointRepository {
	return &TestStaticEndpointRepository{
		StaticEndpointRepository: NewStaticEndpointRepository(),
