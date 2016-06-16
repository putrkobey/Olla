package sherpa

import (
	"github.com/putrkobey/Olla/internal/adapter/proxy/config"
	"github.com/putrkobey/Olla/internal/core/constants"
)

// Configuration holds Sherpa proxy settings
type Configuration struct {
	config.BaseProxyConfig
}

// GetProxyProfile overrides base to default to streaming profile for Sherpa
func (c *Configuration) GetProxyProfile() string {
	if c.Profile == "" {
		// Majority of users will use streaming profile
