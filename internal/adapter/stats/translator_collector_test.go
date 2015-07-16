package stats

import (
	"sync"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// TestTranslatorCollector_RecordPassthrough verifies passthrough request tracking
func TestTranslatorCollector_RecordPassthrough(t *testing.T) {
	collector := NewTranslatorCollector()

	// Record multiple passthrough requests
	for range 5 {
		event := ports.TranslatorRequestEvent{
			TranslatorName: "anthropic",
			Model:          "claude-3-5-sonnet-20241022",
			Mode:           constants.TranslatorModePassthrough,
			FallbackReason: constants.FallbackReasonNone,
			Success:        true,
			Latency:        100 * time.Millisecond,
			IsStreaming:    false,
		}
		collector.Record(event)
	}

	// Verify stats
	stats := collector.GetStats()
	if len(stats) != 1 {
		t.Fatalf("Expected 1 translator, got %d", len(stats))
	}

	anthropicStats, exists := stats["anthropic"]
	if !exists {
		t.Fatal("Anthropic translator stats not found")
	}

	if anthropicStats.TotalRequests != 5 {
		t.Errorf("Expected 5 total requests, got %d", anthropicStats.TotalRequests)
	}
	if anthropicStats.SuccessfulRequests != 5 {
		t.Errorf("Expected 5 successful requests, got %d", anthropicStats.SuccessfulRequests)
	}
	if anthropicStats.FailedRequests != 0 {
		t.Errorf("Expected 0 failed requests, got %d", anthropicStats.FailedRequests)
	}
	if anthropicStats.PassthroughRequests != 5 {
		t.Errorf("Expected 5 passthrough requests, got %d", anthropicStats.PassthroughRequests)
	}
	if anthropicStats.TranslationRequests != 0 {
		t.Errorf("Expected 0 translation requests, got %d", anthropicStats.TranslationRequests)
	}

	// Verify no fallback reasons recorded for passthrough
	if anthropicStats.FallbackNoCompatibleEndpoints != 0 {
		t.Errorf("Expected 0 fallback no compatible endpoints, got %d", anthropicStats.FallbackNoCompatibleEndpoints)
	}
	if anthropicStats.FallbackTranslatorDoesNotSupportPassthrough != 0 {
		t.Errorf("Expected 0 fallback translator does not support passthrough, got %d", anthropicStats.FallbackTranslatorDoesNotSupportPassthrough)
	}
	if anthropicStats.FallbackCannotPassthrough != 0 {
		t.Errorf("Expected 0 fallback cannot passthrough, got %d", anthropicStats.FallbackCannotPassthrough)
	}
}

// TestTranslatorCollector_RecordTranslationWithFallback verifies translation mode and fallback reason tracking
func TestTranslatorCollector_RecordTranslationWithFallback(t *testing.T) {
	collector := NewTranslatorCollector()

	// Test each fallback reason
	testCases := []struct {
		name           string
		fallbackReason constants.TranslatorFallbackReason
		count          int
	}{
		{
			name:           "no compatible endpoints",
			fallbackReason: constants.FallbackReasonNoCompatibleEndpoints,
			count:          3,
		},
		{
			name:           "translator does not support passthrough",
			fallbackReason: constants.FallbackReasonTranslatorDoesNotSupportPassthrough,
			count:          2,
		},
		{
			name:           "cannot passthrough",
			fallbackReason: constants.FallbackReasonCannotPassthrough,
			count:          4,
		},
	}

	totalTranslations := 0
	for _, tc := range testCases {
		for range tc.count {
			event := ports.TranslatorRequestEvent{
				TranslatorName: "anthropic",
				Model:          "claude-3-5-sonnet-20241022",
				Mode:           constants.TranslatorModeTranslation,
				FallbackReason: tc.fallbackReason,
				Success:        true,
				Latency:        150 * time.Millisecond,
				IsStreaming:    false,
			}
			collector.Record(event)
			totalTranslations++
		}
	}

	// Verify stats
	stats := collector.GetStats()
	anthropicStats := stats["anthropic"]

	if anthropicStats.TotalRequests != int64(totalTranslations) {
		t.Errorf("Expected %d total requests, got %d", totalTranslations, anthropicStats.TotalRequests)
	}
	if anthropicStats.TranslationRequests != int64(totalTranslations) {
		t.Errorf("Expected %d translation requests, got %d", totalTranslations, anthropicStats.TranslationRequests)
	}
	if anthropicStats.PassthroughRequests != 0 {
		t.Errorf("Expected 0 passthrough requests, got %d", anthropicStats.PassthroughRequests)
	}

	// Verify fallback reason counters
	if anthropicStats.FallbackNoCompatibleEndpoints != 3 {
		t.Errorf("Expected 3 fallback no compatible endpoints, got %d", anthropicStats.FallbackNoCompatibleEndpoints)
	}
	if anthropicStats.FallbackTranslatorDoesNotSupportPassthrough != 2 {
		t.Errorf("Expected 2 fallback translator does not support passthrough, got %d", anthropicStats.FallbackTranslatorDoesNotSupportPassthrough)
	}
	if anthropicStats.FallbackCannotPassthrough != 4 {
		t.Errorf("Expected 4 fallback cannot passthrough, got %d", anthropicStats.FallbackCannotPassthrough)
	}
}

// TestTranslatorCollector_ConcurrentAccess verifies thread-safety under concurrent load
func TestTranslatorCollector_ConcurrentAccess(t *testing.T) {
	collector := NewTranslatorCollector()

	const numGoroutines = 100
	const requestsPerGoroutine = 50

	var wg sync.WaitGroup

	// Launch goroutines recording passthrough events
	for range numGoroutines / 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range requestsPerGoroutine {
				event := ports.TranslatorRequestEvent{
					TranslatorName: "anthropic",
					Model:          "claude-3-5-sonnet-20241022",
					Mode:           constants.TranslatorModePassthrough,
					FallbackReason: constants.FallbackReasonNone,
					Success:        true,
					Latency:        100 * time.Millisecond,
					IsStreaming:    false,
				}
				collector.Record(event)
			}
		}()
	}

	// Launch goroutines recording translation events with fallback
	for range numGoroutines / 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range requestsPerGoroutine {
				event := ports.TranslatorRequestEvent{
					TranslatorName: "anthropic",
					Model:          "claude-3-5-sonnet-20241022",
					Mode:           constants.TranslatorModeTranslation,
					FallbackReason: constants.FallbackReasonNoCompatibleEndpoints,
					Success:        true,
					Latency:        150 * time.Millisecond,
					IsStreaming:    true,
				}
				collector.Record(event)
			}
		}()
	}

	wg.Wait()

	// Verify final counts
	stats := collector.GetStats()
	anthropicStats := stats["anthropic"]

	expectedTotal := int64(numGoroutines * requestsPerGoroutine)
	if anthropicStats.TotalRequests != expectedTotal {
		t.Errorf("Expected %d total requests, got %d", expectedTotal, anthropicStats.TotalRequests)
	}

	expectedPassthrough := int64(numGoroutines/2) * int64(requestsPerGoroutine)
	if anthropicStats.PassthroughRequests != expectedPassthrough {
		t.Errorf("Expected %d passthrough requests, got %d", expectedPassthrough, anthropicStats.PassthroughRequests)
	}

	expectedTranslation := int64(numGoroutines/2) * int64(requestsPerGoroutine)
	if anthropicStats.TranslationRequests != expectedTranslation {
		t.Errorf("Expected %d translation requests, got %d", expectedTranslation, anthropicStats.TranslationRequests)
	}

	if anthropicStats.FallbackNoCompatibleEndpoints != expectedTranslation {
		t.Errorf("Expected %d fallback no compatible endpoints, got %d", expectedTranslation, anthropicStats.FallbackNoCompatibleEndpoints)
	}

	// Verify success count
	if anthropicStats.SuccessfulRequests != expectedTotal {
		t.Errorf("Expected %d successful requests, got %d", expectedTotal, anthropicStats.SuccessfulRequests)
	}
}

// TestTranslatorCollector_PassthroughRate verifies passthrough rate calculation
func TestTranslatorCollector_PassthroughRate(t *testing.T) {
	testCases := []struct {
		name                string
		passthroughCount    int
		translationCount    int
		expectedPassthrough int64
		expectedTranslation int64
	}{
		{
			name:                "all passthrough",
			passthroughCount:    10,
			translationCount:    0,
			expectedPassthrough: 10,
			expectedTranslation: 0,
		},
		{
			name:                "all translation",
			passthroughCount:    0,
			translationCount:    10,
			expectedPassthrough: 0,
			expectedTranslation: 10,
		},
		{
			name:                "50/50 split",
			passthroughCount:    5,
			translationCount:    5,
			expectedPassthrough: 5,
			expectedTranslation: 5,
		},
		{
			name:                "70/30 split",
			passthroughCount:    7,
			translationCount:    3,
			expectedPassthrough: 7,
			expectedTranslation: 3,
		},
		{
			name:                "zero requests",
			passthroughCount:    0,
			translationCount:    0,
			expectedPassthrough: 0,
			expectedTranslation: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			collector := NewTranslatorCollector()

			// Record passthrough requests
			for range tc.passthroughCount {
				event := ports.TranslatorRequestEvent{
					TranslatorName: "anthropic",
					Model:          "claude-3-5-sonnet-20241022",
					Mode:           constants.TranslatorModePassthrough,
					FallbackReason: constants.FallbackReasonNone,
					Success:        true,
					Latency:        100 * time.Millisecond,
					IsStreaming:    false,
				}
				collector.Record(event)
			}

			// Record translation requests
			for range tc.translationCount {
				event := ports.TranslatorRequestEvent{
					TranslatorName: "anthropic",
					Model:          "claude-3-5-sonnet-20241022",
					Mode:           constants.TranslatorModeTranslation,
					FallbackReason: constants.FallbackReasonNoCompatibleEndpoints,
					Success:        true,
					Latency:        150 * time.Millisecond,
					IsStreaming:    false,
				}
				collector.Record(event)
			}

			// Verify stats
			stats := collector.GetStats()

			// Handle zero requests case
			if tc.passthroughCount == 0 && tc.translationCount == 0 {
				if len(stats) != 0 {
					t.Errorf("Expected 0 translators for zero requests, got %d", len(stats))
				}
				return
			}

			anthropicStats := stats["anthropic"]

			if anthropicStats.PassthroughRequests != tc.expectedPassthrough {
				t.Errorf("Expected %d passthrough requests, got %d", tc.expectedPassthrough, anthropicStats.PassthroughRequests)
			}
			if anthropicStats.TranslationRequests != tc.expectedTranslation {
				t.Errorf("Expected %d translation requests, got %d", tc.expectedTranslation, anthropicStats.TranslationRequests)
			}

			// Calculate and verify passthrough rate
			total := tc.expectedPassthrough + tc.expectedTranslation
			expectedRate := float64(0)
			if total > 0 {
				expectedRate = float64(tc.expectedPassthrough) / float64(total) * 100
			}

			actualRate := float64(0)
			if anthropicStats.TotalRequests > 0 {
				actualRate = float64(anthropicStats.PassthroughRequests) / float64(anthropicStats.TotalRequests) * 100
			}

			if actualRate != expectedRate {
				t.Errorf("Expected passthrough rate %.2f%%, got %.2f%%", expectedRate, actualRate)
			}
		})
	}
}

// TestTranslatorCollector_MultipleTranslators verifies stats are tracked separately per translator
func TestTranslatorCollector_MultipleTranslators(t *testing.T) {
	collector := NewTranslatorCollector()

	translators := []struct {
		name            string
		passthroughReqs int
		translationReqs int
	}{
		{"anthropic", 10, 5},
		{"openai", 8, 12},
		{"ollama", 15, 3},
	}

	// Record events for each translator
	for _, translator := range translators {
		// Passthrough requests
		for range translator.passthroughReqs {
			event := ports.TranslatorRequestEvent{
				TranslatorName: translator.name,
				Model:          "test-model",
				Mode:           constants.TranslatorModePassthrough,
				FallbackReason: constants.FallbackReasonNone,
				Success:        true,
				Latency:        100 * time.Millisecond,
				IsStreaming:    false,
			}
			collector.Record(event)
		}

		// Translation requests
		for range translator.translationReqs {
			event := ports.TranslatorRequestEvent{
				TranslatorName: translator.name,
				Model:          "test-model",
				Mode:           constants.TranslatorModeTranslation,
				FallbackReason: constants.FallbackReasonCannotPassthrough,
				Success:        true,
				Latency:        150 * time.Millisecond,
				IsStreaming:    false,
			}
			collector.Record(event)
		}
	}

	// Verify GetTranslatorStats returns all translators
	stats := collector.GetStats()
	if len(stats) != len(translators) {
		t.Fatalf("Expected %d translators, got %d", len(translators), len(stats))
	}

	// Verify stats are tracked separately
	for _, translator := range translators {
		translatorStats, exists := stats[translator.name]
		if !exists {
			t.Errorf("Stats not found for translator %s", translator.name)
			continue
		}

		expectedTotal := int64(translator.passthroughReqs + translator.translationReqs)
		if translatorStats.TotalRequests != expectedTotal {
			t.Errorf("Translator %s: expected %d total requests, got %d", translator.name, expectedTotal, translatorStats.TotalRequests)
		}
