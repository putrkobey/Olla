// /internal/status/models poller. Drives ModelsPanel. Detailed/grouped form
// is requested so the panel can render family groups without client re-group.
//
// Per-model request/latency stats (previously sourced from a secondary fetch
// to /internal/stats/models) are intentionally NOT fetched here: nothing on
// the proxy path calls RecordModelRequest, so those figures are always zero
// and presenting them would read as "never used" when the truth is "never
// counted". Wiring per-model request counting on the proxy path is deferred
// to a later change.
import type { ModelStatusResponse } from '../types';
import { createPollStore } from './poll-store.svelte';
