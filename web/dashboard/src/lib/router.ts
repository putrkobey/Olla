// URL hash router for the dashboard SPA. The hash carries the active panel
// and, when the operator jumped to a specific endpoint, the target row's
// stableId - so a refresh or shared link restores the exact view, and the
// browser's back/forward traverses panel switches and endpoint jumps in
// arrival order.
//
// Scheme (round-trippable via parseHash/serializeRoute):
//   #overview                  panel only
//   #endpoints                 panel only
//   #models                    panel only
//   #endpoints/ep-<stableId>   panel + targeted endpoint row
//
// Sync model: user-initiated nav (tab click, jump-to-endpoint) calls
// pushRoute, which uses history.pushState. pushState is silent, so the
// hashchange listener only fires on genuine history traversal (back/forward)
// or a manual URL edit, never on a click-jump - that's what keeps a click
// from double-triggering the restore path.

import { navigation, type Section, SECTIONS } from './stores/navigation.svelte';

export interface Route {
  panel: Section;
  // The endpoint row's stableId (the post-hash DOM key), present only when
  // the route pins a specific endpoint row. Stored as the bare key so the
  // restore path can getElementById(`ep-${endpointKey}`) without re-hashing.
  endpointKey?: string;
}

const EP_PREFIX = 'ep-';

// stableId emits [0-9a-z]+; reuse that as the validity gate so a malformed
// fragment (percent-encoding, stray slashes, a copied URL truncated mid-key)
// degrades to a panel-only route instead of masquerading as a target.
const KEY_RE = /^[0-9a-z]+$/;
