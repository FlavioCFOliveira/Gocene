// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene91

import (
	"testing"
)

// TestRWCodec_CodecConstructor verifies that Lucene91Codec is constructible
// and sets Name and Version correctly.
//
// In the Java test tree, Lucene91RWCodec is a test-support class; in Gocene
// the production codec that the RW variant would wrap is tested here.
func TestRWCodec_CodecConstructor(t *testing.T) {
	c := NewLucene91Codec("9.1.0")
	if c.Name != "Lucene91Codec" {
		t.Errorf("Name: got %q, want %q", c.Name, "Lucene91Codec")
	}
	if c.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", c.Version, "9.1.0")
	}
}

// TestRWCodec_CodecCustomVersion verifies a different version string.
func TestRWCodec_CodecCustomVersion(t *testing.T) {
	c := NewLucene91Codec("10.4.0")
	if c.Version != "10.4.0" {
		t.Errorf("Version: got %q, want %q", c.Version, "10.4.0")
	}
}

// TestRWCodec_CodecNameIsConstant verifies Name is always "Lucene91Codec".
func TestRWCodec_CodecNameIsConstant(t *testing.T) {
	c := NewLucene91Codec("any")
	if c.Name != "Lucene91Codec" {
		t.Errorf("Name: got %q, want %q", c.Name, "Lucene91Codec")
	}
}

// TestRWCodec_ReaderConstructor verifies the reader type is constructible.
func TestRWCodec_ReaderConstructor(t *testing.T) {
	r := NewLucene91HnswVectorsReader("9.1.0")
	if r.Name != "Lucene91HnswVectorsReader" {
		t.Errorf("Name: got %q, want %q", r.Name, "Lucene91HnswVectorsReader")
	}
	if r.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", r.Version, "9.1.0")
	}
}

// TestRWCodec_GraphConstructor verifies the on-heap graph type.
func TestRWCodec_GraphConstructor(t *testing.T) {
	g := NewLucene91OnHeapHnswGraph("9.1.0")
	if g.Name != "Lucene91OnHeapHnswGraph" {
		t.Errorf("Name: got %q, want %q", g.Name, "Lucene91OnHeapHnswGraph")
	}
	if g.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", g.Version, "9.1.0")
	}
}

// TestRWCodec_NeighborArrayConstructor verifies the neighbor array type.
func TestRWCodec_NeighborArrayConstructor(t *testing.T) {
	n := NewLucene91NeighborArray("9.1.0")
	if n.Name != "Lucene91NeighborArray" {
		t.Errorf("Name: got %q, want %q", n.Name, "Lucene91NeighborArray")
	}
	if n.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", n.Version, "9.1.0")
	}
}

// TestRWCodec_BoundsCheckerConstructor verifies the bounds checker type.
func TestRWCodec_BoundsCheckerConstructor(t *testing.T) {
	b := NewLucene91BoundsChecker("9.1.0")
	if b.Name != "Lucene91BoundsChecker" {
		t.Errorf("Name: got %q, want %q", b.Name, "Lucene91BoundsChecker")
	}
	if b.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", b.Version, "9.1.0")
	}
}

// TestRWCodec_FormatConstructor verifies the format type.
func TestRWCodec_FormatConstructor(t *testing.T) {
	f := NewLucene91HnswVectorsFormat("9.1.0")
	if f.Name != "Lucene91HnswVectorsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene91HnswVectorsFormat")
	}
	if f.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", f.Version, "9.1.0")
	}
}
