import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach } from 'vitest';
import RangeBar from './RangeBar.svelte';

// Regression coverage for finding 6: `avg` used to default to 0 and get fed
// straight into fmtMs(), so a genuinely-missing average (older backend, or
// an endpoint with no traffic yet) rendered as a confident "0ms" instead of
// a no-data placeholder - and the caller had derived that 0 by parseInt()-ing
// a health-check latency string in the first place (see EndpointsPanel).

interface RangeBarProps {
  min?: number;
  avg?: number | null;
  max?: number;
  globalMax?: number;
  label?: string;
}

let component: ReturnType<typeof mount> | undefined;

afterEach(() => {
  if (component) unmount(component);
  document.body.innerHTML = '';
});

function render(props: RangeBarProps): void {
  component = mount(RangeBar, { target: document.body, props });
  flushSync();
}

describe('RangeBar', () => {
  it('renders the numeric average, formatted, alongside the range', () => {
