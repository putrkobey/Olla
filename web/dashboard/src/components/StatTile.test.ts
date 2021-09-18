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
