package pool

// Pool is a strongly typed wrapper around sync.Pool with optional Reset() support.
// It eliminates the need for unsafe type assertions (interface{} casts) and plays
// nicely with golangci-lint. Objects returned from Get() are guaranteed to be the
// correct type. If the pooled type implements the Resettable interface, it will be
// automatically zeroed before being returned to the pool via Put().
//
// Designed for internal use where the constructor guarantees type safety, so the
// type assertion in Get() is safe and explicitly silenced.
//
// Example:
//
//	type RequestContext struct { ... }
//	func (r *RequestContext) Reset() { ... }
//
//	pool, err := NewLitePool(func() *RequestContext {
//	  return &RequestContext{}
//	})
//	if err != nil {
//	  // handle error
//	}
//
//	ctx, err := pool.Get()
//	if err != nil {
//	  // handle error
//	}
//	...
