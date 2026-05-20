// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// This file ports org.apache.lucene.index.Test2BBinaryDocValues
// (Apache Lucene 10.4.0).
//
// The Java suite is annotated @Monster with an effectively unlimited
// TimeoutSuite: each method indexes IndexWriter.MAX_DOCS (~2 billion)
// documents carrying a BinaryDocValuesField, force-merges to a single
// segment, and verifies every value via DirectoryReader. A full run takes
// roughly six hours with a 5 GB heap, so both methods are skipped by default.
//
// Status: stubbed (skipped). Beyond the Monster runtime, Gocene currently
// lacks the IndexWriter / BinaryDocValues iteration primitives this suite
// exercises, so the bodies remain unimplemented (Sprint 55 option c).
// The test methods are mapped 1:1 with the Java source to preserve the
// porting surface for a future sprint.

// Test2BBinaryDocValuesFixedBinary ports Test2BBinaryDocValues.testFixedBinary.
//
// It indexes IndexWriter.MAX_DOCS documents, each with a fixed 4-byte binary
// doc-values field encoding the document ordinal, force-merges to one segment,
// and asserts that every BinaryDocValues value round-trips.
func Test2BBinaryDocValuesFixedBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("monster test: skipped in -short mode")
	}
	t.Skip("monster test: indexes ~2B docs, ~6h runtime and multiple GB of heap; IndexWriter/BinaryDocValues iteration infrastructure not yet available")
}

// Test2BBinaryDocValuesVariableBinary ports Test2BBinaryDocValues.testVariableBinary.
//
// It indexes IndexWriter.MAX_DOCS documents, each with a variable-length binary
// doc-values field holding a VInt-encoded ordinal, force-merges to one segment,
// and asserts that every BinaryDocValues value decodes back to the expected VInt.
func Test2BBinaryDocValuesVariableBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("monster test: skipped in -short mode")
	}
	t.Skip("monster test: indexes ~2B docs, ~6h runtime and multiple GB of heap; IndexWriter/BinaryDocValues iteration infrastructure not yet available")
}
