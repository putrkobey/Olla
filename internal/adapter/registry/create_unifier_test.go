package registry

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/unifier"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

func TestCreateUnifier_UsesCallerFallbackAndLogsOnce(t *testing.T) {
	t.Parallel()

	cfg := unifier.DefaultConfig()
	for name, create := range map[string]func() (ports.ModelUnifier, error){
