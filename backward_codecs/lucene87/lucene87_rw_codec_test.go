// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene87

import "testing"

func TestLucene87Codec_New(t *testing.T) {
	c := NewLucene87Codec("1.0")
	if c == nil {
		t.Fatal("NewLucene87Codec returned nil")
	}
	if c.Name != "Lucene87Codec" {
		t.Fatalf("got Name=%q, want %q", c.Name, "Lucene87Codec")
	}
}

func TestLucene87Codec_Version(t *testing.T) {
	c := NewLucene87Codec("87.0")
	if c.Version != "87.0" {
		t.Fatalf("got Version=%q, want %q", c.Version, "87.0")
	}
}
