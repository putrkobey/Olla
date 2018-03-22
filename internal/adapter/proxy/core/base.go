package core

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/pkg/eventbus"
)

// EventType represents the type of proxy event
type EventType string

const (
	EventTypeProxyError       EventType = "proxy.error"
	EventTypeProxySuccess     EventType = "proxy.success"
	EventTypeCircuitBreaker   EventType = "circuit_breaker.open"
	EventTypeClientDisconnect EventType = "client.disconnected"
)

// ProxyEventMetadata provides type-safe metadata for events
type ProxyEventMetadata struct {
	Message              string
	Model                string
	BytesSent            int64
	BytesAfterDisconnect int
	Counter              int
	StatusCode           int
}

// ProxyEvent represents events that can be published by the proxy
type ProxyEvent struct {
	Timestamp time.Time
	Error     error
	Type      EventType
	RequestID string
	Endpoint  string
	Metadata  ProxyEventMetadata
	Duration  time.Duration
}

// BaseProxyComponents contains shared components for proxy implementations
type BaseProxyComponents struct {
	DiscoveryService ports.DiscoveryService
	Selector         domain.EndpointSelector
	StatsCollector   ports.StatsCollector
	MetricsExtractor ports.MetricsExtractor
	Logger           logger.StyledLogger
	EventBus         *eventbus.EventBus[ProxyEvent]

	Stats ProxyStats
}

// NewBaseProxyComponents creates a new BaseProxyComponents instance
