<script lang="ts">
  // Tab presentation over the shared navigation store. Panels never import
  // this store directly, only App.svelte's router and this component do -
  // that indirection is what would let a sidebar presenter replace tabs
  // later without any panel change. navigatePanel also pushes a history
  // entry so browser back/forward traverses panel switches.
  import { navigation, type Section } from '../lib/stores/navigation.svelte';
  import { navigatePanel } from '../lib/router';

  const LABELS: Record<Section, string> = { overview: 'Overview', endpoints: 'Endpoints', models: 'Models' };

  function activate(name: Section): void {
    navigatePanel(name);
    // Move focus to the newly-active tab once the DOM has updated.
    queueMicrotask(() => {
      document.getElementById(`tab-${name}`)?.focus();
