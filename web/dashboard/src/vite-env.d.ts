/// <reference types="vite/client" />

// The dashboard ships as a browser SPA, but its component tests run under
// vitest (Node + jsdom). The DOM lib covers `document`/`window`, but the
// Node `global` object the tests assign `global.fetch` on is not declared by
// the DOM lib and `@types/node` is intentionally not a dependency. When the
