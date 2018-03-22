<script lang="ts">
  import './app.css';
  import DashboardLayout from './layout/DashboardLayout.svelte';
  import OverviewPanel from './panels/OverviewPanel.svelte';
  import EndpointsPanel from './panels/EndpointsPanel.svelte';
  import ModelsPanel from './panels/ModelsPanel.svelte';
  import { pollScheduler } from './lib/poll-scheduler';
  import { startClock } from './lib/clock.svelte';
  import { theme } from './lib/stores/theme.svelte';
  import { navigation } from './lib/stores/navigation.svelte';
  // Overview is always-on (StatusStrip renders on every panel), so App owns
  // its lifecycle. Endpoints/models stores are started/stopped by their panels.
  import { overview } from './lib/stores/overview.svelte';
  // Module side-effect: registers endpoints/models jobs with the scheduler.
  import './lib/stores/endpoints.svelte';
  import './lib/stores/models.svelte';
