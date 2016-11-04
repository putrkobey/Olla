<script lang="ts">
  // `avg` is deliberately nullable: it's the average PROXY request latency,
  // a separate metric from `min`/`max` (also proxy latency) that may be
  // absent on a backend that hasn't shipped it yet, or null on an endpoint
  // with no traffic yet. Never derive it from a health-check figure -
  // health-check latency (response_time) measures a different thing
  // entirely (see EndpointsPanel/OverviewPanel).
  import { fmtMs, scalePct } from '../lib/format';

  interface Props {
    min?: number;
    avg?: number | null;
    max?: number;
    globalMax?: number;
    label?: string;
  }
  let { min = 0, avg = null, max = 0, globalMax = 0, label = 'latency' }: Props = $props();

  const noData = $derived(!max);
  // Narrow avg to a finite number up front so the downstream derives keep a
  // `number` type without re-checking at every use site.
  const avgNum = $derived(typeof avg === 'number' && Number.isFinite(avg) ? avg : null);
  const hasAvg = $derived(avgNum !== null);
  const fillPct = $derived(noData || avgNum === null ? 0 : scalePct(avgNum, globalMax));
  const tickPct = $derived(noData ? 0 : scalePct(max, globalMax));
  const avgText = $derived(avgNum === null ? '—' : fmtMs(avgNum));

  const ariaLabel = $derived(
