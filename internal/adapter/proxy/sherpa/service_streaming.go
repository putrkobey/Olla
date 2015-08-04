package sherpa

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"

	"github.com/putrkobey/Olla/internal/adapter/proxy/config"
	"github.com/putrkobey/Olla/internal/adapter/proxy/core"

	"github.com/putrkobey/Olla/internal/adapter/proxy/common"
	"github.com/putrkobey/Olla/internal/logger"
)

// streamState tracks the state of an active stream
type streamState struct {
	lastReadTime         time.Time
	lastChunkBuffer      *SimpleRingBuffer // Ring buffer for last 8KB of data
	contentType          string
	totalBytes           int
	readCount            int
	bytesAfterDisconnect int
	clientDisconnected   bool
	isStreaming          bool
}

// readResult contains the result of a read operation
type readResult struct {
	err error
	n   int
}

// streamResponseWithTimeout performs buffered streaming with read timeout protection
// This is CRITICAL for edge servers to prevent hanging on unresponsive backends (SHERPA-105)
func (s *Service) streamResponseWithTimeout(clientCtx, upstreamCtx context.Context, w http.ResponseWriter, resp *http.Response, buffer []byte, rlog logger.StyledLogger) (int, []byte, error) {
	state := &streamState{
		lastReadTime:    time.Now(),
		contentType:     resp.Header.Get(constants.HeaderContentType),
		isStreaming:     core.AutoDetectStreamingMode(clientCtx, resp, s.configuration.GetProxyProfile()),
		lastChunkBuffer: NewSimpleRingBuffer(8192), // Keep last 8KB for metrics
	}

	readTimeout := s.getReadTimeout()
	rlog.Debug("starting response stream", "read_timeout", readTimeout, "buffer_size", len(buffer), "profile", s.configuration.GetProxyProfile())

	// Create combined context for cancellation
	combinedCtx, cancel := s.createCombinedContext(clientCtx, upstreamCtx)
