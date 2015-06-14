package constants

const (
	DefaultContentTypeJSON = ContentTypeJSON

	// Standard Content Types
	ContentTypeJSON           = "application/json"
	ContentTypeText           = "text/plain"
	ContentTypePrometheus     = "text/plain; version=0.0.4; charset=utf-8"
	ContentTypeHTML           = "text/html"
	ContentTypeXML            = "application/xml"
	ContentTypeJavaScript     = "application/javascript"
	ContentTypeCSS            = "text/css"
	ContentTypeFormURLEncoded = "application/x-www-form-urlencoded"

	// Streaming Content Types
	ContentTypeEventStream = "text/event-stream"
	ContentTypeNDJSON      = "application/x-ndjson"
	ContentTypeStreamJSON  = "application/stream+json"
	ContentTypeJSONSeq     = "application/json-seq"
	ContentTypeTextUTF8    = "text/plain; charset=utf-8"

	// Binary/Document Content Types
