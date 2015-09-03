<script lang="ts">
  // Single component enforcing "glyph + colour + text" for every status
  // indicator. Never colour alone (colour-blind / greyscale).
  interface Props {
    status?: string;
  }
