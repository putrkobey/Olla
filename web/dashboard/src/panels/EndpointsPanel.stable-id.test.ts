import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';
import { endpoints } from '../lib/stores/endpoints.svelte';
import EndpointsPanel from './EndpointsPanel.svelte';

// Regression coverage: sanitiseDisplayURL (server-side) strips query and
// fragment before `url` reaches the frontend, so two distinct endpoints that
// differ only by query string (e.g. two API keys against the same host)
// arrive with an IDENTICAL `url` field. The panel used to key rows on
// `row.url ?? row.name`, so keying on url alone would collide these two rows
// under the same each-key, exactly the each_key_duplicate failure mode the
// dup-names/keys regressions already cover for `name`.
//
// The backend now emits a stable `id` derived from the raw (pre-sanitisation)
// URL - see stableEndpointID in handler_status_endpoints.go - specifically so
// the frontend has an identity that survives sanitisation. This test proves
// the panel actually uses it: distinct rows, distinct DOM ids, for two
// endpoints sharing a display url.

let component: ReturnType<typeof mount> | undefined;
afterEach(() => {
  if (component) unmount(component);
  document.body.innerHTML = '';
});

function jsonResponse(body: unknown) {
  return {
    status: 200,
    ok: true,
    headers: { get: () => null },
    json: async () => body,
  };
}

