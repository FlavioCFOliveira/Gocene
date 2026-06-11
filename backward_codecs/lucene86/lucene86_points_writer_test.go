// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene86

import "testing"

func TestLucene86PointsFormat_New(t *testing.T) {
	f := NewLucene86PointsFormat("1.0")
	if f == nil {
		t.Fatal("NewLucene86PointsFormat returned nil")
	}
	if f.Name != "Lucene86PointsFormat" {
		t.Fatalf("got Name=%q, want %q", f.Name, "Lucene86PointsFormat")
	}
}

func TestLucene86PointsFormat_Version(t *testing.T) {
	f := NewLucene86PointsFormat("v2")
	if f.Version != "v2" {
		t.Fatalf("got Version=%q, want %q", f.Version, "v2")
	}
}

func TestLucene86PointsReader_New(t *testing.T) {
	r := NewLucene86PointsReader("1.0")
	if r == nil {
		t.Fatal("NewLucene86PointsReader returned nil")
	}
	if r.Name != "Lucene86PointsReader" {
		t.Fatalf("got Name=%q, want %q", r.Name, "Lucene86PointsReader")
	}
}
