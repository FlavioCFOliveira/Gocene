// Copyright 2026 The Gocene Authors. Licensed under the Apache License, Version 2.0.
// Derived from Apache Lucene 10.4.0 (org.apache.lucene.util.hnsw.MockVectorValues).

package hnsw

// GOC-4303 — Sprint 56 stub (Sprint 55 option (c) policy).
//
// Java reference:
//   lucene/core/src/test/org/apache/lucene/util/hnsw/MockVectorValues.java
//   (package-private test fixture, 90 lines, extends FloatVectorValues).
//
// MockVectorValues is the in-memory float-vector test fixture that
// HnswGraphTestCase and its concrete subclasses (TestHnswFloatVectorGraph,
// TestHnswByteVectorGraph) use to feed deterministic float[][] data into
// graph builders and searchers, with random scratch-buffer aliasing to
// surface caller-side vector-reuse bugs.
//
// In Go, the verbatim port is blocked on surface that is not yet present
// in Gocene:
//   1. index.FloatVectorValues currently exposes Get(docID) only; it lacks
//      the Lucene KnnVectorValues base API used by MockVectorValues:
//        - VectorValue(ord) []float32
//        - Copy() FloatVectorValues
//        - Iterator() DocIndexIterator (createDenseIterator equivalent)
//        - OrdToDoc(ord) int
//      See [[hnsw_float_vector_graph_test.go]] (GOC-4301) for the parallel
//      blocker on the consumer side.
//   2. LuceneTestCase.random() — the JUnit random source used to flip
//      between returning values[ord] directly and aliasing through scratch
//      — has no Gocene equivalent in this package yet.
//   3. ArrayUtil.copyArray(float[][]) — the deep-copy helper used by
//      copy() — is not exposed here; either util.ArrayUtil.CopyArray must
//      gain the [][]float32 overload or callers must inline a manual copy.
//
// Deferred surface (informational; not yet implemented):
//   - mockVectorValues struct { dimension int; values, denseValues [][]float32;
//     numVectors int; scratch []float32 }
//   - newMockVectorValuesFromValues(values [][]float32) *mockVectorValues
//   - (m *mockVectorValues) Size() int
//   - (m *mockVectorValues) Dimension() int
//   - (m *mockVectorValues) Copy() *mockVectorValues
//   - (m *mockVectorValues) VectorValue(ord int) []float32  (with random
//     scratch-buffer aliasing for alias-bug detection)
//   - (m *mockVectorValues) Iterator() index.DocIndexIterator  (dense)
//
// Tracking:
//   - GOC-4303 (this stub)
//   - Blocked on: index.FloatVectorValues / KnnVectorValues surface
//     parity (GOC-4301 and the shared HnswGraphTestCase helpers from
//     [[graph_test_case_test.go]] / GOC-4300).
