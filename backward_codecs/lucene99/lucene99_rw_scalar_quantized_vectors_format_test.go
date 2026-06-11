// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene99

import (
	"testing"
)

// TestNewLucene99ScalarQuantizedVectorsFormat_Defaults verifies that
// NewLucene99ScalarQuantizedVectorsFormat sets Name and Version correctly.
func TestNewLucene99ScalarQuantizedVectorsFormat_Defaults(t *testing.T) {
	f := NewLucene99ScalarQuantizedVectorsFormat("9.9.0")
	if f.Name != "Lucene99ScalarQuantizedVectorsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene99ScalarQuantizedVectorsFormat")
	}
	if f.Version != "9.9.0" {
		t.Errorf("Version: got %q, want %q", f.Version, "9.9.0")
	}
}

// TestNewLucene99ScalarQuantizedVectorsReader_Defaults verifies that
// NewLucene99ScalarQuantizedVectorsReader sets Name and Version correctly.
func TestNewLucene99ScalarQuantizedVectorsReader_Defaults(t *testing.T) {
	r := NewLucene99ScalarQuantizedVectorsReader("test")
	if r.Name != "Lucene99ScalarQuantizedVectorsReader" {
		t.Errorf("Name: got %q, want %q", r.Name, "Lucene99ScalarQuantizedVectorsReader")
	}
	if r.Version != "test" {
		t.Errorf("Version: got %q, want %q", r.Version, "test")
	}
}

// TestNewOffHeapQuantizedByteVectorValues_Defaults verifies that
// NewOffHeapQuantizedByteVectorValues sets Name and Version correctly.
func TestNewOffHeapQuantizedByteVectorValues_Defaults(t *testing.T) {
	v := NewOffHeapQuantizedByteVectorValues("9.9.0")
	if v.Name != "OffHeapQuantizedByteVectorValues" {
		t.Errorf("Name: got %q, want %q", v.Name, "OffHeapQuantizedByteVectorValues")
	}
	if v.Version != "9.9.0" {
		t.Errorf("Version: got %q, want %q", v.Version, "9.9.0")
	}
}

// TestNewOffHeapQuantizedByteVectorValues_EmptyVersion verifies that an empty
// version is accepted.
func TestNewOffHeapQuantizedByteVectorValues_EmptyVersion(t *testing.T) {
	v := NewOffHeapQuantizedByteVectorValues("")
	if v.Version != "" {
		t.Errorf("Version: got %q, want empty", v.Version)
	}
}
