package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// Ollama's "tools" is stored as function_calling; asking for either spelling finds it.
func TestGetModelsByCapability_FunctionCallingAliases(t *testing.T) {
	t.Parallel()

	reg := createTestUnifiedRegistry()
