import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';
import { endpoints } from '../lib/stores/endpoints.svelte';
import EndpointsPanel from './EndpointsPanel.svelte';

// C7 regression: PctBar derived its colour purely from success_rate_num and
// request_count, with no reference to endpoint status. An OFFLINE endpoint
// with lifetime counters therefore rendered a full green success bar beside
// its red offline pill - confidently advertising 100% success for a backend
// that is down. The fix gates the bar's COLOUR on status: an offline endpoint
// with history renders the neutral bucket, not green.
//
// A follow-up regression: the first attempt at that fix gated the FIGURE
// away too (hasData && !down), so an offline endpoint with 10,000 requests
// and an endpoint that never took a single request both rendered "no data" -
// pixel-identical, throwing away real history exactly when an operator needs
// it. The fix distinguishes "no data" (never traded) from "historical"
// (down, but we have a real figure): the latter keeps the number and real
// fill width, muted, with no green/amber/red implication.

let component: ReturnType<typeof mount> | undefined;
afterEach(() => {
  if (component) unmount(component);
  document.body.innerHTML = '';
});

