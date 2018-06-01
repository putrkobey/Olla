import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';
import type { SystemSummary, EndpointResponse, PollStatus } from '../lib/types';

// C6 regression: StatusStrip read overview.data without consulting
// overview.status, so during an outage it displayed confident numbers while
// the panels below reported unreachable. The fix surfaces a stale/error
// indicator on the strip when overview.status is 'stale' or 'error', reusing
// the data-state pattern the panels already use.
//
// The overview store's `status` is closure-private with no setter, so this
// test substitutes a controllable fake store to exercise the component's
// reaction to each status value in isolation.

interface Holder {
  status: PollStatus;
  sys: SystemSummary;
  endpoints: EndpointResponse[];
}

const holder = vi.hoisted<Holder>(() => ({
  status: 'ok',
  sys: {
    status: 'healthy',
    endpoints_up: '2/2',
    success_rate: '99.0%',
    avg_latency: '20ms',
    total_traffic: '0',
    uptime: '0s',
    version: 'v0.0.29',
    commit: 'abc',
    start_time: new Date().toISOString(),
    active_connections: 0,
    security_violations: 0,
    total_requests: 0,
    total_failures: 0,
    has_traffic: false,
  },
  endpoints: [],
}));

vi.mock('../lib/stores/overview.svelte', () => ({
  overview: {
    get data() {
      return { system: holder.sys, endpoints: holder.endpoints };
    },
    get status() {
      return holder.status;
    },
  },
}));
