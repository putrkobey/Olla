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
