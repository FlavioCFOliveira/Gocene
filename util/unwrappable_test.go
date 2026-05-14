// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

// stringHolder is an interface used to exercise Unwrappable[stringHolder].
// Using an interface for T is the common Lucene pattern: a filter chain
// declares "this is some Reader" and each Unwrap step returns another Reader.
type stringHolder interface {
	value() string
}

type leafHolder struct{ s string }

func (l leafHolder) value() string { return l.s }

type wrappingHolder struct {
	inner stringHolder
	tag   string
}

func (w wrappingHolder) value() string { return w.tag + "/" + w.inner.value() }

// Unwrap returns the wrapped delegate, satisfying Unwrappable[stringHolder].
func (w wrappingHolder) Unwrap() stringHolder { return w.inner }

// TestUnwrapAll_Chain validates the canonical case: a multi-level wrapper
// resolves to its innermost holder.
func TestUnwrapAll_Chain(t *testing.T) {
	leaf := leafHolder{s: "L"}
	chain := wrappingHolder{
		tag: "outer",
		inner: wrappingHolder{
			tag: "middle",
			inner: wrappingHolder{
				tag:   "inner",
				inner: leaf,
			},
		},
	}
	got := UnwrapAll[stringHolder](chain)
	if got.value() != "L" {
		t.Fatalf("got %q, want %q", got.value(), "L")
	}
	if _, ok := got.(leafHolder); !ok {
		t.Fatalf("got %T, want leafHolder", got)
	}
}

// TestUnwrapAll_NotWrapper confirms a non-Unwrappable value is returned
// unchanged.
func TestUnwrapAll_NotWrapper(t *testing.T) {
	leaf := leafHolder{s: "x"}
	got := UnwrapAll[stringHolder](leaf)
	if got != leaf {
		t.Fatalf("got %v, want %v", got, leaf)
	}
}

// TestUnwrapAll_SingleLevel exercises a one-step unwrap.
func TestUnwrapAll_SingleLevel(t *testing.T) {
	wrap := wrappingHolder{tag: "w", inner: leafHolder{s: "y"}}
	got := UnwrapAll[stringHolder](wrap)
	if got.value() != "y" {
		t.Fatalf("got %q, want %q", got.value(), "y")
	}
}
