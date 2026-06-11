// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene86

import "testing"

func TestLucene86PointsFormat_NewPointsFormat(t *testing.T) {
	pf := NewLucene86PointsFormat("9.0")
	if pf == nil {
		t.Fatal("NewLucene86PointsFormat returned nil")
	}
	if pf.Name != "Lucene86PointsFormat" {
		t.Fatalf("got Name=%q, want %q", pf.Name, "Lucene86PointsFormat")
	}
}

func TestLucene86PointsFormat_EmptyVersion(t *testing.T) {
	pf := NewLucene86PointsFormat("")
	if pf == nil {
		t.Fatal("NewLucene86PointsFormat returned nil")
	}
	if pf.Version != "" {
		t.Fatalf("got Version=%q, want empty", pf.Version)
	}
}

func TestLucene86PointsReader_NewPointsReader(t *testing.T) {
	pr := NewLucene86PointsReader("9.0")
	if pr == nil {
		t.Fatal("NewLucene86PointsReader returned nil")
	}
	if pr.Name != "Lucene86PointsReader" {
		t.Fatalf("got Name=%q, want %q", pr.Name, "Lucene86PointsReader")
	}
}

func TestLucene86PointsReader_Version(t *testing.T) {
	pr := NewLucene86PointsReader("reader-v1")
	if pr.Version != "reader-v1" {
		t.Fatalf("got Version=%q, want %q", pr.Version, "reader-v1")
	}
}
