import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';
import { endpoints } from '../lib/stores/endpoints.svelte';
import { models } from '../lib/stores/models.svelte';
import { overview } from '../lib/stores/overview.svelte';
import EndpointsPanel from './EndpointsPanel.svelte';
import ModelsPanel from './ModelsPanel.svelte';
import OverviewPanel from './OverviewPanel.svelte';

// Regression coverage for the row-key collision bug class fixed piecemeal in
// eb14006 (Endpoints/Models) and again here (Overview's glance table). Any
// SortableTable consumer that keys its each-block on a lossy CSS-safe slug,
// rather than the row's exact unique name, blanks its ENTIRE table body the
// moment two rows collide once slugged - Svelte's each_key_duplicate, with no
// error boundary anywhere in the app to contain it. "node.a" and "node-a"
// both slug to "node-a"; that pair is the fixture used throughout.
//
// Parameterised across every SortableTable consumer (Endpoints, Models
// grouped, Models flat, Overview glance) rather than one test per panel, so a
// future table added without unique rowId coverage fails here automatically
// instead of shipping the same bug a fourth time.

const COLLIDING_NAMES = ['node.a', 'node-a'];

function jsonResponse(body: unknown): {
  status: number;
  ok: boolean;
  headers: { get: () => string | null };
  json: () => Promise<unknown>;
} {
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
    endpoints_up: '2/2',
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

function endpointNamed(name: string) {
  return {
    name,
    type: 'ollama',
    status: 'healthy',
    priority: 100,
    success_rate: '99.0%',
    request_count: 10,
    model_count: 1,
    avg_latency_ms: 20,
    min_latency_ms: 5,
    max_latency_ms: 40,
  };
}

function modelNamed(name: string) {
  return {
    name,
    params: '8B',
    quant: 'q4_k_m',
    size: '4.5 GB',
    endpoints: ['ollama-1'],
    last_seen_at: new Date().toISOString(),
  };
}

// Component type is intentionally loose: the panels carry their own prop
// shapes, and the harness only mounts with a target (no props).
type AnyPanel = new (...args: unknown[]) => unknown;
interface Case {
  label: string;
  Component: AnyPanel;
  seed(): Promise<void>;
  renderedNames(): string[];
}

const cases: Case[] = [
  {
    label: 'EndpointsPanel',
    Component: EndpointsPanel as unknown as AnyPanel,
    async seed() {
      global.fetch = vi.fn(async () =>
        jsonResponse({
          endpoints: COLLIDING_NAMES.map(endpointNamed),
          total_count: 2,
          healthy_count: 2,
          routable_count: 2,
