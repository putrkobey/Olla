// Svelte 5 runes class. Module-scope singleton so every importer shares one
// piece of state. Lifted from the prior prototype's pattern, ported to a
// proper class with private state.

export type ThemeMode = 'auto' | 'light' | 'dark';

const STORAGE_KEY = 'olla-theme';
const MODES: readonly ThemeMode[] = ['auto', 'light', 'dark'];

function systemPrefersDark(): boolean {
  if (typeof window === 'undefined' || !window.matchMedia) return false;
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function applyModeToDom(mode: ThemeMode): void {
  const root = document.documentElement;
  if (mode === 'auto') {
    root.removeAttribute('data-theme');
  } else {
    root.setAttribute('data-theme', mode);
  }
}

function readStored(): ThemeMode {
  if (typeof localStorage === 'undefined') return 'auto';
  const v = localStorage.getItem(STORAGE_KEY);
  return MODES.includes(v as ThemeMode) ? (v as ThemeMode) : 'auto';
}

type MediaQueryListLike = {
  matches: boolean;
  media: string;
  addEventListener(event: 'change', cb: (e: MediaQueryListEvent) => void): void;
  removeEventListener(event: 'change', cb: (e: MediaQueryListEvent) => void): void;
};

class ThemeStore {
  #mode: ThemeMode = $state('auto');
  // Mirrors the OS preference as reactive state. `resolved` reading a plain
  // matchMedia() call (rather than this field) was the item-15 bug: Svelte
  // only reruns $derived/effects when a $state dependency changes, so a
  // getter that called window.matchMedia() directly never triggered a
  // recompute when the OS flipped mid-session, and the toggle's aria-label
  // went stale even though the CSS media query repainted the page correctly.
  #systemDark: boolean = $state(systemPrefersDark());
  #media: MediaQueryListLike | null = null;
  #onSystemChange = (e: MediaQueryListEvent): void => {
    this.#systemDark = e.matches;
  };

  constructor() {
    this.#mode = readStored();
    applyModeToDom(this.#mode);

    if (typeof window !== 'undefined' && window.matchMedia) {
