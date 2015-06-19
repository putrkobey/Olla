import { describe, it, expect } from 'vitest';
import {
  fmtBytes,
  fmtPct,
  fmtMs,
  fmtInt,
  scalePct,
  fmtDuration,
  fmtAgo,
  fmtUntil,
  fmtUptime,
} from './format';

// Fixed reference instant for the time tests; iso values below are computed
// relative to this so the assertions don't depend on wall clock.
const NOW = Date.UTC(2026, 6, 27, 12, 0, 0);
const iso = (offsetMs: number): string => new Date(NOW - offsetMs).toISOString();
const isoFuture = (offsetMs: number): string => new Date(NOW + offsetMs).toISOString();

// Byte values cross-checked against pkg/format.Bytes (see pkg/format/format.go).
// Drift here means the UI disagrees with the API on the same number, so this
// table is the contract.
const BYTE_CASES: Array<[number, string]> = [
  [0, '0 B'],
  [1, '1 B'],
  [999, '999 B'],
  [1000, '1000 B'],
  [1023, '1023 B'],
  [1024, '1.00 KB'],
  [1536, '1.50 KB'],
  [1048576, '1.00 MB'],
  [1572864, '1.50 MB'],
  [1073741824, '1.00 GB'],
  [1099511627776, '1.00 TB'],
  [1125899906842624, '1.00 PB'],
  // Real fixture from the spec: a model SizeBytes of 5_000_000_000.
  [5000000000, '4.66 GB'],
