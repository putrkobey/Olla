package core

import (
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/version"
)

var (
	proxiedByHeader string
	viaHeader       string
)

func init() {
	proxiedByHeader = version.Name + "/" + version.Version
	viaHeader = "1.1 " + version.ShortName + "/" + version.Version
}

// GetProxiedByHeader returns the X-Proxied-By header value
func GetProxiedByHeader() string {
	return proxiedByHeader
}

// GetViaHeader returns the Via header value
func GetViaHeader() string {
	return viaHeader
}

// CopyHeaders copies headers from originalReq to proxyReq with proper handling.
// endpoint carries the per-endpoint auth and custom header config applied after
// the client headers are copied and the sensitive strip list runs.
func CopyHeaders(proxyReq, originalReq *http.Request, endpoint *domain.Endpoint) {
	// Pre-size based on source to avoid rehashing
	if proxyReq.Header == nil {
		proxyReq.Header = make(http.Header, len(originalReq.Header))
	}
	for header, values := range originalReq.Header {
		// Skip hop-by-hop headers as per RFC 2616 section 13.5.1
		// these headers are connection-specific and shouldn't be forwarded
		if isHopByHopHeader(header) {
			continue
		}

		// SECURITY: Filter sensitive headers to prevent credential leakage
		// TODO: we should consider a more copmrehensive security policy / technique here
		normalisedHeader := http.CanonicalHeaderKey(header)
		if normalisedHeader == constants.HeaderAuthorization ||
			normalisedHeader == constants.HeaderCookie ||
			normalisedHeader == constants.HeaderXAPIKey ||
			normalisedHeader == constants.HeaderXAuthToken ||
			normalisedHeader == constants.HeaderProxyAuthorization {
			continue
