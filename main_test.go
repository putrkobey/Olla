package main

import "testing"

func TestResolveRuntimeLogLevel_UsesConfiguredValue(t *testing.T) {
	t.Parallel()

	warned := false
	got := resolveRuntimeLogLevel("debug", func(string, ...any) { warned = true })

	if got != "debug" {
		t.Errorf("expected configured level 'debug', got %q", got)
	}
	if warned {
		t.Error("did not expect a warning for a valid level")
	}
}

func TestResolveRuntimeLogLevel_EmptyFallsBackToDefault(t *testing.T) {
	t.Parallel()

	warned := false
	got := resolveRuntimeLogLevel("", func(string, ...any) { warned = true })

	if got != DefaultLoggerLevel {
		t.Errorf("expected default level %q for unset config, got %q", DefaultLoggerLevel, got)
	}
	if warned {
		t.Error("an unset logging.level is not an error, so no warning should fire")
	}
}

func TestResolveRuntimeLogLevel_InvalidFallsBackAndWarns(t *testing.T) {
	t.Parallel()

	var gotMsg string
	got := resolveRuntimeLogLevel("verbose", func(msg string, args ...any) { gotMsg = msg })

	if got != DefaultLoggerLevel {
		t.Errorf("expected fallback to default level %q, got %q", DefaultLoggerLevel, got)
	}
	if gotMsg == "" {
		t.Error("expected a warning to be raised for an invalid level")
	}
}

func TestResolveRuntimeLogFormat_NonTTY_UsesConfiguredValue(t *testing.T) {
	t.Parallel()

	warned := false
	got := resolveRuntimeLogFormat("text", false, false, func(string, ...any) { warned = true })

	if got != "text" {
		t.Errorf("expected configured format 'text', got %q", got)
	}
	if warned {
		t.Error("did not expect a warning for a valid format")
	}
}

func TestResolveRuntimeLogFormat_NonTTY_JSONHonoured(t *testing.T) {
	t.Parallel()

	got := resolveRuntimeLogFormat("json", false, false, func(string, ...any) {})

	if got != LogFormatJSON {
		t.Errorf("expected non-TTY output to honour configured json format, got %q", got)
	}
}

func TestResolveRuntimeLogFormat_NonTTY_EmptyFallsBackToDefault(t *testing.T) {
	t.Parallel()

	warned := false
	got := resolveRuntimeLogFormat("", false, false, func(string, ...any) { warned = true })

	if got != DefaultLoggerFormat {
		t.Errorf("expected default format %q for unset config, got %q", DefaultLoggerFormat, got)
	}
	if warned {
		t.Error("an unset logging.format is not an error, so no warning should fire")
	}
}

func TestResolveRuntimeLogFormat_NonTTY_InvalidFallsBackAndWarns(t *testing.T) {
	t.Parallel()

	var gotMsg string
	got := resolveRuntimeLogFormat("yaml", false, false, func(msg string, args ...any) { gotMsg = msg })

	if got != DefaultLoggerFormat {
