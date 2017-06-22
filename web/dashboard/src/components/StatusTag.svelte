<script lang="ts">
  // Single component enforcing "glyph + colour + text" for every status
  // indicator. Never colour alone (colour-blind / greyscale).
  interface Props {
    status?: string;
  }
  let { status = 'healthy' }: Props = $props();

  type Meta = { glyph: string; cls: string; label: string };

  const STATUS: Record<string, Meta> = {
    healthy: { glyph: '●', cls: 'green', label: 'healthy' },
