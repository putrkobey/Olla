import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';
import SparkStrip, { nearestSampleIndex } from './SparkStrip.svelte';
import { history } from '../lib/stores/history.svelte';
import type { Sample } from '../lib/stores/history.svelte';

// Samples are injected via the `samples` prop rather than the singleton store
// so the rendering assertions are deterministic and independent of poll
// timing. The production path (no prop) reads history.samples via the same
// $derived, so the prop only bypasses the store for tests.

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  document.body.innerHTML = '';
  // history is a module-scoped singleton shared across every test in this
  // file. Without reset, samples appended by a mount that bypassed the prop
  // (or by the $effect firing on an unstubbed overview tick) leak into the
  // next test and corrupt its view-derived assertions.
  history.reset();
});

function render(props?: { samples?: Sample[] }): void {
  component = mount(SparkStrip, { target: document.body, props: props ?? {} });
  flushSync();
}

function makeSample(over: Partial<Sample> = {}): Sample {
  return {
    t: Date.now(),
    totalRequests: 0,
    totalBytes: 0,
    activeConnections: 0,
    reqPerSec: 0,
    bytesPerSec: 0,
    ...over,
  };
}

// jsdom returns all-zero rects by default; the pointer handler maps x via
// getBoundingClientRect, so tests need a deterministic width. This stubs the
// chart element's rect to a 100x48 box at the origin: clientX == fraction.
function stubChartRect(width = 100): HTMLElement {
  const chart = document.querySelector('.spark-chart') as HTMLElement;
  vi.spyOn(chart, 'getBoundingClientRect').mockReturnValue({
    left: 0,
    top: 0,
    width,
    height: 48,
    right: width,
    bottom: 48,
    x: 0,
    y: 0,
    toJSON: () => {},
  } as DOMRect);
  return chart;
}

describe('nearestSampleIndex (pure helper)', () => {
  it('maps fractions to the nearest sample index', () => {
    expect(nearestSampleIndex(0, 4)).toBe(0);
    expect(nearestSampleIndex(0.34, 4)).toBe(1); // 0.34 * 3 = 1.02 -> round 1
    expect(nearestSampleIndex(0.5, 4)).toBe(2); // 0.5 * 3 = 1.5 -> round 2
    expect(nearestSampleIndex(1, 4)).toBe(3);
  });

  it('clamps out-of-range fractions into [0, n-1]', () => {
    expect(nearestSampleIndex(-0.5, 4)).toBe(0);
    expect(nearestSampleIndex(1.5, 4)).toBe(3);
  });

  it('returns -1 for empty arrays, 0 for single-element arrays', () => {
    expect(nearestSampleIndex(0.5, 0)).toBe(-1);
    expect(nearestSampleIndex(0.5, 1)).toBe(0);
  });
});

describe('SparkStrip', () => {
  it('renders the SVG element immediately so the strip height is stable from first paint', () => {
    render({ samples: [] });
    const svg = document.querySelector('.spark-svg') as SVGSVGElement | null;
    expect(svg).toBeTruthy();
  });

  it('shows the gathering-data hint with fewer than 2 samples', () => {
    render({ samples: [makeSample({ reqPerSec: 1.5, activeConnections: 3 })] });
    expect(document.querySelector('.spark-hint')).toBeTruthy();
    expect(document.body.textContent).toContain('gathering data');
  });

  it('hides the hint and draws both paths once 2+ samples arrive', () => {
    render({
      samples: [
        makeSample({ reqPerSec: 1, activeConnections: 2 }),
        makeSample({ reqPerSec: 2, activeConnections: 3 }),
      ],
    });
    expect(document.querySelector('.spark-hint')).toBeNull();
    const area = document.querySelector('.spark-area') as SVGPathElement | null;
    const line = document.querySelector('.spark-line') as SVGPathElement | null;
