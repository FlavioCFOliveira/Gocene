// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene99

import (
	"testing"
)

// TestLucene99Codec_NameConstant verifies that the codec Name is always
// "Lucene99Codec".
func TestLucene99Codec_NameConstant(t *testing.T) {
	c1 := NewLucene99Codec("v1")
	c2 := NewLucene99Codec("v2")
	if c1.Name != c2.Name {
		t.Errorf("Name should be constant: c1.Name=%q c2.Name=%q", c1.Name, c2.Name)
	}
	if c1.Name != "Lucene99Codec" {
		t.Errorf("Name: got %q, want %q", c1.Name, "Lucene99Codec")
	}
}

// TestLucene99ScalarQuantizedVectorsFormat_Name verifies the format Name field.
func TestLucene99ScalarQuantizedVectorsFormat_Name(t *testing.T) {
	f := NewLucene99ScalarQuantizedVectorsFormat("9.9.0")
	if f.Name != "Lucene99ScalarQuantizedVectorsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene99ScalarQuantizedVectorsFormat")
	}
}

// TestLucene99ScalarQuantizedVectorsFormat_Version verifies the format Version field.
func TestLucene99ScalarQuantizedVectorsFormat_Version(t *testing.T) {
	f := NewLucene99ScalarQuantizedVectorsFormat("10.0")
	if f.Version != "10.0" {
		t.Errorf("Version: got %q, want %q", f.Version, "10.0")
	}
}

// TestLucene99ScalarQuantizedVectorsReader_NameConstant verifies the reader Name
// is always consistent.
func TestLucene99ScalarQuantizedVectorsReader_NameConstant(t *testing.T) {
	r1 := NewLucene99ScalarQuantizedVectorsReader("a")
	r2 := NewLucene99ScalarQuantizedVectorsReader("b")
	if r1.Name != r2.Name {
		t.Errorf("Name should be constant: r1.Name=%q r2.Name=%q", r1.Name, r2.Name)
	}
}
