// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	"testing"
)

// TestNewDecompressor verifies that NewDecompressor builds a Decompressor with
// the expected name and version fields.
func TestNewDecompressor(t *testing.T) {
	d := NewDecompressor("1.0")
	if d.Name != "Decompressor" {
		t.Errorf("Name: got %q want %q", d.Name, "Decompressor")
	}
	if d.Version != "1.0" {
		t.Errorf("Version: got %q want %q", d.Version, "1.0")
	}
}

// TestDecompressor_VersionVariants verifies Version preservation.
func TestDecompressor_VersionVariants(t *testing.T) {
	versions := []string{"", "0.9", "1.0", "2.0.0"}
	for _, v := range versions {
		d := NewDecompressor(v)
		if d.Version != v {
			t.Errorf("Version: got %q want %q", d.Version, v)
		}
	}
}

// TestDecompressor_UniqueInstances verifies each call returns a distinct pointer.
func TestDecompressor_UniqueInstances(t *testing.T) {
	a := NewDecompressor("1.0")
	b := NewDecompressor("1.0")
	if a == b {
		t.Error("NewDecompressor must return a new instance on each call")
	}
}
