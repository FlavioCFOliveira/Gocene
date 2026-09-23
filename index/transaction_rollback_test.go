// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestTransactionRollback.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const transactionRollbackFieldRecordID = "record_id"

// transactionRollbackRollBackLast rolls back index to a chosen ID.
func transactionRollbackRollBackLast(t *testing.T, dir store.Directory, id int) {
	t.Helper()
	ids := "-" + strconv.Itoa(id)
	var last *index.IndexCommit
	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("listCommits: %v", err)
	}
	for _, commit := range commits {
		ud := commit.GetUserData()
		if len(ud) > 0 {
			if strings.HasSuffix(ud["index"], ids) {
				last = commit
			}
		}
	}

	if last == nil {
		t.Fatalf("Couldn't find commit point %d", id)
	}

	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetIndexDeletionPolicy(&rollbackDeletionPolicy{rollbackPoint: id})
	iwc.SetIndexCommit(last)
	w := mustNewIndexWriter(t, dir, iwc)
	data := map[string]string{"index": "Rolled back to 1-" + strconv.Itoa(id)}
	w.SetLiveCommitData(maps.All(data))
	mustClose(t, w)
}

func TestTransactionRollbackRepeatedRollBacks(t *testing.T) {
	dir := transactionRollbackSetUp(t)
	defer mustClose(t, dir) // tearDown()

	expectedLastRecordID := 100
	for expectedLastRecordID > 10 {
		expectedLastRecordID -= 10
		transactionRollbackRollBackLast(t, dir, expectedLastRecordID)

		expecteds := make(map[int]bool)
		for i := 1; i < expectedLastRecordID+1; i++ {
			expecteds[i] = true
		}
		transactionRollbackCheckExpecteds(t, dir, expecteds)
	}
}

func transactionRollbackCheckExpecteds(t *testing.T, dir store.Directory, expecteds map[int]bool) {
	t.Helper()
	r := mustOpenDirectoryReader(t, dir)

	// Perhaps not the most efficient approach but meets our needs here.
	liveDocs, err := index.MultiBitsGetLiveDocs(r)
	if err != nil {
		t.Fatalf("MultiBits.getLiveDocs: %v", err)
	}
	storedFields, err := r.StoredFields()
	if err != nil {
		t.Fatalf("storedFields: %v", err)
	}
	for i := 0; i < r.MaxDoc(); i++ {
		if liveDocs == nil || liveDocs.Get(i) {
			sval := docGet(storedDocument(t, storedFields, i), transactionRollbackFieldRecordID)
			if sval != nil {
				val, err := strconv.Atoi(*sval)
				if err != nil {
					t.Fatalf("parseInt(%q): %v", *sval, err)
				}
				if !expecteds[val] {
					t.Fatalf("Did not expect document #%d", val)
				}
				delete(expecteds, val)
			}
		}
	}
	mustClose(t, r)
	if len(expecteds) != 0 {
		t.Fatalf("Should have 0 docs remaining : %d", len(expecteds))
	}
}

// transactionRollbackSetUp ports setUp(): builds an index of records 1 to 100,
// committing after each batch of 10.
func transactionRollbackSetUp(t *testing.T) *store.MockDirectoryWrapper {
	t.Helper()
	dir := newDirectory()

	sdp := keepAllDeletionPolicy{}
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetIndexDeletionPolicy(sdp)
	w := mustNewIndexWriter(t, dir, iwc)

	for currentRecordID := 1; currentRecordID <= 100; currentRecordID++ {
		doc := document.NewDocument()
		doc.Add(newTextField(t, transactionRollbackFieldRecordID, strconv.Itoa(currentRecordID), true))
		mustAddDocument(t, w, doc)

		if currentRecordID%10 == 0 {
			data := map[string]string{"index": fmt.Sprintf("records 1-%d", currentRecordID)}
			w.SetLiveCommitData(maps.All(data))
			mustCommit(t, w)
		}
	}

	mustClose(t, w)
	return dir
}

// rollbackDeletionPolicy ports RollbackDeletionPolicy: rolls back to previous
// commit point.
type rollbackDeletionPolicy struct {
	rollbackPoint int
}

func (p *rollbackDeletionPolicy) OnCommit([]index.Commit) error { return nil }

func (p *rollbackDeletionPolicy) OnInit(commits []index.Commit) error {
	for _, commit := range commits {
		userData := commit.GetUserData()
		if len(userData) > 0 {
			// Label for a commit point is "Records 1-30"
			// This code reads the last id ("30" in this example) and deletes it
			// if it is after the desired rollback point
			x := userData["index"]
			lastVal := x[strings.LastIndex(x, "-")+1:]
			last, err := strconv.Atoi(lastVal)
			if err != nil {
				return err
			}
			if last > p.rollbackPoint {
				if err := commit.Delete(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (p *rollbackDeletionPolicy) Clone() index.IndexDeletionPolicy {
	return &rollbackDeletionPolicy{rollbackPoint: p.rollbackPoint}
}

// deleteLastCommitPolicy ports DeleteLastCommitPolicy.
type deleteLastCommitPolicy struct{}

func (deleteLastCommitPolicy) OnCommit([]index.Commit) error { return nil }

func (deleteLastCommitPolicy) OnInit(commits []index.Commit) error {
	return commits[len(commits)-1].Delete()
}

func (p deleteLastCommitPolicy) Clone() index.IndexDeletionPolicy { return p }

func TestTransactionRollbackRollbackDeletionPolicy(t *testing.T) {
	dir := transactionRollbackSetUp(t)
	defer mustClose(t, dir) // tearDown()

	for i := 0; i < 2; i++ {
		// Unless you specify a prior commit point, rollback should not work:
		iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		iwc.SetIndexDeletionPolicy(deleteLastCommitPolicy{})
		mustClose(t, mustNewIndexWriter(t, dir, iwc))
		r := mustOpenDirectoryReader(t, dir)
		if r.NumDocs() != 100 {
			t.Fatalf("expected 100 docs, got %d", r.NumDocs())
		}
		mustClose(t, r)
	}
}

// keepAllDeletionPolicy ports KeepAllDeletionPolicy: keeps all commit points
// (used to build index).
type keepAllDeletionPolicy struct{}

func (keepAllDeletionPolicy) OnCommit([]index.Commit) error      { return nil }
func (keepAllDeletionPolicy) OnInit([]index.Commit) error        { return nil }
func (p keepAllDeletionPolicy) Clone() index.IndexDeletionPolicy { return p }
