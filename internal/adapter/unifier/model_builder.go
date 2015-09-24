package unifier

import (
	"fmt"
	"strings"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// ModelBuilder simplifies the creation of unified models
type ModelBuilder struct {
	model    *domain.UnifiedModel
	metadata map[string]interface{}
}

// NewModelBuilder creates a new model builder
func NewModelBuilder() *ModelBuilder {
	return &ModelBuilder{
		model: &domain.UnifiedModel{
			Aliases:         []domain.AliasEntry{},
			SourceEndpoints: []domain.SourceEndpoint{},
			Capabilities:    []string{},
			LastSeen:        time.Now(),
		},
		metadata: make(map[string]interface{}),
	}
}

// WithID sets the model ID
func (b *ModelBuilder) WithID(id string) *ModelBuilder {
	b.model.ID = id
	return b
}

// WithFamily sets the model family and variant
func (b *ModelBuilder) WithFamily(family, variant string) *ModelBuilder {
	b.model.Family = family
	b.model.Variant = variant
	return b
}

// WithParameters sets the parameter size and count
func (b *ModelBuilder) WithParameters(size string, count int64) *ModelBuilder {
	b.model.ParameterSize = size
	b.model.ParameterCount = count
	return b
}

// WithQuantization sets the quantization level
func (b *ModelBuilder) WithQuantization(quantization string) *ModelBuilder {
	b.model.Quantization = quantization
	return b
}

// WithFormat sets the model format
func (b *ModelBuilder) WithFormat(format string) *ModelBuilder {
	b.model.Format = format
	return b
}

// WithContextLength sets the max context length
func (b *ModelBuilder) WithContextLength(length int64) *ModelBuilder {
	if length > 0 {
		b.model.MaxContextLength = &length
	}
	return b
}

// WithDiskSize sets the disk size
func (b *ModelBuilder) WithDiskSize(size int64) *ModelBuilder {
	b.model.DiskSize = size
	return b
}

// AddAlias adds an alias entry
func (b *ModelBuilder) AddAlias(name, source string) *ModelBuilder {
	b.model.Aliases = append(b.model.Aliases, domain.AliasEntry{
		Name:   name,
		Source: source,
	})
	return b
}

// AddSourceEndpoint adds a source endpoint
func (b *ModelBuilder) AddSourceEndpoint(endpoint domain.SourceEndpoint) *ModelBuilder {
	b.model.SourceEndpoints = append(b.model.SourceEndpoints, endpoint)
	return b
}

// AddCapability adds a capability
func (b *ModelBuilder) AddCapability(capability string) *ModelBuilder {
	b.model.Capabilities = append(b.model.Capabilities, capability)
	return b
}

// AddCapabilities adds multiple capabilities
func (b *ModelBuilder) AddCapabilities(capabilities []string) *ModelBuilder {
	b.model.Capabilities = append(b.model.Capabilities, capabilities...)
	return b
}

// WithMetadata adds a metadata entry
func (b *ModelBuilder) WithMetadata(key string, value interface{}) *ModelBuilder {
	b.metadata[key] = value
	return b
}

// WithDigest sets the digest in metadata
func (b *ModelBuilder) WithDigest(digest string) *ModelBuilder {
	if digest != "" {
		b.metadata["digest"] = digest
	}
	return b
}

// WithPlatform sets the platform in metadata
func (b *ModelBuilder) WithPlatform(platform string) *ModelBuilder {
	b.metadata["platform"] = platform
	return b
}

// WithPublisher sets the publisher in metadata
func (b *ModelBuilder) WithPublisher(publisher string) *ModelBuilder {
	if publisher != "" {
		b.metadata["publisher"] = publisher
	}
	return b
}

// WithConfidence sets the metadata confidence score
func (b *ModelBuilder) WithConfidence(confidence float64) *ModelBuilder {
	b.metadata["metadata_confidence"] = confidence
	return b
}

// Build creates the final unified model
func (b *ModelBuilder) Build() *domain.UnifiedModel {
	b.model.Metadata = b.metadata
	return b.model
}

// ModelExtractor extracts model attributes from various sources
type ModelExtractor struct{}

