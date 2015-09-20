<script module lang="ts">
  // Pure helper exported so the pointer-to-index mapping is unit-testable
  // without a DOM. Maps a 0..1 fraction (pointer x relative to chart width)
  // to the index of the nearest sample in an n-element series.
  export function nearestSampleIndex(fraction: number, n: number): number {
    if (n <= 0) return -1;
    if (n === 1) return 0;
    const clamped = Math.max(0, Math.min(1, fraction));
    return Math.round(clamped * (n - 1));
  }
</script>

<script lang="ts">
  // Hand-rolled inline-SVG sparkline with a req/s y-axis scale, a pointer
  // hover reading, and outage bands (a muted-red wash per run of failed-poll
  // ticks, with a thin red edge at the drop and recovery points). No chart
  // library (hard rule).
  //
  // Sampling is driven by the overview store's onTick callback wired in
  // overview.svelte.ts, which feeds the history ring buffer on BOTH success
  // and failure. This component is purely a view of history.samples; it does
  // not sample or poll itself.

  import { history, type Sample } from '../lib/stores/history.svelte';
  import { fmtInt, fmtDuration } from '../lib/format';

  interface Props {
    // Injectable for tests; production reads the singleton store.
    samples?: Sample[];
  }
  let { samples }: Props = $props();

  const view = $derived<Sample[]>(samples ?? history.samples);

  // viewBox dimensions: arbitrary because preserveAspectRatio="none" stretches
  // the drawing to the container. The 100x30 box gives the path math a clean
  // range; rendered height is fixed by .spark-svg so there is no layout shift.
  const W = 100;
  const H = 30;

  const step = $derived(view.length >= 2 ? W / (view.length - 1) : 0);
  const maxReq = $derived(Math.max(1, ...view.map((s) => s.reqPerSec)));
  const maxConn = $derived(Math.max(1, ...view.map((s) => s.activeConnections)));

  // Ceiling label for the req/s axis: one decimal under 10, integer above.
  const maxReqLabel = $derived(maxReq < 10 ? maxReq.toFixed(1) : Math.round(maxReq).toString());

  // Area wash: one closed sub-path per run of consecutive non-error samples.
  // Error samples produce a visual gap (the run is closed at the boundary)
  // rather than bridging the outage with fabricated data.
  function buildArea(list: Sample[], maxR: number): string {
    const n = list.length;
    if (n < 2) return '';
    const s = W / (n - 1);
    let d = '';
    let runStartX = 0;
    let lastX = 0;
    let inRun = false;
    for (let i = 0; i < n; i++) {
      const sample = list[i];
      if (sample.error) {
        if (inRun) {
          d += ` L ${lastX} ${H} L ${runStartX} ${H} Z`;
          inRun = false;
        }
        continue;
      }
      const x = i * s;
      const y = H - (sample.reqPerSec / maxR) * H;
      if (!inRun) {
        d += ` M ${x} ${y}`;
        runStartX = x;
        inRun = true;
      } else {
        d += ` L ${x} ${y}`;
      }
      lastX = x;
    }
    if (inRun) {
      d += ` L ${lastX} ${H} L ${runStartX} ${H} Z`;
    }
    return d.trim();
  }

  // Stroke path (edge line or connections): open sub-paths per run of
  // non-error samples. A lone M from a single-point run draws nothing.
  function buildLine(list: Sample[], pick: (s: Sample) => number, max: number): string {
    const n = list.length;
    if (n < 2) return '';
    const s = W / (n - 1);
    let d = '';
    let inRun = false;
    for (let i = 0; i < n; i++) {
      const sample = list[i];
      if (sample.error) {
        inRun = false;
        continue;
