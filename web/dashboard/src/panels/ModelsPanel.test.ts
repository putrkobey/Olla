import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';
import { models } from '../lib/stores/models.svelte';
import { stableId } from '../lib/dom-id';
import ModelsPanel from './ModelsPanel.svelte';

// Coverage: the Models panel was trimmed to discovery-only columns
// (name, params, quant, size, endpoints, last seen). Per-model traffic
// columns and the per_endpoint tooltip lookup were removed because nothing
// on the proxy path populates those figures and per_endpoint has no prior
// art. These tests pin the trim so a future revert is caught.

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

async function refreshWith(payload: Record<string, unknown>, ready: () => void) {
  global.fetch = vi.fn(async () => jsonResponse(payload));
  models.refresh();
  // Wait for the specific payload via a caller-supplied sentinel rather
  // than status alone: the store is a module singleton reused across
  // tests, so status may already read 'ok' from a previous test before
  // this refresh resolves.
  await vi.waitFor(() => ready());
  flushSync();
}

const groupedPayload = {
  model_groups: [
    {
      family: 'qwen',
      model_count: 2,
      endpoints: ['ollama-1'],
      models: [
        {
          name: 'qwen3:8b',
          params: '8B',
          quant: 'q4_k_m',
          size: '4.5 GB',
          endpoints: ['ollama-1'],
          last_seen_at: new Date().toISOString(),
          // Fields the trim removed must be ignored if a legacy backend
          // still sends them. Include them to prove the panel no longer
          // renders them regardless of wire shape.
          per_endpoint: [{ endpoint: 'ollama-1', parameter_size: '8B', size_bytes: 4831838208 }],
          total_requests: 999,
