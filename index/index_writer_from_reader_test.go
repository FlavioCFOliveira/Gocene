// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriterFromReader.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"errors"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func iwfrOpen(t *testing.T, w *index.IndexWriter) *index.DirectoryReader {
	t.Helper()
	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("DirectoryReader.open(w): %v", err)
	}
	return r
}

// iwfrOpenIfChanged renders DirectoryReader.openIfChanged(r).
func iwfrOpenIfChanged(t *testing.T, r *index.DirectoryReader) *index.DirectoryReader {
	t.Helper()
	r2, err := index.OpenIfChanged(r)
	if err != nil {
		t.Fatalf("openIfChanged: %v", err)
	}
	return r2
}

func iwfrExpectIllegalArgument(t *testing.T, dir store.Directory, iwc *index.IndexWriterConfig) error {
	t.Helper()
	w, err := index.NewIndexWriter(dir, iwc)
	if err == nil {
		mustClose(t, w)
		t.Fatal("expected IllegalArgumentException")
	}
	return err
}

func iwfrAssertMaxDoc(t *testing.T, want, got int) {
	t.Helper()
	if want != got {
		t.Fatalf("expected maxDoc %d, got %d", want, got)
	}
}

// TestIndexWriterFromReaderRightAfterCommit: pull NRT reader immediately after
// writer has committed.
func TestIndexWriterFromReaderRightAfterCommit(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	mustAddDocument(t, w, document.NewDocument())
	mustCommit(t, w)

	r := iwfrOpen(t, w)
	iwfrAssertMaxDoc(t, 1, r.MaxDoc())
	mustClose(t, w)

	iwc := newIndexWriterConfig()
	iwc.SetIndexCommit(r.GetIndexCommit())

	w2 := mustNewIndexWriter(t, dir, iwc)
	mustClose(t, r)

	iwfrAssertMaxDoc(t, 1, iwDocStats(t, w2).MaxDoc)
	mustAddDocument(t, w2, document.NewDocument())
	iwfrAssertMaxDoc(t, 2, iwDocStats(t, w2).MaxDoc)
	mustClose(t, w2)

	r2 := mustOpenDirectoryReader(t, dir)
	iwfrAssertMaxDoc(t, 2, r2.MaxDoc())
	mustClose(t, r2, dir)
}

// TestIndexWriterFromReaderFromNonNRTReader: open from non-NRT reader.
func TestIndexWriterFromReaderFromNonNRTReader(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	mustAddDocument(t, w, document.NewDocument())
	mustClose(t, w)

	r := mustOpenDirectoryReader(t, dir)
	iwfrAssertMaxDoc(t, 1, r.MaxDoc())
	iwc := newIndexWriterConfig()
	iwc.SetIndexCommit(r.GetIndexCommit())

	w2 := mustNewIndexWriter(t, dir, iwc)
	iwfrAssertMaxDoc(t, 1, r.MaxDoc())
	mustClose(t, r)

	iwfrAssertMaxDoc(t, 1, iwDocStats(t, w2).MaxDoc)
	mustAddDocument(t, w2, document.NewDocument())
	iwfrAssertMaxDoc(t, 2, iwDocStats(t, w2).MaxDoc)
	mustClose(t, w2)

	r2 := mustOpenDirectoryReader(t, dir)
	iwfrAssertMaxDoc(t, 2, r2.MaxDoc())
	mustClose(t, r2, dir)
}

// TestIndexWriterFromReaderWithNoFirstCommit: pull NRT reader from a writer on
// a new index with no commit.
func TestIndexWriterFromReaderWithNoFirstCommit(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	mustAddDocument(t, w, document.NewDocument())
	r := iwfrOpen(t, w)
	if err := w.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	iwc := newIndexWriterConfig()
	iwc.SetIndexCommit(r.GetIndexCommit())

	err := iwfrExpectIllegalArgument(t, dir, iwc)
	if want := "cannot use IndexWriterConfig.setIndexCommit() when index has no commit"; err.Error() != want {
		t.Fatalf("expected %q, got %q", want, err.Error())
	}

	mustClose(t, r, dir)
}

// TestIndexWriterFromReaderAfterCommitThenIndex: pull NRT reader after writer
// has committed and then indexed another doc.
func TestIndexWriterFromReaderAfterCommitThenIndex(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	mustAddDocument(t, w, document.NewDocument())
	mustCommit(t, w)
	mustAddDocument(t, w, document.NewDocument())

	r := iwfrOpen(t, w)
	iwfrAssertMaxDoc(t, 2, r.MaxDoc())
	mustClose(t, w)

	iwc := newIndexWriterConfig()
	iwc.SetIndexCommit(r.GetIndexCommit())

	err := iwfrExpectIllegalArgument(t, dir, iwc)
	if !strings.Contains(err.Error(), "the provided reader is stale: its prior commit file") {
		t.Fatalf("unexpected message: %q", err.Error())
	}

	mustClose(t, r, dir)
}

// TestIndexWriterFromReaderNRTRollback: pull NRT reader after writer has
// committed and then before indexing another doc.
func TestIndexWriterFromReaderNRTRollback(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	mustAddDocument(t, w, document.NewDocument())
	mustCommit(t, w)

	r := iwfrOpen(t, w)
	iwfrAssertMaxDoc(t, 1, r.MaxDoc())

	// Add another doc
	mustAddDocument(t, w, document.NewDocument())
	iwfrAssertMaxDoc(t, 2, iwDocStats(t, w).MaxDoc)
	mustClose(t, w)

	iwc := newIndexWriterConfig()
	iwc.SetIndexCommit(r.GetIndexCommit())
	err := iwfrExpectIllegalArgument(t, dir, iwc)
	if !strings.Contains(err.Error(), "the provided reader is stale: its prior commit file") {
		t.Fatalf("unexpected message: %q", err.Error())
	}

	mustClose(t, r, dir)
}

func TestIndexWriterFromReaderRandom(t *testing.T) {
	dir := newDirectory()

	numOps := atLeast(100)

	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())

	// We must have a starting commit for this test because whenever we rollback
	// with an NRT reader, the commit before that NRT reader must exist
	mustCommit(t, w)

	r := iwfrOpen(t, w)
	nrtReaderNumDocs := 0
	writerNumDocs := 0

	commitAfterNRT := false

	liveIDs := make(map[int]bool)
	nrtLiveIDs := make(map[int]bool)
	copySet := func(s map[int]bool) map[int]bool {
		out := make(map[int]bool, len(s))
		for k := range s {
			out[k] = true
		}
		return out
	}

	for op := 0; op < numOps; op++ {
		if r.NumDocs() != nrtReaderNumDocs {
			t.Fatalf("op %d: expected %d docs, got %d", op, nrtReaderNumDocs, r.NumDocs())
		}
		switch rand.Intn(5) {
		case 0:
			// add doc
			doc := document.NewDocument()
			doc.Add(newStringField(t, "id", strconv.Itoa(op), false))
			mustAddDocument(t, w, doc)
			liveIDs[op] = true
			writerNumDocs++

		case 1:
			// delete docs
			if len(liveIDs) > 0 {
				id := rand.Intn(op)
				if _, err := w.DeleteDocuments([]index.Term{*index.NewTerm("id", strconv.Itoa(id))}); err != nil {
					t.Fatalf("deleteDocuments: %v", err)
				}
				if liveIDs[id] {
					delete(liveIDs, id)
					writerNumDocs--
				}
			}

		case 2:
			// reopen NRT reader
			r2 := iwfrOpenIfChanged(t, r)
			if r2 != nil {
				mustClose(t, r)
				r = r2
				nrtReaderNumDocs = writerNumDocs
				nrtLiveIDs = copySet(liveIDs)
			} else if r.NumDocs() != nrtReaderNumDocs {
				t.Fatalf("expected %d docs, got %d", nrtReaderNumDocs, r.NumDocs())
			}
			commitAfterNRT = false

		case 3:
			if !commitAfterNRT {
				// rollback writer to last nrt reader
				if rand.Intn(2) == 0 {
					mustClose(t, w, r)
					r = mustOpenDirectoryReader(t, dir)
					if r.NumDocs() != writerNumDocs {
						t.Fatalf("expected %d docs, got %d", writerNumDocs, r.NumDocs())
					}
					nrtReaderNumDocs = writerNumDocs
					nrtLiveIDs = copySet(liveIDs)
				} else if err := w.Rollback(); err != nil {
					t.Fatalf("rollback: %v", err)
				}
				iwc := newIndexWriterConfig()
				iwc.SetIndexCommit(r.GetIndexCommit())
				w = mustNewIndexWriter(t, dir, iwc)
				writerNumDocs = nrtReaderNumDocs
				liveIDs = copySet(nrtLiveIDs)
				mustClose(t, r)
				r = iwfrOpen(t, w)
			}

		case 4:
			mustCommit(t, w)
			commitAfterNRT = true
		}
	}

	mustClose(t, w, r, dir)
}

func TestIndexWriterFromReaderConsistentFieldNumbers(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	// Empty first commit:
	mustCommit(t, w)

	doc := document.NewDocument()
	doc.Add(newStringField(t, "f0", "foo", false))
	mustAddDocument(t, w, doc)

	r := iwfrOpen(t, w)
	iwfrAssertMaxDoc(t, 1, r.MaxDoc())

	doc = document.NewDocument()
	doc.Add(newStringField(t, "f1", "foo", false))
	mustAddDocument(t, w, doc)

	r2 := iwfrOpenIfChanged(t, r)
	if r2 == nil {
		t.Fatal("assertNotNull(r2)")
	}
	mustClose(t, r)
	iwfrAssertMaxDoc(t, 2, r2.MaxDoc())
	if err := w.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	iwc := newIndexWriterConfig()
	iwc.SetIndexCommit(r2.GetIndexCommit())

	w2 := mustNewIndexWriter(t, dir, iwc)
	mustClose(t, r2)

	doc = document.NewDocument()
	doc.Add(newStringField(t, "f1", "foo", false))
	doc.Add(newStringField(t, "f0", "foo", false))
	mustAddDocument(t, w2, doc)
	mustClose(t, w2, dir)
}

func TestIndexWriterFromReaderInvalidOpenMode(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	mustAddDocument(t, w, document.NewDocument())
	mustCommit(t, w)

	r := iwfrOpen(t, w)
	iwfrAssertMaxDoc(t, 1, r.MaxDoc())
	mustClose(t, w)

	iwc := newIndexWriterConfig()
	iwc.SetOpenMode(index.Create)
	iwc.SetIndexCommit(r.GetIndexCommit())
	err := iwfrExpectIllegalArgument(t, dir, iwc)
	if want := "cannot use IndexWriterConfig.setIndexCommit() with OpenMode.CREATE"; err.Error() != want {
		t.Fatalf("expected %q, got %q", want, err.Error())
	}

	mustClose(t, r, dir)
}

func TestIndexWriterFromReaderOnClosedReader(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	mustAddDocument(t, w, document.NewDocument())
	mustCommit(t, w)

	r := iwfrOpen(t, w)
	iwfrAssertMaxDoc(t, 1, r.MaxDoc())
	commit := r.GetIndexCommit()
	mustClose(t, r, w)

	iwc := newIndexWriterConfig()
	iwc.SetIndexCommit(commit)
	w2, err := index.NewIndexWriter(dir, iwc)
	var ace *store.AlreadyClosedException
	if !errors.As(err, &ace) {
		if w2 != nil {
			mustClose(t, w2)
		}
		t.Fatalf("expected AlreadyClosedException, got %v", err)
	}

	mustClose(t, r, dir)
}

func TestIndexWriterFromReaderStaleNRTReader(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	mustAddDocument(t, w, document.NewDocument())
	mustCommit(t, w)

	r := iwfrOpen(t, w)
	iwfrAssertMaxDoc(t, 1, r.MaxDoc())
	mustAddDocument(t, w, document.NewDocument())

	r2 := iwfrOpenIfChanged(t, r)
	if r2 == nil {
		t.Fatal("assertNotNull(r2)")
	}
	mustClose(t, r2)
	if err := w.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	iwc := newIndexWriterConfig()
	iwc.SetIndexCommit(r.GetIndexCommit())
	w = mustNewIndexWriter(t, dir, iwc)
	if n := iwDocStats(t, w).NumDocs; n != 1 {
		t.Fatalf("expected numDocs 1, got %d", n)
	}

	mustClose(t, r)
	r3 := iwfrOpen(t, w)
	if r3.NumDocs() != 1 {
		t.Fatalf("expected 1 doc, got %d", r3.NumDocs())
	}

	mustAddDocument(t, w, document.NewDocument())
	r4 := iwfrOpenIfChanged(t, r3)
	mustClose(t, r3)
	if r4 == nil || r4.NumDocs() != 2 {
		t.Fatalf("expected 2 docs, got %v", r4)
	}
	mustClose(t, r4, w)

	mustClose(t, r, dir)
}

func TestIndexWriterFromReaderAfterRollback(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	mustAddDocument(t, w, document.NewDocument())
	mustCommit(t, w)
	mustAddDocument(t, w, document.NewDocument())

	r := iwfrOpen(t, w)
	iwfrAssertMaxDoc(t, 2, r.MaxDoc())
	if err := w.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	iwc := newIndexWriterConfig()
	iwc.SetIndexCommit(r.GetIndexCommit())
	w = mustNewIndexWriter(t, dir, iwc)
	if n := iwDocStats(t, w).NumDocs; n != 2 {
		t.Fatalf("expected numDocs 2, got %d", n)
	}

	mustClose(t, r, w)

	r2 := mustOpenDirectoryReader(t, dir)
	if r2.NumDocs() != 2 {
		t.Fatalf("expected 2 docs, got %d", r2.NumDocs())
	}
	mustClose(t, r2, dir)
}

// keepAllCommitsPolicy renders the anonymous IndexDeletionPolicy whose onInit
// and onCommit do nothing (keep all commits).
type keepAllCommitsPolicy struct{}

func (keepAllCommitsPolicy) OnInit([]index.Commit) error        { return nil }
func (keepAllCommitsPolicy) OnCommit([]index.Commit) error      { return nil }
func (p keepAllCommitsPolicy) Clone() index.IndexDeletionPolicy { return p }

// TestIndexWriterFromReaderAfterCommitThenIndexKeepCommits: pull NRT reader
// after writer has committed and then indexed another doc.
func TestIndexWriterFromReaderAfterCommitThenIndexKeepCommits(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()

	// Keep all commits:
	iwc.SetIndexDeletionPolicy(keepAllCommitsPolicy{})

	w := mustNewIndexWriter(t, dir, iwc)
	mustAddDocument(t, w, document.NewDocument())
	mustCommit(t, w)
	mustAddDocument(t, w, document.NewDocument())

	r := iwfrOpen(t, w)
	iwfrAssertMaxDoc(t, 2, r.MaxDoc())
	mustAddDocument(t, w, document.NewDocument())

	r2 := iwfrOpen(t, w)
	iwfrAssertMaxDoc(t, 3, r2.MaxDoc())
	mustClose(t, r2, w)

	// r is not stale because, even though we've committed the original writer
	// since it was open, we are keeping all commit points:
	iwc = newIndexWriterConfig()
	iwc.SetIndexCommit(r.GetIndexCommit())
	w2 := mustNewIndexWriter(t, dir, iwc)
	iwfrAssertMaxDoc(t, 2, iwDocStats(t, w2).MaxDoc)
	mustClose(t, r, w2, dir)
}
