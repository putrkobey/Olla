// /internal/status poller. Drives the StatusStrip, OverviewPanel and the
// SparkStrip's history buffer. The onTick callback is the per-tick signal
// that feeds the ring buffer on BOTH success and failure, so the chart can
// render outage markers when Olla is unreachable.
import type { StatusResponse } from '../types';
import { createPollStore } from './poll-store.svelte';
import { history } from './history.svelte';

const OVERVIEW_INTERVAL_MS = 5000;

// Data endpoint (not a SPA asset), so it lives at /internal/status regardless
// of the SPA base. Olla mounts the proxy listener at /, so this is correct in
