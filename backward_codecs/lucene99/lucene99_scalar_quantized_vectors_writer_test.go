// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene99

import (
	"testing"
)

// TestNewLucene99ScalarQuantizedVectorsReader_Name verifies the reader's Name
// field matches the Java class name.
func TestNewLucene99ScalarQuantizedVectorsReader_Name(t *testing.T) {
	r := NewLucene99ScalarQuantizedVectorsReader("test")
	if r.Name != "Lucene99ScalarQuantizedVectorsReader" {
		t.Errorf("Name: got %q, want %q", r.Name, "Lucene99ScalarQuantizedVectorsReader")
	}
}

// TestNewLucene99ScalarQuantizedVectorsReader_Version verifies the reader's
// Version field.
func TestNewLucene99ScalarQuantizedVectorsReader_Version(t *testing.T) {
	r := NewLucene99ScalarQuantizedVectorsReader("9.9.0")
	if r.Version != "9.9.0" {
		t.Errorf("Version: got %q, want %q", r.Version, "9.9.0")
	}
}

// TestNewLucene99Codec_Defaults verifies that NewLucene99Codec sets Name and
// Version correctly.
func TestNewLucene99Codec_Defaults(t *testing.T) {
	c := NewLucene99Codec("9.9.0")
	if c.Name != "Lucene99Codec" {
		t.Errorf("Name: got %q, want %q", c.Name, "Lucene99Codec")
	}
	if c.Version != "9.9.0" {
		t.Errorf("Version: got %q, want %q", c.Version, "9.9.0")
	}
}

// TestNewLucene99Codec_MultipleVersions verifies that NewLucene99Codec stores
// different version strings correctly.
func TestNewLucene99Codec_MultipleVersions(t *testing.T) {
	versions := []string{"", "9.9", "9.9.0", "10.4.0", "v0", "v1"}
	for _, v := range versions {
		c := NewLucene99Codec(v)
		if c.Version != v {
			t.Errorf("Version: got %q, want %q", c.Version, v)
		}
	}
}
