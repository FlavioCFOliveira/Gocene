// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene86

import "testing"

func TestLucene86Codec_New(t *testing.T) {
	c := NewLucene86Codec("1.0")
	if c == nil {
		t.Fatal("NewLucene86Codec returned nil")
	}
	if c.Name != "Lucene86Codec" {
		t.Fatalf("got Name=%q, want %q", c.Name, "Lucene86Codec")
	}
}

func TestLucene86Codec_Version(t *testing.T) {
	c := NewLucene86Codec("86.0")
	if c.Version != "86.0" {
		t.Fatalf("got Version=%q, want %q", c.Version, "86.0")
	}
}
