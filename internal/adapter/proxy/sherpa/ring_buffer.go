package sherpa

import "sync"

// SimpleRingBuffer implements a thread-safe ring buffer for storing the last N bytes
// This is much simpler than hand-rolling complex buffer logic
type SimpleRingBuffer struct {
	data     []byte
	capacity int
	mu       sync.Mutex
}

// NewSimpleRingBuffer creates a new ring buffer with specified capacity
func NewSimpleRingBuffer(capacity int) *SimpleRingBuffer {
	return &SimpleRingBuffer{
		data:     make([]byte, 0, capacity),
		capacity: capacity,
	}
}

// Write adds data to the ring buffer, keeping only the last 'capacity' bytes
func (rb *SimpleRingBuffer) Write(p []byte) (n int, err error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	n = len(p)

