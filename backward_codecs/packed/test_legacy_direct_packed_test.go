// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"testing"
)

// TestNewLegacyDirectReader verifies the constructor sets Name and Version.
func TestNewLegacyDirectReader(t *testing.T) {
	r := NewLegacyDirectReader("1.0")
	if r.Name != "LegacyDirectReader" {
		t.Errorf("Name: got %q want %q", r.Name, "LegacyDirectReader")
	}
	if r.Version != "1.0" {
		t.Errorf("Version: got %q want %q", r.Version, "1.0")
	}
}

// TestLegacyDirectReader_VersionVariants verifies version preservation.
func TestLegacyDirectReader_VersionVariants(t *testing.T) {
	versions := []string{"", "0.9", "1.0", "2.0.0"}
	for _, v := range versions {
		r := NewLegacyDirectReader(v)
		if r.Version != v {
			t.Errorf("Version: got %q want %q", r.Version, v)
		}
	}
}

// TestNewLegacyDirectWriter verifies the constructor sets Name and Version.
func TestNewLegacyDirectWriter(t *testing.T) {
	w := NewLegacyDirectWriter("1.0")
	if w.Name != "LegacyDirectWriter" {
		t.Errorf("Name: got %q want %q", w.Name, "LegacyDirectWriter")
	}
	if w.Version != "1.0" {
		t.Errorf("Version: got %q want %q", w.Version, "1.0")
	}
}

// TestLegacyDirectWriter_VersionVariants verifies version preservation.
func TestLegacyDirectWriter_VersionVariants(t *testing.T) {
	versions := []string{"", "0.9", "1.0", "2.0.0"}
	for _, v := range versions {
		w := NewLegacyDirectWriter(v)
		if w.Version != v {
			t.Errorf("Version: got %q want %q", w.Version, v)
		}
	}
}

// TestNewLegacyPackedInts verifies the constructor sets Name and Version.
func TestNewLegacyPackedInts(t *testing.T) {
	p := NewLegacyPackedInts("1.0")
	if p.Name != "LegacyPackedInts" {
		t.Errorf("Name: got %q want %q", p.Name, "LegacyPackedInts")
	}
	if p.Version != "1.0" {
		t.Errorf("Version: got %q want %q", p.Version, "1.0")
	}
}

// TestLegacyDirectTypes_UniqueInstances verifies each constructor returns a
// distinct pointer.
func TestLegacyDirectTypes_UniqueInstances(t *testing.T) {
	if NewLegacyDirectReader("x") == NewLegacyDirectReader("x") {
		t.Error("LegacyDirectReader must return distinct instances")
	}
	if NewLegacyDirectWriter("x") == NewLegacyDirectWriter("x") {
		t.Error("LegacyDirectWriter must return distinct instances")
	}
	if NewLegacyPackedInts("x") == NewLegacyPackedInts("x") {
		t.Error("LegacyPackedInts must return distinct instances")
	}
}
