package olla

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/core"
	"github.com/putrkobey/Olla/internal/logger"
)

// streamState manages the state during streaming
type streamState struct {
	disconnectTime       time.Time
	lastChunk            []byte
	totalBytes           int
	bytesAfterDisconnect int
	lastChunkBuf         [8192]byte
	clientDisconnected   bool
}

// handleClientDisconnect processes client disconnection during streaming
func (s *Service) handleClientDisconnect(state *streamState, rlog logger.StyledLogger) {
	if !state.clientDisconnected {
		state.clientDisconnected = true
		state.disconnectTime = time.Now()
		rlog.Debug("client disconnected during streaming", "bytes_sent", state.totalBytes)

		// Publish client disconnect event
		s.PublishEvent(core.ProxyEvent{
			Type: core.EventTypeClientDisconnect,
			Metadata: core.ProxyEventMetadata{
				BytesSent: int64(state.totalBytes),
			},
		})
	}
}

// writeStreamData writes data to the response and handles flushing
func writeStreamData(w http.ResponseWriter, data []byte, isStreaming bool, rc *http.ResponseController) (int, error) {
	written, err := w.Write(data)
	if err != nil {
		return written, err
	}

	// Force data out for real-time streaming
	// ResponseController provides better error handling than direct Flusher interface
	if isStreaming {
		if flushErr := rc.Flush(); flushErr != nil {
			// Don't fail on flush errors - response may still succeed
			// Flush errors are typically benign (e.g., client already disconnected)
		}
	}

	return written, nil
}

// shouldStopAfterDisconnect checks if we should stop streaming after client disconnect
func shouldStopAfterDisconnect(bytesAfterDisconnect int, disconnectTime time.Time) bool {
	return bytesAfterDisconnect > ClientDisconnectionBytesThreshold ||
