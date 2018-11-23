import { describe, it, expect, beforeAll, beforeEach, afterEach, vi } from 'vitest';
import { jumpToEndpoint, jumpToEndpointKey, flashRow } from './jump-to-endpoint';
import { stableId } from './dom-id';
import { navigation } from './stores/navigation.svelte';

// The cold deep-link path branches on endpoints.hasData. The real store is a
// module singleton that none of these tests start, so by default we report
// hasData=true to exercise the fast DOM-poll path that the existing tests
// cover; the cold-link test flips this to false via the same holder.
const epStore = vi.hoisted<{ hasData: boolean }>(() => ({ hasData: true }));
vi.mock('./stores/endpoints.svelte', () => ({
  endpoints: {
    get hasData() {
      return epStore.hasData;
    },
  },
}));

// jsdom doesn't implement scrollIntoView; stub it so the helper doesn't throw
// and so the tests can assert it was actually called.
beforeAll(() => {
  Element.prototype.scrollIntoView = vi.fn();
});

beforeEach(() => {
  document.body.innerHTML = '';
  navigation.set('overview');
  history.replaceState(null, '', '#overview');
  epStore.hasData = true;
  // scrollIntoView is stubbed once in beforeAll; clear the call log so each
  // test's "was called / was not called" assertion sees only its own jumps.
  vi.mocked(Element.prototype.scrollIntoView).mockClear();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('flashRow', () => {
  it('adds the flash class, then removes it once the hold elapses', () => {
    vi.useFakeTimers();
    const row = document.createElement('tr');
    document.body.append(row);

    flashRow(row);
    expect(row.classList.contains('ep-flash')).toBe(true);

    vi.advanceTimersByTime(2000);
    expect(row.classList.contains('ep-flash')).toBe(false);
  });

  it('clears an existing flash on another row before lighting the new one', () => {
    vi.useFakeTimers();
    const a = document.createElement('tr');
    const b = document.createElement('tr');
    document.body.append(a, b);

    flashRow(a);
    expect(a.classList.contains('ep-flash')).toBe(true);

    flashRow(b);
    expect(a.classList.contains('ep-flash')).toBe(false);
    expect(b.classList.contains('ep-flash')).toBe(true);
  });

  it('re-flashes the same row on a second call mid-fade', () => {
    vi.useFakeTimers();
    const row = document.createElement('tr');
    document.body.append(row);

    flashRow(row);
    vi.advanceTimersByTime(1000);
    expect(row.classList.contains('ep-flash')).toBe(true);

