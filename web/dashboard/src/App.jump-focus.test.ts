import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, beforeAll, vi } from 'vitest';
import App from './App.svelte';
import { navigation } from './lib/stores/navigation.svelte';
import { overview } from './lib/stores/overview.svelte';
import { endpoints } from './lib/stores/endpoints.svelte';
import { stableId } from './lib/dom-id';

// Regression coverage: "jump to endpoint" on the Overview glance table looked
// up `ep-<slug>` in the DOM, but the generic row path in SortableTable never
// set an `id` on its <tr>, so the lookup always returned null. scrollIntoView
// never ran, and since the panel swap unmounts OverviewPanel entirely
// (App.svelte's {#if}), the clicked button was destroyed and keyboard focus
// fell through to <body> with nothing to replace it.

// jsdom doesn't implement scrollIntoView; stub it so the fixed code doesn't
// throw and so the test can assert it was actually called.
beforeAll(() => {
  Element.prototype.scrollIntoView = vi.fn();
});

let component: ReturnType<typeof mount> | undefined;
afterEach(() => {
  if (component) unmount(component);
  document.body.innerHTML = '';
  navigation.set('overview');
  history.replaceState(null, '', '#overview');
});

function jsonResponse(body: unknown) {
  return {
    status: 200,
    ok: true,
    headers: { get: () => null },
    json: async () => body,
  };
}

const sysBody = {
  system: {
    status: 'healthy',
    endpoints_up: '1/1',
    success_rate: '99.0%',
    avg_latency: '20ms',
    total_traffic: '1.00 GB',
    active_connections: 0,
    total_requests: 10,
    total_failures: 0,
    security_violations: 0,
    version: 'v0.0.29',
    commit: 'abc123',
    start_time: new Date().toISOString(),
  },
  proxy: { engine: 'olla', balancer: 'priority' },
};

async function seedData() {
  global.fetch = vi.fn(async (url) => {
    if (String(url).includes('/internal/status/endpoints')) {
      return jsonResponse({
        endpoints: [
          {
            name: 'ollama-1',
            url: 'http://ollama-1:11434',
            type: 'ollama',
