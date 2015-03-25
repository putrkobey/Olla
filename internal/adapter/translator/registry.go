package translator

import (
	"fmt"
	"sort"
	"sync"

	"github.com/putrkobey/Olla/internal/logger"
)

// manages registered translators, scales to unlimited formats without hardcoded deps
type Registry struct {
	translators map[string]RequestTranslator
	logger      logger.StyledLogger
	mu          sync.RWMutex
}

// NewRegistry creates a new translator registry instance
func NewRegistry(logger logger.StyledLogger) *Registry {
	return &Registry{
		translators: make(map[string]RequestTranslator),
		logger:      logger,
	}
}

// Register adds a translator to the registry
// uses format name (from Name()) to keep things consistent
func (r *Registry) Register(name string, translator RequestTranslator) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if translator == nil {
		r.logger.Error("Attempted to register nil translator, ignoring")
		return
	}

	if name == "" {
		r.logger.Warn("Attempted to register translator with empty name, using translator.Name() instead")
		name = translator.Name()
		if name == "" {
			r.logger.Error("Translator has empty name, cannot register")
			return
