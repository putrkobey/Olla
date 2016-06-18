import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';
import { endpoints } from '../lib/stores/endpoints.svelte';
import EndpointsPanel from './EndpointsPanel.svelte';

// C8 regression: the Endpoints name cell rendered an inline badge-type span
// showing e.type AND a separate Type column also showing e.type - the same
// value twice per row. The fix drops the inline badge (the Type column is the
// canonical place for it).

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

async function refreshWith(endpoint: Record<string, unknown>) {
  global.fetch = vi.fn(async () =>
