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
