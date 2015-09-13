import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';
import { overview } from '../lib/stores/overview.svelte';
import { endpoints } from '../lib/stores/endpoints.svelte';
import OverviewPanel from '../panels/OverviewPanel.svelte';
// Vite's ?raw suffix inlines the file's source as a string; the ambient
// declaration comes from src/vite-env.d.ts (vite/client types).
import statTileSrc from './StatTile.svelte?raw';

// C4 regression: StatTile exposed valueHtml/subHtml props that bypassed
// Svelte's escaping via {@html}. Inert today (only closed server-controlled
// enums/integers flow through them) but an opt-in XSS footgun on a shared
// component. The fix expresses the same rich markup through Svelte snippets,
// which compose into the DOM as normal content (escaped, no raw HTML sink).

let component: ReturnType<typeof mount> | undefined;
afterEach(() => {
  if (component) unmount(component);
  document.body.innerHTML = '';
});

// Partial Response shape the mock returns; only the fields the stores read.
type MockResponse = {
  status: number;
  ok: boolean;
  headers: { get: () => string | null };
  json: () => Promise<unknown>;
};

describe('StatTile has no {@html} sink', () => {
  // Fail-first: the current StatTile source carries {@html} and the
  // valueHtml/subHtml props, so this assertion fails until the component is
  // migrated to snippets.
  it('source contains no {@html} tags and no valueHtml/subHtml props', () => {
    expect(statTileSrc).not.toMatch(/\{@html/);
    expect(statTileSrc).not.toMatch(/valueHtml/);
    expect(statTileSrc).not.toMatch(/subHtml/);
  });
