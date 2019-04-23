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
