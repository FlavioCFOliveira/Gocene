// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestCheckIndex.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// baseTestCheckIndexMissing names the test-framework superclass whose
// inherited test bodies the first four tests delegate to.
const baseTestCheckIndexMissing = "org.apache.lucene.tests.index.BaseTestCheckIndex is not ported"

// checkIndexTest renders the per-test setUp/tearDown of TestCheckIndex: a
// fresh directory closed after the test.
func checkIndexTest(t *testing.T) store.Directory {
	t.Helper()
	directory := newDirectory()
	t.Cleanup(func() {
		if err := directory.Close(); err != nil {
			t.Errorf("tearDown: close directory: %v", err)
		}
	})
	return directory
}

func TestCheckIndexDeletedDocs(t *testing.T) {
	checkIndexTest(t)
	t.Fatal(baseTestCheckIndexMissing + " (testDeletedDocs(Directory))")
}

func TestCheckIndexChecksumsOnly(t *testing.T) {
	checkIndexTest(t)
	t.Fatal(baseTestCheckIndexMissing + " (testChecksumsOnly(Directory))")
}

func TestCheckIndexChecksumsOnlyVerbose(t *testing.T) {
	checkIndexTest(t)
	t.Fatal(baseTestCheckIndexMissing + " (testChecksumsOnlyVerbose(Directory))")
}

func TestCheckIndexObtainsLock(t *testing.T) {
	checkIndexTest(t)
	t.Fatal(baseTestCheckIndexMissing + " (testObtainsLock(Directory))")
}

func TestCheckIndexCheckIndexAllValid(t *testing.T) {
	checkIndexTest(t)
	dir := newDirectory()
	defer mustClose(t, dir)
	config := newIndexWriterConfig()
	config.SetSoftDeletesField("soft_delete")
	t.Fatal("org.apache.lucene.index.SoftDeletesRetentionMergePolicy(String, Supplier<Query>, MergePolicy) is not ported " +
		"(the Go constructor takes no retention query); org.apache.lucene.tests.analysis.CannedTokenStream/Token " +
		"and TestUtil.checkIndex(Directory, int, boolean, boolean, ByteArrayOutputStream) are not ported")
}

func TestCheckIndexInvalidThreadCountArgument(t *testing.T) {
	checkIndexTest(t)
	t.Fatal("org.apache.lucene.index.CheckIndex#parseOptions(String[]) is not ported")
}

// checkIndexRandomVector renders the private randomVector(int).
func checkIndexRandomVector(dim int) []float32 {
	v := make([]float32, dim)
	for i := 0; i < dim; i++ {
		v[i] = rand.Float32()
	}
	util.L2Normalize(v)
	return v
}

// deleteNothingIndexDeletionPolicy is the private
// DeleteNothingIndexDeletionPolicy: it never deletes any commit points. Do not
// use in production!!
type deleteNothingIndexDeletionPolicy struct{}

var deleteNothingIndexDeletionPolicyInstance index.IndexDeletionPolicy = deleteNothingIndexDeletionPolicy{}

func (deleteNothingIndexDeletionPolicy) OnInit([]index.Commit) error   { return nil }
func (deleteNothingIndexDeletionPolicy) OnCommit([]index.Commit) error { return nil }

// Clone carries IndexDeletionPolicy's Go-only Clone member; the policy is
// stateless.
func (p deleteNothingIndexDeletionPolicy) Clone() index.IndexDeletionPolicy { return p }

// https://github.com/apache/lucene/issues/7820 -- when the most recent commit
// point in the index is OK, but older commit points are broken, CheckIndex
// fails to detect and correct that, while opening an IndexWriter on the index
// will fail since IndexWriter loads all commit points on init
func TestCheckIndexPriorBrokenCommitPoint(t *testing.T) {
	checkIndexTest(t)
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal("org.apache.lucene.tests.store.MockDirectoryWrapper#setCheckIndexOnClose(boolean) is not ported")
}
