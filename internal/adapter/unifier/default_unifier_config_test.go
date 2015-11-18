package unifier

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/putrkobey/Olla/internal/logger"
)

// The default unifier must honour an operator-supplied cleanup interval and TTL.
// Previously CreateWithConfig threw the config away and hard-coded 5m/24h, so
// model_registry.unification.cleanup_interval silently had no effect.
func TestNewDefaultUnifierWithConfig_HonoursCleanupAndTTL(t *testing.T) {
