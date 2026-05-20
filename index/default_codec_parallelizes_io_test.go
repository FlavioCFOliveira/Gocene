// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests verifying that the default codec
// parallelizes I/O by issuing prefetch requests before blocking reads.
//
// Ported from Apache Lucene's
// org.apache.lucene.index.TestDefaultCodecParallelizesIO
// Source: lucene/core/src/test/org/apache/lucene/index/TestDefaultCodecParallelizesIO.java
//
// GOC-4149 (Sprint 55, option c): every test in the upstream suite asserts
// that a batch of prepareSeekExact / storedFields.prefetch calls triggers
// fewer serial I/O operations than the same batch issued one-by-one. That
// assertion is built end-to-end on infrastructure Gocene does not yet have:
//
//   - SerialIOCountingDirectory (org.apache.lucene.tests.store): the Directory
//     wrapper whose count() drives every assertion. No equivalent exists in
//     store/.
//   - LineFileDocs: the randomized document source feeding the 10k-document
//     index built in @BeforeClass. Only referenced in comments / a skipped
//     test; not implemented.
//   - getOnlyLeafReader (LuceneTestCase): the single-leaf accessor used to
//     reach Terms / StoredFields. Not present.
//   - A DirectoryReader.open round-trip over an IndexWriter+forceMerge(1)
//     index reaching Terms and StoredFields off a leaf (blocked by the
//     SegmentReader coreReaders gap).
//
// Without SerialIOCountingDirectory the suite cannot exercise its single
// reason to exist, so this file is a structural stub: both upstream test
// methods are present so the suite shape matches upstream, and each calls
// t.Skip with the precise missing dependency.
package index_test

import "testing"

const skipDefaultCodecParallelizesIO = "GOC-4149: requires SerialIOCountingDirectory I/O-counting wrapper, LineFileDocs and a DirectoryReader leaf round-trip reaching Terms/StoredFields; none available in Gocene"

// TestDefaultCodecParallelizesIO_TermsSeekExact ports testTermsSeekExact.
// The upstream test prepares several prepareSeekExact suppliers, resolves
// them, and asserts the directory's serial I/O count grew by fewer than the
// number of suppliers, proving the term lookups were prefetched in parallel.
func TestDefaultCodecParallelizesIO_TermsSeekExact(t *testing.T) {
	t.Skip(skipDefaultCodecParallelizesIO)
}

// TestDefaultCodecParallelizesIO_StoredFields ports testStoredFields.
// The upstream test prefetches twenty random documents from StoredFields,
// retrieves them, and asserts the directory's serial I/O count grew by fewer
// than twenty, proving the stored-field reads were prefetched in parallel.
func TestDefaultCodecParallelizesIO_StoredFields(t *testing.T) {
	t.Skip(skipDefaultCodecParallelizesIO)
}
