// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"strings"
	"testing"
)

// namedLeaf is a minimal Accountable with a custom label, used to make
// AccountableToString output deterministic regardless of the default %v
// rendering of Go pointers.
type namedLeaf struct {
	label string
	size  int64
}

func (n *namedLeaf) RamBytesUsed() int64      { return n.size }
func (n *namedLeaf) AccountableLabel() string { return n.label }
func (n *namedLeaf) String() string           { return n.label }

type namedParent struct {
	label    string
	size     int64
	children []Accountable
}

func (n *namedParent) RamBytesUsed() int64 {
	total := n.size
	for _, c := range n.children {
		total += c.RamBytesUsed()
	}
	return total
}
func (n *namedParent) GetChildResources() []Accountable { return n.children }
func (n *namedParent) AccountableLabel() string         { return n.label }
func (n *namedParent) String() string                   { return n.label }

func TestHumanReadableUnits(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 bytes"},
		{1, "1 bytes"},
		{1023, "1023 bytes"},
		{1024, "1 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1 MB"},
		{1024*1024 + 512*1024, "1.5 MB"},
		{1024 * 1024 * 1024, "1 GB"},
	}
	for _, tc := range tests {
		got := humanReadableUnits(tc.bytes)
		if got != tc.want {
			t.Errorf("humanReadableUnits(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

func TestNamedAccountableBytes(t *testing.T) {
	a := NamedAccountableBytes("foo", 42)
	if got := a.RamBytesUsed(); got != 42 {
		t.Errorf("RamBytesUsed() = %d, want 42", got)
	}
	if got := accountableLabel(a); got != "foo" {
		t.Errorf("label = %q, want %q", got, "foo")
	}
	if got := GetChildResources(a); len(got) != 0 {
		t.Errorf("GetChildResources() = %v, want empty", got)
	}
}

func TestNamedAccountable_WrapsExisting(t *testing.T) {
	leaf := &namedLeaf{label: "leaf", size: 100}
	wrapped := NamedAccountable("desc", leaf)

	if got := wrapped.RamBytesUsed(); got != 100 {
		t.Errorf("RamBytesUsed() = %d, want 100", got)
	}
	if got := accountableLabel(wrapped); got != "desc [leaf]" {
		t.Errorf("label = %q, want %q", got, "desc [leaf]")
	}
}

func TestNamedAccountables_SortsByDescription(t *testing.T) {
	in := map[string]Accountable{
		"bravo": &namedLeaf{label: "bravo-val", size: 2},
		"alpha": &namedLeaf{label: "alpha-val", size: 1},
		"delta": &namedLeaf{label: "delta-val", size: 3},
	}
	out := NamedAccountables("prefix", in)
	if len(out) != 3 {
		t.Fatalf("got %d resources, want 3", len(out))
	}
	want := []string{
		"prefix 'alpha' [alpha-val]",
		"prefix 'bravo' [bravo-val]",
		"prefix 'delta' [delta-val]",
	}
	for i, w := range want {
		if got := accountableLabel(out[i]); got != w {
			t.Errorf("out[%d] label = %q, want %q", i, got, w)
		}
	}
}

func TestAccountableToString_FlatLeaf(t *testing.T) {
	a := &namedLeaf{label: "root", size: 0}
	got := AccountableToString(a)
	want := "root: 0 bytes\n"
	if got != want {
		t.Errorf("AccountableToString = %q, want %q", got, want)
	}
}

func TestAccountableToString_NestedTree(t *testing.T) {
	gc := &namedLeaf{label: "gc", size: 1024 * 1024}
	c1 := &namedParent{label: "c1", size: 0, children: []Accountable{gc}}
	c2 := &namedLeaf{label: "c2", size: 2048}
	root := &namedParent{label: "root", size: 0, children: []Accountable{c1, c2}}

	got := AccountableToString(root)

	// depth 0: "root: <size>\n"
	// depth 1: "|-- c1: <size>\n"
	// depth 2: "    |-- gc: 1 MB\n"
	// depth 1: "|-- c2: 2 KB\n"
	if !strings.HasPrefix(got, "root: ") {
		t.Errorf("missing root prefix in %q", got)
	}
	if !strings.Contains(got, "\n|-- c1: ") {
		t.Errorf("missing depth-1 c1 marker in %q", got)
	}
	if !strings.Contains(got, "\n    |-- gc: 1 MB\n") {
		t.Errorf("missing depth-2 gc marker in %q", got)
	}
	if !strings.Contains(got, "\n|-- c2: 2 KB\n") {
		t.Errorf("missing depth-1 c2 marker in %q", got)
	}
}
