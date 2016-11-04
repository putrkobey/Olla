// Shared "jump to endpoint" helper used by Overview's glance table, Models'
// endpoint pills, and the URL-restore path. Swaps to the Endpoints panel,
// waits for the target row to render (covering the first-navigation fetch
// race), then scrolls it into view, focuses it, and applies a brief
// accent-tint flash so the operator can spot which row was the target amid a
// long fleet.
//
// Both call sites used to carry their own copy of this logic; the duplicate
// drifted (Overview omitted the retry loop, so its first jump after a cold
// load silently no-op'd). One helper closes that gap.

import { tick } from 'svelte';
import { stableId } from './dom-id';
import { navigation } from './stores/navigation.svelte';
import { endpoints } from './stores/endpoints.svelte';
import { pushRoute } from './router';

const EP_PREFIX = 'ep-';
const FLASH_CLASS = 'ep-flash';
// Hold the tint for ~1s then fade over the rest; the matching CSS animation
// in components.css uses the same 1.8s envelope. Couple them via a single
// constant so the class is removed exactly when the animation lands on its
// transparent end-state, never while the tint is still visible.
const FLASH_HOLD_MS = 1800;
// First-navigation race: EndpointsPanel starts its store on mount and the
// fetch is async, so the target row may not exist immediately after the panel
// swap. ~8 tries x ~50ms caps the wait near 400ms and returns the instant
// the row appears, without a busy windmill.
