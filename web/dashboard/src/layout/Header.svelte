<script lang="ts">
  import ThemeToggle from '../components/ThemeToggle.svelte';
  import ollaThumb from '../assets/logo/olla-thumb.png';
  import { getNow } from '../lib/clock.svelte';

  // Live clock ticking once per second, grounding the operator that "this
