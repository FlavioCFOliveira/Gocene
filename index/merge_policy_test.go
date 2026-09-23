// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestMergePolicy.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// mergeSpecificationAwaitMissing names the production member the three
// wait/timeout tests call.
const mergeSpecificationAwaitMissing = "org.apache.lucene.index.MergePolicy.MergeSpecification#await(long, TimeUnit) is not ported"

// assertNotCompleted renders assertFalse(m.hasCompletedSuccessfully().isPresent()).
func assertNotCompleted(t *testing.T, m *index.OneMerge) {
	t.Helper()
	if m.HasFinished() {
		t.Fatal("hasCompletedSuccessfully().isPresent() is true")
	}
}

func TestMergePolicyWaitForOneMerge(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	ms := createRandomMergeSpecification(t, dir, 1+rand.Intn(10))
	for _, m := range ms.Merges {
		assertNotCompleted(t, m)
	}
	t.Fatal(mergeSpecificationAwaitMissing)
}

func TestMergePolicyTimeout(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	ms := createRandomMergeSpecification(t, dir, 3)
	for _, m := range ms.Merges {
		assertNotCompleted(t, m)
	}
	t.Fatal(mergeSpecificationAwaitMissing)
}

func TestMergePolicyTimeoutLargeNumberOfMerges(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	ms := createRandomMergeSpecification(t, dir, 10000)
	for _, m := range ms.Merges {
		assertNotCompleted(t, m)
	}
	t.Fatal(mergeSpecificationAwaitMissing)
}

func TestMergePolicyFinishTwice(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	spec := createRandomMergeSpecification(t, dir, 1)
	oneMerge := spec.Merges[0]
	if err := oneMerge.Close(true, false, func(*index.MergeReader) error { return nil }); err != nil {
		t.Fatalf("close(true, false): %v", err)
	}
	// expectThrows(IllegalStateException.class, () -> oneMerge.close(false, false, mr -> {}))
	if err := oneMerge.Close(false, false, func(*index.MergeReader) error { return nil }); err == nil {
		t.Fatal("expected IllegalStateException from the second close")
	}
}

func TestMergePolicyTotalMaxDoc(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	spec := createRandomMergeSpecification(t, dir, 1)
	docs := 0
	oneMerge := spec.Merges[0]
	for _, info := range oneMerge.Segments {
		docs += info.SegmentInfo().MaxDoc()
	}
	if docs != oneMerge.TotalMaxDoc {
		t.Fatalf("totalMaxDoc: expected %d, got %d", docs, oneMerge.TotalMaxDoc)
	}
}

// createRandomMergeSpecification renders the private static helper. The
// twelve-argument SegmentInfo constructor
// new SegmentInfo(dir, Version.LATEST, Version.LATEST, name, maxDoc,
// isCompoundFile, false, null, emptyMap, id, emptyMap, null) is rendered
// through the SegmentInfo setters.
func createRandomMergeSpecification(t *testing.T, dir store.Directory, numMerges int) *index.MergeSpecification {
	t.Helper()
	r := rand.New(rand.NewSource(rand.Int63()))
	ms := index.NewMergeSpecification()
	for ii := 0; ii < numMerges; ii++ {
		si := index.NewSegmentInfo(util.RandomSimpleString(r, 0, 10), r.Intn(1000), dir)
		si.SetVersion(util.Latest.String())
		si.SetMinVersion(util.Latest.String())
		si.SetUseCompoundFile(r.Intn(2) == 0)
		si.SetHasBlocks(false)
		si.SetDiagnostics(map[string]string{})
		if err := si.SetID([]byte(util.RandomSimpleString(r, util.IDLength, util.IDLength))); err != nil {
			t.Fatalf("setID: %v", err)
		}
		si.SetAttributes(map[string]string{})
		segments := []*index.SegmentCommitInfo{index.NewSegmentCommitInfo(si, 0, 0, 0, 0, 0, util.RandomId())}
		ms.Add(index.NewOneMerge(segments))
	}
	return ms
}
