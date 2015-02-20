import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach } from 'vitest';
import Fixture from './SortableTable.grouped-fixture.svelte';
import type { Column } from './SortableTable.svelte';

// Regression coverage for finding 8: the Models panel's grouped view passed
// rows={[]} to SortableTable and rendered its own closure-local, never-sorted
// row list in groupSnippet. Clicking a header flipped the indicator and
// aria-sort, but the DOM order never moved - also an a11y defect, since
// aria-sort then announced a state the rows didn't reflect.

interface FixtureRow extends Record<string, unknown> {
  id: string;
  name: string;
  n: number;
}
interface FixtureGroup {
  id: string;
  rows: FixtureRow[];
}

const columns: Column[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'n', label: 'Count', sortable: true, num: true },
];

// Values chosen so lexicographic string sort ("100" < "2" < "9") and numeric
// sort (2 < 9 < 100) disagree - this is what catches a sort that silently
// fell back to comparing formatted strings.
const groups: FixtureGroup[] = [
  {
    id: 'alpha',
    rows: [
      { id: 'a-100', name: 'a-100', n: 100 },
      { id: 'a-9', name: 'a-9', n: 9 },
      { id: 'a-2', name: 'a-2', n: 2 },
    ],
  },
  {
    id: 'beta',
    rows: [
      { id: 'b-50', name: 'b-50', n: 50 },
      { id: 'b-1', name: 'b-1', n: 1 },
    ],
  },
];

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  document.body.innerHTML = '';
});
