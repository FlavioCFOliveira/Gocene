// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriterCommit.java
// (Apache Lucene 10.5.0). The @Nightly testCommitOnCloseDiskUsage and
// testCommitThreadSafety live in index_writer_commit_monster_test.go.

package index_test

import (
	"errors"
	"maps"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func addDocs(t testing.TB, w *index.IndexWriter, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		testIndexWriterAddDoc(t, w)
	}
}

// Simple test for "commit on close": open writer then add a bunch of docs,
// making sure reader does not see these docs until writer is closed.
func TestIndexWriterCommitCommitOnClose(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	addDocs(t, writer, 14)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader, dir)
	newSearcher(t, reader)
}

// Simple test for "commit on close": open writer, then add a bunch of docs,
// making sure reader does not see them until writer has closed. Then instead
// of closing the writer, call abort and verify reader sees nothing was
// added. Then verify we can open the index and add docs to it.
func TestIndexWriterCommitCommitOnCloseAbort(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(10)
	writer := mustNewIndexWriter(t, dir, conf)
	addDocs(t, writer, 14)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader, dir)
	newSearcher(t, reader)
}

// Verify that calling forceMerge when writer is open for "commit on close"
// works correctly both for rollback() and close().
func TestIndexWriterCommitCommitOnCloseForceMerge(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(10)
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(10))
	writer := mustNewIndexWriter(t, dir, conf)
	for j := 0; j < 17; j++ {
		testIndexWriterAddDocWithIndex(t, writer, j)
	}
	mustClose(t, writer)

	appendWriter := func() *index.IndexWriter {
		conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		conf.SetOpenMode(index.Append)
		return mustNewIndexWriter(t, dir, conf)
	}
	writer = appendWriter()
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	// Open a reader before closing (commiting) the writer; reader should see
	// index as multi-seg at this point:
	assertLeavesAndDocs(t, dir, -2, -1)

	// Abort the writer:
	if err := writer.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	assertNoUnreferencedFiles(t, dir, "aborted writer after forceMerge")

	// Open a reader after aborting writer; reader should still see index as
	// multi-segment
	assertLeavesAndDocs(t, dir, -2, -1)

	writer = appendWriter()
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, writer)

	assertNoUnreferencedFiles(t, dir, "aborted writer after forceMerge")

	// Open a reader after aborting writer; reader should see index as one
	// segment
	assertLeavesAndDocs(t, dir, 1, -1)
	mustClose(t, dir)
}

func assertReaderNumDocs(t testing.TB, expected int, r index.IndexReaderInterface) {
	t.Helper()
	if r.NumDocs() != expected {
		t.Fatalf("numDocs: expected %d, got %d", expected, r.NumDocs())
	}
}

// openIfChanged renders DirectoryReader.openIfChanged(DirectoryReader).
func openIfChanged(t testing.TB, r *index.DirectoryReader) *index.DirectoryReader {
	t.Helper()
	nr, err := index.OpenIfChanged(r)
	if err != nil {
		t.Fatalf("openIfChanged: %v", err)
	}
	if nr == nil {
		return nil
	}
	dr, ok := nr.(*index.DirectoryReader)
	if !ok {
		t.Fatalf("openIfChanged returned %T, want *DirectoryReader", nr)
	}
	return dr
}

func openCommitWriter(t testing.TB, dir store.Directory) *index.IndexWriter {
	t.Helper()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(5))
	writer := mustNewIndexWriter(t, dir, conf)
	mustCommit(t, writer)
	return writer
}

// LUCENE-1044: test writer.commit() when ac=false
func TestIndexWriterCommitForceCommit(t *testing.T) {
	dir := newDirectory()

	writer := openCommitWriter(t, dir)

	addDocs(t, writer, 23)

	reader := mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 0, reader)
	mustCommit(t, writer)
	reader2 := openIfChanged(t, reader)
	if reader2 == nil {
		t.Fatal("assertNotNull(reader2)")
	}
	assertReaderNumDocs(t, 0, reader)
	assertReaderNumDocs(t, 23, reader2)
	mustClose(t, reader)

	addDocs(t, writer, 17)
	assertReaderNumDocs(t, 23, reader2)
	mustClose(t, reader2)
	reader = mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 23, reader)
	mustClose(t, reader)
	mustCommit(t, writer)

	reader = mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 40, reader)
	mustClose(t, reader, writer, dir)
}

func findCommitByTag(t testing.TB, dir store.Directory, tag string) *index.IndexCommit {
	t.Helper()
	for _, c := range mustListCommits(t, dir) {
		if c.GetUserData()["tag"] == tag {
			return c
		}
	}
	return nil
}

func TestIndexWriterCommitFutureCommit(t *testing.T) {
	dir := newDirectory()

	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetIndexDeletionPolicy(index.NoDeletionPolicyInstance)
	w := mustNewIndexWriter(t, dir, conf)
	doc := document.NewDocument()
	mustAddDocument(t, w, doc)

	// commit to "first"
	commitData := map[string]string{"tag": "first"}
	w.SetLiveCommitData(maps.All(commitData))
	mustCommit(t, w)

	// commit to "second"
	mustAddDocument(t, w, doc)
	commitData["tag"] = "second"
	w.SetLiveCommitData(maps.All(commitData))
	mustClose(t, w)

	// open "first" with IndexWriter
	commit := findCommitByTag(t, dir, "first")
	if commit == nil {
		t.Fatal("assertNotNull(commit)")
	}

	conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetIndexDeletionPolicy(index.NoDeletionPolicyInstance)
	conf.SetIndexCommit(commit)
	w = mustNewIndexWriter(t, dir, conf)

	assertWriterDocStats(t, w, -1, 1)

	// commit IndexWriter to "third"
	mustAddDocument(t, w, doc)
	commitData["tag"] = "third"
	w.SetLiveCommitData(maps.All(commitData))
	mustClose(t, w)

	// make sure "second" commit is still there
	if findCommitByTag(t, dir, "second") == nil {
		t.Fatal("assertNotNull(commit)")
	}

	mustClose(t, dir)
}

func TestIndexWriterCommitZeroCommits(t *testing.T) {
	// Tests that if we don't call commit(), the directory has 0 commits. This
	// has changed since LUCENE-2386, where before IW would always commit on a
	// fresh new index.
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	_, err := index.ListCommits(dir)
	var infe *index.IndexNotFoundException
	if err == nil || !errors.As(err, &infe) {
		t.Fatalf("expected IndexNotFoundException from listCommits, got %v", err)
	}

	// No changes still should generate a commit, because it's a new index.
	mustClose(t, writer)
	if n := len(mustListCommits(t, dir)); n != 1 {
		t.Fatalf("expected 1 commits!: got %d", n)
	}
	mustClose(t, dir)
}

func mustPrepareCommit(t testing.TB, w *index.IndexWriter) {
	t.Helper()
	if _, err := w.PrepareCommit(); err != nil {
		t.Fatalf("prepareCommit: %v", err)
	}
}

// LUCENE-1274: test writer.prepareCommit()
func TestIndexWriterCommitPrepareCommit(t *testing.T) {
	dir := newDirectory()

	writer := openCommitWriter(t, dir)

	addDocs(t, writer, 23)

	reader := mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 0, reader)

	mustPrepareCommit(t, writer)

	reader2 := mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 0, reader2)

	mustCommit(t, writer)

	reader3 := openIfChanged(t, reader)
	if reader3 == nil {
		t.Fatal("assertNotNull(reader3)")
	}
	assertReaderNumDocs(t, 0, reader)
	assertReaderNumDocs(t, 0, reader2)
	assertReaderNumDocs(t, 23, reader3)
	mustClose(t, reader, reader2)

	addDocs(t, writer, 17)

	assertReaderNumDocs(t, 23, reader3)
	mustClose(t, reader3)
	reader = mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 23, reader)
	mustClose(t, reader)

	mustPrepareCommit(t, writer)

	reader = mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 23, reader)
	mustClose(t, reader)

	mustCommit(t, writer)
	reader = mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 40, reader)
	mustClose(t, reader, writer, dir)
}

// LUCENE-1274: test writer.prepareCommit()
func TestIndexWriterCommitPrepareCommitRollback(t *testing.T) {
	dir := newDirectory()

	writer := openCommitWriter(t, dir)

	addDocs(t, writer, 23)

	reader := mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 0, reader)

	mustPrepareCommit(t, writer)

	reader2 := mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 0, reader2)

	if err := writer.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if reader3 := openIfChanged(t, reader); reader3 != nil {
		t.Fatalf("assertNull(reader3): got %v", reader3)
	}
	assertReaderNumDocs(t, 0, reader)
	assertReaderNumDocs(t, 0, reader2)
	mustClose(t, reader, reader2)

	writer = mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	addDocs(t, writer, 17)

	reader = mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 0, reader)
	mustClose(t, reader)

	mustPrepareCommit(t, writer)

	reader = mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 0, reader)
	mustClose(t, reader)

	mustCommit(t, writer)
	reader = mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 17, reader)
	mustClose(t, reader, writer, dir)
}

// LUCENE-1274
func TestIndexWriterCommitPrepareCommitNoChanges(t *testing.T) {
	dir := newDirectory()

	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	mustPrepareCommit(t, writer)
	mustCommit(t, writer)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 0, reader)
	mustClose(t, reader, dir)
}

// LUCENE-1382
func TestIndexWriterCommitCommitUserData(t *testing.T) {
	dir := newDirectory()
	twoBufferedWriter := func() *index.IndexWriter {
		conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		conf.SetMaxBufferedDocs(2)
		return mustNewIndexWriter(t, dir, conf)
	}
	w := twoBufferedWriter()
	addDocs(t, w, 17)
	mustClose(t, w)

	r := mustOpenDirectoryReader(t, dir)
	// commit(Map) never called for this index
	if n := len(r.GetIndexCommit().GetUserData()); n != 0 {
		t.Fatalf("getUserData().size(): expected 0, got %d", n)
	}
	mustClose(t, r)

	w = twoBufferedWriter()
	addDocs(t, w, 17)
	data := map[string]string{"label": "test1"}
	w.SetLiveCommitData(maps.All(data))
	mustClose(t, w)

	r = mustOpenDirectoryReader(t, dir)
	if got := r.GetIndexCommit().GetUserData()["label"]; got != "test1" {
		t.Fatalf("label: expected test1, got %q", got)
	}
	mustClose(t, r)

	w = mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, w, dir)
}

func TestIndexWriterCommitPrepareCommitThenClose(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	mustAddDocument(t, w, document.NewDocument())

	mustPrepareCommit(t, w)
	if err := w.Close(); err == nil {
		t.Fatal("expected IllegalStateException from close() after prepareCommit()")
	}
	mustCommit(t, w)
	mustClose(t, w)

	r := mustOpenDirectoryReader(t, dir)
	if r.MaxDoc() != 1 {
		t.Fatalf("maxDoc: expected 1, got %d", r.MaxDoc())
	}
	mustClose(t, r, dir)
}

// LUCENE-7335: make sure commit data is late binding
func TestIndexWriterCommitCommitDataIsLive(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	mustAddDocument(t, w, document.NewDocument())

	commitData := map[string]string{"foo": "bar"}

	// make sure "foo" / "bar" doesn't take
	w.SetLiveCommitData(maps.All(commitData))

	clear(commitData)
	commitData["boo"] = "baz"

	// this finally does the commit, and should burn "boo" / "baz"
	mustClose(t, w)

	commits := mustListCommits(t, dir)
	if len(commits) != 1 {
		t.Fatalf("commits.size(): expected 1, got %d", len(commits))
	}

	data := commits[0].GetUserData()
	if len(data) != 1 {
		t.Fatalf("data.size(): expected 1, got %d (%v)", len(data), data)
	}
	if data["boo"] != "baz" {
		t.Fatalf("boo: expected baz, got %q", data["boo"])
	}
	mustClose(t, dir)
}
