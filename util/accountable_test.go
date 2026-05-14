// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

// testLeafAccountable is a minimal Accountable that does not expose any
// child resources. Used to exercise the GetChildResources fallback for
// objects that only implement the Accountable interface.
type testLeafAccountable struct {
	size int64
}

func (a *testLeafAccountable) RamBytesUsed() int64 { return a.size }

// testParentAccountable owns child Accountables and reports them through
// the AccountableWithChildren extension.
type testParentAccountable struct {
	size     int64
	children []Accountable
}

func (a *testParentAccountable) RamBytesUsed() int64 {
	total := a.size
	for _, c := range a.children {
		total += c.RamBytesUsed()
	}
	return total
}

func (a *testParentAccountable) GetChildResources() []Accountable {
	return a.children
}

func TestAccountable_LeafReturnsNilChildren(t *testing.T) {
	a := &testLeafAccountable{size: 42}

	if got := a.RamBytesUsed(); got != 42 {
		t.Errorf("RamBytesUsed() = %d, want 42", got)
	}
	if got := GetChildResources(a); got != nil {
		t.Errorf("GetChildResources() = %v, want nil", got)
	}
}

func TestAccountable_ParentExposesChildren(t *testing.T) {
	child1 := &testLeafAccountable{size: 10}
	child2 := &testLeafAccountable{size: 20}
	parent := &testParentAccountable{
		size:     5,
		children: []Accountable{child1, child2},
	}

	if got := parent.RamBytesUsed(); got != 35 {
		t.Errorf("RamBytesUsed() = %d, want 35", got)
	}
	children := GetChildResources(parent)
	if len(children) != 2 {
		t.Fatalf("GetChildResources() returned %d children, want 2", len(children))
	}
	if children[0] != Accountable(child1) || children[1] != Accountable(child2) {
		t.Errorf("GetChildResources() returned unexpected children: %v", children)
	}
}

// TestAccountable_ZeroValue ensures an Accountable returning zero bytes is
// a valid, well-defined state (Lucene allows zero for empty objects).
func TestAccountable_ZeroValue(t *testing.T) {
	a := &testLeafAccountable{size: 0}
	if got := a.RamBytesUsed(); got != 0 {
		t.Errorf("RamBytesUsed() = %d, want 0", got)
	}
}
