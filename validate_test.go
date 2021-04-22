package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/config"
)

func TestBuildValidationReport_AllClean(t *testing.T) {
	t.Parallel()

	items := []validationItem{
		{Name: "config.yaml", Detail: "loaded from config/config.yaml"},
		{Name: "models.yaml", Detail: "loaded from config/models.yaml"},
		{Name: "profiles", Detail: "9 profile(s) loaded"},
	}

	report, exitCode := buildValidationReport(items)

	if exitCode != 0 {
		t.Errorf("expected exit code 0 for a fully clean report, got %d", exitCode)
	}
	if !strings.Contains(report, "[OK]   config.yaml") {
		t.Errorf("expected an OK line for config.yaml, got:\n%s", report)
	}
	if !strings.Contains(report, "Result: PASS") {
		t.Errorf("expected a PASS result line, got:\n%s", report)
	}
	if strings.Contains(report, "FAIL") || strings.Contains(report, "WARN") {
		t.Errorf("clean items should not produce any FAIL/WARN lines, got:\n%s", report)
	}
}

func TestBuildValidationReport_FatalErrorFailsAndExitsNonZero(t *testing.T) {
	t.Parallel()

	items := []validationItem{
		{Name: "config.yaml", Err: errors.New("failed to parse config.yaml: yaml: line 3: bad indentation")},
	}

	report, exitCode := buildValidationReport(items)

	if exitCode == 0 {
		t.Error("expected a non-zero exit code when a config source has a fatal error")
	}
	if !strings.Contains(report, "[FAIL] config.yaml") {
		t.Errorf("expected a FAIL line naming config.yaml, got:\n%s", report)
	}
	if !strings.Contains(report, "bad indentation") {
		t.Errorf("expected the underlying error text in the report, got:\n%s", report)
	}
	if !strings.Contains(report, "Result: FAIL") {
		t.Errorf("expected a FAIL result line, got:\n%s", report)
	}
}

func TestBuildValidationReport_WarningsFailAndExitNonZero(t *testing.T) {
	t.Parallel()

	items := []validationItem{
		{Name: "config.yaml", Detail: "loaded from config/config.yaml"},
		{
			Name:     "models.yaml",
			Detail:   "using embedded defaults",
