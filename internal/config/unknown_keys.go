package config

import (
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// maxKeyWalkDepth bounds the node walk. Decoding has already succeeded by the
// time we get here, so this is only a guard against pathological alias chains.
const maxKeyWalkDepth = 64

// maxKeyWalkVisits caps total node visits. Merge sequences can reference an
// anchor repeatedly (even cyclically, when an explicit key overrides the merge
// at decode time), so depth alone does not bound the work.
const maxKeyWalkVisits = 100_000

type keyWalk struct {
	seen   map[UnknownKey]struct{}
	out    []UnknownKey
	budget int
}

// report records k once. Merge keys and aliases revisit the same mapping node,
// so without this a merge bomb lists one typo thousands of times.
func (w *keyWalk) report(k UnknownKey) {
	if _, dup := w.seen[k]; dup {
		return
	}
	w.seen[k] = struct{}{}
	w.out = append(w.out, k)
}

// step spends one visit and reports whether the walk may continue.
func (w *keyWalk) step() bool {
	w.budget--
	return w.budget >= 0
}

var yamlUnmarshalerType = reflect.TypeOf((*yaml.Unmarshaler)(nil)).Elem()

// UnknownKey is a key present in the config file that no Config field reads.
type UnknownKey struct {
