package main

import (
	"fmt"
	"strings"

	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/adapter/unifier"
	"github.com/putrkobey/Olla/internal/config"
)

// validationItem captures the outcome of validating a single configuration
// source. Kept independent of how the report gets formatted (buildValidationReport)
// so the formatting/exit-code logic can be tested without touching the real
// filesystem or config packages.
type validationItem struct {
	Err       error    // the source is entirely unusable
	Name      string   // e.g. "config.yaml"
	Detail    string   // human-readable summary, shown when there's no fatal Err
	Warnings  []string // non-fatal issues - the source is usable, typically via fallback
	IgnoredIn string   // file the ignored keys came from, for the section heading
	Ignored   []string // keys the file sets that nothing reads; reported but never affect the verdict
}

// buildValidationReport formats a human-readable --validate-config summary
// from already-gathered items and decides the process exit code.
//
// A file that exists but doesn't fully parse is treated as a hard-gate
// failure here (exit 1), even though the running server itself would just
// warn and fall back to defaults for the same file - that graceful runtime
// degradation is exactly what --validate-config exists to catch ahead of
// time, not paper over.
func buildValidationReport(items []validationItem) (report string, exitCode int) {
	var buf strings.Builder
	buf.WriteString("Olla Configuration Validation\n")
	buf.WriteString("==============================\n")

	clean := true
	for _, item := range items {
		switch {
		case item.Err != nil:
			fmt.Fprintf(&buf, "[FAIL] %-12s %v\n", item.Name, item.Err)
			clean = false
		case len(item.Warnings) > 0:
			fmt.Fprintf(&buf, "[WARN] %-12s %s (%d issue(s))\n", item.Name, item.Detail, len(item.Warnings))
			for _, w := range item.Warnings {
				fmt.Fprintf(&buf, "         - %s\n", w)
			}
			clean = false
