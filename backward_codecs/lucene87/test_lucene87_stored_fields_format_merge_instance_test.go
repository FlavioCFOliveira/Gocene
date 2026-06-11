// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene87

import "testing"

func TestLZ4WithPresetDictCompressionMode_EmptyVersion(t *testing.T) {
	m := NewLZ4WithPresetDictCompressionMode("")
	if m == nil {
		t.Fatal("NewLZ4WithPresetDictCompressionMode returned nil")
	}
	if m.Version != "" {
		t.Fatalf("got Version=%q, want empty", m.Version)
	}
}

func TestDeflateWithPresetDictCompressionMode_EmptyVersion(t *testing.T) {
	m := NewDeflateWithPresetDictCompressionMode("")
	if m == nil {
		t.Fatal("NewDeflateWithPresetDictCompressionMode returned nil")
	}
	if m.Version != "" {
		t.Fatalf("got Version=%q, want empty", m.Version)
	}
}

func TestLZ4WithPresetDictCompressionMode_Name(t *testing.T) {
	m := NewLZ4WithPresetDictCompressionMode("1.0")
	if m.Name != "LZ4WithPresetDictCompressionMode" {
		t.Fatalf("got Name=%q, want %q", m.Name, "LZ4WithPresetDictCompressionMode")
	}
}

func TestDeflateWithPresetDictCompressionMode_Name(t *testing.T) {
	m := NewDeflateWithPresetDictCompressionMode("1.0")
	if m.Name != "DeflateWithPresetDictCompressionMode" {
		t.Fatalf("got Name=%q, want %q", m.Name, "DeflateWithPresetDictCompressionMode")
	}
}
