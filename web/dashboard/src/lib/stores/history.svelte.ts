// Ring-buffer of overview snapshots for the SparkStrip. One sample per poll
// tick (success AND failure) via the overview store's onTick callback. Failed
// ticks push an error sample (error: true, no counter data); the chart draws
// a red marker at each error sample's x position and breaks the area/line
// paths around it so the outage is visible rather than bridged over.
//
// Singleton module-scope state so every importer (component + tests) shares
// one buffer, matching how overview/theme are wired elsewhere.

import type { StatusResponse } from '../types';

export interface Sample {
  // epoch ms
  t: number;
  totalRequests: number;
  totalBytes: number;
  activeConnections: number;
  // derived per-tick rates
  reqPerSec: number;
  bytesPerSec: number;
  // true on a failed-poll marker. Error samples carry no counter data and
  // must not feed the delta baseline or restart detection.
  error?: boolean;
}

// ~15 min of samples at the 5 s overview cadence.
export const MAX_SAMPLES = 180;

// If no sample at all (success or error) arrives within this window, the
// panel was hidden or the tab was backgrounded beyond the scheduler's backoff
// cadence, so the delta baseline is stale. Error ticks update lastSampleT so
// a visible outage (where error ticks keep firing) does NOT trip this guard:
// the next successful sample legitimately deltas across the outage window.
const STALE_GAP_MS = 30_000;

// --- pure helpers (exported for unit testing) ---

/** Clamp negative deltas to 0: a counter reset must not render as a negative spike. */
export function clampNonNegative(n: number): number {
  return Number.isFinite(n) && n > 0 ? n : 0;
}

/** Per-second rate from a raw counter delta over dtMs. */
export function computeRate(delta: number, dtMs: number): number {
  if (!Number.isFinite(delta) || !Number.isFinite(dtMs) || dtMs <= 0) return 0;
  return clampNonNegative(delta) / (dtMs / 1000);
}

/** True when the herd's start_time changed between snapshots: the process
 *  restarted and every counter reset, so historical deltas are meaningless
 *  and the buffer must be re-seeded from the new era. */
export function detectRestart(prevStart: string | null, nextStart: string | null): boolean {
  if (!prevStart || !nextStart) return false;
  return prevStart !== nextStart;
}

// Re-parses total_traffic ("172.02 GB") back to raw bytes so it can be
// delta'd. Mirrors the local helper in OverviewPanel; kept here so the store
// is self-contained and the parsing is unit-testable in isolation.
export function parseTrafficBytes(s: string | undefined): number {
