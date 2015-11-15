// Single ticking instant for any component that needs to re-render
// time-derived values (relative "Xs ago", uptime, "in 30s"). Status
// surfaces render absolute instants from the API as relative strings, and
// those go stale every second; rather than give each panel its own
// setInterval, this module is the one shared clock - the poll scheduler
// owns all timers in the SPA. Reads via now() are reactive when consumed
// inside $derived.

import { pollScheduler } from './poll-scheduler';

const CLOCK_INTERVAL_MS = 1000;
const CLOCK_JOB_NAME = 'clock';
