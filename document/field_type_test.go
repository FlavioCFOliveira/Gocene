// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestFieldType mirrors selected scenarios from Lucene's TestFieldType.java
// (10.4.0): equality, copy constructor, attribute map, freeze semantics,
// dimension validation, and vector attributes.

func TestFieldType_Defaults(t *testing.T) {
	ft := NewFieldType()
	if ft.IsFrozen() {
		t.Fatalf("new FieldType should not be frozen")
	}
	if ft.IsStored() {
		t.Fatalf("default Stored should be false")
	}
	if ft.IsTokenized() {
		t.Fatalf("Gocene default Tokenized should be false (back-compat divergence)")
	}
	if got, want := ft.GetIndexOptions(), index.IndexOptionsNone; got != want {
		t.Fatalf("default IndexOptions = %v, want %v", got, want)
	}
	if got, want := ft.GetVectorEncoding(), index.VectorEncodingFloat32; got != want {
		t.Fatalf("default VectorEncoding = %v, want %v", got, want)
	}
	if got, want := ft.GetVectorSimilarityFunction(), index.VectorSimilarityFunctionEuclidean; got != want {
		t.Fatalf("default VectorSimilarityFunction = %v, want %v", got, want)
	}
	if ft.DocValuesSkipIndexType() != index.DocValuesSkipIndexTypeNone {
		t.Fatalf("default DocValuesSkipIndex should be None")
	}
}

func TestFieldType_LuceneDefaults(t *testing.T) {
	ft := NewLuceneFieldType()
	if !ft.IsTokenized() {
		t.Fatalf("Lucene-flavored FieldType should default Tokenized=true")
	}
}

func TestFieldType_CopyConstructor(t *testing.T) {
	src := NewFieldType()
	src.SetStored(true)
	src.SetIndexOptions(index.IndexOptionsDocs)
	src.SetDimensions(2, 4)
	src.SetVectorAttributes(8, index.VectorEncodingByte, index.VectorSimilarityFunctionCosine)
	src.PutAttribute("k", "v")
	src.Freeze()

	dup := NewFieldTypeFrom(src)
	if dup.IsFrozen() {
		t.Fatalf("copy must not inherit frozen state")
	}
	if !dup.Equals(src) {
		t.Fatalf("copy must be equal to source")
	}
	if got := dup.GetAttributes()["k"]; got != "v" {
		t.Fatalf("attribute lost in copy: got %q", got)
	}
	// Mutating the copy must not affect the source.
	dup.PutAttribute("k", "v2")
	if src.GetAttributes()["k"] != "v" {
		t.Fatalf("source attribute was mutated through copy")
	}
}

func TestFieldType_FreezePanics(t *testing.T) {
	ft := NewFieldType()
	ft.Freeze()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on mutating a frozen FieldType")
		}
	}()
	ft.SetStored(true)
}

func TestFieldType_SetDimensionsValidation(t *testing.T) {
	cases := []struct {
		name    string
		dim     int
		idxDim  int
		nb      int
		wantPan bool
	}{
		{"zeroes ok", 0, 0, 0, false},
		{"normal ok", 2, 2, 4, false},
		{"indexed-less", 4, 2, 8, false},
		{"neg dim", -1, 0, 4, true},
		{"neg idx", 2, -1, 4, true},
		{"idx>dim", 2, 3, 4, true},
		{"neg nb", 2, 2, -1, true},
		{"zero nb with dim>0", 2, 2, 0, true},
		{"idx>0 with dim==0", 0, 1, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ft := NewFieldType()
			defer func() {
				r := recover()
				if c.wantPan && r == nil {
					t.Fatalf("expected panic for %+v", c)
				}
				if !c.wantPan && r != nil {
					t.Fatalf("unexpected panic for %+v: %v", c, r)
				}
			}()
			ft.SetDimensionsIndexed(c.dim, c.idxDim, c.nb)
		})
	}
}

func TestFieldType_SetVectorAttributes(t *testing.T) {
	ft := NewFieldType()
	ft.SetVectorAttributes(128, index.VectorEncodingFloat32, index.VectorSimilarityFunctionDotProduct)
	if ft.GetVectorDimension() != 128 {
		t.Fatalf("vector dim = %d", ft.GetVectorDimension())
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for non-positive vector dim")
		}
	}()
	ft.SetVectorAttributes(0, index.VectorEncodingFloat32, index.VectorSimilarityFunctionDotProduct)
}

func TestFieldType_String(t *testing.T) {
	ft := NewFieldType()
	ft.SetStored(true)
	ft.SetIndexed(true)
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqs)
	ft.SetTokenized(true)
	ft.SetStoreTermVectors(true)
	ft.SetStoreTermVectorOffsets(true)
	s := ft.String()
	if !strings.Contains(s, "stored") || !strings.Contains(s, "indexed") {
		t.Fatalf("string %q missing stored/indexed", s)
	}
	if !strings.Contains(s, "termVectorOffsets") {
		t.Fatalf("string %q missing termVectorOffsets", s)
	}
}

func TestFieldType_PutAttribute(t *testing.T) {
	ft := NewFieldType()
	if prev := ft.PutAttribute("a", "1"); prev != "" {
		t.Fatalf("expected empty prev for first put, got %q", prev)
	}
	if prev := ft.PutAttribute("a", "2"); prev != "1" {
		t.Fatalf("expected prev=1, got %q", prev)
	}
	if got := ft.GetAttributes()["a"]; got != "2" {
		t.Fatalf("attribute not updated: got %q", got)
	}
	ft.Freeze()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on put after freeze")
		}
	}()
	ft.PutAttribute("x", "y")
}
