<script lang="ts">
  import { theme } from '../lib/stores/theme.svelte';

  // One icon-only button that cycles auto -> light -> dark -> auto. The
  // glyph reflects `mode` (not `resolved`) so each step is a visible change
  // even when the resolved theme happens to match the next mode. The store
  // owns the mode, persistence and OS-preference reactivity; this component
  // only renders the current state and forwards clicks to cycle().
  const byMode: Record<string, { icon: string; label: string }> = {
    auto: { icon: '◐', label: 'Theme: auto (follows system). Click for light.' },
    light: { icon: '☀', label: 'Theme: light. Click for dark.' },
    dark: { icon: '☾', label: 'Theme: dark. Click for auto.' },
