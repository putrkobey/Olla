package handlers

import (
	"encoding/json"
	"net/http"
	"runtime"

	"github.com/putrkobey/Olla/internal/core/constants"

	"github.com/putrkobey/Olla/internal/version"
)

type VersionResponse struct {
	API                      APIInfo           `json:"api"`
	Links                    map[string]string `json:"links"`
	Build                    BuildInfo         `json:"build"`
	Name                     string            `json:"name"`
	Version                  string            `json:"version"`
	Edition                  string            `json:"edition"`
	Description              string            `json:"description"`
	Capabilities             []string          `json:"capabilities"`
	ExperimentalCapabilities []string          `json:"capabilities_experimental"`
	SupportedBackends        []string          `json:"supported_backends"`
}

type BuildInfo struct {
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

type APIInfo struct {
	Endpoints map[string]string `json:"endpoints"`
	Version   string            `json:"version"`
}

