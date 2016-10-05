import { describe, it, expect, beforeEach, vi } from 'vitest';

// Regression coverage for the toggle-cycle bug (item 12): the old
// auto -> light -> dark -> auto cycle went to a fixed next step regardless
// of what the system preference actually resolved to, so a click could be a
// silent no-op (e.g. system already light, auto -> light). `toggle()` must
// always flip away from whatever is currently rendered.

// Captures the 'change' listener the store registers so tests can simulate
// the OS flipping preference live, the way item 15's bug was actually
// reproduced (page.emulateMedia with no reload).
function mockMatchMedia(prefersDark: boolean) {
  let listener: ((e: { matches: boolean }) => void) | null = null;
  const addEventListener = vi.fn((event: string, cb: (e: { matches: boolean }) => void) => {
    if (event === 'change') listener = cb;
  });
  const removeEventListener = vi.fn((event: string, cb: (e: { matches: boolean }) => void) => {
    if (event === 'change' && listener === cb) listener = null;
  });

  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: query.includes('dark') ? prefersDark : false,
    media: query,
    addEventListener,
    removeEventListener,
  }));

  return {
    /** Simulates the OS preference changing without a page reload. */
    fireChange(matches: boolean) {
      listener?.({ matches });
    },
  };
}

beforeEach(() => {
  localStorage.clear();
  vi.resetModules();
});

describe('theme toggle', () => {
  it('system-light: first click on auto flips to dark, a visible change', async () => {
    mockMatchMedia(false);
    const { theme } = await import('./theme.svelte');
    expect(theme.mode).toBe('auto');
    expect(theme.resolved).toBe('light');

    theme.toggle();

    expect(theme.mode).toBe('dark');
    expect(theme.resolved).toBe('dark');
  });

  it('system-dark: first click on auto flips to light, a visible change', async () => {
    mockMatchMedia(true);
    const { theme } = await import('./theme.svelte');
    expect(theme.mode).toBe('auto');
    expect(theme.resolved).toBe('dark');

    theme.toggle();

    expect(theme.mode).toBe('light');
    expect(theme.resolved).toBe('light');
  });

  it('subsequent clicks keep toggling light/dark, each one a visible change', async () => {
    mockMatchMedia(false);
    const { theme } = await import('./theme.svelte');

    theme.toggle(); // auto (resolves light) -> dark
    expect(theme.resolved).toBe('dark');
    theme.toggle(); // dark -> light
    expect(theme.resolved).toBe('light');
    theme.toggle(); // light -> dark
    expect(theme.resolved).toBe('dark');
