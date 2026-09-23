// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestMultiIndexMergeScheduler.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// combinedMergeSchedulerSyncOverrideMissing names what the two close tests
// need: an anonymous MultiIndexMergeScheduler.CombinedMergeScheduler subclass
// overriding sync(Directory), handed to the MultiIndexMergeScheduler
// constructor. Gocene's MultiIndexMergeScheduler holds the concrete
// *CombinedMergeScheduler, so an override is never dispatched to.
const combinedMergeSchedulerSyncOverrideMissing = "overriding org.apache.lucene.index.MultiIndexMergeScheduler.CombinedMergeScheduler#sync(Directory) " +
	"is not ported: MultiIndexMergeScheduler(Directory, CombinedMergeScheduler) takes the concrete *CombinedMergeScheduler"

func TestMultiIndexMergeSchedulerCloseSingle(t *testing.T) {
	directory := newDirectory()
	defer mustClose(t, directory)
	t.Fatal(combinedMergeSchedulerSyncOverrideMissing)
}

func TestMultiIndexMergeSchedulerCloseMultiple(t *testing.T) {
	// Write to many indexes and do merges on them.
	const directoryCount = 10
	var directoriesToBeClosed []interface{ Close() error }
	for i := 0; i < directoryCount; i++ {
		directoriesToBeClosed = append(directoriesToBeClosed, newDirectory())
	}
	defer mustClose(t, directoriesToBeClosed...)
	t.Fatal(combinedMergeSchedulerSyncOverrideMissing)
}

func assertSingletonPresent(t *testing.T, present bool, step string) {
	t.Helper()
	if got := index.PeekCombinedSingleton() != nil; got != present {
		t.Fatalf("CombinedMergeScheduler.peekSingleton() != null at %s: expected %v, got %v", step, present, got)
	}
}

func TestMultiIndexMergeSchedulerReferenceCounting(t *testing.T) {
	assertSingletonPresent(t, false, "0")

	directory1 := newDirectory()
	mims1 := index.NewMultiIndexMergeScheduler(directory1)
	assertSingletonPresent(t, true, "1")

	directory2 := newDirectory()
	mims2 := index.NewMultiIndexMergeScheduler(directory2)
	assertSingletonPresent(t, true, "2")

	mustClose(t, mims1, directory1)
	assertSingletonPresent(t, true, "1")

	mustClose(t, mims2, directory2)
	assertSingletonPresent(t, false, "0")

	directory3 := newDirectory()
	mims3 := index.NewMultiIndexMergeScheduler(directory3)
	assertSingletonPresent(t, true, "1")

	mustClose(t, mims3, directory3)
	assertSingletonPresent(t, false, "0")
}
