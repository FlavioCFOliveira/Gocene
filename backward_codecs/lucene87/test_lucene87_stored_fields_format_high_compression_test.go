// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene87

import "testing"

func TestLZ4WithPresetDictCompressionMode_New(t *testing.T) {
	m := NewLZ4WithPresetDictCompressionMode("1.0")
	if m == nil {
		t.Fatal("NewLZ4WithPresetDictCompressionMode returned nil")
	}
	if m.Name != "LZ4WithPresetDictCompressionMode" {
		t.Fatalf("got Name=%q, want %q", m.Name, "LZ4WithPresetDictCompressionMode")
	}
}

func TestLZ4WithPresetDictCompressionMode_Version(t *testing.T) {
	m := NewLZ4WithPresetDictCompressionMode("lz4-v2")
	if m.Version != "lz4-v2" {
		t.Fatalf("got Version=%q, want %q", m.Version, "lz4-v2")
	}
}

func TestDeflateWithPresetDictCompressionMode_New(t *testing.T) {
	m := NewDeflateWithPresetDictCompressionMode("1.0")
	if m == nil {
		t.Fatal("NewDeflateWithPresetDictCompressionMode returned nil")
	}
	if m.Name != "DeflateWithPresetDictCompressionMode" {
		t.Fatalf("got Name=%q, want %q", m.Name, "DeflateWithPresetDictCompressionMode")
	}
}

func TestDeflateWithPresetDictCompressionMode_Version(t *testing.T) {
	m := NewDeflateWithPresetDictCompressionMode("deflate-v3")
	if m.Version != "deflate-v3" {
		t.Fatalf("got Version=%q, want %q", m.Version, "deflate-v3")
	}
}
