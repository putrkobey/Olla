package inspector

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/putrkobey/Olla/internal/core/constants"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/pkg/pool"
)

const (
	BodyInspectorName = "body"
	MaxBodySize       = 1024 * 1024 // 1MB max body size for full body inspection (capabilities etc.)

	// modelScanSize is the hard ceiling on bytes read when scanning for the top-level "model"
	// field in large requests (e.g. vision requests with base64 image payloads). The scan
	// stops as soon as the field is found, so for the common case where "model" appears near
	// the start only a handful of bytes are consumed. The ceiling exists to bound memory use
	// when "model" appears after large fields (e.g. a multi-megabyte "messages" array);
	// 8 MB accommodates typical vision payloads while staying well within practical limits.
	// Requests where "model" is absent or appears beyond this ceiling will not be routed by
	// model name - capability-based or default routing applies instead.
	modelScanSize = 8 * 1024 * 1024

	// peekWindow is the span of leading whitespace examined per step when deciding whether a body is
	// a JSON object; it is also the bufio size, so it is the only per-request allocation added.
	peekWindow = 256

	// maxLeadingWhitespace caps how much leading whitespace we look past before giving up.
	maxLeadingWhitespace = 4096
)

type modelRequest struct {
	Model string `json:"model"`
}

// BodyInspector extracts model names from request bodies
type BodyInspector struct {
	logger      logger.StyledLogger
	bufferPool  *pool.Pool[*bytes.Buffer]
	maxBodySize int64
}

func NewBodyInspector(logger logger.StyledLogger) (*BodyInspector, error) {
	bufPool, err := pool.NewLitePool(func() *bytes.Buffer {
		return new(bytes.Buffer)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create buffer pool: %w", err)
	}

	return &BodyInspector{
		logger:      logger,
		maxBodySize: MaxBodySize,
		bufferPool:  bufPool,
	}, nil
}

func (bi *BodyInspector) Name() string {
	return BodyInspectorName
}

func (bi *BodyInspector) Inspect(ctx context.Context, r *http.Request, profile *domain.RequestProfile) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}

	// The body decides, not the header: curl -d sends form-urlencoded, fetch() with a string
	// body sends text/plain and plenty of scripts send nothing, yet all carry JSON. Skipping on
	// Content-Type left those requests model-less, so they were routed to any compatible host.
	// Multipart and encoded bodies are opaque to us (we never decompress), and anything that
	// does not open with '{' cannot hold a top-level model field.
	if bi.opaqueBody(r) {
		return nil
	}
	if !bi.looksLikeJSONObject(r) {
		return nil
	}

	// Large requests (e.g. vision with base64 images) still need the model name for routing,
	// but buffering the full body just to parse JSON is wasteful and would OOM for
	// multi-megabyte payloads. Instead stream-scan up to modelScanSize bytes (see inspectLarge).
	if r.ContentLength > bi.maxBodySize {
		bi.inspectLarge(r, profile)
		return nil
	}

	buffer, err := bi.bufferPool.Get()
	if err != nil {
		return fmt.Errorf("body inspector: buffer pool exhausted: %w", err)
	}
	defer func() {
		buffer.Reset()
		bi.bufferPool.Put(buffer)
	}()

	limitedReader := io.LimitReader(r.Body, bi.maxBodySize)

	if _, err := io.Copy(buffer, limitedReader); err != nil {
		bi.logger.Debug("Failed to read request body", "error", err)
		// Hand back what we consumed, then the same error: io.Copy has already taken it from
		// the original body, which may well return a clean EOF next. Dropping the prefix would
		// turn a partial body into an empty one; dropping the error would hide the truncation.
		r.Body = readCloser{
			Reader: io.MultiReader(bytes.NewReader(append([]byte(nil), buffer.Bytes()...)), errReader{err: err}),
			Closer: r.Body,
		}
		return nil
	}

	// Copy the buffer bytes before the deferred Reset()/Put() invalidates the slice.
	// buffer.Bytes() aliases the pool buffer's internal array; once Reset() runs the
	// underlying array can be reused for another request, corrupting r.Body reads.
	restored := append([]byte(nil), buffer.Bytes()...)

	// Restore the body for downstream handlers by creating a new reader that combines
	// what we've already read with any remaining unread content
	origBody := r.Body
	r.Body = readCloser{
		Reader: io.MultiReader(bytes.NewReader(restored), origBody),
		Closer: origBody,
	}

	modelName := bi.extractModelName(buffer.Bytes())
	if modelName != "" {
		profile.ModelName = modelName
		bi.logger.Debug("Extracted model name from request body", "model", modelName)
	}

	// Detect required capabilities from the request
	capabilities := bi.detectRequiredCapabilities(buffer.Bytes())
	if capabilities != nil {
		// Only set capabilities if they require special features beyond basic chat
		if capabilities.VisionUnderstanding || capabilities.FunctionCalling ||
			capabilities.Embeddings || capabilities.CodeGeneration {
			// the path inspector may already have set what the path implies
			// (rerank, classification, native embed); keep it beside the body's
			if prior := profile.ModelCapabilities; prior != nil {
				capabilities.Rerank = prior.Rerank
				capabilities.Classification = prior.Classification
				// a rerank or classification path outranks the body's embeddings
				// guess: rerank bodies carry `input`/`texts`, and no model is
				// both an embedder and a reranker, so keeping both would
				// match nothing
				if prior.Rerank || prior.Classification {
					capabilities.Embeddings = prior.Embeddings
				} else {
					capabilities.Embeddings = capabilities.Embeddings || prior.Embeddings
				}
			}
			profile.ModelCapabilities = capabilities
			bi.logger.Debug("Detected required capabilities from request",
				"vision", capabilities.VisionUnderstanding,
				"functions", capabilities.FunctionCalling,
				"embeddings", capabilities.Embeddings,
				"code", capabilities.CodeGeneration,
				"streaming", capabilities.StreamingSupport)
		}
	}

	return nil
}

// opaqueBody reports bodies we must not look inside: multipart (the model, if any, is a form
// part) and anything with a Content-Encoding, which we deliberately never decompress.
func (bi *BodyInspector) opaqueBody(r *http.Request) bool {
	ct := strings.ToLower(r.Header.Get(constants.HeaderContentType))
	if strings.HasPrefix(strings.TrimSpace(ct), "multipart/") {
		bi.logger.Debug("Skipping body inspection for multipart content", "content_type", ct)
		return true
	}
	for _, enc := range r.Header.Values(constants.HeaderContentEncoding) {
		if !isIdentityEncoding(enc) {
			bi.logger.Debug("Skipping body inspection for encoded content", "content_encoding", enc)
			return true
		}
	}
	return false
}

// isIdentityEncoding reports whether a Content-Encoding value means no encoding. "identity" is
// the registered name for that, and some clients send it explicitly. An empty value is treated
// the same way. A list such as "gzip, identity" is not identity: it still carries a real coding.
func isIdentityEncoding(enc string) bool {
	enc = strings.TrimSpace(enc)
	return enc == "" || strings.EqualFold(enc, "identity")
}

// looksLikeJSONObject peeks at the first non-whitespace byte without consuming it. It asks for
// one more byte at a time so it never waits for data beyond what the decision needs. The body
// is always left readable from its first byte.
func (bi *BodyInspector) looksLikeJSONObject(r *http.Request) bool {
	origBody := r.Body
	br := bufio.NewReaderSize(origBody, peekWindow)

	// skipped holds whitespace consumed to look further ahead; it is replayed ahead of br so
	// the body is always left readable from its first byte. Only a padded body allocates it.
	var skipped []byte
	defer func() {
		var rd io.Reader = br
		if len(skipped) > 0 {
			rd = io.MultiReader(bytes.NewReader(skipped), br)
		}
		r.Body = readCloser{Reader: rd, Closer: origBody}
	}()

	for {
		for n := 1; n <= peekWindow; n++ {
			head, _ := br.Peek(n)
			if len(head) < n {
				// EOF or read error before any decision byte: nothing to inspect.
				return false
			}
			switch head[n-1] {
			case ' ', '\t', '\r', '\n':
				continue
			case '{':
				return true
			default:
				bi.logger.Debug("Skipping body inspection for non-object body")
				return false
			}
		}
		// The window is all whitespace. Keep looking, but only up to a fixed bound: a body that
		// is still blank after that is not a JSON request, and handing unbounded whitespace to
		// the streaming decoder makes it rescan the accumulated prefix on every short read.
		if len(skipped)+peekWindow >= maxLeadingWhitespace {
			bi.logger.Debug("Skipping body inspection for whitespace-padded body")
			return false
		}
		head, _ := br.Peek(peekWindow)
		skipped = append(skipped, head...)
		if _, err := br.Discard(peekWindow); err != nil {
			return false
		}
	}
}

// errReader returns err on every read.
type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

// inspectLarge scans r.Body for the top-level "model" key using a TeeReader so every byte read
// is captured for restoration, with token-level iteration that skips large nested values
// without buffering them. This handles "messages" (with large base64 images) preceding "model".
func (bi *BodyInspector) inspectLarge(r *http.Request, profile *domain.RequestProfile) {
	// captured is a locally-allocated bytes.Buffer (not from a sync.Pool), so
	// captured.Bytes() is safe to use directly without an extra copy - unlike the
	// small-body path which must copy buffer.Bytes() to avoid pool aliasing
	// once the deferred Reset()/Put() runs.
	captured := &bytes.Buffer{}
	origBody := r.Body
	tee := io.TeeReader(io.LimitReader(origBody, modelScanSize), captured)

	modelName := extractTopLevelModelFieldFromReader(tee)

	// Restore body: bytes the decoder read (captured via TeeReader) + remaining original body.
	// The decoder may have stopped before the limit, so origBody still holds the unconsumed tail.
	// readCloser ensures Close() propagates to origBody to drain the connection and avoid leaks.
	r.Body = readCloser{
		Reader: io.MultiReader(bytes.NewReader(captured.Bytes()), origBody),
		Closer: origBody,
	}

	if modelName != "" {
		profile.ModelName = bi.normalizeModelName(modelName)
		bi.logger.Debug("Extracted model name from large request body scan",
			"model", profile.ModelName, "content_length", r.ContentLength)
	} else {
		bi.logger.Debug("Could not extract model name from large request body",
			"content_length", r.ContentLength)
	}
}

func (bi *BodyInspector) extractModelName(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	// Fast path: complete JSON - unmarshal directly.
	var req modelRequest
	if err := json.Unmarshal(body, &req); err == nil && req.Model != "" {
		return bi.normalizeModelName(req.Model)
	}

	// Streaming path: works on both complete and truncated JSON (e.g. a 64 KB prefix of a
	// multi-megabyte vision request). Scan only top-level tokens so we never descend into
	// the large base64 image string embedded in the messages array.
	if model := extractTopLevelModelFieldFromReader(bytes.NewReader(body)); model != "" {
		return bi.normalizeModelName(model)
	}
