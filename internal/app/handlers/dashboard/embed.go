package dashboard

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"
)

// serveEpoch is a stable, non-zero timestamp used as the Last-Modified value
// for embedded assets. The embed FS itself carries no mtime (files report the
// zero time), so http.ServeContent would otherwise emit no Last-Modified and
// skip If-Modified-Since handling entirely, defeating conditional GET for
// clients that prefer that header. Using process start time is a deliberate
// proxy: imperfect as a content timestamp, but it shifts only across restarts
// and the strong ETag remains the authoritative freshness signal, so a changed
// serveEpoch never causes a wrong 200 (only a missed 304 on the first request
// after restart).
var serveEpoch = time.Now()

// distRoot holds the embedded dashboard SPA bundle. It is regenerated from
// web/dashboard/ by `make build-web` and is gitignored: only a .gitkeep
// sentinel stays committed so the embed is non-empty and a fresh checkout
// compiles with `go build ./...` without Node installed. Release targets run
// build-web before compiling, so shipped binaries carry the real SPA; a binary
// built without that step serves a loud not-built response (see Handler) rather
// than a silent placeholder.
//
//go:embed all:dist
var embeddedFS embed.FS

// notBuiltWarn ensures the "dashboard not built" warning fires once per
// process. Handler() is invoked during route registration at startup, so the
// sync.Once collapses what is already a single call rather than request-time
// repeats.
var notBuiltWarn sync.Once

const (
	distRoot  = "dist"
	indexFile = "index.html"
)

// mimeOverrides patches extensions Go's registry gets wrong or omits. Without
// these, .js falls back to text/plain on some hosts and .woff2 is unknown,
// forcing content-sniffing on every response.
var mimeOverrides = map[string]string{
	".html":  "text/html; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".mjs":   "text/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".svg":   "image/svg+xml",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".png":   "image/png",
	".ico":   "image/x-icon",
	".json":  "application/json; charset=utf-8",
	".webp":  "image/webp",
}

// hashedAsset matches Vite's content-hashed output names, e.g.
// assets/index-AbC123.js or index-9f3b2c1d.css. Detecting the hash lets us
// advertise long-lived immutable caching, since the URL itself changes whenever
// the content does. Vite uses an 8-char hash by default; be permissive on
// length to survive future config changes.
var hashedAsset = regexp.MustCompile(`-[A-Za-z0-9_-]{6,}\.(js|mjs|css|woff2?|png|svg|ico|webp|json)$`)

// Handler serves the embedded dashboard. It is the value passed to
// RegisterRoutes so the access middleware applies uniformly to /dashboard/
// and every sub-path. The handler is safe for concurrent use; the embedded FS
// is read-only and the per-file ETag is derived deterministically from bytes.
func Handler() http.Handler {
	sub, err := fs.Sub(embeddedFS, distRoot)
	if err != nil {
		// Should never happen: distRoot ships populated via go:embed.
		// Guard anyway so a future rename cannot panic at request time.
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "dashboard assets unavailable", http.StatusInternalServerError)
		})
	}
	return dashboardHandler(sub)
}

// AssetsBuilt reports whether the embedded dist carries a built index.html.
// It is the same check dashboardHandler uses to pick between the SPA handler
// and the not-built 503 handler, surfaced separately so startup logging can
// distinguish a binary that carries real assets from one carrying only the
// .gitkeep sentinel (e.g. a binary produced by `go install` without
// `make build-web`). A false return is not an error: the route still mounts
// and answers 503 with a clear message, the binary just should not claim the
// dashboard is "ready".
func AssetsBuilt() bool {
	sub, err := fs.Sub(embeddedFS, distRoot)
	if err != nil {
		return false
	}
	return assetsBuiltIn(sub)
}

// assetsBuiltIn is the FS-level check both Handler (for route selection) and
// AssetsBuilt (for startup logging) consult. Split out so the two call sites
// cannot drift on what "built" means, and so tests can drive the sentinel-only
// and populated branches via a synthetic fstest.MapFS without disturbing the
// package-level embed.
func assetsBuiltIn(root fs.FS) bool {
	info, err := fs.Stat(root, indexFile)
	return err == nil && !info.IsDir()
}

// dashboardHandler returns the SPA handler when the embedded dist carries a
// built index.html. If only the .gitkeep sentinel is present (the binary was
// compiled without `make build-web`), it returns a not-built handler that logs
// a startup warning once and serves 503, so a stale or unbuilt dashboard is
// obvious to the operator instead of silently serving a placeholder shell.
func dashboardHandler(root fs.FS) http.Handler {
	if !assetsBuiltIn(root) {
		return notBuiltHandler()
	}
	h, err := newSPAHandler(root)
	if err != nil {
