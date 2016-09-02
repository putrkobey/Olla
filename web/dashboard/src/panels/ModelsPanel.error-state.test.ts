import { flushSync, mount, unmount } from 'svelte';
import { describe, it, expect, afterEach, vi } from 'vitest';
import { models } from '../lib/stores/models.svelte';
import ModelsPanel from './ModelsPanel.svelte';

// Regression coverage: on a first-load fetch failure, models.data stays null,
// so groups/flatRecent both derive to [] and the panel fell through to the
// "No models discovered yet" empty-state copy - rendered right alongside
// StatusBanner's error banner. An operator reading both at once sees "the
// backends genuinely have no models" rather than "the request failed".
