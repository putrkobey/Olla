<script lang="ts">
  import { endpoints } from '../lib/stores/endpoints.svelte';
  import SortableTable from '../components/SortableTable.svelte';
  import type { Column, SortState } from '../components/SortableTable.svelte';
  import StatusBanner from '../components/StatusBanner.svelte';
  import StatusTag from '../components/StatusTag.svelte';
  import RangeBar from '../components/RangeBar.svelte';
  import PctBar from '../components/PctBar.svelte';
  import { fmtAgo, fmtUntil } from '../lib/format';
  import { stableId } from '../lib/dom-id';
  import { getNow as liveNow } from '../lib/clock.svelte';
  import type { EndpointSummary } from '../lib/types';

  // View-model row: the contract row plus the numeric variants the table sorts
  // and the bars scale on. success_rate arrives pre-formatted ("98.5%") so a
  // numeric sibling field drives sort/PctBar; avg_latency_ms is normalised to
  // number|null so RangeBar can pick its no-data placeholder. Declared as a
  // type alias (not an interface) so it picks up an implicit index signature
  // and satisfies SortableTable's `Row extends Record<string, unknown>`.
  type EndpointRow = Omit<EndpointSummary, 'avg_latency_ms'> & {
    success_rate_num: number;
    avg_latency_ms: number | null;
  };

  const data = $derived(endpoints.data?.endpoints ?? []);
  const loading = $derived(endpoints.status === 'loading');
  // App.svelte only mounts this panel when it is the active one, so the panel
  // is always rendered active once it appears.

  // This panel is the lifecycle owner of the endpoints store. Mount
  // activates the job (start() fires an immediate tick, so a tab switch
  // refreshes rather than showing stale data); unmount stops it so the
  // endpoints payload stops firing while Overview or Models is open. The
  // scheduler handles aborting the in-flight tick on stop.
  $effect(() => {
    endpoints.start();
    return () => endpoints.stop();
  });

  const globalLatencyMax = $derived(data.reduce((m, e) => Math.max(m, e.max_latency_ms ?? 0), 0));

  // success_rate arrives pre-formatted ("98.5%") so we expose a numeric
  // variant for sort/scaling. avg_latency_ms is the API's own numeric field
  // (average PROXY request latency) - it must NOT be derived from
  // response_time, which is the last HEALTH-CHECK probe's latency, an
  // unrelated metric. It may be absent (older backend) or null (no traffic
  // yet), in which case RangeBar shows the no-data placeholder rather than a
  // misleading 0.
  const rows: EndpointRow[] = $derived(
    data.map((e) => ({
      ...e,
      success_rate_num: parseFloat(e.success_rate) || 0,
      avg_latency_ms: typeof e.avg_latency_ms === 'number' ? e.avg_latency_ms : null,
    }))
  );

  // Re-evaluated each second so the relative "Xs ago" labels stay live
  // without per-row setInterval. Reading liveNow() inside $derived wires
  // the dependency on the shared clock.
  const now = $derived(liveNow());

  const columns: Column[] = [
