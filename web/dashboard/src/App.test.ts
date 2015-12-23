import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';
import App from './App.svelte';
import { navigation } from './lib/stores/navigation.svelte';

// Regression coverage for finding 11: App.svelte used to hold its own
// `current` state, separate from the `navigation` store NavTabs renders
// aria-selected/tabindex/arrow-key position from. Clicking an endpoint row
// on Overview (a programmatic jump) rendered the Endpoints panel while the
// tab bar kept announcing "Overview, selected" and held keyboard focus
// position there - it only self-healed on the next explicit tab click.

// App.svelte starts the poll scheduler and fetches on mount; stub fetch so
// that doesn't error out mid-test (the panels aren't what's under test here).
global.fetch = vi.fn(async () => ({
  status: 200,
  ok: true,
  headers: { get: () => null },
  json: async () => ({}),
}));

let component: ReturnType<typeof mount> | undefined;
afterEach(() => {
  if (component) unmount(component);
  document.body.innerHTML = '';
  navigation.set('overview'); // leave the singleton store clean for other test files
});

describe('App navigation authority', () => {
  it('a programmatic jump updates both the rendered panel and the tab bar state', () => {
    component = mount(App, { target: document.body });
    flushSync();

    expect(document.getElementById('panel-overview')).not.toBeNull();
    expect(document.getElementById('tab-overview')!.getAttribute('aria-selected')).toBe('true');

    // Simulate a programmatic jump the way OverviewPanel's "jump to
    // endpoint" link does, via the same navigation authority App uses.
    navigation.set('endpoints');
    flushSync();

