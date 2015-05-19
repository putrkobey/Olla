package handlers

import (
	"net/http"

	"github.com/putrkobey/Olla/internal/core/constants"
)

func (a *Application) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.logger.Info("HTTP request",
			"method", r.Method,
