// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene91

import (
	"testing"
)

// TestGraphBuilder_BoundsChecker verifies that Lucene91BoundsChecker is
// constructible and sets Name and Version correctly.
//
// In the Java test tree, Lucene91HnswGraphBuilder is a test-support class; in
// Gocene the production types that compose with the graph builder are tested here.
func TestGraphBuilder_BoundsChecker(t *testing.T) {
	b := NewLucene91BoundsChecker("9.1.0")
	if b.Name != "Lucene91BoundsChecker" {
		t.Errorf("Name: got %q, want %q", b.Name, "Lucene91BoundsChecker")
	}
	if b.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", b.Version, "9.1.0")
	}
}

// TestGraphBuilder_HnswVectorsFormat verifies that the format type is constructible.
func TestGraphBuilder_HnswVectorsFormat(t *testing.T) {
	f := NewLucene91HnswVectorsFormat("9.1.0")
	if f.Name != "Lucene91HnswVectorsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene91HnswVectorsFormat")
	}
	if f.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", f.Version, "9.1.0")
	}
}

// TestGraphBuilder_NeighborArray verifies that the neighbor array type is
// constructible.
func TestGraphBuilder_NeighborArray(t *testing.T) {
	n := NewLucene91NeighborArray("9.1.0")
	if n.Name != "Lucene91NeighborArray" {
		t.Errorf("Name: got %q, want %q", n.Name, "Lucene91NeighborArray")
	}
	if n.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", n.Version, "9.1.0")
	}
}

// TestGraphBuilder_OnHeapHnswGraph verifies that the on-heap graph type is
// constructible.
func TestGraphBuilder_OnHeapHnswGraph(t *testing.T) {
	g := NewLucene91OnHeapHnswGraph("9.1.0")
	if g.Name != "Lucene91OnHeapHnswGraph" {
		t.Errorf("Name: got %q, want %q", g.Name, "Lucene91OnHeapHnswGraph")
	}
	if g.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", g.Version, "9.1.0")
	}
}

// TestGraphBuilder_HnswVectorsReader verifies that the reader type is constructible.
func TestGraphBuilder_HnswVectorsReader(t *testing.T) {
	r := NewLucene91HnswVectorsReader("9.1.0")
	if r.Name != "Lucene91HnswVectorsReader" {
		t.Errorf("Name: got %q, want %q", r.Name, "Lucene91HnswVectorsReader")
	}
	if r.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", r.Version, "9.1.0")
	}
}

// TestGraphBuilder_Codec verifies that the codec type is constructible.
func TestGraphBuilder_Codec(t *testing.T) {
	c := NewLucene91Codec("9.1.0")
	if c.Name != "Lucene91Codec" {
		t.Errorf("Name: got %q, want %q", c.Name, "Lucene91Codec")
	}
	if c.Version != "9.1.0" {
		t.Errorf("Version: got %q, want %q", c.Version, "9.1.0")
	}
}
