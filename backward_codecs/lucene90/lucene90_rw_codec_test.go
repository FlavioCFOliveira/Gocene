// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import "testing"

func TestLucene90Codec_New(t *testing.T) {
	c := NewLucene90Codec("1.0")
	if c == nil {
		t.Fatal("NewLucene90Codec returned nil")
	}
	if c.Name != "Lucene90Codec" {
		t.Fatalf("got Name=%q, want %q", c.Name, "Lucene90Codec")
	}
}

func TestLucene90Codec_Version(t *testing.T) {
	c := NewLucene90Codec("90.0")
	if c.Version != "90.0" {
		t.Fatalf("got Version=%q, want %q", c.Version, "90.0")
	}
}
