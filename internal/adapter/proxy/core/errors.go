package core

import "errors"

// ErrCircuitBreakerOpen signals that a per-endpoint circuit breaker rejected
// the attempt before any network I/O. ExecuteWithRetry treats it as
// failover-eligible but must NOT route it through the connection-error path,
