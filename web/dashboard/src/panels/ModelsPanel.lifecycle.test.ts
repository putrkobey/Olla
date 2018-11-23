import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';
import { pollScheduler } from '../lib/poll-scheduler';
import ModelsPanel from './ModelsPanel.svelte';

// Acceptance: the models store is activated with the scheduler ONLY
// while the Models panel is mounted. The heaviest payload
// (models?detailed=true&group=family) must stop firing the moment the
// operator switches to Overview or Endpoints.

global.fetch = vi.fn(async () => ({
  status: 200,
