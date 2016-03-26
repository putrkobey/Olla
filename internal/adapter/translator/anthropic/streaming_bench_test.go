package anthropic

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// buildTextStream constructs a synthetic OpenAI SSE stream with n text delta chunks
// followed by a finish chunk and the done marker. Used to exercise the content_block_delta
// hot path without the scanner/parse overhead dominating the benchmark.
func buildTextStream(n int, chunkText string) io.Reader {
	var sb strings.Builder
	// First chunk carries model name.
	sb.WriteString(textChunk("chatcmpl-bench", "claude-3-5-sonnet-20241022", chunkText))
	for i := 1; i < n; i++ {
		sb.WriteString(textChunk("chatcmpl-bench", "", chunkText))
	}
	sb.WriteString(finishChunk("chatcmpl-bench", "stop"))
	sb.WriteString(doneChunk())
	return strings.NewReader(sb.String())
}

// buildThinkingStream constructs a synthetic stream with n reasoning delta chunks
// followed by a single text chunk, finish, and done.
func buildThinkingStream(n int, chunkText string) io.Reader {
	var sb strings.Builder
	sb.WriteString(reasoningChunk("chatcmpl-bench-r", "deepseek-r1", chunkText))
	for i := 1; i < n; i++ {
		sb.WriteString(reasoningChunk("chatcmpl-bench-r", "", chunkText))
	}
	sb.WriteString(textChunk("chatcmpl-bench-r", "", "result"))
	sb.WriteString(finishChunk("chatcmpl-bench-r", "stop"))
	sb.WriteString(doneChunk())
	return strings.NewReader(sb.String())
}

// buildToolStream constructs a stream with a single tool call whose arguments arrive
// in n small chunks, exercising the input_json_delta hot path.
func buildToolStream(n int) io.Reader {
	var sb strings.Builder
	sb.WriteString(toolStartChunk("chatcmpl-bench-t", 0, "call_bench", "bench_tool"))
	for i := range n {
		sb.WriteString(toolArgsChunk("chatcmpl-bench-t", 0, fmt.Sprintf(`{"n":%d}`, i)))
	}
	sb.WriteString(finishChunk("chatcmpl-bench-t", "tool_calls"))
	sb.WriteString(doneChunk())
	return strings.NewReader(sb.String())
}

// BenchmarkStreaming_ContentDeltas measures allocations on the text_delta hot path.
// Run with: go test -bench=BenchmarkStreaming_ContentDeltas -benchmem -count=5
func BenchmarkStreaming_ContentDeltas(b *testing.B) {
	const chunks = 50 // representative of a medium-length response

	tr := newTestTranslator()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		b.StopTimer()
		stream := buildTextStream(chunks, "hello ")
		rec := httptest.NewRecorder()
		b.StartTimer()

		if err := tr.TransformStreamingResponse(ctx, stream, rec, nil); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStreaming_ThinkingDeltas measures allocations on the thinking_delta hot path.
// Run with: go test -bench=BenchmarkStreaming_ThinkingDeltas -benchmem -count=5
func BenchmarkStreaming_ThinkingDeltas(b *testing.B) {
	const chunks = 50

	tr := newTestTranslator()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		b.StopTimer()
		stream := buildThinkingStream(chunks, "thinking step ")
		rec := httptest.NewRecorder()
		b.StartTimer()

