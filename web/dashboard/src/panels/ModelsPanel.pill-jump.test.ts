import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, beforeAll, vi } from 'vitest';
import { models } from '../lib/stores/models.svelte';
import { navigation } from '../lib/stores/navigation.svelte';
import { stableId } from '../lib/dom-id';
import ModelsPanel from './ModelsPanel.svelte';

// Coverage: endpoint pills click-through to the matching EndpointsPanel
// row by STABLE ID (not display name). Two endpoints can share a display name
// (EndpointsPanel.dup-names.test), so a name->id lookup is forbidden; the
// backend pairs endpoints[i] with endpoint_ids[i], and each pill resolves to
// ep-${stableId(endpoint_id)}.
//
// Production double-hash: the server emits endpoint_ids as ALREADY-hashed
// opaque tokens (Go's stableEndpointID(url), e.g. "w5c2gc"), and EndpointsPanel
// renders each row's DOM id as ep-${stableId(endpoint_id)} - i.e. the client
// applies stableId to the server's opaque id. So endpoint_ids in these tests
// carry distinct opaque tokens (sid-aaa / sid-bbb, NOT raw urls), and planted
// target rows use ep-${stableId('sid-aaa')} etc. This mirrors the wire path
// pill -> stableId(endpoint_id) -> row, and would catch a regression in either
// hash. The same-name collision case below must land on DISTINCT rows via the
// two distinct ids - the exact failure mode a name-keyed lookup would silently
// get wrong.

// jsdom doesn't implement scrollIntoView; stub it so the jump helper doesn't
// throw and so the test can assert it ran.
beforeAll(() => {
  Element.prototype.scrollIntoView = vi.fn();
});

// The shared jump helper now consults endpoints.hasData before DOM-polling for
// the target row (cold deep-link wait). These tests pre-plant the row and
// exercise the fast path, so report hasData=true to bypass the cold wait.
vi.mock('../lib/stores/endpoints.svelte', () => ({
  endpoints: {
    get hasData() {
      return true;
    },
  },
}));

let component: ReturnType<typeof mount> | undefined;
afterEach(() => {
  if (component) unmount(component);
  component = undefined;
  document.body.innerHTML = '';
  navigation.set('overview');
  history.replaceState(null, '', '#overview');
  vi.useRealTimers();
});

function jsonResponse(body: unknown) {
  return {
    status: 200,
    ok: true,
    headers: { get: () => null },
    json: async () => body,
  };
}

// The models store is a module singleton reused across tests, so a sentinel
// that only checks status (or a field like the model name that's identical
// between two tests) will resolve on the PREVIOUS test's stale data before
// this refresh lands. Each test uses a UNIQUE model name and the sentinel
// waits for that name, guaranteeing the new payload has committed.
async function refreshWith(payload: Record<string, unknown>, ready: () => void) {
  global.fetch = vi.fn(async () => jsonResponse(payload));
  models.refresh();
  await vi.waitFor(() => ready());
  flushSync();
}

describe('ModelsPanel endpoint pill click-through', () => {
  it('renders each pill as a button keyed on endpoint_ids, not display name', async () => {
    // Both endpoints share the SAME display name but have distinct ids. A
    // name-keyed lookup would resolve both pills to the same row.
    const payload = {
      model_groups: [
        {
          family: 'qwen',
          model_count: 1,
          endpoints: ['ollama', 'ollama'],
          models: [
            {
              name: 'collide-alpha',
              params: '8B',
              quant: 'q4_k_m',
              size: '4.5 GB',
              endpoints: ['ollama', 'ollama'],
              endpoint_ids: ['sid-aaa', 'sid-bbb'],
              last_seen_at: new Date().toISOString(),
