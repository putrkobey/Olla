package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/putrkobey/Olla/internal/core/constants"
)

func (a *Application) stickyStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(http.StatusOK)

	if a.stickyStatsFn == nil {
		// Sticky sessions are disabled — return a stable JSON shape so callers
		// can branch on the "enabled" field rather than on status codes.
