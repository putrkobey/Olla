<script lang="ts">
  import ThemeToggle from '../components/ThemeToggle.svelte';
  import ollaThumb from '../assets/logo/olla-thumb.png';
  import { getNow } from '../lib/clock.svelte';

  // Live clock ticking once per second, grounding the operator that "this
  // view is live, the data is from N seconds ago". Sourced from the shared
  // clock rather than a component-local setInterval, so it gets the
  // visibility backoff for free like every other live-updating value.
  const clockText = $derived(
    new Date(getNow()).toLocaleTimeString('en-AU', { hour12: false })
  );
</script>
