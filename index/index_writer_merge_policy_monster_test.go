// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test port of
// lucene/core/src/test/org/apache/lucene/index/TestIndexWriterMergePolicy.java
// (Apache Lucene 10.5.0), built only with the gocene_monsters tag as Lucene
// runs it only when nightly tests are enabled.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// Test the case where both mergeFactor and maxBufferedDocs change
func TestIndexWriterMergePolicyMaxBufferedDocsChange(t *testing.T) {
	dir := newDirectory()

	writer := newMergePolicyTestWriter(t, dir, 101, newMockMergePolicy(), index.NewSerialMergeScheduler())

	// leftmost* segment has 1 doc
	// rightmost* segment has 100 docs
	for i := 1; i <= 100; i++ {
		for j := 0; j < i; j++ {
			indexWriterMergePolicyAddDoc(t, writer)
			indexWriterMergePolicyCheckInvariants(t, writer)
		}
		mustClose(t, writer)

		conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		conf.SetOpenMode(index.Append)
		conf.SetMaxBufferedDocs(101)
		conf.SetMergePolicy(newMockMergePolicy())
		conf.SetMergeScheduler(index.NewSerialMergeScheduler())
		writer = mustNewIndexWriter(t, dir, conf)
	}

	mustClose(t, writer)
	ldmp := newMockMergePolicy()
	ldmp.setMergeFactor(10)
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetOpenMode(index.Append)
	conf.SetMaxBufferedDocs(10)
	conf.SetMergePolicy(ldmp)
	conf.SetMergeScheduler(index.NewSerialMergeScheduler())
	writer = mustNewIndexWriter(t, dir, conf)

	// merge policy only fixes segments on levels where merges
	// have been triggered, so check invariants after all adds
	for i := 0; i < 100; i++ {
		indexWriterMergePolicyAddDoc(t, writer)
	}
	indexWriterMergePolicyCheckInvariants(t, writer)

	for i := 100; i < 1000; i++ {
		indexWriterMergePolicyAddDoc(t, writer)
	}
	mustCommit(t, writer)
	t.Fatal(indexWriterWaitForMergesMissing)
}
