// /internal/status/endpoints poller. Drives EndpointsPanel + Overview's herd glance.
import type { EndpointStatusResponse } from '../types';
import { createPollStore } from './poll-store.svelte';
