// Mirrors pkg/format.Bytes so the wire JSON and the UI never disagree on
// units, precision or labels. Keep this in lockstep with the Go function.

const BYTES_UNITS = ['KB', 'MB', 'GB', 'TB', 'PB'];
const BYTES_UNIT = 1024;

/**
 * Format a byte count. Matches pkg/format.Bytes exactly:
 * - < 1024 -> "<n> B" (integer, single space, capital B)
 * - otherwise -> "<value>.2f <Unit>" with unit drawn from KB,MB,GB,TB,PB
 */
export function fmtBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) bytes = 0;
  bytes = Math.floor(bytes);

  if (bytes < BYTES_UNIT) {
    return `${bytes} B`;
  }

  let div = BYTES_UNIT;
  let exp = 0;
  for (let n = Math.floor(bytes / BYTES_UNIT); n >= BYTES_UNIT; n = Math.floor(n / BYTES_UNIT)) {
    div *= BYTES_UNIT;
    exp++;
  }
  if (exp >= BYTES_UNITS.length) exp = BYTES_UNITS.length - 1;

  const value = bytes / div;
  return `${value.toFixed(2)} ${BYTES_UNITS[exp]}`;
}

// Locale-independent grouping; en-AU matches en-GB comma grouping.
const INT_LOCALE = 'en-AU';

/** Integer with thousands separators. Pass null/undefined -> "0". */
export function fmtInt(n: number | null | undefined): string {
  if (n === null || n === undefined || Number.isNaN(n)) return '0';
  return Math.floor(Number(n)).toLocaleString(INT_LOCALE);
}

/**
 * Percentage with one decimal, mirroring pkg/format.Percentage for non-boundary
 * values. The Go variant special-cases 0 and 100 to skip the decimal; this JS
 * version does too so the dashboard doesn't show "0.0%" vs the API's "0%".
 */
export function fmtPct(value: number): string {
  if (!Number.isFinite(value)) return '0%';
  if (value === 0) return '0%';
  if (value === 100) return '100%';
  return `${value.toFixed(1)}%`;
}

/**
 * Latency in milliseconds. Mirrors pkg/format.Latency:
 * - 0 -> "0ms"
 * - >= 1000 -> "<x.x>s"
 * - < 10 -> single-digit form ("5ms" not "05ms"), but the JS path just prints
 *   the integer so we keep parity with the >9ms branch.
 */
export function fmtMs(ms: number): string {
  if (!ms || ms <= 0) return '0ms';
  if (ms >= 1000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.floor(ms)}ms`;
}

/**
 * Sqrt-scaled percentage for visual bars where small values must remain
 * legible against a much larger max (a 28ms p95 vs a 29400ms p99 in the
 * same track). Matches the mockup's scalePct.
 */
export function scalePct(value: number, max: number): number {
  if (!max || value <= 0) return 0;
  return Math.min(100, Math.sqrt(value / max) * 100);
}

/** Choose a status colour bucket given a percentage and whether we have data. */
export function pctBucket(pct: number, hasData: boolean): 'neutral' | 'green' | 'amber' | 'red' {
  if (!hasData) return 'neutral';
  if (pct >= 99) return 'green';
