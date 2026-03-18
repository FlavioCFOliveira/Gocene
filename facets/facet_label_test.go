// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import "testing"

func TestNewFacetLabel(t *testing.T) {
	fl := NewFacetLabel("a", "b", "c")
	if fl.Length() != 3 {
		t.Errorf("Expected length 3, got %d", fl.Length())
	}
	if fl.Get(0) != "a" || fl.Get(1) != "b" || fl.Get(2) != "c" {
		t.Error("Components don't match")
	}
}

func TestNewFacetLabelEmpty(t *testing.T) {
	fl := NewFacetLabelEmpty()
	if !fl.IsEmpty() {
		t.Error("Expected empty label")
	}
	if fl.Length() != 0 {
		t.Errorf("Expected length 0, got %d", fl.Length())
	}
}

func TestFacetLabelGet(t *testing.T) {
	fl := NewFacetLabel("first", "second", "third")

	if fl.Get(0) != "first" {
		t.Errorf("Expected 'first', got '%s'", fl.Get(0))
	}
	if fl.Get(1) != "second" {
		t.Errorf("Expected 'second', got '%s'", fl.Get(1))
	}
	if fl.Get(2) != "third" {
		t.Errorf("Expected 'third', got '%s'", fl.Get(2))
	}
	if fl.Get(-1) != "" {
		t.Error("Expected empty string for negative index")
	}
	if fl.Get(10) != "" {
		t.Error("Expected empty string for out of bounds index")
	}
}

func TestFacetLabelFirstLast(t *testing.T) {
	fl := NewFacetLabel("a", "b", "c")

	if fl.First() != "a" {
		t.Errorf("Expected first 'a', got '%s'", fl.First())
	}
	if fl.Last() != "c" {
		t.Errorf("Expected last 'c', got '%s'", fl.Last())
	}

	empty := NewFacetLabelEmpty()
	if empty.First() != "" {
		t.Error("Expected empty first for empty label")
	}
	if empty.Last() != "" {
		t.Error("Expected empty last for empty label")
	}
}

func TestFacetLabelSubPath(t *testing.T) {
	fl := NewFacetLabel("a", "b", "c", "d")

	sub := fl.SubPath(1, 3)
	if sub.Length() != 2 {
		t.Errorf("Expected length 2, got %d", sub.Length())
	}
	if sub.Get(0) != "b" || sub.Get(1) != "c" {
		t.Error("SubPath components don't match")
	}

	// Test with -1 as end
	sub2 := fl.SubPath(2, -1)
	if sub2.Length() != 2 {
		t.Errorf("Expected length 2, got %d", sub2.Length())
	}
	if sub2.Get(0) != "c" || sub2.Get(1) != "d" {
		t.Error("SubPath with -1 end doesn't match")
	}

	// Test invalid range
	sub3 := fl.SubPath(3, 2)
	if !sub3.IsEmpty() {
		t.Error("Expected empty label for invalid range")
	}
}

func TestFacetLabelParent(t *testing.T) {
	fl := NewFacetLabel("a", "b", "c")

	parent := fl.Parent()
	if parent == nil {
		t.Fatal("Expected non-nil parent")
	}
	if parent.Length() != 2 {
		t.Errorf("Expected parent length 2, got %d", parent.Length())
	}
	if parent.Get(0) != "a" || parent.Get(1) != "b" {
		t.Error("Parent components don't match")
	}

	// Test root parent
	root := NewFacetLabel("single")
	if root.Parent() == nil {
		t.Error("Expected nil parent for single component")
	}

	// Test empty parent
	empty := NewFacetLabelEmpty()
	if empty.Parent() != nil {
		t.Error("Expected nil parent for empty label")
	}
}

func TestFacetLabelAppend(t *testing.T) {
	fl := NewFacetLabel("a", "b")

	appended := fl.Append("c", "d")
	if appended.Length() != 4 {
		t.Errorf("Expected length 4, got %d", appended.Length())
	}
	if fl.Length() != 2 {
		t.Error("Original label should not be modified")
	}

	// Test append to nil
	var nilLabel *FacetLabel
	appended2 := nilLabel.Append("x")
	if appended2.Length() != 1 || appended2.Get(0) != "x" {
		t.Error("Append to nil should create new label")
	}
}

func TestFacetLabelEquals(t *testing.T) {
	fl1 := NewFacetLabel("a", "b", "c")
	fl2 := NewFacetLabel("a", "b", "c")
	fl3 := NewFacetLabel("a", "b")
	fl4 := NewFacetLabel("x", "y", "z")

	if !fl1.Equals(fl2) {
		t.Error("Expected equal labels to be equal")
	}
	if fl1.Equals(fl3) {
		t.Error("Expected different length labels to not be equal")
	}
	if fl1.Equals(fl4) {
		t.Error("Expected different content labels to not be equal")
	}
	if !fl1.Equals(fl1) {
		t.Error("Expected label to equal itself")
	}
	if fl1.Equals(nil) {
		t.Error("Expected label to not equal nil")
	}
}

func TestFacetLabelCompareTo(t *testing.T) {
	fl1 := NewFacetLabel("a", "b")
	fl2 := NewFacetLabel("a", "b")
	fl3 := NewFacetLabel("a", "c")
	fl4 := NewFacetLabel("a")
	fl5 := NewFacetLabel("b")

	if fl1.CompareTo(fl2) != 0 {
		t.Error("Expected equal labels to compare to 0")
	}
	if fl1.CompareTo(fl3) >= 0 {
		t.Error("Expected 'ab' < 'ac'")
	}
	if fl3.CompareTo(fl1) <= 0 {
		t.Error("Expected 'ac' > 'ab'")
	}
	if fl4.CompareTo(fl1) >= 0 {
		t.Error("Expected shorter label to be less")
	}
	if fl1.CompareTo(fl4) <= 0 {
		t.Error("Expected longer label to be greater")
	}
	if fl4.CompareTo(fl5) >= 0 {
		t.Error("Expected 'a' < 'b'")
	}
}

func TestFacetLabelString(t *testing.T) {
	tests := []struct {
		label    *FacetLabel
		expected string
	}{
		{NewFacetLabelEmpty(), "/"},
		{NewFacetLabel("a"), "/a"},
		{NewFacetLabel("a", "b", "c"), "/a/b/c"},
	}

	for _, tt := range tests {
		result := tt.label.String()
		if result != tt.expected {
			t.Errorf("String() = %s, want %s", result, tt.expected)
		}
	}

	// Test nil
	var nilLabel *FacetLabel
	if nilLabel.String() != "/" {
		t.Error("Expected nil label to stringify as '/'")
	}
}

func TestFacetLabelHashCode(t *testing.T) {
	fl1 := NewFacetLabel("a", "b")
	fl2 := NewFacetLabel("a", "b")
	fl3 := NewFacetLabel("a", "c")

	if fl1.HashCode() != fl2.HashCode() {
		t.Error("Expected equal labels to have same hash code")
	}
	if fl1.HashCode() == fl3.HashCode() {
		t.Error("Expected different labels to likely have different hash codes")
	}
}

func TestFacetLabelCopy(t *testing.T) {
	fl := NewFacetLabel("a", "b", "c")
	copy := fl.Copy()

	if !fl.Equals(copy) {
		t.Error("Expected copy to equal original")
	}

	// Modify copy should not affect original
	copy.Components[0] = "x"
	if fl.Get(0) != "a" {
		t.Error("Modifying copy should not affect original")
	}
}
