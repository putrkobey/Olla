<script lang="ts">
  import { fmtPct, pctBucket } from '../lib/format';

  // `status` gates the bar's COLOUR on endpoint health: a down backend
  // (offline, unhealthy, critical) with lifetime counters would otherwise
  // paint a confident green bar beside its red status pill. It must not gate
  // the FIGURE away too - an endpoint that served 10,000 requests before
  // dying carries real history, and rendering it identically to an endpoint
  // that never took a request throws that history away exactly when an
  // operator needs it. Optional and backward-compatible - callers that omit
  // status get the original behaviour.
