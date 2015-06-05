import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

// Regression coverage for finding 7: refresh(name) (the "retry now" button)
// ran the job immediately without clearing the timer already queued by the
// previous #scheduleNext. The old timer still fired later, and since #run's
// finally unconditionally rescheduled, that produced a second live recurring
// chain - one extra permanent chain per manual retry, with no way to cancel
// the leaked ones.

beforeEach(() => {
  vi.resetModules();
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('pollScheduler.refresh', () => {
  it('a burst of manual refreshes leaves exactly one live recurring chain', async () => {
    const { pollScheduler } = await import('./poll-scheduler');

    let calls = 0;
    const tick = vi.fn(async () => {
      calls++;
    });
    pollScheduler.register('job', 1000, tick);
    pollScheduler.start();
    pollScheduler.start('job');

    // start() fires an immediate tick (delay 0).
    await vi.advanceTimersByTimeAsync(0);
    expect(calls).toBe(1);

    // Three manual "retry now" clicks in quick succession, each completing
    // before the next fires.
    pollScheduler.refresh('job');
    await vi.advanceTimersByTimeAsync(0);
    pollScheduler.refresh('job');
    await vi.advanceTimersByTimeAsync(0);
    pollScheduler.refresh('job');
    await vi.advanceTimersByTimeAsync(0);
    expect(calls).toBe(4);

    // Advance one full interval plus the maximum jitter window. With the bug,
    // each refresh leaked a duplicate chain, so this window would fire 4
    // more ticks (one per leaked chain); fixed, it fires exactly one.
    const before = calls;
    await vi.advanceTimersByTimeAsync(1200);
    expect(calls - before).toBe(1);

    pollScheduler.stop();
