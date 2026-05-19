// Copyright 2026 The Gocene Authors. Licensed under the Apache License, Version 2.0.
// Derived from Apache Lucene 10.4.0 (org.apache.lucene.util.hnsw.TestHnswFloatVectorGraph).

package hnsw

// GOC-4301 — Sprint 56 stub (Sprint 55 option (c) policy).
//
// Java reference:
//   lucene/core/src/test/org/apache/lucene/util/hnsw/TestHnswFloatVectorGraph.java
//   (concrete subclass of HnswGraphTestCase<float[]>, 148 lines).
//
// Per [[graph_test_case_test.go]] (GOC-4300), the JUnit "abstract test class +
// concrete subclass" pattern maps to per-encoding test files sharing
// unexported helpers in this package. The shared helpers (buildScorerSupplier,
// buildScorer, circularVectorValues, MockVectorValues, createRandomFloatVectors)
// are not yet implemented and depend on follow-up sprints porting the
// indexing-stack types referenced by Lucene's base.
//
// Java surface to port once the helpers land:
//   - getVectorEncoding() == VectorEncoding.FLOAT32
//   - knnQuery(field, vector, k) -> KnnFloatVectorQuery   (not yet ported)
//   - randomVector(dim), getTargetVector() == {1, 0}
//   - vectorValues(size, dim), vectorValues(float[][]),
//     vectorValues(LeafReader, fieldName),
//     vectorValues(size, dim, pregenerated, offset)        (LeafReader pending)
//   - knnVectorField(name, vector, sim) -> KnnFloatVectorField
//   - circularVectorValues(nDoc) -> CircularFloatVectorValues
//   - testSearchWithSkewedAcceptOrds (concrete, 1000 docs, EUCLIDEAN,
//     FixedBitSet acceptOrds [500, 1000), asserts 10 hits & sum < 5100)
//
// Tracking:
//   - GOC-4301 (this stub)
//   - Blocked on: shared HnswGraphTestCase helpers (GOC-4300) and
//     KnnFloatVectorQuery / LeafReader-based vector-values plumbing.
