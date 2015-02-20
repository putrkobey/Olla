package olla

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkStreamingLastChunk benchmarks the streaming last chunk capture with and without pre-allocated buffer.
// This demonstrates the allocation reduction from using a pre-allocated buffer in streamState.
func BenchmarkStreamingLastChunk(b *testing.B) {
	// Simulate a typical streaming response with EOF on the last chunk
	// Pre-create streamState to measure only the EOF handling allocation
	b.Run("PreAllocated_SmallChunk", func(b *testing.B) {
		buffer := make([]byte, 8192)
		data := []byte(`{"choices":[{"delta":{"content":"test"},"finish_reason":"stop"}]}`)
		w := httptest.NewRecorder()
		state := &streamState{} // Allocate once outside the loop

		b.ResetTimer()
		b.ReportAllocs()

		for range b.N {
			resp := &http.Response{
				Body: io.NopCloser(bytes.NewReader(data)),
			}

			// Simulate EOF read (last chunk scenario)
			n, err := resp.Body.Read(buffer)
			if n > 0 {
				if errors.Is(err, io.EOF) {
					if n <= len(state.lastChunkBuf) {
						copy(state.lastChunkBuf[:], buffer[:n])
						state.lastChunk = state.lastChunkBuf[:n]
					} else {
						state.lastChunk = make([]byte, n)
						copy(state.lastChunk, buffer[:n])
					}
				}
				w.Write(buffer[:n])
			}
		}
	})

	b.Run("PreAllocated_LargeChunk", func(b *testing.B) {
		buffer := make([]byte, 8192)
		// 4KB chunk - typical size
		data := make([]byte, 4096)
		for i := range data {
			data[i] = byte(i % 256)
		}
		w := httptest.NewRecorder()
		state := &streamState{} // Allocate once outside the loop

		b.ResetTimer()
		b.ReportAllocs()

		for range b.N {
			resp := &http.Response{
				Body: io.NopCloser(bytes.NewReader(data)),
			}

			n, err := resp.Body.Read(buffer)
			if n > 0 {
				if errors.Is(err, io.EOF) {
					if n <= len(state.lastChunkBuf) {
						copy(state.lastChunkBuf[:], buffer[:n])
						state.lastChunk = state.lastChunkBuf[:n]
					} else {
