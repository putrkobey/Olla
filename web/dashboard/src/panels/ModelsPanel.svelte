<script lang="ts">
  import { models } from '../lib/stores/models.svelte';
  import SortableTable from '../components/SortableTable.svelte';
  import type { Column, SortState } from '../components/SortableTable.svelte';
  import StatusBanner from '../components/StatusBanner.svelte';
  import { fmtAgo } from '../lib/format';
  import { stableId } from '../lib/dom-id';
  import { jumpToEndpoint } from '../lib/jump-to-endpoint';
  import { getNow as liveNow } from '../lib/clock.svelte';
  import type { ModelSummary } from '../lib/types';

  // View-model row: the contract row plus derived numeric fields the table
  // sorts on (size_bytes from the size string, endpoints_count from the
  // endpoints list length). Omit+intersect so the row type flows through a
  // mapped type, which picks up the implicit index signature required by
  // SortableTable's `Row extends Record<string, unknown>` constraint; a
  // plain `ModelSummary & {...}` intersection does not.
  type ModelRow = Omit<ModelSummary, never> & {
    size_bytes: number;
    endpoints_count: number;
  };

  const loading = $derived(models.status === 'loading');
  // Mounted only when active (App.svelte does the routing).

  // This panel owns the models store lifecycle. The models payload is
  // the heaviest (detailed+group), so it must only poll while Models is open.
  // start() fires an immediate tick on mount; stop() deactivates and aborts
  // any in-flight request on unmount.
  $effect(() => {
    models.start();
    return () => models.stop();
  });

  // Detailed+group form: model_groups is populated. Fall back to recent_models
  // if the backend didn't honour the query params.
  const groups = $derived(models.data?.model_groups ?? []);
  const flatRecent = $derived(models.data?.recent_models ?? []);

  // Re-evaluated each tick so last_seen_at stays "Xs ago" live.
  const now = $derived(liveNow());

  // Discovery-only columns. Per-model traffic figures (requests/success/
  // p95/p99) are intentionally absent: nothing on the proxy path populates
  // them, so they would always read zero and mislead the operator. Wiring
  // that up on the proxy path is deferred to a later change.
  const columns: Column[] = [
    { key: 'name', label: 'Model', sortable: true, sticky: true },
    { key: 'params', label: 'Params', sortable: true },
    { key: 'quant', label: 'Quant', sortable: true },
    { key: 'size_bytes', label: 'Size', sortable: true, num: true, align: 'right' },
    { key: 'endpoints_count', label: 'Endpoints', sortable: true, num: true, align: 'right' },
    { key: 'last_seen_at', label: 'Last seen', sortable: false },
  ];

  // Derive a numeric size for sort/aria from the model's own size string.
  // Previously this preferred a summed per_endpoint bytes total, but
  // per_endpoint has no prior art in the backend response and was cut.
  function sizeBytesOf(m: ModelSummary): number {
    if (!m.size) return 0;
    const u: Record<string, number> = { B: 1, KB: 1024, MB: 1024 ** 2, GB: 1024 ** 3, TB: 1024 ** 4, PB: 1024 ** 5 };
    const match = String(m.size).match(/^([\d.]+)\s*(B|KB|MB|GB|TB|PB)$/);
    return match ? parseFloat(match[1]) * u[match[2]] : 0;
  }

  function normalise(m: ModelSummary): ModelRow {
    return {
      ...m,
      size_bytes: sizeBytesOf(m),
      endpoints_count: m.endpoints?.length ?? 0,
    };
  }

  // Pill click-through delegates to the shared jumpToEndpoint helper
  // (lib/jump-to-endpoint). Endpoint display names are legitimately
  // non-unique (EndpointsPanel.dup-names.test), so the target must be keyed
  // on the per-model endpoint_ids array, not a name->id lookup. The backend
  // sorts endpoint_ids positionally against endpoints, so endpoints[i] is
  // hosted on endpoint_ids[i]. The helper owns the retry/scroll/focus/flash
  // concerns; see its header comment for the first-navigation fetch race.

  // Group order: alphabetical, with "unknown" pushed to the end (matches backend).
  function groupOrder(g: typeof groups): typeof groups {
    return [...g].sort((a, b) => {
      if (a.family === 'unknown') return 1;
      if (b.family === 'unknown') return -1;
      return a.family < b.family ? -1 : a.family > b.family ? 1 : 0;
    });
