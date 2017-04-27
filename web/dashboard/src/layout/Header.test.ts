import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';
import Header from './Header.svelte';
import { startClock } from '../lib/clock.svelte';
import { pollScheduler } from '../lib/poll-scheduler';

// Regression coverage for finding 13: Header ran its own setInterval(...,
// 1000) instead of reading the shared clock, contradicting the stated
// invariant (App.svelte / clock.svelte.js) that the scheduler owns every
