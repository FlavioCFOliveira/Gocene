// Copyright 2026 The Gocene Authors. Licensed under the Apache License, Version 2.0.
// Derived from Apache Lucene 10.4.0 (org.apache.lucene.util.hnsw.HnswGraphTestCase).

package hnsw

// GOC-4300 — Sprint 56 stub (Sprint 55 option (c) policy).
//
// Java reference: lucene/core/src/test/org/apache/lucene/util/hnsw/HnswGraphTestCase.java
// (1505 lines, abstract base parameterised over T ∈ {float32[], byte[]}).
//
// Lucene's HnswGraphTestCase is a JUnit abstract base providing a shared test
// fixture for both HnswFloatVectorGraph and HnswByteVectorGraph subclasses. It
// exposes generic abstract hooks (vectorValues, knnVectorField, knnQuery,
// randomVector, getTargetVector, circularVectorValues, getVectorEncoding) and
// concrete reusable scenarios (testRandomReadWriteAndMerge, testSearch,
// testSortedAndUnsortedIndicesReturnSameResults, testAknnDiverse,
// testRamUsageEstimate, and many more).
//
// In Go, this base cannot be ported verbatim because:
//   1. Go has no test-class inheritance; JUnit "abstract test class +
//      concrete subclass" maps to either (a) a TestMain-style helper package,
//      (b) a generic helper function set parameterised on T, or
//      (c) per-encoding test files that share unexported helpers in this
//      package.
//   2. The base depends on indexing-stack types (IndexWriter, DirectoryReader,
//      CodecReader, KnnVectorsFormat plumbing) that are still being ported in
//      later sprints.
//
// Sprint 55 selected option (c). The eventual ports of
// hnsw_float_vector_graph_test.go and hnsw_byte_vector_graph_test.go will
// share helpers exported by this file; the helpers themselves are deferred
// until those sibling tests land, so this file currently only documents the
// intended surface.
//
// Deferred surface (informational; not yet implemented):
//   - buildScorerSupplier(vectors) RandomVectorScorerSupplier
//   - buildScorer(vectors, query) RandomVectorScorer
//   - randomVectorValues(size, dim, encoding)
//   - circularVectorValues(nDoc, encoding)
//   - assertGraphEqual(expected, actual *OnHeapHnswGraph)
//   - assertGraphContainsGraph(big, small *OnHeapHnswGraph)
//
// Tracking:
//   - GOC-4300 (this stub)
//   - Follow-ups will be filed when the indexing-stack dependencies land.
