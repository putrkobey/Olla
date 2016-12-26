<script module lang="ts">
  // Exported so callers (panels) can share the column / sort-state shape
  // without re-declaring it. Lives in module scope: instance scope cannot
  // hold `export` modifiers, and these types don't depend on `Row`.
  export type SortDirection = 'asc' | 'desc';
  export interface SortState {
    key: string;
    dir: SortDirection;
  }
  export interface Column {
    key: string;
    label: string;
    sortable?: boolean;
    num?: boolean;
    align?: 'right';
    sticky?: boolean;
    title?: string;
  }
</script>

<script lang="ts" generics="Row extends Record<string, unknown>">
  import type { Snippet } from 'svelte';
  import { untrack } from 'svelte';

  // Generic column-driven table. Children are rendered via the `row` snippet,
  // receiving the sorted row. Column sort state is owned here per-table
  // instance; the parent only describes the columns and provides the data.
  //
  // `columns`: [{ key, label, sortable=false, num=false, align, sticky=false, title }]
  //   `title` is an optional caveat rendered as the header's native tooltip -
  //   e.g. qualifying a metric's real meaning when the label alone doesn't
  //   have room to. `num` is sort semantics only (numeric compare). `align` is the
  //   presentation concern ('right' today) and is the ONLY thing that drives
  //   right-alignment, on both the header and the cell - see `cellClass`
  //   below. Keeping these separate means a future composite column (a bar,
  //   chips) declares `align: 'right'` and gets a real alignment mechanism
  //   (margin-left: auto on its flex child, via components.css) instead of
  //   text-align, which only ever moved inline text and silently did nothing
  //   for block/flex content (C10).
  // `rows`: array of objects keyed by column.key
  // `initialSort`: { key, dir: 'asc'|'desc' } or null
  // `rowId`: (row, i) => string, used as the tr KEY (defaults to index). Must
  //   be the row's exact unique identity - never a lossy transform (e.g. a
  //   CSS-safe slug), or two rows that collapse to the same slug trigger
  //   Svelte's each_key_duplicate and blank the entire table body.
  // `rowDomId`: optional (row, i) => string, used as the tr's `id` ATTRIBUTE
  //   in the generic rowSnippet path (e.g. so another panel can scrollIntoView
  //   + focus a specific row). Deliberately separate from `rowId`: the DOM id
  //   can use a CSS-safe slug since a jump target only needs to exist, not be
  //   collision-proof, but that same slug must never be used as the each key.
  // `rowSnippet`: snippet receiving { row } for each body row's <td> markup
  // `groupSnippet`: optional snippet rendered between family groups (Models
  //   panel), receiving { rows, sort, sortRows } - `sortRows(list)` applies
  //   the table's current comparator/sort state to a group's own row array,
  //   so the header click that flips `aria-sort` also reorders each group.

  interface Props {
    columns?: Column[];
    rows?: Row[];
    initialSort?: SortState | null;
    rowId?: (row: Row, i: number) => string;
    rowDomId?: ((row: Row, i: number) => string) | null;
    rowSnippet?: Snippet<[{ row: Row; cellClass: (key: string) => string }]> | null;
    groupSnippet?:
      | Snippet<
          [
            {
              rows: Row[];
              sort: SortState | null;
              sortRows: (list: Row[]) => Row[];
              cellClass: (key: string) => string;
            }
          ]
        >
      | null;
