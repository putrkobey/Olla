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
