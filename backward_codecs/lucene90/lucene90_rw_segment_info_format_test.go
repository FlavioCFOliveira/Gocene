// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import "testing"

func TestLucene90SegmentInfoFormat_New(t *testing.T) {
	f := NewLucene90SegmentInfoFormat("1.0")
	if f == nil {
		t.Fatal("NewLucene90SegmentInfoFormat returned nil")
	}
	if f.Name != "Lucene90SegmentInfoFormat" {
		t.Fatalf("got Name=%q, want %q", f.Name, "Lucene90SegmentInfoFormat")
	}
}

func TestLucene90SegmentInfoFormat_Version(t *testing.T) {
	f := NewLucene90SegmentInfoFormat("s90")
	if f.Version != "s90" {
		t.Fatalf("got Version=%q, want %q", f.Version, "s90")
	}
}

func TestLucene90FieldInfosFormat_New(t *testing.T) {
	f := NewLucene90FieldInfosFormat("1.0")
	if f == nil {
		t.Fatal("NewLucene90FieldInfosFormat returned nil")
	}
	if f.Name != "Lucene90FieldInfosFormat" {
		t.Fatalf("got Name=%q, want %q", f.Name, "Lucene90FieldInfosFormat")
	}
}

func TestLucene90FieldInfosFormat_Version(t *testing.T) {
	f := NewLucene90FieldInfosFormat("fi90")
	if f.Version != "fi90" {
		t.Fatalf("got Version=%q, want %q", f.Version, "fi90")
	}
}
