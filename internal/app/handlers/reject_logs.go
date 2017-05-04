package handlers

import (
	"strconv"

	"github.com/putrkobey/Olla/internal/logger"
)

// rejectLogs samples the "Request rejected" WARN for requests Olla refuses to route (an
// unknown model under strict routing, a model or provider with no healthy host). A client
// can repeat these at wire speed, and each repeat used to write full detail lines.
//
// The key is the response status plus the client IP, status first because the throttle truncates
// long keys and a long forwarded identity must not push the status off the end. The status is a finite cause, so one
// client's 404s do not hide its 503s, while a client rotating model names still costs one
// line per window rather than one per name. The throttle caps its tracked keys.
type rejectLogs struct {
	request *logger.Throttle
