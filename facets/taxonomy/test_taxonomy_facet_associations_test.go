// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy_test

// TestTaxonomyFacetAssociations ports assertions from
// org.apache.lucene.facet.taxonomy.TestTaxonomyFacetAssociations.
//
// Tests that require a full index+taxonomy write+search cycle are deferred
// with t.Skip; unit-testable parts (field construction, payload encoding,
// aggregation functions) run unconditionally.

import (
	"encoding/binary"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/facets/taxonomy"
)

// TestIntAssociationFacetField_Encoding mirrors the int-association encoding
// verified implicitly by the Java test's @BeforeClass index build.
func TestIntAssociationFacetField_Encoding(t *testing.T) {
	values := []int32{0, 1, -1, 42, -100, 0x7FFFFFFF, -0x80000000}
	for _, v := range values {
		f := taxonomy.NewIntAssociationFacetField(v, "int", "child")
		if f.Dim != "int" {
			t.Errorf("Dim: want %q, got %q", "int", f.Dim)
		}
		if f.Value != v {
			t.Errorf("Value: want %d, got %d", v, f.Value)
		}
		// Payload round-trip via IntAssociationFromBytes
		got := taxonomy.IntAssociationFromBytes(f.Association)
		if got != v {
			t.Errorf("round-trip(%d): got %d", v, got)
		}
		// Verify big-endian encoding
		wantBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(wantBytes, uint32(v))
		for i, b := range wantBytes {
			if f.Association[i] != b {
				t.Errorf("byte[%d] encoding(%d): want 0x%02x, got 0x%02x", i, v, b, f.Association[i])
			}
		}
	}
}

// TestFloatAssociationFacetField_Encoding mirrors float-association encoding.
func TestFloatAssociationFacetField_Encoding(t *testing.T) {
	values := []float32{0.0, 1.0, -1.0, 3.14, -2.71, 1e10, -1e-5}
	for _, v := range values {
		f := taxonomy.NewFloatAssociationFacetField(v, "float", "child")
		if f.Dim != "float" {
			t.Errorf("Dim: want %q, got %q", "float", f.Dim)
		}
		if f.Value != v {
			t.Errorf("Value: want %v, got %v", v, f.Value)
		}
		got := taxonomy.FloatAssociationFromBytes(f.Association)
		if got != v {
			t.Errorf("round-trip(%v): got %v", v, got)
		}
	}
}

// TestAssociationAggregationFunction_Sum verifies SUM aggregation.
func TestAssociationAggregationFunction_Sum(t *testing.T) {
	sum := taxonomy.SUM
	if sum.AggregateInt(3, 7) != 10 {
		t.Errorf("SUM int(3,7): want 10, got %d", sum.AggregateInt(3, 7))
	}
	if sum.AggregateInt(-5, 5) != 0 {
		t.Errorf("SUM int(-5,5): want 0, got %d", sum.AggregateInt(-5, 5))
	}
	if sum.Aggregate(1.5, 2.5) != 4.0 {
		t.Errorf("SUM float(1.5,2.5): want 4.0, got %v", sum.Aggregate(1.5, 2.5))
	}
}

// TestAssociationAggregationFunction_Max verifies MAX aggregation.
func TestAssociationAggregationFunction_Max(t *testing.T) {
	max := taxonomy.MAX
	if max.AggregateInt(3, 7) != 7 {
		t.Errorf("MAX int(3,7): want 7, got %d", max.AggregateInt(3, 7))
	}
	if max.AggregateInt(10, 5) != 10 {
		t.Errorf("MAX int(10,5): want 10, got %d", max.AggregateInt(10, 5))
	}
	if max.AggregateInt(-1, -2) != -1 {
		t.Errorf("MAX int(-1,-2): want -1, got %d", max.AggregateInt(-1, -2))
	}
	if max.Aggregate(3.0, 7.0) != 7.0 {
		t.Errorf("MAX float(3,7): want 7.0, got %v", max.Aggregate(3.0, 7.0))
	}
}

// TestAssociationFacetsConfig mirrors the Java @BeforeClass FacetsConfig
// construction with setIndexFieldName + setMultiValued.
func TestAssociationFacetsConfig(t *testing.T) {
	cfg := facets.NewFacetsConfig()
	cfg.SetIndexFieldName("int", "$facets.int")
	cfg.SetMultiValued("int", true)
	cfg.SetIndexFieldName("int_random", "$facets.int")
	cfg.SetMultiValued("int_random", true)
	cfg.SetIndexFieldName("float", "$facets.float")
	cfg.SetMultiValued("float", true)

	if cfg.GetDimConfig("int").IndexFieldName != "$facets.int" {
		t.Errorf("int index field: want $facets.int, got %q", cfg.GetDimConfig("int").IndexFieldName)
	}
	if !cfg.GetDimConfig("int").MultiValued {
		t.Error("int: expected multi-valued")
	}
	if cfg.GetDimConfig("float").IndexFieldName != "$facets.float" {
		t.Errorf("float index field: want $facets.float, got %q", cfg.GetDimConfig("float").IndexFieldName)
	}
}

// -- Integration stubs -------------------------------------------------------

func TestTaxonomyFacetAssociations_IntSum(t *testing.T) {
	t.Skip("requires IndexWriter + FacetsCollector + TaxonomyFacetIntAssociations pipeline")
}

func TestTaxonomyFacetAssociations_IntMax(t *testing.T) {
	t.Skip("requires IndexWriter + FacetsCollector + TaxonomyFacetIntAssociations pipeline")
}

func TestTaxonomyFacetAssociations_FloatSum(t *testing.T) {
	t.Skip("requires IndexWriter + FacetsCollector + TaxonomyFacetFloatAssociations pipeline")
}

func TestTaxonomyFacetAssociations_FloatMax(t *testing.T) {
	t.Skip("requires IndexWriter + FacetsCollector + TaxonomyFacetFloatAssociations pipeline")
}
