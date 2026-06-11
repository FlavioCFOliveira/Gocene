// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import "testing"

func TestLucene90HnswVectorsFormat_New(t *testing.T) {
	f := NewLucene90HnswVectorsFormat("1.0")
	if f == nil {
		t.Fatal("NewLucene90HnswVectorsFormat returned nil")
	}
	if f.Name != "Lucene90HnswVectorsFormat" {
		t.Fatalf("got Name=%q, want %q", f.Name, "Lucene90HnswVectorsFormat")
	}
}

func TestLucene90HnswVectorsFormat_Version(t *testing.T) {
	f := NewLucene90HnswVectorsFormat("v2")
	if f.Version != "v2" {
		t.Fatalf("got Version=%q, want %q", f.Version, "v2")
	}
}

func TestLucene90HnswVectorsReader_New(t *testing.T) {
	r := NewLucene90HnswVectorsReader("1.0")
	if r == nil {
		t.Fatal("NewLucene90HnswVectorsReader returned nil")
	}
	if r.Name != "Lucene90HnswVectorsReader" {
		t.Fatalf("got Name=%q, want %q", r.Name, "Lucene90HnswVectorsReader")
	}
}

func TestLucene90HnswVectorsReader_Version(t *testing.T) {
	r := NewLucene90HnswVectorsReader("v3")
	if r.Version != "v3" {
		t.Fatalf("got Version=%q, want %q", r.Version, "v3")
	}
}
