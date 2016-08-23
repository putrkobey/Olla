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
