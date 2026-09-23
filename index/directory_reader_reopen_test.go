// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestDirectoryReaderReopen.java
// (Apache Lucene 10.5.0).
//
// The *WithExecutor tests open readers through DirectoryReader.open(Directory,
// ExecutorService), DirectoryReader.open(IndexCommit, ExecutorService) and the
// openIfChanged(..., ExecutorService) overloads, none of which is ported; each
// runs its setup and fails at the first such call, naming it.

package index_test

import (
	"maps"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Missing members the Java tests reach.
const (
	assertIndexEqualsMissing         = "TestDirectoryReader.assertIndexEquals(DirectoryReader, DirectoryReader) is not ported"
	openWithExecutorMissing          = "org.apache.lucene.index.DirectoryReader#open(Directory, ExecutorService) is not ported"
	openCommitWithExecutorMissing    = "org.apache.lucene.index.DirectoryReader#open(IndexCommit, ExecutorService) is not ported"
	openIfChangedWithExecutorMissing = "org.apache.lucene.index.DirectoryReader#openIfChanged(DirectoryReader, IndexCommit, ExecutorService) is not ported"
	callStackContainsMissing         = "MockDirectoryWrapper.Failure#callStackContainsAnyOf(String...) is not ported"
)

func TestDirectoryReaderReopenReopen(t *testing.T) {
	dir1 := newDirectory()
	defer mustClose(t, dir1)

	directoryReaderReopenCreateIndex(t, dir1, false)
	directoryReaderReopenPerformDefaultTests(t, func() *index.DirectoryReader { return mustOpenDirectoryReader(t, dir1) })
}

// LUCENE-1228: IndexWriter.commit() does not update the index version
// populate an index in iterations.
// at the end of every iteration, commit the index and reopen/recreate the reader.
// in each iteration verify the work of previous iteration.
// try this once with reopen once recreate, on both RAMDir and FSDir.
func TestDirectoryReaderReopenCommitReopen(t *testing.T) {
	dir := newDirectory()
	directoryReaderReopenDoTestReopenWithCommit(t, dir, true)
	mustClose(t, dir)
}

func TestDirectoryReaderReopenCommitRecreate(t *testing.T) {
	dir := newDirectory()
	directoryReaderReopenDoTestReopenWithCommit(t, dir, false)
	mustClose(t, dir)
}

func directoryReaderReopenDoTestReopenWithCommit(t *testing.T, dir store.Directory, withReopen bool) {
	t.Helper()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetOpenMode(index.Create)
	conf.SetMergeScheduler(index.NewSerialMergeScheduler())
	conf.SetMergePolicy(newLogMergePolicy())
	iwriter := mustNewIndexWriter(t, dir, conf)
	mustCommit(t, iwriter)
	reader := mustOpenDirectoryReader(t, dir)
	defer func() { mustClose(t, iwriter, reader) }()
	const m = 3
	customType := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	customType.SetTokenized(false)
	customType2 := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	customType2.SetTokenized(false)
	customType2.SetOmitNorms(true)
	customType3 := document.NewFieldType()
	customType3.SetStored(true)
	for i := 0; i < 4; i++ {
		for j := 0; j < m; j++ {
			doc := document.NewDocument()
			id := strconv.Itoa(i) + "_" + strconv.Itoa(j)
			doc.Add(newField(t, "id", id, customType))
			doc.Add(newField(t, "id2", id, customType2))
			doc.Add(newField(t, "id3", id, customType3))
			mustAddDocument(t, iwriter, doc)
			if i > 0 {
				k := i - 1
				n := j + k*m
				storedFields, err := reader.StoredFields()
				if err != nil {
					t.Fatalf("storedFields: %v", err)
				}
				prevIterationDoc := storedDocument(t, storedFields, n)
				if prevIterationDoc == nil {
					t.Fatal("assertNotNull(prevItereationDoc)")
				}
				got := docGet(prevIterationDoc, "id")
				want := strconv.Itoa(k) + "_" + strconv.Itoa(j)
				if got == nil || *got != want {
					t.Fatalf("id: expected %s, got %v", want, got)
				}
			}
		}
		mustCommit(t, iwriter)
		if withReopen {
			// reopen
			if r2 := openIfChanged(t, reader); r2 != nil {
				mustClose(t, reader)
				reader = r2
			}
		} else {
			// recreate
			mustClose(t, reader)
			reader = mustOpenDirectoryReader(t, dir)
		}
	}
}

// directoryReaderReopenPerformDefaultTests renders the private
// performDefaultTests(TestReopen), whose first check is
// TestDirectoryReader.assertIndexEquals(index1, index2).
func directoryReaderReopenPerformDefaultTests(t *testing.T, openReader func() *index.DirectoryReader) {
	t.Helper()
	index1 := openReader()
	index2 := openReader()
	defer mustClose(t, index1, index2)
	t.Fatal(assertIndexEqualsMissing)
}

func TestDirectoryReaderReopenThreadSafety(t *testing.T) {
	dir := newDirectory()
	// NOTE: this also controls the number of threads!
	n := nextInt(20, 40)

	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	for i := 0; i < n; i++ {
		mustAddDocument(t, writer, directoryReaderReopenCreateDocument(t, i, 3))
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, writer)

	firstReader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, firstReader, dir)
	// Task 1 searches through newSearcher and the verifying tasks call
	// TestDirectoryReader.assertIndexEquals.
	t.Fatal("org.apache.lucene.tests.search.AssertingIndexSearcher (built by LuceneTestCase.newSearcher(IndexReader)) and " +
		assertIndexEqualsMissing)
}

// directoryReaderReopenCreateIndex renders the public static
// createIndex(Random, Directory, boolean).
func directoryReaderReopenCreateIndex(t testing.TB, dir store.Directory, multiSegment bool) {
	t.Helper()
	var mp index.MergePolicy
	if multiSegment {
		mp = index.NewNoMergePolicy()
	} else {
		mp = index.NewLogDocMergePolicy()
	}

	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(mp)
	w := mustNewIndexWriter(t, dir, conf)

	for i := 0; i < 100; i++ {
		mustAddDocument(t, w, directoryReaderReopenCreateDocument(t, i, 4))
		if multiSegment && i%10 == 0 {
			mustCommit(t, w)
		}
	}

	if !multiSegment {
		if err := w.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	}

	mustClose(t, w)

	r := mustOpenDirectoryReader(t, dir)
	leaves, err := r.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	if multiSegment {
		if !(len(leaves) > 1) {
			t.Fatalf("assertTrue(r.leaves().size() > 1): %d", len(leaves))
		}
	} else if len(leaves) != 1 {
		t.Fatalf("assertTrue(r.leaves().size() == 1): %d", len(leaves))
	}
	mustClose(t, r)
}

// directoryReaderReopenCreateDocument renders the public static
// createDocument(int, int).
func directoryReaderReopenCreateDocument(t testing.TB, n, numFields int) *document.Document {
	var sb strings.Builder
	doc := document.NewDocument()
	sb.WriteString("a")
	sb.WriteString(strconv.Itoa(n))
	customType2 := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	customType2.SetTokenized(false)
	customType2.SetOmitNorms(true)
	customType3 := document.NewFieldType()
	customType3.SetStored(true)
	doc.Add(mustNewTextFieldNoRandom(t, "field1", sb.String(), true))
	doc.Add(mustNewFieldNoRandom(t, "fielda", sb.String(), customType2))
	doc.Add(mustNewFieldNoRandom(t, "fieldb", sb.String(), customType3))
	sb.WriteString(" b")
	sb.WriteString(strconv.Itoa(n))
	for i := 1; i < numFields; i++ {
		doc.Add(mustNewTextFieldNoRandom(t, "field"+strconv.Itoa(i+1), sb.String(), true))
	}
	return doc
}

func mustNewTextFieldNoRandom(t testing.TB, name, value string, stored bool) *document.TextField {
	t.Helper()
	f, err := document.NewTextField(name, value, stored)
	if err != nil {
		t.Fatalf("TextField: %v", err)
	}
	return f
}

func mustNewFieldNoRandom(t testing.TB, name, value string, ft *document.FieldType) *document.Field {
	t.Helper()
	f, err := document.NewField(name, value, ft)
	if err != nil {
		t.Fatalf("Field: %v", err)
	}
	return f
}

// directoryReaderReopenKeepAllCommits is the package-private static
// KeepAllCommits deletion policy.
type directoryReaderReopenKeepAllCommits struct{}

func (directoryReaderReopenKeepAllCommits) OnInit([]index.Commit) error   { return nil }
func (directoryReaderReopenKeepAllCommits) OnCommit([]index.Commit) error { return nil }

// Clone carries IndexDeletionPolicy's Go-only Clone member; the policy is
// stateless.
func (p directoryReaderReopenKeepAllCommits) Clone() index.IndexDeletionPolicy { return p }

// reopenOnCommitIndex renders the indexing prologue of testReopenOnCommit
// and its executor variant: 8 commits labelled by their "index" user data.
func reopenOnCommitIndex(t *testing.T, dir store.Directory) {
	t.Helper()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetIndexDeletionPolicy(directoryReaderReopenKeepAllCommits{})
	conf.SetMaxBufferedDocs(-1)
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(10))
	writer := mustNewIndexWriter(t, dir, conf)
	for i := 0; i < 4; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "id", strconv.Itoa(i), false))
		mustAddDocument(t, writer, doc)
		writer.SetLiveCommitData(maps.All(map[string]string{"index": strconv.Itoa(i)}))
		mustCommit(t, writer)
	}
	for i := 0; i < 4; i++ {
		mustDeleteTerm(t, writer, "id", strconv.Itoa(i))
		writer.SetLiveCommitData(maps.All(map[string]string{"index": strconv.Itoa(4 + i)}))
		mustCommit(t, writer)
	}
	mustClose(t, writer)
}

func TestDirectoryReaderReopenReopenOnCommit(t *testing.T) {
	dir := newDirectory()
	reopenOnCommitIndex(t, dir)

	r := mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 0, r)

	commits := mustListCommits(t, dir)
	for _, commit := range commits {
		nr, err := index.OpenIfChangedWithCommit(r, commit)
		if err != nil {
			t.Fatalf("openIfChanged(r, commit): %v", err)
		}
		if nr == nil {
			t.Fatal("assertNotNull(r2)")
		}
		r2, ok := nr.(*index.DirectoryReader)
		if !ok {
			t.Fatalf("openIfChanged returned %T", nr)
		}
		if r2 == r {
			t.Fatal("assertTrue(r2 != r)")
		}

		s := commit.GetUserData()
		var v int
		if len(s) == 0 {
			// First commit created by IW
			v = -1
		} else {
			v, err = strconv.Atoi(s["index"])
			if err != nil {
				t.Fatalf("Integer.parseInt(%q): %v", s["index"], err)
			}
		}
		if v < 4 {
			assertReaderNumDocs(t, 1+v, r2)
		} else {
			assertReaderNumDocs(t, 7-v, r2)
		}
		mustClose(t, r)
		r = r2
	}
	mustClose(t, r, dir)
}

// openIfChangedNRTToCommitSetup renders the shared prologue of
// testOpenIfChangedNRTToCommit and its executor variant.
func openIfChangedNRTToCommitSetup(t *testing.T, dir store.Directory) (*index.IndexWriter, *index.IndexCommit) {
	t.Helper()
	// Can't use RIW because it randomly commits:
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	doc.Add(newStringField(t, "field", "value", false))
	mustAddDocument(t, w, doc)
	mustCommit(t, w)
	commits := mustListCommits(t, dir)
	if len(commits) != 1 {
		t.Fatalf("commits.size(): expected 1, got %d", len(commits))
	}
	mustAddDocument(t, w, doc)
	return w, commits[0]
}

func TestDirectoryReaderReopenOpenIfChangedNRTToCommit(t *testing.T) {
	dir := newDirectory()
	w, commit := openIfChangedNRTToCommitSetup(t, dir)
	r := openReaderFromWriter(t, w)

	assertReaderNumDocs(t, 2, r)
	r2, err := index.OpenIfChangedWithCommit(r, commit)
	if err != nil {
		t.Fatalf("openIfChanged(r, commit): %v", err)
	}
	if r2 == nil {
		t.Fatal("assertNotNull(r2)")
	}
	mustClose(t, r)
	assertReaderNumDocs(t, 1, r2)
	mustClose(t, w, r2, dir)
}

// overDecRefSetup renders the shared prologue of
// testOverDecRefDuringReopen and its executor variant.
func overDecRefSetup(t *testing.T, dir store.Directory) *index.IndexWriter {
	t.Helper()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(index.NewNoMergePolicy())
	iwc.SetCodec(index.GetDefaultCodec())
	w := mustNewIndexWriter(t, dir, iwc)
	doc := document.NewDocument()
	doc.Add(newStringField(t, "id", "id", false))
	mustAddDocument(t, w, doc)
	doc = document.NewDocument()
	doc.Add(newStringField(t, "id", "id2", false))
	mustAddDocument(t, w, doc)
	mustCommit(t, w)
	return w
}

func TestDirectoryReaderReopenOverDecRefDuringReopen(t *testing.T) {
	dir := newDirectory()
	w := overDecRefSetup(t, dir)

	// Open reader w/ one segment w/ 2 docs:
	r := mustOpenDirectoryReader(t, dir)

	// Delete 1 doc from the segment:
	mustDeleteTerm(t, w, "id", "id")
	mustCommit(t, w)

	defer mustClose(t, r, w, dir)
	// Fail when reopen tries to open the live docs file:
	t.Fatal(callStackContainsMissing)
}

// invalidReindexSetup renders the shared prologue of the testNPEAfterInvalidReindex
// tests: a one-segment index with one deleted document, a reader on it, and
// the index blown away.
func invalidReindexSetup(t *testing.T, dir store.Directory) *index.DirectoryReader {
	t.Helper()
	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	w := mustNewIndexWriter(t, dir, conf)
	doc := document.NewDocument()
	doc.Add(newStringField(t, "id", "id", false))
	mustAddDocument(t, w, doc)
	doc = document.NewDocument()
	doc.Add(newStringField(t, "id", "id2", false))
	mustAddDocument(t, w, doc)
	mustDeleteTerm(t, w, "id", "id")
	mustCommit(t, w)
	mustClose(t, w)

	// Open reader w/ one segment w/ 2 docs, 1 deleted:
	r := mustOpenDirectoryReader(t, dir)

	// Blow away the index:
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	for _, fileName := range files {
		if err := dir.DeleteFile(fileName); err != nil {
			t.Fatalf("deleteFile(%s): %v", fileName, err)
		}
	}
	return r
}

func invalidReindex1Writes(t *testing.T, dir store.Directory) {
	t.Helper()
	w := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	doc.Add(newStringField(t, "id", "id", false))
	doc.Add(numericDVField(t, "ndv", 13))
	mustAddDocument(t, w, doc)
	doc = document.NewDocument()
	doc.Add(newStringField(t, "id", "id2", false))
	mustAddDocument(t, w, doc)
	mustCommit(t, w)
	doc = document.NewDocument()
	doc.Add(newStringField(t, "id", "id2", false))
	mustAddDocument(t, w, doc)
	mustUpdateNumericDocValue(t, w, index.NewTerm("id", "id"), "ndv", 17)
	mustCommit(t, w)
	mustClose(t, w)
}

func invalidReindex2Writes(t *testing.T, dir store.Directory) {
	t.Helper()
	w := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	doc.Add(newStringField(t, "id", "id", false))
	doc.Add(numericDVField(t, "ndv", 13))
	mustAddDocument(t, w, doc)
	mustCommit(t, w)
	doc = document.NewDocument()
	doc.Add(newStringField(t, "id", "id2", false))
	mustAddDocument(t, w, doc)
	mustCommit(t, w)
	mustClose(t, w)
}

// expectOpenIfChangedIllegalState renders
// expectThrows(IllegalStateException.class, () -> DirectoryReader.openIfChanged(r)).
func expectOpenIfChangedIllegalState(t *testing.T, r *index.DirectoryReader) {
	t.Helper()
	nr, err := index.OpenIfChanged(r)
	if err == nil {
		if nr != nil && nr != index.IndexReaderInterface(r) {
			mustClose(t, nr)
		}
		t.Fatal("expected IllegalStateException from openIfChanged")
	}
}

func TestDirectoryReaderReopenNPEAfterInvalidReindex1(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	r := invalidReindexSetup(t, dir)
	invalidReindex1Writes(t, dir)

	expectOpenIfChangedIllegalState(t, r)

	mustClose(t, r, dir)
}

func TestDirectoryReaderReopenNPEAfterInvalidReindex2(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	r := invalidReindexSetup(t, dir)
	invalidReindex2Writes(t, dir)

	expectOpenIfChangedIllegalState(t, r)

	mustClose(t, r, dir)
}

// nrtmSnapshotWriter renders the SnapshotDeletionPolicy writer of the NRTM
// tests.
func nrtmSnapshotWriter(t *testing.T, dir store.Directory, noMerge bool) (*index.IndexWriter, *index.SnapshotDeletionPolicy) {
	t.Helper()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	if noMerge {
		iwc.SetMergePolicy(index.NewNoMergePolicy())
	}
	snapshotter := index.NewSnapshotDeletionPolicy(index.NewKeepOnlyLastCommitDeletionPolicy())
	iwc.SetIndexDeletionPolicy(snapshotter)
	writer := mustNewIndexWriter(t, dir, iwc)
	mustCommit(t, writer) // make sure all index metadata is written out
	return writer, snapshotter
}

func mustSnapshot(t testing.TB, sdp *index.SnapshotDeletionPolicy) *index.IndexCommit {
	t.Helper()
	c, err := sdp.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	ic, ok := c.(*index.IndexCommit)
	if !ok {
		t.Fatalf("snapshot returned %T, want *IndexCommit", c)
	}
	return ic
}

func keyDoc(t testing.TB, value string) *document.Document {
	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "key", value, true))
	return doc
}

// nrtmDeletesSetup renders the shared indexing of the testNRTMdeletes tests.
func nrtmDeletesSetup(t *testing.T, dir store.Directory) (*index.IndexWriter, *index.SnapshotDeletionPolicy, *index.IndexCommit) {
	t.Helper()
	writer, snapshotter := nrtmSnapshotWriter(t, dir, true)

	mustAddDocument(t, writer, keyDoc(t, "value1"))
	mustAddDocument(t, writer, keyDoc(t, "value2"))

	mustCommit(t, writer)

	ic1 := mustSnapshot(t, snapshotter)

	mustUpdateDocument(t, writer, index.NewTerm("key", "value1"), keyDoc(t, "value3"))
	return writer, snapshotter, ic1
}

func openCommitReader(t testing.TB, ic *index.IndexCommit) *index.DirectoryReader {
	t.Helper()
	r, err := index.OpenDirectoryReaderAtCommit(ic)
	if err != nil {
		t.Fatalf("DirectoryReader.open(commit): %v", err)
	}
	return r
}

func openIfChangedToCommit(t testing.TB, r *index.DirectoryReader, ic *index.IndexCommit) *index.DirectoryReader {
	t.Helper()
	nr, err := index.OpenIfChangedWithCommit(r, ic)
	if err != nil {
		t.Fatalf("openIfChanged(r, commit): %v", err)
	}
	dr, ok := nr.(*index.DirectoryReader)
	if !ok || dr == nil {
		t.Fatalf("openIfChanged(r, commit) returned %T", nr)
	}
	return dr
}

// assertSameCore renders assertSame(latest.leaves().get(0).reader()
// .getCoreCacheHelper().getKey(), oldest.leaves().get(0).reader()
// .getCoreCacheHelper().getKey()).
func assertSameCore(t testing.TB, latest, oldest *index.DirectoryReader) {
	t.Helper()
	a := firstLeaf(t, latest).GetCoreCacheHelper()
	b := firstLeaf(t, oldest).GetCoreCacheHelper()
	if a == nil || b == nil || a.CacheKey() != b.CacheKey() {
		t.Fatal("sharing same core: core cache keys differ")
	}
}

func mustRelease(t testing.TB, sdp *index.SnapshotDeletionPolicy, ic *index.IndexCommit) {
	t.Helper()
	if err := sdp.Release(ic); err != nil {
		t.Fatalf("release: %v", err)
	}
}

// test reopening backwards from a non-NRT reader (with document deletes)
func TestDirectoryReaderReopenNRTMdeletes(t *testing.T) {
	dir := newDirectory()
	writer, snapshotter, ic1 := nrtmDeletesSetup(t, dir)

	mustCommit(t, writer)

	ic2 := mustSnapshot(t, snapshotter)
	latest := openCommitReader(t, ic2)
	assertLeafCount(t, 2, latest)

	// This reader will be used for searching against commit point 1
	oldest := openIfChangedToCommit(t, latest, ic1)
	assertLeafCount(t, 1, oldest)

	// sharing same core
	assertSameCore(t, latest, oldest)

	mustClose(t, latest, oldest)

	mustRelease(t, snapshotter, ic1)
	mustRelease(t, snapshotter, ic2)
	mustClose(t, writer, dir)
}

// test reopening backwards from an NRT reader (with document deletes)
func TestDirectoryReaderReopenNRTMdeletes2(t *testing.T) {
	dir := newDirectory()
	writer, snapshotter, ic1 := nrtmDeletesSetup(t, dir)

	latest := openReaderFromWriter(t, writer)
	assertLeafCount(t, 2, latest)

	// This reader will be used for searching against commit point 1
	oldest := openIfChangedToCommit(t, latest, ic1)

	// This reader should not see the deletion:
	assertReaderNumDocs(t, 2, oldest)
	if oldest.HasDeletions() {
		t.Fatal("assertFalse(oldest.hasDeletions())")
	}

	mustRelease(t, snapshotter, ic1)
	assertLeafCount(t, 1, oldest)

	// sharing same core
	assertSameCore(t, latest, oldest)

	mustClose(t, latest, oldest, writer, dir)
}

// nrtmUpdatesSetup renders the shared indexing of the testNRTMupdates tests.
func nrtmUpdatesSetup(t *testing.T, dir store.Directory) (*index.IndexWriter, *index.SnapshotDeletionPolicy, *index.IndexCommit) {
	t.Helper()
	writer, snapshotter := nrtmSnapshotWriter(t, dir, false)

	doc := keyDoc(t, "value1")
	doc.Add(numericDVField(t, "dv", 1))
	mustAddDocument(t, writer, doc)

	mustCommit(t, writer)

	ic1 := mustSnapshot(t, snapshotter)

	mustUpdateNumericDocValue(t, writer, index.NewTerm("key", "value1"), "dv", 2)
	return writer, snapshotter, ic1
}

func assertOnlyLeafDV(t testing.TB, r *index.DirectoryReader, expected int64) {
	t.Helper()
	values := leafNumeric(t, getOnlyLeafReader(t, r), "dv")
	assertNextNumeric(t, values, 0, expected)
}

// test reopening backwards from a non-NRT reader with DV updates
func TestDirectoryReaderReopenNRTMupdates(t *testing.T) {
	dir := newDirectory()
	writer, snapshotter, ic1 := nrtmUpdatesSetup(t, dir)

	mustCommit(t, writer)

	ic2 := mustSnapshot(t, snapshotter)
	latest := openCommitReader(t, ic2)
	assertLeafCount(t, 1, latest)

	// This reader will be used for searching against commit point 1
	oldest := openIfChangedToCommit(t, latest, ic1)
	assertLeafCount(t, 1, oldest)

	// sharing same core
	assertSameCore(t, latest, oldest)

	assertOnlyLeafDV(t, oldest, 1)
	assertOnlyLeafDV(t, latest, 2)

	mustClose(t, latest, oldest)

	mustRelease(t, snapshotter, ic1)
	mustRelease(t, snapshotter, ic2)
	mustClose(t, writer, dir)
}

// test reopening backwards from an NRT reader with DV updates
func TestDirectoryReaderReopenNRTMupdates2(t *testing.T) {
	dir := newDirectory()
	writer, snapshotter, ic1 := nrtmUpdatesSetup(t, dir)

	latest := openReaderFromWriter(t, writer)
	assertLeafCount(t, 1, latest)

	// This reader will be used for searching against commit point 1
	oldest := openIfChangedToCommit(t, latest, ic1)
	assertLeafCount(t, 1, oldest)

	// sharing same core
	assertSameCore(t, latest, oldest)

	assertOnlyLeafDV(t, oldest, 1)
	assertOnlyLeafDV(t, latest, 2)

	mustClose(t, latest, oldest)

	mustRelease(t, snapshotter, ic1)
	mustClose(t, writer, dir)
}

// deleteIndexFilesSetup renders the shared body of
// testDeleteIndexFilesWhileReaderStillOpen and its executor variant, up to
// the reopen.
func deleteIndexFilesSetup(t *testing.T, dir store.Directory, open func() *index.DirectoryReader) *index.DirectoryReader {
	t.Helper()
	w := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	doc.Add(newStringField(t, "field", "value", false))
	mustAddDocument(t, w, doc)
	// Creates single segment index:
	mustClose(t, w)

	r := open()
	if r == nil {
		return nil
	}

	// Abuse: remove all files while reader is open; one is supposed to use
	// IW.deleteAll, or open a new IW with OpenMode.CREATE instead:
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	for _, file := range files {
		if err := dir.DeleteFile(file); err != nil {
			t.Fatalf("deleteFile(%s): %v", file, err)
		}
	}

	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	w = mustNewIndexWriter(t, dir, conf)
	doc = document.NewDocument()
	doc.Add(newStringField(t, "field", "value", false))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(newStringField(t, "field", "value2", false))
	mustAddDocument(t, w, doc)

	// Writes same segment, this time with two documents:
	mustCommit(t, w)

	mustDeleteTerm(t, w, "field", "value2")

	mustAddDocument(t, w, doc)

	// Writes another segments file, so openIfChanged sees that the index has
	// in fact changed:
	mustClose(t, w)
	return r
}

// LUCENE-5931: we make a "best effort" to catch this abuse and throw a
// clear(er) exception than what would otherwise look like hard to explain
// index corruption during searching
func TestDirectoryReaderReopenDeleteIndexFilesWhileReaderStillOpen(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	r := deleteIndexFilesSetup(t, dir, func() *index.DirectoryReader { return mustOpenDirectoryReader(t, dir) })

	expectOpenIfChangedIllegalState(t, r)
}

// reuseUnchangedLeafReaderSetup renders the shared prologue of
// testReuseUnchangedLeafReaderOnDVUpdate and its executor variant.
func reuseUnchangedLeafReaderSetup(t *testing.T, dir store.Directory) *index.IndexWriter {
	t.Helper()
	indexWriterConfig := newIndexWriterConfig()
	indexWriterConfig.SetMergePolicy(index.NewNoMergePolicy())
	writer := mustNewIndexWriter(t, dir, indexWriterConfig)

	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "1", true))
	doc.Add(newStringFieldNoRandom(t, "version", "1", true))
	doc.Add(numericDVField(t, "some_docvalue", 2))
	mustAddDocument(t, writer, doc)
	doc = document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "2", true))
	doc.Add(newStringFieldNoRandom(t, "version", "1", true))
	mustAddDocument(t, writer, doc)
	mustCommit(t, writer)
	return writer
}

func assertReaderCountsDeleted(t testing.TB, r *index.DirectoryReader, numDocs, maxDoc, deleted int) {
	t.Helper()
	if r.NumDocs() != numDocs || r.MaxDoc() != maxDoc || r.NumDeletedDocs() != deleted {
		t.Fatalf("numDocs/maxDoc/numDeletedDocs: expected %d/%d/%d, got %d/%d/%d",
			numDocs, maxDoc, deleted, r.NumDocs(), r.MaxDoc(), r.NumDeletedDocs())
	}
}

func TestDirectoryReaderReopenReuseUnchangedLeafReaderOnDVUpdate(t *testing.T) {
	dir := newDirectory()
	writer := reuseUnchangedLeafReaderSetup(t, dir)
	reader := mustOpenDirectoryReader(t, dir)
	assertReaderCountsDeleted(t, reader, 2, 2, 0)

	mustUpdateDocValues(t, writer, index.NewTerm("id", "1"), numericDVField(t, "some_docvalue", 1).Field)
	mustCommit(t, writer)
	newReader := openIfChanged(t, reader)
	if newReader == reader {
		t.Fatal("assertNotSame(newReader, reader)")
	}
	mustClose(t, reader)
	reader = newReader
	assertReaderCountsDeleted(t, reader, 2, 2, 0)

	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "3", true))
	doc.Add(newStringFieldNoRandom(t, "version", "3", true))
	mustUpdateDocument(t, writer, index.NewTerm("id", "3"), doc)
	mustCommit(t, writer)

	newReader = openIfChanged(t, reader)
	if newReader == reader {
		t.Fatal("assertNotSame(newReader, reader)")
	}
	if n := len(newReader.GetSequentialSubReaders()); n != 2 {
		t.Fatalf("newReader.getSequentialSubReaders().size(): expected 2, got %d", n)
	}
	if n := len(reader.GetSequentialSubReaders()); n != 1 {
		t.Fatalf("reader.getSequentialSubReaders().size(): expected 1, got %d", n)
	}
	if reader.GetSequentialSubReaders()[0] != newReader.GetSequentialSubReaders()[0] {
		t.Fatal("assertSame(reader.getSequentialSubReaders().get(0), newReader.getSequentialSubReaders().get(0))")
	}
	mustClose(t, reader)
	reader = newReader
	assertReaderCountsDeleted(t, reader, 3, 3, 0)
	mustClose(t, reader, writer, dir)
}

func TestDirectoryReaderReopenReopenWithExecutor(t *testing.T) {
	dir1 := newDirectory()
	defer mustClose(t, dir1)
	directoryReaderReopenCreateIndex(t, dir1, false)
	t.Fatal(openWithExecutorMissing)
}

func TestDirectoryReaderReopenCommitReopenWithExecutor(t *testing.T) {
	directoryReaderReopenDoTestReopenWithCommitWithExecutor(t)
}

// directoryReaderReopenDoTestReopenWithCommitWithExecutor renders the
// executor overload of doTestReopenWithCommit: its first reader comes from
// DirectoryReader.open(dir, executorService).
func directoryReaderReopenDoTestReopenWithCommitWithExecutor(t *testing.T) {
	t.Helper()
	dir := newDirectory()
	defer mustClose(t, dir)
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetOpenMode(index.Create)
	conf.SetMergeScheduler(index.NewSerialMergeScheduler())
	conf.SetMergePolicy(newLogMergePolicy())
	iwriter := mustNewIndexWriter(t, dir, conf)
	mustCommit(t, iwriter)
	defer mustClose(t, iwriter)
	t.Fatal(openWithExecutorMissing)
}

func TestDirectoryReaderReopenCommitRecreateWithExecutor(t *testing.T) {
	directoryReaderReopenDoTestReopenWithCommitWithExecutor(t)
}

func TestDirectoryReaderReopenThreadSafetyWithExecutor(t *testing.T) {
	dir := newDirectory()
	// NOTE: this also controls the number of threads!
	n := nextInt(20, 40)

	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	for i := 0; i < n; i++ {
		mustAddDocument(t, writer, directoryReaderReopenCreateDocument(t, i, 3))
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, writer, dir)
	t.Fatal(openWithExecutorMissing)
}

func TestDirectoryReaderReopenReopenOnCommitWithExecutor(t *testing.T) {
	dir := newDirectory()
	reopenOnCommitIndex(t, dir)
	mustClose(t, dir)
	t.Fatal(openWithExecutorMissing)
}

func TestDirectoryReaderReopenOpenIfChangedNRTToCommitWithExecutor(t *testing.T) {
	dir := newDirectory()
	w, _ := openIfChangedNRTToCommitSetup(t, dir)
	r := openReaderFromWriter(t, w)

	assertReaderNumDocs(t, 2, r)
	defer mustClose(t, r, w, dir)
	t.Fatal(openIfChangedWithExecutorMissing)
}

func TestDirectoryReaderReopenOverDecRefDuringReopenWithExecutor(t *testing.T) {
	dir := newDirectory()
	w := overDecRefSetup(t, dir)
	defer mustClose(t, w, dir)
	t.Fatal(openWithExecutorMissing)
}

func TestDirectoryReaderReopenNPEAfterInvalidReindex1WithExecutor(t *testing.T) {
	invalidReindexWithExecutorSetup(t)
}

func TestDirectoryReaderReopenNPEAfterInvalidReindex2WithExecutor(t *testing.T) {
	invalidReindexWithExecutorSetup(t)
}

// invalidReindexWithExecutorSetup renders the prologue of the executor
// variants of testNPEAfterInvalidReindex, up to their
// DirectoryReader.open(dir, executorService).
func invalidReindexWithExecutorSetup(t *testing.T) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	w := mustNewIndexWriter(t, dir, conf)
	doc := document.NewDocument()
	doc.Add(newStringField(t, "id", "id", false))
	mustAddDocument(t, w, doc)
	doc = document.NewDocument()
	doc.Add(newStringField(t, "id", "id2", false))
	mustAddDocument(t, w, doc)
	mustDeleteTerm(t, w, "id", "id")
	mustCommit(t, w)
	mustClose(t, w, dir)
	t.Fatal(openWithExecutorMissing)
}

func TestDirectoryReaderReopenNRTMdeletesWithExecutor(t *testing.T) {
	dir := newDirectory()
	writer, snapshotter, ic1 := nrtmDeletesSetup(t, dir)
	mustCommit(t, writer)
	ic2 := mustSnapshot(t, snapshotter)
	defer func() {
		mustRelease(t, snapshotter, ic1)
		mustRelease(t, snapshotter, ic2)
		mustClose(t, writer, dir)
	}()
	t.Fatal(openCommitWithExecutorMissing)
}

func TestDirectoryReaderReopenNRTMdeletes2WithExecutor(t *testing.T) {
	dir := newDirectory()
	writer, snapshotter, ic1 := nrtmDeletesSetup(t, dir)
	latest := openReaderFromWriter(t, writer)
	assertLeafCount(t, 2, latest)
	defer func() {
		mustRelease(t, snapshotter, ic1)
		mustClose(t, latest, writer, dir)
	}()
	t.Fatal(openIfChangedWithExecutorMissing)
}

func TestDirectoryReaderReopenNRTMupdatesWithExecutor(t *testing.T) {
	dir := newDirectory()
	writer, snapshotter, ic1 := nrtmUpdatesSetup(t, dir)
	mustCommit(t, writer)
	ic2 := mustSnapshot(t, snapshotter)
	defer func() {
		mustRelease(t, snapshotter, ic1)
		mustRelease(t, snapshotter, ic2)
		mustClose(t, writer, dir)
	}()
	t.Fatal(openCommitWithExecutorMissing)
}

func TestDirectoryReaderReopenNRTMupdates2WithExecutor(t *testing.T) {
	dir := newDirectory()
	writer, snapshotter, ic1 := nrtmUpdatesSetup(t, dir)
	latest := openReaderFromWriter(t, writer)
	assertLeafCount(t, 1, latest)
	defer func() {
		mustRelease(t, snapshotter, ic1)
		mustClose(t, latest, writer, dir)
	}()
	t.Fatal(openIfChangedWithExecutorMissing)
}

func TestDirectoryReaderReopenDeleteIndexFilesWhileReaderStillOpenWithExecutor(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	deleteIndexFilesSetup(t, dir, func() *index.DirectoryReader {
		t.Fatal(openWithExecutorMissing)
		return nil
	})
}

func TestDirectoryReaderReopenReuseUnchangedLeafReaderOnDVUpdateWithExecutor(t *testing.T) {
	dir := newDirectory()
	writer := reuseUnchangedLeafReaderSetup(t, dir)
	defer mustClose(t, writer, dir)
	t.Fatal(openWithExecutorMissing)
}
