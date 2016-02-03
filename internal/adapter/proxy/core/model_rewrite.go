package core

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// CanonicalModel records that a client's model name resolved to a different native
// name without being rewritten, as when "all-minilm" is served as "all-minilm:latest".
// The forwarded body keeps the client's spelling; this only names the model for stats.
//
// Targets does the same for an alias: it maps the model name an alias rewrites to
// (what the backend receives) onto its native name, so alias and direct requests for
// one model record under one stats key.
type CanonicalModel struct {
	Targets   map[string]string
	Requested string
	Native    string
}

// ResolvedModelName returns the model name actually dispatched to endpoint:
// the alias map's per-endpoint entry if the request was alias-resolved, the
// native name when a bare Ollama name was resolved to it, otherwise
// requestedModel unchanged (the common case, where the requested
// name already is the resolved name). Callers should use this - not the raw
// requested/alias name - when recording model-level stats
// (StatsCollector.RecordModelRequest via RecordSuccess/RecordFailure), so
// /internal/stats/models aggregates under the model the backend actually
// served rather than fragmenting across whatever alias each client happened
// to send.
func ResolvedModelName(ctx context.Context, endpoint *domain.Endpoint, requestedModel string) string {
	if aliasMap, ok := ctx.Value(constants.ContextModelAliasMapKey).(map[string]string); ok && endpoint != nil {
		if actualModel, ok := aliasMap[endpoint.GetURLString()]; ok {
			if canonical, ok := ctx.Value(constants.ContextCanonicalModelKey).(CanonicalModel); ok {
				if native, ok := canonical.Targets[actualModel]; ok {
					return native
				}
			}
			return actualModel
		}
	}
	if canonical, ok := ctx.Value(constants.ContextCanonicalModelKey).(CanonicalModel); ok && canonical.Native != "" && canonical.Requested == requestedModel {
		return canonical.Native
	}
	return requestedModel
}

// RewriteModelForAlias checks whether the request uses a model alias and, if the
// selected endpoint has an alias mapping, rewrites the "model" field in the JSON
// request body to the actual model name the backend expects. The original request
// body is replaced with the rewritten content.
//
// The replacement is done via targeted byte-splice to preserve the original
// JSON formatting, key ordering, and whitespace — making the result byte-identical
// to the input except for the model name change.
//
// This is a no-op when:
//   - there is no alias map in the context
//   - the selected endpoint is not in the alias map
//   - the request has no body or is not JSON
//   - the body does not contain a top-level "model" field
func RewriteModelForAlias(ctx context.Context, r *http.Request, endpoint *domain.Endpoint) {
	aliasMap, ok := ctx.Value(constants.ContextModelAliasMapKey).(map[string]string)
	if !ok || len(aliasMap) == 0 {
		return
	}

	actualModel, ok := aliasMap[endpoint.GetURLString()]
	if !ok {
		return
	}

	if r.Body == nil || r.ContentLength == 0 {
		return
	}

	// Read the current body
	bodyBytes, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil || len(bodyBytes) == 0 {
		// restore original body on error
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return
	}

	// Perform targeted replacement of the model field value
	rewritten := rewriteModelField(bodyBytes, actualModel)

	// Replace the request body with the rewritten content
	r.Body = io.NopCloser(bytes.NewReader(rewritten))
	r.ContentLength = int64(len(rewritten))
}

// rewriteModelField performs a targeted replacement of the top-level "model" field's
// string value in a JSON body. Uses a JSON token scanner to find the precise byte
