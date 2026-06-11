// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	"testing"
)

// TestNewCompressor verifies that NewCompressor builds a Compressor with the
// expected name and version fields.
func TestNewCompressor(t *testing.T) {
	c := NewCompressor("1.0")
	if c.Name != "Compressor" {
		t.Errorf("Name: got %q want %q", c.Name, "Compressor")
	}
	if c.Version != "1.0" {
		t.Errorf("Version: got %q want %q", c.Version, "1.0")
	}
}

// TestCompressor_VersionVariants verifies Version preservation.
func TestCompressor_VersionVariants(t *testing.T) {
	versions := []string{"", "0.9", "1.0", "2.0.0"}
	for _, v := range versions {
		c := NewCompressor(v)
		if c.Version != v {
			t.Errorf("Version: got %q want %q", c.Version, v)
		}
	}
}

// TestCompressor_UniqueInstances verifies each call returns a distinct pointer.
func TestCompressor_UniqueInstances(t *testing.T) {
	a := NewCompressor("1.0")
	b := NewCompressor("1.0")
	if a == b {
		t.Error("NewCompressor must return a new instance on each call")
	}
}
