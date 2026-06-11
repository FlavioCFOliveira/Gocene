// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	"testing"
)

// TestNewCompressionMode verifies that NewCompressionMode builds a CompressionMode
// with the expected name and version fields.
func TestNewCompressionMode(t *testing.T) {
	m := NewCompressionMode("1.0")
	if m.Name != "CompressionMode" {
		t.Errorf("Name: got %q want %q", m.Name, "CompressionMode")
	}
	if m.Version != "1.0" {
		t.Errorf("Version: got %q want %q", m.Version, "1.0")
	}
}

// TestCompressionMode_VersionVariants verifies that Version is preserved as-is.
func TestCompressionMode_VersionVariants(t *testing.T) {
	versions := []string{"", "0.9", "1.0", "2.0.0", "10.4.0"}
	for _, v := range versions {
		m := NewCompressionMode(v)
		if m.Version != v {
			t.Errorf("Version: got %q want %q", m.Version, v)
		}
	}
}

// TestCompressionMode_UniqueInstances verifies each call returns a distinct pointer.
func TestCompressionMode_UniqueInstances(t *testing.T) {
	a := NewCompressionMode("1.0")
	b := NewCompressionMode("1.0")
	if a == b {
		t.Error("NewCompressionMode must return a new instance on each call")
	}
}
