import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';
import { endpoints } from '../lib/stores/endpoints.svelte';
import EndpointsPanel from './EndpointsPanel.svelte';

// Regression coverage for finding 6: the panel used to derive the RangeBar's
// average from `response_time` (parseInt on the last HEALTH-CHECK probe's
// formatted latency string) instead of the API's own `avg_latency_ms`
// (average PROXY request latency - a different metric entirely). A 1500ms
// health check formatted as "1.5s" and parseInt("1.5s", 10) silently
// produced 1, so the bar rendered a 1ms average for an endpoint that took
// 1.5 SECONDS to answer its last health check.
//
// Note: deliberately static imports, no vi.resetModules() - the `endpoints`
// store is a module singleton, and resetting modules mid-file desyncs
// Svelte's internal client runtime from the already-mounted component.

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

async function refreshWithEndpoint(endpoint: Record<string, unknown>) {
  global.fetch = vi.fn(async () =>
    jsonResponse({ endpoints: [endpoint], total_count: 1, healthy_count: 1, routable_count: 1 })
  );
  endpoints.refresh();
  // Wait for this specific endpoint's payload, not just status === 'ok' -
  // the store is a module singleton reused across tests, so status may
  // already read 'ok' from a previous test before this refresh resolves.
  await vi.waitFor(() => expect(endpoints.data?.endpoints?.[0]?.name).toBe(endpoint.name));
  flushSync();
}

async function refreshWithEndpoints(endpointList: Record<string, unknown>[]) {
  global.fetch = vi.fn(async () =>
    jsonResponse({
      endpoints: endpointList,
      total_count: endpointList.length,
      healthy_count: endpointList.length,
      routable_count: endpointList.length,
    })
  );
  endpoints.refresh();
  const lastName = endpointList[endpointList.length - 1].name;
  await vi.waitFor(() =>
    expect(endpoints.data?.endpoints?.[endpointList.length - 1]?.name).toBe(lastName)
  );
  flushSync();
}

describe('EndpointsPanel latency column', () => {
  it('uses avg_latency_ms, not a parse of response_time, for the RangeBar average', async () => {
    component = mount(EndpointsPanel, { target: document.body });
    flushSync();

    await refreshWithEndpoint({
      name: 'ollama-1',
      type: 'ollama',
