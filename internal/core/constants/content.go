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
	ContentTypePDF            = "application/pdf"
	ContentTypeZIP            = "application/zip"
	ContentTypeGZIP           = "application/gzip"
	ContentTypeTAR            = "application/x-tar"
	ContentTypeRAR            = "application/x-rar"
	ContentType7Z             = "application/x-7z-compressed"
	ContentTypeOctetStream    = "application/octet-stream"
	ContentTypeExcel          = "application/vnd.ms-excel"
	ContentTypeWordDOCX       = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	ContentTypeOfficeDocument = "application/vnd.openxmlformats-officedocument"
	ContentTypeWordDOC        = "application/msword"
	ContentTypePowerPoint     = "application/vnd.ms-powerpoint"

	// Image Content Types
	ContentTypeImagePNG  = "image/png"
	ContentTypeImageJPEG = "image/jpeg"
	ContentTypeImageWebP = "image/webp"
	ContentTypeImageSVG  = "image/svg+xml"

	// Video Content Types
	ContentTypeVideoMP4  = "video/mp4"
	ContentTypeVideoWebM = "video/webm"

	// Media Type Prefixes
	ContentTypePrefixImage = "image/"
	ContentTypePrefixVideo = "video/"
	ContentTypePrefixAudio = "audio/"
	ContentTypePrefixFont  = "font/"
	ContentTypePrefixModel = "model/"

	// Combined Accept Headers
	AcceptJSONTextAny = "application/json, text/plain, */*"

	// Standard HTTP Headers
	HeaderContentType       = "Content-Type"
	HeaderContentEncoding   = "Content-Encoding"
	HeaderAccept            = "Accept"
	HeaderAuthorization     = "Authorization"
	HeaderUserAgent         = "User-Agent"
	HeaderCacheControl      = "Cache-Control"
	HeaderCookie            = "Cookie"
	HeaderVia               = "Via"
	HeaderAcceptEncoding    = "Accept-Encoding"
	HeaderConnection        = "Connection"
	HeaderKeepAlive         = "Keep-Alive"
	HeaderProxyAuthenticate = "Proxy-Authenticate"
	HeaderTE                = "TE"
