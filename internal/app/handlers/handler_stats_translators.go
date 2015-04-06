package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/pkg/format"
)

type TranslatorStatsResponse struct {
	Timestamp   time.Time              `json:"timestamp"`
	Translators []TranslatorStatsEntry `json:"translators"`
	Summary     TranslatorStatsSummary `json:"summary"`
}

type TranslatorStatsEntry struct {
	TranslatorName                              string `json:"translator_name"`
	SuccessRate                                 string `json:"success_rate"`
	PassthroughRate                             string `json:"passthrough_rate"`
	AverageLatency                              string `json:"average_latency"`
	TotalRequests                               int64  `json:"total_requests"`
	SuccessfulRequests                          int64  `json:"successful_requests"`
	FailedRequests                              int64  `json:"failed_requests"`
	PassthroughRequests                         int64  `json:"passthrough_requests"`
	TranslationRequests                         int64  `json:"translation_requests"`
	StreamingRequests                           int64  `json:"streaming_requests"`
	NonStreamingRequests                        int64  `json:"non_streaming_requests"`
	FallbackNoCompatibleEndpoints               int64  `json:"fallback_no_compatible_endpoints"`
	FallbackTranslatorDoesNotSupportPassthrough int64  `json:"fallback_translator_does_not_support_passthrough"`
	FallbackCannotPassthrough                   int64  `json:"fallback_cannot_passthrough"`
}

type TranslatorStatsSummary struct {
	OverallSuccessRate string `json:"overall_success_rate"`
	OverallPassthrough string `json:"overall_passthrough_rate"`
	TotalTranslators   int    `json:"total_translators"`
