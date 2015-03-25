package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/converter"
	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// Every handler that serialises the unified view reads *UnifiedModel pointers
// the registry published without a lock. This loops that route table while
// discovery merges a second endpoint, removes it and re-registers, so a handler
// that reads Aliases, SourceEndpoints or Metadata mid-mutation fails under -race
// (or crashes the run with a map fatal without it).
func TestUnifiedViewHandlers_ConcurrentWithRegistration(t *testing.T) {
