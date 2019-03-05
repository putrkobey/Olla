package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/adapter/stats"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

func createTestFactoryLogger() logger.StyledLogger {
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	return logger.NewPlainStyledLogger(log)
}

func createTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			RateLimits: config.ServerRateLimits{
				GlobalRequestsPerMinute: 1000,
				PerIPRequestsPerMinute:  100,
				BurstSize:               50,
				HealthRequestsPerMinute: 500,
				CleanupInterval:         5 * time.Minute,
				TrustProxyHeaders:       false,
				TrustedProxyCIDRs:       []string{"127.0.0.0/8", "192.168.0.0/16"},
			},
			RequestLimits: config.ServerRequestLimits{
				MaxBodySize:   10 * 1024 * 1024, // 10MB
				MaxHeaderSize: 64 * 1024,        // 64KB
			},
		},
	}
}

func createTestStatsCollector(log logger.StyledLogger) ports.StatsCollector {
	return stats.NewCollector(log)
}
func createNewTestSecurityServices() (*Services, *Adapters) {
	cfg := createTestConfig()
	return createNewTestSecurityServicesWithConfig(cfg)
}
func createNewTestSecurityServicesWithConfig(cfg *config.Config) (*Services, *Adapters) {
	logger := createTestFactoryLogger()
	statsCollector := createTestStatsCollector(logger)
	return NewSecurityServices(cfg, statsCollector, logger)
}
func TestNewSecurityServices(t *testing.T) {
	services, adapters := createNewTestSecurityServices()

	if services == nil {
		t.Fatal("NewSecurityServices returned nil services")
	}
	if adapters == nil {
		t.Fatal("NewSecurityServices returned nil adapters")
	}

	if services.Chain == nil {
		t.Error("SecurityServices.Chain is nil")
	}
	if services.Metrics == nil {
		t.Error("SecurityServices.Metrics is nil")
	}

	if adapters.RateLimit == nil {
		t.Error("SecurityAdapters.RateLimit is nil")
	}
	if adapters.SizeValidation == nil {
		t.Error("SecurityAdapters.SizeValidation is nil")
	}
	if adapters.Metrics == nil {
		t.Error("SecurityAdapters.Metrics is nil")
	}
	if adapters.Chain == nil {
		t.Error("SecurityAdapters.Chain is nil")
	}
}

func TestSecurityServices_ChainValidators(t *testing.T) {
	services, adapters := createNewTestSecurityServices()
	defer adapters.Stop()

	validators := services.Chain.GetValidators()
	if len(validators) != 2 {
		t.Errorf("Expected 2 validators in chain, got %d", len(validators))
	}

	expectedNames := []string{"rate_limit", "size_limit"}
	for i, validator := range validators {
		if validator.Name() != expectedNames[i] {
			t.Errorf("Expected validator %d to be %q, got %q", i, expectedNames[i], validator.Name())
		}
	}
}

func TestSecurityServices_ChainValidation_AllPass(t *testing.T) {
	services, adapters := createNewTestSecurityServices()
	defer adapters.Stop()

	ctx := context.Background()
	req := ports.SecurityRequest{
		ClientID:      "192.168.1.100",
		Endpoint:      "/api/test",
		Method:        "POST",
		BodySize:      1024,
		HeaderSize:    512,
		Headers:       map[string][]string{"Content-Type": {"application/json"}},
		IsHealthCheck: false,
	}

	result, err := services.Chain.Validate(ctx, req)
	if err != nil {
		t.Fatalf("Chain validation failed: %v", err)
