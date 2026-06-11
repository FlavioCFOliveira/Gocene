// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene91

import (
	"testing"
)

// TestWriter_FormatConstructor verifies that Lucene91HnswVectorsFormat
// is constructible and sets fields correctly.
//
// In the Java test tree, Lucene91HnswVectorsWriter is a test-support class; in
// Gocene the production format that a writer would compose with is tested here.
func TestWriter_FormatConstructor(t *testing.T) {
	f := NewLucene91HnswVectorsFormat("9.1.0")
	if f.Name != "Lucene91HnswVectorsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene91HnswVectorsFormat")
	}
	if f.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", f.Version, "9.1.0")
	}
}

// TestWriter_FormatCustomVersion verifies a different version string.
func TestWriter_FormatCustomVersion(t *testing.T) {
	f := NewLucene91HnswVectorsFormat("10.4.0")
	if f.Version != "10.4.0" {
		t.Errorf("Version: got %q, want %q", f.Version, "10.4.0")
	}
}

// TestWriter_ReaderConstructor verifies the reader type is constructible.
func TestWriter_ReaderConstructor(t *testing.T) {
	r := NewLucene91HnswVectorsReader("9.1.0")
	if r.Name != "Lucene91HnswVectorsReader" {
		t.Errorf("Name: got %q, want %q", r.Name, "Lucene91HnswVectorsReader")
	}
	if r.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", r.Version, "9.1.0")
	}
}

// TestWriter_BoundsCheckerConstructor verifies the bounds checker type.
func TestWriter_BoundsCheckerConstructor(t *testing.T) {
	b := NewLucene91BoundsChecker("9.1.0")
	if b.Name != "Lucene91BoundsChecker" {
		t.Errorf("Name: got %q, want %q", b.Name, "Lucene91BoundsChecker")
	}
	if b.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", b.Version, "9.1.0")
	}
}

// TestWriter_CodecConstructor verifies the codec type is constructible.
func TestWriter_CodecConstructor(t *testing.T) {
	c := NewLucene91Codec("9.1.0")
	if c.Name != "Lucene91Codec" {
		t.Errorf("Name: got %q, want %q", c.Name, "Lucene91Codec")
	}
	if c.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", c.Version, "9.1.0")
	}
}

// TestWriter_NeighborArrayConstructor verifies the neighbor array type.
func TestWriter_NeighborArrayConstructor(t *testing.T) {
	n := NewLucene91NeighborArray("9.1.0")
	if n.Name != "Lucene91NeighborArray" {
		t.Errorf("Name: got %q, want %q", n.Name, "Lucene91NeighborArray")
	}
	if n.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", n.Version, "9.1.0")
	}
}
