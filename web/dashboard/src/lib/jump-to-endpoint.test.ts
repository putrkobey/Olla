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

    flashRow(row);
    expect(row.classList.contains('ep-flash')).toBe(true);
    // The hold timer was reset, so the class is still present at what would
    // have been the original removal point.
    vi.advanceTimersByTime(1500);
    expect(row.classList.contains('ep-flash')).toBe(true);
    vi.advanceTimersByTime(500);
    expect(row.classList.contains('ep-flash')).toBe(false);
  });
});

describe('jumpToEndpoint', () => {
  it('swaps to endpoints, pushes the hash, scrolls, focuses and flashes a present row', async () => {
    const rawId = 'http://ollama-1:11434';
    const domId = `ep-${stableId(rawId)}`;
    const row = document.createElement('tr');
    row.id = domId;
    row.tabIndex = -1;
    document.body.append(row);

    await jumpToEndpoint(rawId);

    expect(navigation.current).toBe('endpoints');
    expect(location.hash).toBe(`#endpoints/${domId}`);
    expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith({ block: 'center' });
    expect(document.activeElement).toBe(row);
    expect(row.classList.contains('ep-flash')).toBe(true);
  });

  it('retries until the row appears, then scrolls/focuses/flashes', async () => {
    vi.useFakeTimers();
    const rawId = 'race-id';
    const domId = `ep-${stableId(rawId)}`;
    const row = document.createElement('tr');
    row.id = domId;
    row.tabIndex = -1;

    // Kick off the jump before the row exists - mirroring the first-nav race
    // where EndpointsPanel's fetch hasn't resolved yet.
    const p = jumpToEndpoint(rawId);
    // Drain the tick + the first couple of 50ms retries (all miss).
    await vi.advanceTimersByTimeAsync(110);
    expect(document.activeElement).not.toBe(row);

    // The fetch lands; the next retry tick finds the row.
    document.body.append(row);
    await vi.advanceTimersByTimeAsync(60);
    await p;

    expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith({ block: 'center' });
    expect(document.activeElement).toBe(row);
    expect(row.classList.contains('ep-flash')).toBe(true);
  });

  it('bails cleanly when the row never appears', async () => {
    vi.useFakeTimers();
    const p = jumpToEndpoint('no-such-row');
    // Well past the full retry window (~8 x 50ms).
    await vi.advanceTimersByTimeAsync(1000);
    await p;
    expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled();
  });

  it('cold deep-link: waits for the endpoints store first refresh, then reveals', async () => {
    // Mirrors a fresh tab landing on #endpoints/<id>: the EndpointsPanel store
    // hasn't fetched yet so hasData is false and the row cannot exist. The
    // helper must wait for the first refresh (hasData flips true) and only
    // then run the DOM poll that finds and focuses the row. The fast 8x50ms
    // retry alone would expire before the fetch lands.
    vi.useFakeTimers();
    const rawId = 'cold-deep-link';
    const domId = `ep-${stableId(rawId)}`;
    const row = document.createElement('tr');
    row.id = domId;
    row.tabIndex = -1;
