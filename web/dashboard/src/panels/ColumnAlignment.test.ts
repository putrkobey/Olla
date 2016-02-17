import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';
import { endpoints } from '../lib/stores/endpoints.svelte';
import { models } from '../lib/stores/models.svelte';
import EndpointsPanel from './EndpointsPanel.svelte';
import ModelsPanel from './ModelsPanel.svelte';

// C10 regression: `td.num { text-align: right }` only positions INLINE content.
// The Success (PctBar), Latency (RangeBar) and Endpoints (chips) cells contain
// block/flex children that ignore text-align, so the header right-aligned
// while the cell content stayed hard left - a wide dead gap. The fix
// separates `num` (sort semantics) from alignment (presentation): composite
// cells carry an explicit `align-right` marker whose CSS pushes their flex
// children right via auto margin. jsdom does not lay out flex, so this asserts
// the structural decision; visual confirmation is deferred to the browser
// pass (WP6).
//
// Follow-up (post-remediation review): `column.align` was originally added to
// the column definitions but never read anywhere - the `align-right` class on
// each <td> was a hand-authored literal, so the field was documentation-only
// and a future composite column could still be added without the guard doing
// anything. `SortableTable` now derives both the header's and the cell's
// class from `column.align` via a `cellClass` helper handed to the row/group
// snippets, so panels no longer author "num align-right" literals at all -
// asserted below via the header class alongside the cell class.

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

describe('composite numeric cells carry an explicit alignment class', () => {
  it('marks the Success and Latency cells align-right in EndpointsPanel', async () => {
    component = mount(EndpointsPanel, { target: document.body });
    flushSync();

    global.fetch = vi.fn(async () =>
      jsonResponse({
        endpoints: [
          {
            name: 'ollama-1',
            url: 'http://ollama-1:11434',
            type: 'ollama',
            status: 'healthy',
            priority: 100,
            success_rate: '99.0%',
            request_count: 10,
            model_count: 1,
            avg_latency_ms: 20,
            min_latency_ms: 5,
            max_latency_ms: 40,
          },
        ],
        total_count: 1,
        healthy_count: 1,
        routable_count: 1,
      })
    );
    endpoints.refresh();
    await vi.waitFor(() => expect(endpoints.data?.endpoints?.[0]?.name).toBe('ollama-1'));
    flushSync();

    const row = document.querySelector('tbody tr')!;
    const tds = [...row.querySelectorAll('td')];

    const respRateCell = tds.find((td) => td.querySelector('.pct-cell'))!;
