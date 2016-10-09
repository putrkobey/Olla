<script lang="ts">
  import { overview } from '../lib/stores/overview.svelte';
  import { fmtUptime } from '../lib/format';
  import { getNow as liveNow } from '../lib/clock.svelte';
  import type { EndpointResponse } from '../lib/types';

  // The strip renders from whatever the overview store last saw. Before the
  // first poll lands, render dashes so the operator can see "pending" rather
  // than a structurally empty header.
  const sys = $derived(overview.data?.system);

  // Without consulting overview.status the strip would display confident
  // numbers during an outage while the panels below report unreachable. The
  // data-state attribute mirrors the panels' pattern so CSS and assistive tech
  // can tell a live strip from a stale one, and the visible marker gives the
  // operator a non-numeric signal that the figures are frozen.
  const stateStatus = $derived(
    overview.status === 'stale' || overview.status === 'error' ? overview.status : null
  );
  const staleWord = $derived(
    overview.status === 'stale' ? 'stale' : overview.status === 'error' ? 'unreachable' : null
  );

  // No-traffic signal mirroring OverviewPanel: the has_traffic flag is
  // always present, with total_requests===0 as a fallback for older backends.
  // Used to drop the Resp. rate cell to a no-data dash so the strip matches
  // the OverviewPanel tile rather than showing a literal "N/A".
  const hasTraffic = $derived(sys?.has_traffic === true || (sys?.total_requests ?? 0) > 0);

  // "degraded" on its own tells the operator nothing actionable. Derive a
  // short reason from real endpoint data (offline count) rather than
  // fabricate one; when healthy or when we have no endpoint data yet, say
  // nothing so the strip stays quiet. Breaker state is deliberately not
