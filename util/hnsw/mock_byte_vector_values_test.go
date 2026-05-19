// Copyright 2026 The Gocene Authors. Licensed under the Apache License, Version 2.0.
// Derived from Apache Lucene 10.4.0 (org.apache.lucene.util.hnsw.MockByteVectorValues).

package hnsw

// GOC-4304 — Sprint 56 stub (Sprint 55 option (c) policy).
//
// Java reference:
//   lucene/core/src/test/org/apache/lucene/util/hnsw/MockByteVectorValues.java
//   (package-private test fixture, 96 lines, extends ByteVectorValues).
//
// MockByteVectorValues is the byte-vector sibling of [[mock_vector_values_test.go]]
// (GOC-4303). It feeds deterministic byte[][] data into HnswGraphTestCase via
// TestHnswByteVectorGraph (see [[hnsw_byte_vector_graph_test.go]] / GOC-4302)
// and mirrors the float fixture's random scratch-buffer aliasing — same
// alias-bug detection pattern, byte payload — plus the BytesRef-backed
// binaryValue field that codecs consume.
//
// In Go, the verbatim port is blocked on surface that is not yet present
// in Gocene:
//   1. index.ByteVectorValues currently lacks the Lucene KnnVectorValues base
//      API used by MockByteVectorValues:
//        - VectorValue(ord) []byte
//        - Copy() ByteVectorValues
//        - Iterator() DocIndexIterator (createDenseIterator equivalent)
//        - OrdToDoc(ord) int
//      Same blocker shape as the float side; see GOC-4301/4303 stubs.
//   2. LuceneTestCase.random() — the JUnit random source used to flip
//      between returning values[ord] directly and aliasing through scratch
//      — has no Gocene equivalent in this package yet.
//   3. ArrayUtil.copyArray(byte[][]) — the deep-copy helper used by copy()
//      — is not exposed here; either util.ArrayUtil.CopyArray must gain the
//      [][]byte overload or callers must inline a manual copy.
//   4. util.BytesRef vs raw []byte: Lucene exposes a reusable BytesRef whose
//      length is reset to the vector dimension; the Gocene equivalent and
//      its lifecycle inside the byte VectorValue path still needs design.
//
// Deferred surface (informational; not yet implemented):
//   - mockByteVectorValues struct { dimension int; values, denseValues [][]byte;
//     numVectors int; binaryValue util.BytesRef; scratch []byte }
//   - newMockByteVectorValuesFromValues(values [][]byte) *mockByteVectorValues
//   - (m *mockByteVectorValues) Size() int
//   - (m *mockByteVectorValues) Dimension() int
//   - (m *mockByteVectorValues) Copy() *mockByteVectorValues
//   - (m *mockByteVectorValues) VectorValue(ord int) []byte  (with random
//     scratch-buffer aliasing for alias-bug detection)
//   - (m *mockByteVectorValues) Iterator() index.DocIndexIterator  (dense)
//
// Tracking:
//   - GOC-4304 (this stub)
//   - Sibling: GOC-4303 ([[mock_vector_values_test.go]]) — float counterpart.
//   - Blocked on: index.ByteVectorValues / KnnVectorValues surface parity
//     (GOC-4302) and the shared HnswGraphTestCase helpers from
//     [[graph_test_case_test.go]] / GOC-4300.
