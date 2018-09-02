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
}

func (a *Application) buildModelSummaries(
	modelMap map[string]*domain.EndpointModels,
	endpointNames map[string]string,
	endpointIDs map[string]string,
	aliases map[string][]string,
) []ModelSummary {
	uniqueModels := make(map[string]*ModelSummary, maxModelsCapacity)

	// Iterate endpoint URLs in sorted order, not map order. modelMap's range
	// order is randomised per call, and the multi-endpoint merge below seeds a
	// model's scalar metadata (Family/Type/Size/Params/Quant/Capabilities)
	// from the first endpoint visited. Without a stable visitation order the
	// same fleet can flip a shared model between rich and empty metadata
	// across polls, churning the ETag despite no real state change. Lowest
	// sorted URL is the deterministic tie-breaker when two endpoints carry
	// conflicting non-empty values for the same field.
	endpointURLs := make([]string, 0, len(modelMap))
	for endpointURL := range modelMap {
		endpointURLs = append(endpointURLs, endpointURL)
	}
	sort.Strings(endpointURLs)

	for _, endpointURL := range endpointURLs {
		endpointModels := modelMap[endpointURL]
		endpointName := endpointNames[endpointURL]
		if endpointName == "" {
			// An endpoint with no configured name, or a stale model-map
			// entry with no repository match, falls back to the raw URL, which
			// can carry credentials in a query string (e.g. ?api_key=secret).
			// Sanitise it the same way sanitiseDisplayURL does for the other
			// status handlers before it ever reaches a response.
			endpointName = sanitiseDisplayURL(endpointURL)
		}
		// The stable ID mirrors the name-resolution path so the two arrays
		// line up positionally. For URLs without an explicit ID entry (a
		// stale model-map key with no repository match) the base hash of the
		// sanitised URL is used, matching the fallback the name path takes.
		// This path has no sibling context, so it cannot disambiguate
		// collisions - the trade-off for not silently dropping the model.
		endpointID := endpointIDs[endpointURL]
		if endpointID == "" {
			endpointID = stableEndpointID(endpointURL)
		}

		for _, model := range endpointModels.Models {
			existing, exists := uniqueModels[model.Name]
			if !exists {
				uniqueModels[model.Name] = a.createModelSummary(model, []string{endpointName}, []string{endpointID})
				continue
			}

			existing.Endpoints = append(existing.Endpoints, endpointName)
			existing.EndpointIDs = append(existing.EndpointIDs, endpointID)

			// A model hosted on several endpoints must deterministically
			// report the newest last-seen timestamp, not whichever
			// endpoint the map iteration happened to visit last.
			if newerModelTimestamp(model.LastSeen, existing.LastSeenAt) {
				existing.LastSeen = format.TimeAgo(model.LastSeen)
				ls := model.LastSeen
				existing.LastSeenAt = &ls
			}

			// Merge scalar metadata prefer-non-empty: an endpoint that knows
			// the model's family/quant/params/etc. wins over one that reported
			// only minimal details. On two conflicting non-empty values the
			// sorted-first endpoint (the one already on `existing`) wins - a
			// purely deterministic rule, since "richer" is not a total order
			// across heterogeneous backends. Only empty fields on the existing
			// summary are overwritten.
			a.mergeModelSummaryFields(existing, model)
		}
	}

	// Attach alias enrichment once per model. The lookup key is the model's
	// raw name; the value already excludes that name from its own sibling set.
	for name, summary := range uniqueModels {
		if len(summary.Aliases) > 0 {
			continue
		}
		if al, ok := aliases[name]; ok && len(al) > 0 {
			// Copy so a downstream defensive sort can't mutate the shared
			// lookup slice between summaries on the same response.
			summary.Aliases = append([]string(nil), al...)
		}
	}

	summaries := make([]ModelSummary, 0, len(uniqueModels))
	for _, summary := range uniqueModels {
		sortModelSummaryEndpoints(summary)
		if len(summary.Aliases) > 0 {
			sort.Strings(summary.Aliases)
		}
		summaries = append(summaries, *summary)
	}

	return summaries
}

// mergeModelSummaryFields backfills empty scalar metadata on an existing
// multi-endpoint summary from a freshly seen endpoint's model. Non-empty
// values on the existing summary are never overwritten: sorted-first wins on
// conflicting non-empty values (a deterministic rule documented above the
// merge loop in buildModelSummaries). Size uses the rendered Bytes string
// rather than the raw int64 because ModelSummary only carries the formatted
// form; on conflict the existing rendering is preserved, and on empty the
// candidate's rendering is applied.
func (a *Application) mergeModelSummaryFields(existing *ModelSummary, model *domain.ModelInfo) {
	if existing.Type == "" {
		existing.Type = modelInfoKind(model)
	}
	if model.Details != nil {
		if existing.Family == "" && model.Details.Family != nil {
			existing.Family = *model.Details.Family
		}
		if existing.Params == "" && model.Details.ParameterSize != nil {
			existing.Params = *model.Details.ParameterSize
		}
		if existing.Quant == "" && model.Details.QuantizationLevel != nil {
			existing.Quant = *model.Details.QuantizationLevel
		}
		// Capabilities are inferred from Details. On an empty existing set,
		// the candidate's details supply them; on a non-empty existing set
		// they are preserved (sorted-first wins).
		if len(existing.Capabilities) == 0 {
			if caps := a.inferCapabilities(model.Details); len(caps) > 0 {
				existing.Capabilities = caps
			}
		}
	}
	// Size renders to a human-readable string; populate from the candidate
	// only when the existing summary has none.
	if existing.Size == "" && model.Size > 0 {
		existing.Size = format.Bytes(uint64(model.Size))
	}
}

// sortModelSummaryEndpoints sorts Endpoints and EndpointIDs together by name,
// keeping the positional pairing the dashboard click-through relies on. Two
// independent sorts would desync them whenever two endpoints share a display
// name, so a single slice of pairs is sorted and split back.
//
// Comparator is Name primary, EndpointID secondary - mirroring
// buildUnifiedEndpoints and endpointsStatusHandler. Name alone is unstable
// when two distinct endpoints share a display name (a legitimate config);
// the ID tie-break keeps their relative order stable across polls so the
// ETag does not churn purely from pair swapping.
//
// buildModelSummaries always appends to both slices together, so lengths
// should never differ in practice - but the invariant is only assumed, not
// enforced by the type system, so this guards it rather than indexing out of
// range. On a mismatch the sort still runs over the common (shorter) prefix:
// leaving the pair entirely as-built would mean map iteration order - which
// is randomised per run - decides the response order, churning the ETag on
// every poll even though nothing actually changed.
func sortModelSummaryEndpoints(summary *ModelSummary) {
	n := len(summary.Endpoints)
	if len(summary.EndpointIDs) < n {
		n = len(summary.EndpointIDs)
	}
	pairs := make([]endpointNameID, n)
	for i := range n {
		pairs[i] = endpointNameID{Name: summary.Endpoints[i], ID: summary.EndpointIDs[i]}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Name != pairs[j].Name {
			return pairs[i].Name < pairs[j].Name
		}
		return pairs[i].ID < pairs[j].ID
	})
	for i := range n {
		summary.Endpoints[i] = pairs[i].Name
		summary.EndpointIDs[i] = pairs[i].ID
	}
}

// buildAliasLookup resolves every alias NAME (plus the canonical unified ID
// when distinct) to its deduplicated, sorted set of sibling identifiers. The
// dashboard looks a model's raw Name up here to render "also known as" tags.
//
// Returns nil when the registry does not expose unified models or the call
// fails; the caller degrades silently (no aliases surfaced) rather than
// failing the status response. Aliases are best-effort enrichment only.
func (a *Application) buildAliasLookup(ctx context.Context) map[string][]string {
	getter, ok := a.modelRegistry.(unifiedModelsGetter)
	if !ok {
		return nil
	}
	unified, err := getter.GetUnifiedModels(ctx)
	if err != nil {
		return nil
	}
	if len(unified) == 0 {
		return nil
	}

	lookup := make(map[string][]string, len(unified)*4)
	for _, um := range unified {
		if um == nil {
			continue
		}
		// Gather every distinct identifier on this model: each alias name
		// plus the canonical ID when it is not already among them.
		seen := make(map[string]struct{}, len(um.Aliases)+1)
		for _, alias := range um.Aliases {
			if alias.Name != "" {
				seen[alias.Name] = struct{}{}
			}
		}
		if um.ID != "" {
			seen[um.ID] = struct{}{}
		}
		if len(seen) < 2 {
			// A model with a single identifier has no siblings to surface.
			continue
		}
		all := make([]string, 0, len(seen))
		for name := range seen {
			all = append(all, name)
		}
		sort.Strings(all)
		// Each identifier resolves to every other identifier on the model.
		// The queried name is excluded from its own result by construction.
		for _, query := range all {
			siblings := make([]string, 0, len(all)-1)
			for _, other := range all {
				if other != query {
					siblings = append(siblings, other)
				}
			}
			if existing, ok := lookup[query]; ok {
				// Same alias name surfacing on two unified models is an
				// ambiguity; merge deterministically so the ETag stays
				// stable across polls rather than swapping between writes.
				merged := mergeStringSets(existing, siblings)
				sort.Strings(merged)
				lookup[query] = merged
			} else {
				lookup[query] = siblings
			}
		}
	}
	return lookup
}

func mergeStringSets(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	for _, s := range b {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
