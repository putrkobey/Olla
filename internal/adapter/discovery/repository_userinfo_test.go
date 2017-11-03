package discovery

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/config"
)

// Config-validation half: an endpoint URL containing user:pass@host must be
// rejected at load time, with an error that points the operator at the auth
// config block AND spells out the exact rewrite so a failed boot is a
// copy-paste fix. The display-side sanitisation (stripping userinfo from the
// url field surfaced in JSON) lives in the handlers; this is the hard gate
// that prevents credentials ever entering the runtime URLString.
//
// The error text must never echo the operator's real username or password:
// a username can itself be a secret in API-key-as-username schemes, so both
// are redacted to placeholders. Only the non-secret host/port/path survive
// verbatim, kept so the rewrite stays copy-pasteable.
func TestEndpointConfigValidation_RejectsUserinfo(t *testing.T) {
	cases := []struct {
		name string
		url  string
		// wantContains are substrings the rewritten error must carry.
		wantContains []string
		// wantAbsent are secrets that must never appear in the error text.
		wantAbsent []string
	}{
		{
			name:         "user and password",
			url:          "https://alice:s3cr3t@ollama.local:11434",
			wantContains: []string{"ollama.local:11434", "type: basic", "<your username>", "<your password>"},
			wantAbsent:   []string{"alice", "s3cr3t"},
		},
		{
			name: "user only",
			url:  "https://s3cruser@ollama.local:11434",
			wantContains: []string{
				"ollama.local:11434", "type: basic", "<your username>", "<your password>",
				"requires a password", "type: bearer", "<your token>",
			},
			wantAbsent: []string{"s3cruser"},
		},
		{
			name:         "userinfo with path and query",
			url:          "http://u:p@host:8080/v1?x=1",
			wantContains: []string{"host:8080", "type: basic", "<your username>", "<your password>"},
			wantAbsent:   []string{"u:p@", ":p@"},
		},
	}

	for _, tc := range cases {
