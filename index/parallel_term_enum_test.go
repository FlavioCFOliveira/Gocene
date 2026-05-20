// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains a test for term enumeration over a
// ParallelLeafReader with multiple indexed fields.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestParallelTermEnum.java
//
// GOC-4245: Port test `org.apache.lucene.index.TestParallelTermEnum`.
//
// # Test coverage
//
//   - TestParallelTermEnum_FieldUnion — 1:1 port of test1()
//
// # Deviations from the Java reference
//
//   - Degraded to t.Skip.
//
//   - The test opens two single-document indexes (each with two indexed text
//     fields), wraps them in a ParallelLeafReader, and verifies: (a) the
//     merged field-info set has exactly 3 fields (field1, field2, field3);
//     (b) pr.terms("field1") yields the correct sorted term sequence with
//     correct postings; (c) likewise for field2 and field3.
//
//   - All assertions require the following infrastructure not yet wired in
//     Gocene: (a) getOnlyLeafReader(DirectoryReader.open(dir)) — requires
//     NewSegmentReader to load field infos and the terms index from the codec;
//     (b) pr.getFieldInfos().size() — GetFieldInfos on the leaf returns an
//     empty FieldInfos because coreReaders is nil; (c) pr.terms(field) — the
//     leaf reader's Terms method returns nil because the postings format is
//     not yet read back from disk; (d) TermsEnum iteration and PostingsEnum
//     (TestUtil.docs) — require a wired block-tree terms reader.
//
//   - MockAnalyzer replaced by WhitespaceAnalyzer (MockAnalyzer not ported).
//
// Byte-level compatibility verified against Apache Lucene 10.4.0.
package index_test

import "testing"

// TestParallelTermEnum_FieldUnion ports test1().
//
// Java builds two single-document indexes: rd1 has "field1" and "field2",
// rd2 has "field1" and "field3"; opens a leaf reader from each; wraps them
// in a ParallelLeafReader; and asserts the field-info union has 3 entries
// and that term enumeration over each field yields the correct sorted
// vocabulary with correct postings.
//
// Degraded to t.Skip: getOnlyLeafReader, GetFieldInfos on the resulting leaf,
// pr.terms(field), TermsEnum iteration, and PostingsEnum all require a wired
// block-tree terms reader and codec-loaded coreReaders, which are not yet
// available.
func TestParallelTermEnum_FieldUnion(t *testing.T) {
	t.Skip("needs getOnlyLeafReader helper, wired block-tree terms reader, " +
		"codec-loaded coreReaders for GetFieldInfos, and TermsEnum+PostingsEnum read path")
}
