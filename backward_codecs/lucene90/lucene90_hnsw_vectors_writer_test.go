// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import "testing"

func TestLucene90HnswGraphBuilder_New(t *testing.T) {
	b := NewLucene90HnswGraphBuilder("1.0")
	if b == nil {
		t.Fatal("NewLucene90HnswGraphBuilder returned nil")
	}
	if b.Name != "Lucene90HnswGraphBuilder" {
		t.Fatalf("got Name=%q, want %q", b.Name, "Lucene90HnswGraphBuilder")
	}
}

func TestLucene90HnswGraphBuilder_Version(t *testing.T) {
	b := NewLucene90HnswGraphBuilder("v2")
	if b.Version != "v2" {
		t.Fatalf("got Version=%q, want %q", b.Version, "v2")
	}
}

func TestLucene90NeighborArray_New(t *testing.T) {
	n := NewLucene90NeighborArray("1.0")
	if n == nil {
		t.Fatal("NewLucene90NeighborArray returned nil")
	}
	if n.Name != "Lucene90NeighborArray" {
		t.Fatalf("got Name=%q, want %q", n.Name, "Lucene90NeighborArray")
	}
}

func TestLucene90NeighborArray_Version(t *testing.T) {
	n := NewLucene90NeighborArray("v3")
	if n.Version != "v3" {
		t.Fatalf("got Version=%q, want %q", n.Version, "v3")
	}
}

func TestLucene90OnHeapHnswGraph_New(t *testing.T) {
	g := NewLucene90OnHeapHnswGraph("1.0")
	if g == nil {
		t.Fatal("NewLucene90OnHeapHnswGraph returned nil")
	}
	if g.Name != "Lucene90OnHeapHnswGraph" {
		t.Fatalf("got Name=%q, want %q", g.Name, "Lucene90OnHeapHnswGraph")
	}
}

func TestLucene90OnHeapHnswGraph_Version(t *testing.T) {
	g := NewLucene90OnHeapHnswGraph("v4")
	if g.Version != "v4" {
		t.Fatalf("got Version=%q, want %q", g.Version, "v4")
	}
}
