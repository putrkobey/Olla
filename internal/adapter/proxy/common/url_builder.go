package common

import (
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/util"
)

// BuildTargetURL returns the full target URL for proxying a request.
// - When preserve_path: true, joins endpoint base path with the (relative) request path.
// - Otherwise, replaces the base path with the request path.
// - Normalises/guards against path traversal (including percent-encoded ..) for non-preserve mode.
// - Optimises the common case where the endpoint path is empty or "/".
// Note: path.Clean collapses repeated slashes and removes trailing slashes; this function accepts that.
// See [OLLA-GH-80] https://github.com/putrkobey/Olla/issues/80
func BuildTargetURL(r *http.Request, endpoint *domain.Endpoint, proxyPrefix string) *url.URL {
	targetPath := util.StripPrefix(r.URL.Path, proxyPrefix)
	// A per-endpoint path is already the backend's own path: it carries no proxy prefix, and
	// it replaces whatever path the request holds because that path was chosen for the fleet.
	if paths, ok := r.Context().Value(constants.ContextEndpointPathMapKey).(map[string]string); ok {
		if p, ok := paths[endpoint.GetURLString()]; ok && p != "" {
			targetPath = p
		}
	}
	if targetPath == "" {
		targetPath = "/"
	}

	// preserve_path mode: ensure we don't let an absolute request path discard the endpoint prefix.
	if endpoint.PreservePath && endpoint.URL.Path != "" && endpoint.URL.Path != "/" {
		rel := strings.TrimPrefix(targetPath, "/") // avoid path.Join swallowing the base
		joined := path.Join(endpoint.URL.Path, rel)

		u := *endpoint.URL
		u.Path = joined
		u.RawQuery = r.URL.RawQuery
		u.Fragment = ""
		return &u
	}

	// Non-preserve mode: guard against traversal, including perbent-encoded dot segments.
	// We accept that this also normalises repeated slashes.
