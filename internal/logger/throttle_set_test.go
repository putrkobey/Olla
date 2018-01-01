package logger

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestThrottleSet_FlushReportsEveryTrackedThrottle(t *testing.T) {
	t.Parallel()

	var a, b summaryRecorder
	set := &ThrottleSet{}
	ta := set.Track(NewThrottle(ThrottleConfig{OnSummary: a.record}))
	tb := set.Track(NewThrottle(ThrottleConfig{OnSummary: b.record}))
	for range 4 {
		ta.Allow("k")
	}
	for range 3 {
		tb.Allow("k")
	}

