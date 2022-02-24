package services

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
)

// fakeManagedService is a minimal ManagedService for exercising ServiceManager
// without pulling in real dependencies. Stop sleeps briefly so concurrent
// Register calls have a real window to race against stopServices' map read.
// startDelay does the same for Start's map read when set.
type fakeManagedService struct {
	name       string
	deps       []string
	startDelay time.Duration
	startErr   error
	rec        *lifecycleRecorder // optional; nil for tests that ignore ordering
}

// lifecycleRecorder captures Start/Stop calls across fakes in call order. The
// mutex is needed because fakes are also driven from concurrent tests.
type lifecycleRecorder struct {
	mu          sync.Mutex
	starts      []string
	stops       []string
	stopCtxErrs []error
}

func (r *lifecycleRecorder) started(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.starts = append(r.starts, name)
}

func (r *lifecycleRecorder) stopped(name string, ctxErr error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stops = append(r.stops, name)
	r.stopCtxErrs = append(r.stopCtxErrs, ctxErr)
}

func (r *lifecycleRecorder) snapshot() (starts, stops []string, ctxErrs []error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.starts), slices.Clone(r.stops), slices.Clone(r.stopCtxErrs)
}

func (f *fakeManagedService) Name() string           { return f.name }
func (f *fakeManagedService) Dependencies() []string { return f.deps }
func (f *fakeManagedService) Start(context.Context) error {
	if f.startDelay > 0 {
		time.Sleep(f.startDelay)
