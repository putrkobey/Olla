<script lang="ts">
  // Renders the inline banner shown when a panel's source is in error or
  // stale. Keeps the prior data rendered (greyed) rather than blanking.

  // Structural minimal view of a PollStore: any panel's store satisfies this
  // without being parameterised on its data type, which the banner never reads.
  interface BannerStore {
    readonly name: string;
    readonly status: 'loading' | 'ok' | 'error' | 'stale';
    readonly lastUpdated: Date | null;
    refresh(): void;
  }

  interface Props {
    store: BannerStore;
  }
  let { store }: Props = $props();

  const show = $derived(store.status === 'error' || store.status === 'stale');
  const isStale = $derived(store.status === 'stale');

  const message = $derived(
    isStale
      ? `Olla's ${store.name} status has been unreachable for a while, showing the last good snapshot and retrying.`
      : `Couldn't reach Olla's ${store.name} status API, retrying.`
  );

  function fmtTime(d: Date | null): string {
    if (!d) return 'never';
    return d.toLocaleTimeString('en-AU', { hour12: false });
