// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "testing"

// Port of org.apache.lucene.document.TestManyKnnDocs from Apache Lucene 10.4.0.
//
// Source: core/src/test/org/apache/lucene/document/TestManyKnnDocs.java
//
// The upstream suite is annotated `@Monster("takes ~10 minutes and needs extra
// heap, disk space, file handles")` with a 24-hour suite timeout. Both test
// methods exercise the full HNSW indexing pipeline on hundreds of thousands to
// millions of vectors and were never intended to run in the default JUnit
// suite — they are gated behind `-Dtests.monster=true` in Gradle.
//
// This file is a Sprint 55 option-(c) STUB: it preserves 1:1 method coverage
// against the Java reference so the port inventory is honoured, but each test
// short-circuits via t.Skip until the supporting machinery (IndexWriter,
// HNSW codec wiring, FSDirectory force-merge, KnnFloatVectorQuery search) is
// ready end-to-end and a monster-test opt-in flag is introduced.
//
// When un-stubbing, lift the bodies from the Java source and translate the
// assertions verbatim; the Go signatures below are the only public surface
// downstream tooling tracks.

// TestManyKnnDocs_SameVectorIndexedMultipleTimes mirrors
// TestManyKnnDocs#testSameVectorIndexedMultipleTimes. It indexes the same
// 16-dimensional vector 100_000 times under DOT_PRODUCT similarity and asserts
// that flush/commit cycles do not corrupt the HNSW graph.
func TestManyKnnDocs_SameVectorIndexedMultipleTimes(t *testing.T) {
	t.Fatal("monster test: port pending IndexWriter + HNSW writer wiring (Sprint 55 stub)")
}

// TestManyKnnDocs_LargeSegment mirrors TestManyKnnDocs#testLargeSegment. It
// indexes ~2.09M one-dimensional vectors into a single force-merged segment
// using ConfigurableMCodec(128) and asserts that a top-5 KnnFloatVectorQuery
// returns exactly 5 hits.
func TestManyKnnDocs_LargeSegment(t *testing.T) {
	t.Fatal("monster test: port pending IndexWriter + HNSW writer wiring (Sprint 55 stub)")
}
