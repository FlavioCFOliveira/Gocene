// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// Port of
// lucene/core/src/test/org/apache/lucene/codecs/lucene94/TestLucene94FieldInfosFormat.java
// (Apache Lucene 10.5.0). Gocene's Lucene94FieldInfosFormat lives in package
// codecs, so the package-private SIMILARITY_FUNCTIONS list is reached from an
// internal test. The class extends
// org.apache.lucene.tests.index.BaseFieldInfoFormatTestCase (not ported); its
// inherited test methods are represented by one failing test naming it.

import (
	"slices"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestLucene94FieldInfosFormat_BaseFieldInfoFormatTestCase(t *testing.T) {
	t.Fatal("requires org.apache.lucene.tests.index.BaseFieldInfoFormatTestCase (not ported)")
}

// Ensures that all expected vector similarity functions are translatable
// in the format.
func TestLucene94FieldInfosFormat_testVectorSimilarityFuncs(t *testing.T) {
	// This does not necessarily have to be all similarity functions, but
	// differences should be considered carefully.
	// VectorSimilarityFunction.values(), in declaration order.
	expectedValues := []index.VectorSimilarityFunction{
		index.VectorSimilarityFunctionEuclidean,
		index.VectorSimilarityFunctionDotProduct,
		index.VectorSimilarityFunctionCosine,
		index.VectorSimilarityFunctionMaximumInnerProduct,
	}

	if !slices.Equal(lucene94SimilarityFunctions[:], expectedValues) {
		t.Fatalf("SIMILARITY_FUNCTIONS = %v, want %v", lucene94SimilarityFunctions, expectedValues)
	}
}
