// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	"testing"
)

// TestNewMatchingReaders verifies that NewMatchingReaders builds a MatchingReaders
// with the expected name and version fields.
func TestNewMatchingReaders(t *testing.T) {
	m := NewMatchingReaders("1.0")
	if m.Name != "MatchingReaders" {
		t.Errorf("Name: got %q want %q", m.Name, "MatchingReaders")
	}
	if m.Version != "1.0" {
		t.Errorf("Version: got %q want %q", m.Version, "1.0")
	}
}

// TestMatchingReaders_VersionVariants verifies Version preservation.
func TestMatchingReaders_VersionVariants(t *testing.T) {
	versions := []string{"", "0.9", "1.0", "2.0.0"}
	for _, v := range versions {
		m := NewMatchingReaders(v)
		if m.Version != v {
			t.Errorf("Version: got %q want %q", m.Version, v)
		}
	}
}

// TestMatchingReaders_UniqueInstances verifies each call returns a distinct pointer.
func TestMatchingReaders_UniqueInstances(t *testing.T) {
	a := NewMatchingReaders("1.0")
	b := NewMatchingReaders("1.0")
	if a == b {
		t.Error("NewMatchingReaders must return a new instance on each call")
	}
}

// TestAllConstructors_ConsistentName verifies that all four constructor types in
// this package produce the correct type-dedicated Name values.
func TestAllConstructors_ConsistentName(t *testing.T) {
	if n := NewCompressionMode("x").Name; n != "CompressionMode" {
		t.Errorf("CompressionMode.Name: got %q want %q", n, "CompressionMode")
	}
	if n := NewCompressor("x").Name; n != "Compressor" {
		t.Errorf("Compressor.Name: got %q want %q", n, "Compressor")
	}
	if n := NewDecompressor("x").Name; n != "Decompressor" {
		t.Errorf("Decompressor.Name: got %q want %q", n, "Decompressor")
	}
	if n := NewMatchingReaders("x").Name; n != "MatchingReaders" {
		t.Errorf("MatchingReaders.Name: got %q want %q", n, "MatchingReaders")
	}
}
