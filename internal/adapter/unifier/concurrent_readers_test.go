package unifier

import (
	"context"
	"net/url"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// The catalog store hands out live pointers that lock-free readers keep using
// while discovery merges, so these tests overlap a mutating UnifyModels with
// readers of the same model. They are only meaningful under -race (or, without
// it, as a crash guard for "concurrent map iteration and map write").

const raceIterations = 100

func raceEndpoint(name, raw string) *domain.Endpoint {
	u, _ := url.Parse(raw)
	return &domain.Endpoint{Name: name, URL: u, URLString: raw, Type: "ollama"}
}

func raceModel(name, digest string) *domain.ModelInfo {
	return &domain.ModelInfo{Name: name, Size: 100, Details: &domain.ModelDetails{Digest: &digest}}
}

// touchModel reads every slice and ranges the Metadata map the way handlers and
// converters do.
func touchModel(m *domain.UnifiedModel) int {
	n := 0
	for _, a := range m.Aliases {
		n += len(a.Name)
	}
	for _, s := range m.SourceEndpoints {
		n += len(s.NativeName) + int(s.DiskSize)
		if s.StateInfo != nil {
			n += len(s.StateInfo.LastError)
		}
	}
	for _, c := range m.Capabilities {
		n += len(c)
	}
	for k, v := range m.Metadata {
		n += len(k)
		if p, ok := v.(map[string]bool); ok {
			for pk := range p {
				n += len(pk)
			}
