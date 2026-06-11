// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import "testing"

func TestCompressionAlgorithm_New(t *testing.T) {
	a := NewCompressionAlgorithm("1.0")
	if a == nil {
		t.Fatal("NewCompressionAlgorithm returned nil")
	}
	if a.Name != "CompressionAlgorithm" {
		t.Fatalf("got Name=%q, want %q", a.Name, "CompressionAlgorithm")
	}
}

func TestCompressionAlgorithm_Version(t *testing.T) {
	a := NewCompressionAlgorithm("v2")
	if a.Version != "v2" {
		t.Fatalf("got Version=%q, want %q", a.Version, "v2")
	}
}

func TestFieldReader_New(t *testing.T) {
	r := NewFieldReader("1.0")
	if r == nil {
		t.Fatal("NewFieldReader returned nil")
	}
	if r.Name != "FieldReader" {
		t.Fatalf("got Name=%q, want %q", r.Name, "FieldReader")
	}
}

func TestFieldReader_Version(t *testing.T) {
	r := NewFieldReader("fr-v1")
	if r.Version != "fr-v1" {
		t.Fatalf("got Version=%q, want %q", r.Version, "fr-v1")
	}
}

func TestStats_New(t *testing.T) {
	s := NewStats("1.0")
	if s == nil {
		t.Fatal("NewStats returned nil")
	}
	if s.Name != "Stats" {
		t.Fatalf("got Name=%q, want %q", s.Name, "Stats")
	}
}

func TestStats_Version(t *testing.T) {
	s := NewStats("st-v1")
	if s.Version != "st-v1" {
		t.Fatalf("got Version=%q, want %q", s.Version, "st-v1")
	}
}

func TestLucene90BlockTreeTermsReader_New(t *testing.T) {
	r := NewLucene90BlockTreeTermsReader("1.0")
	if r == nil {
		t.Fatal("NewLucene90BlockTreeTermsReader returned nil")
	}
	if r.Name != "Lucene90BlockTreeTermsReader" {
		t.Fatalf("got Name=%q, want %q", r.Name, "Lucene90BlockTreeTermsReader")
	}
}

func TestLucene90BlockTreeTermsReader_Version(t *testing.T) {
	r := NewLucene90BlockTreeTermsReader("btr-v1")
	if r.Version != "btr-v1" {
		t.Fatalf("got Version=%q, want %q", r.Version, "btr-v1")
	}
}

// TestConstants verifies the blocktree package constants.
func TestConstants(t *testing.T) {
	if outputFlagsNumBits != 2 {
		t.Fatalf("outputFlagsNumBits: got %d, want 2", outputFlagsNumBits)
	}
	if outputFlagIsFloor != 0x1 {
		t.Fatalf("outputFlagIsFloor: got %d, want 1", outputFlagIsFloor)
	}
	if outputFlagHasTerms != 0x2 {
		t.Fatalf("outputFlagHasTerms: got %d, want 2", outputFlagHasTerms)
	}
	if versionMSBVLongOutput != 1 {
		t.Fatalf("versionMSBVLongOutput: got %d, want 1", versionMSBVLongOutput)
	}
}
