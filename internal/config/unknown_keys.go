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
	Path string // full dotted path, e.g. "proxy.retry"
	Line int
}

func (k UnknownKey) String() string {
	return fmt.Sprintf("%s (line %d)", k.Path, k.Line)
}

// FindUnknownKeys reports every key in the YAML document that Config does not
// model. yaml.v3's KnownFields(true) only yields a type name and line per
// error, with no full path, so this walks the node tree against the struct
// instead. Free-form maps are never reported; their keys are user data.
// It never fails: input that does not decode simply yields nothing.
func FindUnknownKeys(data []byte) []UnknownKey {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	w := &keyWalk{budget: maxKeyWalkVisits, seen: make(map[UnknownKey]struct{})}
	walkKnownKeys(&doc, reflect.TypeOf(Config{}), "", 0, w)
	return w.out
}

func walkKnownKeys(n *yaml.Node, t reflect.Type, path string, depth int, w *keyWalk) {
	if n == nil || depth > maxKeyWalkDepth || !w.step() {
		return
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// Custom unmarshalers own their own schema.
	if t.Implements(yamlUnmarshalerType) || reflect.PointerTo(t).Implements(yamlUnmarshalerType) {
		return
	}

	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			walkKnownKeys(c, t, path, depth+1, w)
		}
	case yaml.AliasNode:
		walkKnownKeys(n.Alias, t, path, depth+1, w)
	case yaml.SequenceNode:
		if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			for i, c := range n.Content {
				walkKnownKeys(c, t.Elem(), fmt.Sprintf("%s[%d]", path, i), depth+1, w)
