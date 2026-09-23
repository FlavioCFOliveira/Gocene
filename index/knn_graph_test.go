// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestKnnGraph.java
// (Apache Lucene 10.5.0): tests indexing of a knn-graph.
//
// Every test runs the @Before setup(), which builds its codecs with
// TestUtil.alwaysKnnVectorsFormat(KnnVectorsFormat); that helper returns an
// anonymous org.apache.lucene.tests.codecs.asserting.AssertingCodec subclass,
// and AssertingCodec is not ported, so each test fails there naming it.

package index_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// knnGraphM renders the static M, reset by @After cleanup().
var knnGraphM = hnsw.DefaultMaxConn

// knnGraphVectorSimilarityFunctions renders VectorSimilarityFunction.values().
var knnGraphVectorSimilarityFunctions = []index.VectorSimilarityFunction{
	index.VectorSimilarityFunctionEuclidean,
	index.VectorSimilarityFunctionDotProduct,
	index.VectorSimilarityFunctionCosine,
	index.VectorSimilarityFunctionMaximumInnerProduct,
}

// knnGraphVectorEncodings renders VectorEncoding.values().
var knnGraphVectorEncodings = []index.VectorEncoding{index.VectorEncodingByte, index.VectorEncodingFloat32}

// knnGraphTest renders the per-test instance state of TestKnnGraph.
type knnGraphTest struct {
	vectorEncoding     index.VectorEncoding
	similarityFunction index.VectorSimilarityFunction
}

// knnGraphSetup renders @Before setup() and registers @After cleanup().
func knnGraphSetup(t *testing.T) *knnGraphTest {
	t.Helper()
	t.Cleanup(func() { knnGraphM = hnsw.DefaultMaxConn })
	s := &knnGraphTest{}
	hnsw.RandSeed = rand.Int63()
	if rand.Intn(2) == 0 {
		knnGraphM = rand.Intn(256) + 3
	}

	similarity := rand.Intn(len(knnGraphVectorSimilarityFunctions)-1) + 1
	s.similarityFunction = knnGraphVectorSimilarityFunctions[similarity]
	s.vectorEncoding = knnGraphVectorEncodings[rand.Intn(len(knnGraphVectorEncodings))]
	t.Fatal("org.apache.lucene.tests.codecs.asserting.AssertingCodec " +
		"(returned by TestUtil.alwaysKnnVectorsFormat(KnnVectorsFormat)) is not ported")
	return s
}

// Basic test of creating documents in a graph
func TestKnnGraphBasic(t *testing.T) { knnGraphSetup(t) }

func TestKnnGraphSingleDocument(t *testing.T) { knnGraphSetup(t) }

// Verify that the graph properties are preserved when merging
func TestKnnGraphMerge(t *testing.T) { knnGraphSetup(t) }

// Test writing and reading of multiple vector fields
func TestKnnGraphMultipleVectorFields(t *testing.T) { knnGraphSetup(t) }

// Verify that searching does something reasonable
func TestKnnGraphSearch(t *testing.T) { knnGraphSetup(t) }

func TestKnnGraphMultiThreadedSearch(t *testing.T) { knnGraphSetup(t) }
