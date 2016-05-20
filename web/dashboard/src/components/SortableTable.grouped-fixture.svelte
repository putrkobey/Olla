<script lang="ts">
  // Test-only fixture: exercises SortableTable's grouped (groupSnippet) path
  // with real Svelte snippet syntax, standing in for ModelsPanel's grouped
  // view without dragging in the whole panel + its stores. See
  // SortableTable.test.ts.
  import SortableTable from './SortableTable.svelte';
  import type { Column } from './SortableTable.svelte';

  interface FixtureRow extends Record<string, unknown> {
    id: string;
    name: string;
    n: number;
  }
  interface FixtureGroup {
    id: string;
    rows: FixtureRow[];
  }

  interface Props {
    columns: Column[];
