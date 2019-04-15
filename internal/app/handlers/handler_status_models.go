package handlers

import (
	"context"
	"encoding/json"
	"hash"
	"hash/fnv"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/pkg/format"
)

const (
	// sized for a typical deployment
	maxModelsCapacity      = 128
	maxEndpointNamesLength = 32

	familyUnknown = "unknown"
)

type ModelSummary struct {
	// Additive field, keep JSON contract backward-compatible: absolute form
	// of LastSeen for the dashboard.
	LastSeenAt   *time.Time `json:"last_seen_at,omitempty"`
	Name         string     `json:"name"`
	Type         string     `json:"type,omitempty"`
	Family       string     `json:"family,omitempty"`
	Size         string     `json:"size,omitempty"`
	Params       string     `json:"params,omitempty"`
	Quant        string     `json:"quant,omitempty"`
	LastSeen     string     `json:"last_seen"`
	Endpoints    []string   `json:"endpoints"`
	EndpointIDs  []string   `json:"endpoint_ids,omitempty"`
	Aliases      []string   `json:"aliases,omitempty"`
	Capabilities []string   `json:"capabilities,omitempty"`
}

// endpointNameID pairs an endpoint display name with its stable ID so the two
// can be sorted together; sorting the slices independently would desync the
// positional pairing that the dashboard click-through relies on.
type endpointNameID struct {
	Name string
	ID   string
}

// unifiedModelsGetter is the narrow local interface used to opt-in to alias
// enrichment without importing the concrete registry into the handler. The
// model registry field is typed as domain.ModelRegistry; this is satisfied by
// the unified-memory registry at runtime and intentionally declared here at
// the consumer so the handler depends only on what it calls.
type unifiedModelsGetter interface {
	GetUnifiedModels(ctx context.Context) ([]*domain.UnifiedModel, error)
}

type ModelGroupSummary struct {
	Family     string         `json:"family"`
	Models     []ModelSummary `json:"models"`
	Endpoints  []string       `json:"endpoints"`
	ModelCount int            `json:"model_count"`
}

type ModelStatusResponse struct {
	Timestamp      time.Time           `json:"timestamp"`
	ModelsByFamily map[string][]string `json:"models_by_family"`
	RecentModels   []ModelSummary      `json:"recent_models"`
	ModelGroups    []ModelGroupSummary `json:"model_groups,omitempty"`
	TotalModels    int                 `json:"total_models"`
	TotalFamilies  int                 `json:"total_families"`
	TotalEndpoints int                 `json:"total_endpoints"`
}

func (a *Application) modelsStatusHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	detailed := r.URL.Query().Get("detailed") == queryValueTrue
	groupBy := r.URL.Query().Get("group")

	modelMap, err := a.modelRegistry.GetEndpointModelMap(ctx)
	if err != nil {
		http.Error(w, "Failed to get models", http.StatusInternalServerError)
		return
	}

	endpoints, err := a.repository.GetAll(ctx)
	if err != nil {
		http.Error(w, "Failed to get endpoints", http.StatusInternalServerError)
		return
	}

	endpointNames := make(map[string]string, len(endpoints))
	// IDs derive once from the endpoint set so colliding siblings get the
	// same deterministic disambiguator the /internal/status/endpoints and
	// /internal/status payloads emit, keeping model->endpoint click-through
	// IDs aligned across every payload.
	endpointIDs := buildEndpointIDs(endpoints)
	for _, ep := range endpoints {
		endpointNames[ep.URLString] = ep.Name
	}

	// Best-effort alias enrichment. Degrades to nil (no aliases surfaced)
	// when the registry lacks a unified view; the response still returns 200.
	aliases := a.buildAliasLookup(ctx)

	allModels := a.buildModelSummaries(modelMap, endpointNames, endpointIDs, aliases)

	response := ModelStatusResponse{
		Timestamp:      time.Now(),
		TotalModels:    len(allModels),
		TotalEndpoints: len(modelMap),
		ModelsByFamily: a.groupModelsByFamily(allModels),
		RecentModels:   a.getRecentModels(allModels),
	}

	response.TotalFamilies = len(response.ModelsByFamily)

	if detailed && groupBy == "family" {
		response.ModelGroups = a.groupModelsByFamilyWithDetails(allModels)
	}

	body, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
	writeJSONWithETag(w, r, body, hashModelStatusResponse(&response))
