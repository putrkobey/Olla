<script lang="ts">
  import { overview } from '../lib/stores/overview.svelte';
  import { endpoints } from '../lib/stores/endpoints.svelte';
  import StatTile from '../components/StatTile.svelte';
  import StatusBanner from '../components/StatusBanner.svelte';
  import StatusTag from '../components/StatusTag.svelte';
  import PctBar from '../components/PctBar.svelte';
  import SortableTable from '../components/SortableTable.svelte';
  import type { Column, SortState } from '../components/SortableTable.svelte';
  import { fmtBytes, fmtInt, fmtUptime, fmtMs } from '../lib/format';
  import { stableId } from '../lib/dom-id';
  import { jumpToEndpoint } from '../lib/jump-to-endpoint';
  import { getNow as liveNow } from '../lib/clock.svelte';
  import type { EndpointSummary } from '../lib/types';

  const sys = $derived(overview.data?.system);
  const proxy = $derived(overview.data?.proxy);
  // security sits at the StatusResponse root, not on system; surface it here so
  // the Security violations tile can show status/blocked/rate-vs-size detail
  // without a second fetch.
  const sec = $derived(overview.data?.security);
  const loading = $derived(overview.status === 'loading');

  // endpoints_up arrives as a pre-formatted "x/y" string; split for the tile.
  const upTotal = $derived(sys?.endpoints_up ?? '/');
  const up = $derived(Number(upTotal.split('/')[0]) || 0);
  const total = $derived(Number(upTotal.split('/')[1]) || 0);

  const successPctNum = $derived(parseFloat(sys?.success_rate ?? '') || 0);
  const trafficBytes = $derived(parseTrafficBytes(sys?.total_traffic));
  // Backend signals no-traffic explicitly so we don't mistake "N/A" / "0%" for
  // a real 0% success rate. Both the response-rate and failure tiles branch on
  // this; total_requests===0 is a fallback for older builds without the flag.
  const hasTraffic = $derived(sys?.has_traffic === true || (sys?.total_requests ?? 0) > 0);
  // Pre-formatted so the tile stays on the plain sub= prop (no snippet needed).
  const failuresSub = $derived(
    hasTraffic ? `${(100 - successPctNum).toFixed(1)}% of traffic` : 'no traffic yet'
  );

  const endpointList = $derived(endpoints.data?.endpoints ?? []);

  // Overview shares the endpoints store with EndpointsPanel: the glance table
  // and the Discovered models / Latency range / Backend types tiles all derive
  // from it. Without starting the store here, a fresh load landing on Overview
  // renders misleading zeros until the operator clicks the Endpoints tab.
  // App.svelte renders only one panel at a time, so this cannot race the
  // EndpointsPanel lifecycle owner.
  $effect(() => {
    endpoints.start();
    return () => endpoints.stop();
  });

  // Aggregate fields already present on EndpointSummary but not previously
  // surfaced. Each tiles out into its own derived so a partial payload never
  // throws and the no-data states stay explicit.
  const totalDiscoveredModels = $derived(
    endpointList.reduce((n, e) => n + (e.model_count ?? 0), 0)
  );

  // Fleet latency range = min of per-endpoint min, max of per-endpoint max,
  // considering only endpoints that actually saw traffic. An idle endpoint
  // arrives as min=max=0; if it fed the min reduction the fleet would render
  // "0ms-..." implying a real fast measurement. Gate on max_latency_ms > 0
  // (mirror of the glance table's request_count guard) so idle endpoints drop
  // out of both the min and max, and an all-idle fleet falls through to the
  // no-data dash via hasFleetLatency.
  const activeForLatency = $derived(endpointList.filter((e) => (e.max_latency_ms ?? 0) > 0));
