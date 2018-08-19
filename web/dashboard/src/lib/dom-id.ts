// Deterministic, DOM-safe id derived from an exact string.
//
// The old cssId() slug stripped punctuation (`[^a-z0-9]+` -> `-`), so two
// distinct names that differ only in punctuation ("node.a" vs "node-a")
