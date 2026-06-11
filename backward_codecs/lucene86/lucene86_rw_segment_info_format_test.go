// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene86

import "testing"

func TestLucene86SegmentInfoFormat_New(t *testing.T) {
	f := NewLucene86SegmentInfoFormat("1.0")
	if f == nil {
		t.Fatal("NewLucene86SegmentInfoFormat returned nil")
	}
	if f.Name != "Lucene86SegmentInfoFormat" {
		t.Fatalf("got Name=%q, want %q", f.Name, "Lucene86SegmentInfoFormat")
	}
}

func TestLucene86SegmentInfoFormat_Version(t *testing.T) {
	f := NewLucene86SegmentInfoFormat("v86")
	if f.Version != "v86" {
		t.Fatalf("got Version=%q, want %q", f.Version, "v86")
	}
}
