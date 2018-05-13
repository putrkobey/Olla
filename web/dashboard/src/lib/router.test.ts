import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
  parseHash,
  serializeRoute,
  routeToHash,
  pushRoute,
  navigatePanel,
  startRouter,
  currentRoute,
} from './router';
import { navigation } from './stores/navigation.svelte';

describe('router parse/serialize', () => {
  it('round-trips every panel-only route', () => {
    for (const panel of ['overview', 'endpoints', 'models'] as const) {
      const r = { panel };
      expect(parseHash(routeToHash(r))).toEqual(r);
    }
  });

  it('round-trips an endpoint-targeted route', () => {
    const r = { panel: 'endpoints' as const, endpointKey: 'abc123' };
    expect(serializeRoute(r)).toBe('endpoints/ep-abc123');
    expect(parseHash('#endpoints/ep-abc123')).toEqual(r);
  });

  it('defaults to overview on empty or bare hash', () => {
