// Copyright 2026 The Gocene Authors. Licensed under the Apache License, Version 2.0.
// Derived from Apache Lucene 10.4.0 (org.apache.lucene.util.hnsw.TestHnswByteVectorGraph).

package hnsw

// GOC-4302 — Sprint 56 stub (Sprint 55 option (c) policy).
//
// Java reference:
//   lucene/core/src/test/org/apache/lucene/util/hnsw/TestHnswByteVectorGraph.java
//   (concrete subclass of HnswGraphTestCase<byte[]>, 135 lines).
//
// Per [[graph_test_case_test.go]] (GOC-4300), the JUnit "abstract test class +
// concrete subclass" pattern maps to per-encoding test files sharing
// unexported helpers in this package. The shared helpers (MockByteVectorValues,
// CircularByteVectorValues, createRandomByteVectors, randomVector8) are not
// yet implemented and depend on follow-up sprints porting the indexing-stack
// types referenced by Lucene's base.
//
// Java surface to port once the helpers land:
//   - getVectorEncoding() == VectorEncoding.BYTE
//   - knnQuery(field, vector, k) -> KnnByteVectorQuery    (not yet ported)
//   - randomVector(dim) -> randomVector8(random(), dim)
//   - getTargetVector() == {1, 0}
//   - vectorValues(size, dim) via MockByteVectorValues.fromValues(
//       createRandomByteVectors(size, dimension, random()))
//   - vectorValues(float[][]) with fitsInByte(v) scaling: identity when all
//     values already fit in [-128, 127] with no fractional part, else *127
//   - vectorValues(size, dim, pregenerated, offset) splicing MockByteVectorValues
//     into a fresh byte[size][] at pregeneratedOffset
//   - vectorValues(LeafReader, fieldName) using ByteVectorValues + ArrayUtil
//     .copyOfSubArray (LeafReader pending)
//   - knnVectorField(name, vector, sim) -> KnnByteVectorField
//   - circularVectorValues(nDoc) -> CircularByteVectorValues
//   - setup(): similarityFunction = randomFrom(VectorSimilarityFunction.values())
//
// Tracking:
//   - GOC-4302 (this stub), sibling of [[hnsw_float_vector_graph_test.go]] (GOC-4301)
//   - Blocked on: shared HnswGraphTestCase helpers (GOC-4300) and
//     KnnByteVectorQuery / LeafReader-based vector-values plumbing.
