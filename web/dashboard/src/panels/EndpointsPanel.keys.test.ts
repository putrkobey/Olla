import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';
import { endpoints } from '../lib/stores/endpoints.svelte';
import EndpointsPanel from './EndpointsPanel.svelte';

// Regression coverage: two endpoints whose names differ only in punctuation
// ("node.a" / "node-a") both slugged to the same CSS-safe id ("node-a"). The
// panel used that lossy slug as the keyed-each identity, so Svelte's
// each_key_duplicate error fired inside a reactive effect and - with no error
// boundary anywhere in the app - blanked the ENTIRE table body, not just the
// colliding rows. Real-world trigger: EndpointConfig.Name is a free-form
// string with no charset restriction, so this is one YAML file away from
// happening in production.

let component: ReturnType<typeof mount> | undefined;
afterEach(() => {
  if (component) unmount(component);
  document.body.innerHTML = '';
