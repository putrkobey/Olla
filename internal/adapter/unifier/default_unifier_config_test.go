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
	cfg := DefaultConfig()
	cfg.CleanupInterval = 10 * time.Minute
	cfg.ModelTTL = 48 * time.Hour

	u, ok := NewDefaultUnifierWithConfig(cfg).(*DefaultUnifier)
	require.True(t, ok)
	assert.Equal(t, 10*time.Minute, u.store.cleanupInterval)
	assert.Equal(t, 48*time.Hour, u.staleThreshold)
}

func TestNewDefaultUnifierWithConfig_ZeroValuesFallBackToDefaults(t *testing.T) {
	u, ok := NewDefaultUnifierWithConfig(Config{}).(*DefaultUnifier)
	require.True(t, ok)
	assert.Equal(t, 5*time.Minute, u.store.cleanupInterval)
	assert.Equal(t, 24*time.Hour, u.staleThreshold)
}
