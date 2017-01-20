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
	defer cancel()

	// Use http.ResponseController for modern flush handling (Go 1.20+)
	// Provides better error handling and cleaner API than type assertion
	rc := http.NewResponseController(w)

	for {
		result, err := s.performTimedRead(combinedCtx, resp.Body, buffer, readTimeout, state, rlog)
		if err != nil {
			return state.totalBytes, state.lastChunkBuffer.Bytes(), err
		}

		if result == nil {
			// Context cancelled during read
			return state.totalBytes, state.lastChunkBuffer.Bytes(), s.handleContextCancellation(clientCtx, upstreamCtx, state, rlog)
		}

		// Process read result
		done, err := s.processReadResult(result, w, buffer, rc, state, rlog)
		if done || err != nil {
			return state.totalBytes, state.lastChunkBuffer.Bytes(), err
		}
	}
}

// getReadTimeout returns the configured read timeout or default
func (s *Service) getReadTimeout() time.Duration {
	readTimeout := s.configuration.GetReadTimeout()
	if readTimeout == 0 {
		readTimeout = config.DefaultReadTimeout
	}
	return readTimeout
}

// createCombinedContext creates a context that cancels when either client or upstream context is done
func (s *Service) createCombinedContext(clientCtx, upstreamCtx context.Context) (context.Context, context.CancelFunc) {
	combinedCtx, cancel := context.WithCancel(context.Background())

	go func() {
		select {
		case <-clientCtx.Done():
			cancel()
		case <-upstreamCtx.Done():
			cancel()
		case <-combinedCtx.Done():
			return // Exit when combined context is cancelled
		}
	}()

	return combinedCtx, cancel
}

