// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

// fakeTermAttribute is a minimal user-defined Attribute sub-interface,
// modelling the typical Lucene pattern {@code interface FooAttribute
// extends Attribute}.
type fakeTermAttribute interface {
	Attribute
	Term() string
}

// fakeTermAttributeImpl is a value type satisfying fakeTermAttribute.
type fakeTermAttributeImpl struct {
	term string
}

func (a *fakeTermAttributeImpl) Term() string { return a.term }

// TestAttribute_AnyTypeSatisfiesMarker confirms that Attribute is the
// empty marker interface declared by Lucene (any value can be referred
// to as an Attribute).
func TestAttribute_AnyTypeSatisfiesMarker(t *testing.T) {
	var a Attribute = &fakeTermAttributeImpl{term: "hello"}
	if a == nil {
		t.Fatal("interface value should not be nil")
	}
}

// TestAttribute_SubInterfaceEmbedding confirms the documented usage
// pattern: declaring a domain attribute interface by embedding the
// marker interface, then satisfying it with a concrete impl.
func TestAttribute_SubInterfaceEmbedding(t *testing.T) {
	var ta fakeTermAttribute = &fakeTermAttributeImpl{term: "lucene"}
	if got := ta.Term(); got != "lucene" {
		t.Errorf("Term() = %q, want %q", got, "lucene")
	}
	// And it is still usable as an Attribute.
	var a Attribute = ta
	if a == nil {
		t.Fatal("sub-interface value should remain assignable to Attribute")
	}
}
