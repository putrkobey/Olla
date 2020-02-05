import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, it, expect } from 'vitest';

// Read the source directly rather than importing it - the tailwind vite
// plugin intercepts .css imports (even with ?raw) and runs its own
// pipeline over them, which strips the plain custom-property declarations
// this test needs to see.
const appCss = readFileSync(
  path.join(path.dirname(fileURLToPath(import.meta.url)), 'app.css'),
  'utf8'
);

// Regression coverage: --blue was missing from the explicit dark theme block
// (:root[data-theme='dark']), so with the OS in light mode and the user
// explicitly picking dark, --blue fell through to the light :root value
// instead of the dark one. Everything the media-query dark block defines
// must also be defined in the explicit dark block, or the same class of
// drift creeps back in unnoticed.

/** Extracts the `--name: value` custom property declarations from a single
