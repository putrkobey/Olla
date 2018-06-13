// Deterministic, DOM-safe id derived from an exact string.
//
// The old cssId() slug stripped punctuation (`[^a-z0-9]+` -> `-`), so two
// distinct names that differ only in punctuation ("node.a" vs "node-a")
// collapsed to the same DOM id. getElementById resolves to whichever row
// rendered first, so "jump to endpoint" silently scrolled to the wrong row.
//
// This hash (FNV-1a 32-bit -> base36) is collision-resistant, not lossless -
// a 32-bit space can theoretically map two distinct strings to the same id,
