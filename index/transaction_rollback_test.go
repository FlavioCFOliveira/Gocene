// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for multi-level transaction rollback via
// IndexDeletionPolicy.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestTransactionRollback.java
package index_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const transactionFieldRecordID = "record_id"

// transactionBuildIndex creates 100 documents, committing after every 10 with
// user data "index" -> "records 1-N".  It mirrors the Java setUp() method.
func transactionBuildIndex(t *testing.T, dir store.Directory) {
	t.Helper()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetIndexDeletionPolicy(index.NewKeepAllDeletionPolicy())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	for id := 1; id <= 100; id++ {
		doc := document.NewDocument()
		f, err := document.NewStringField(transactionFieldRecordID, fmt.Sprintf("%d", id), true)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", id, err)
		}
		if id%10 == 0 {
			w.SetLiveCommitData(map[string]string{"index": fmt.Sprintf("records 1-%d", id)})
			if err := w.Commit(); err != nil {
				t.Fatalf("Commit %d: %v", id, err)
			}
		}
	}
}

// transactionRollBackLast reopens the index at the commit whose user data
// ends with "-<id>" and deletes every newer commit via RollbackDeletionPolicy.
func transactionRollBackLast(t *testing.T, dir store.Directory, id int) {
	t.Helper()
	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	var target *index.IndexCommit
	suffix := "-" + strconv.Itoa(id)
	for _, c := range commits {
		if strings.HasSuffix(c.GetUserData()["index"], suffix) {
			target = c
		}
	}
	if target == nil {
		t.Fatalf("commit point for %d not found", id)
	}

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetIndexDeletionPolicy(&rollbackDeletionPolicy{rollbackPoint: id})
	cfg.SetIndexCommit(target)
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter at commit %d: %v", id, err)
	}
	w.SetLiveCommitData(map[string]string{"index": fmt.Sprintf("Rolled back to 1-%d", id)})
	if err := w.Close(); err != nil {
		t.Fatalf("Close after rollback to %d: %v", id, err)
	}
}

// transactionCheckExpecteds verifies that exactly the record IDs in [1,last]
// are present and live.
func transactionCheckExpecteds(t *testing.T, dir store.Directory, last int) {
	t.Helper()
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	seen := make(map[int]struct{}, last)
	maxDoc := reader.MaxDoc()
	sf, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("StoredFields: %v", err)
	}
	for docID := 0; docID < maxDoc; docID++ {
		v := &recordIDVisitor{}
		if err := sf.Document(docID, v); err != nil {
			t.Fatalf("Document(%d): %v", docID, err)
		}
		if v.value == "" {
			continue
		}
		id, err := strconv.Atoi(v.value)
		if err != nil {
			t.Fatalf("invalid record_id %q: %v", v.value, err)
		}
		if id < 1 || id > last {
			t.Fatalf("unexpected record_id %d (expected 1..%d)", id, last)
		}
		if _, ok := seen[id]; ok {
			t.Fatalf("record_id %d seen more than once", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != last {
		t.Fatalf("expected %d live records, got %d", last, len(seen))
	}
}

// recordIDVisitor collects the "record_id" stored field value.
type recordIDVisitor struct {
	value string
}

func (v *recordIDVisitor) StringField(field string, value string) {
	if field == transactionFieldRecordID {
		v.value = value
	}
}

func (v *recordIDVisitor) BinaryField(field string, value []byte) {}
func (v *recordIDVisitor) IntField(field string, value int)        {}
func (v *recordIDVisitor) LongField(field string, value int64)      {}
func (v *recordIDVisitor) FloatField(field string, value float32)  {}
func (v *recordIDVisitor) DoubleField(field string, value float64) {}

// rollbackDeletionPolicy deletes every commit whose "index" user data ends
// with a record ID larger than the rollback point.
type rollbackDeletionPolicy struct {
	rollbackPoint int
}

func (p *rollbackDeletionPolicy) OnInit(commits []*index.IndexCommit) error {
	for _, c := range commits {
		ud := c.GetUserData()
		if len(ud) == 0 {
			continue
		}
		x := ud["index"]
		if x == "" {
			continue
		}
		idx := strings.LastIndex(x, "-")
		if idx < 0 {
			continue
		}
		last, err := strconv.Atoi(x[idx+1:])
		if err != nil {
			continue
		}
		if last > p.rollbackPoint {
			if err := c.Delete(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *rollbackDeletionPolicy) OnCommit(commits []*index.IndexCommit) error { return nil }
func (p *rollbackDeletionPolicy) Clone() index.IndexDeletionPolicy            { return &rollbackDeletionPolicy{rollbackPoint: p.rollbackPoint} }

// deleteLastCommitPolicy deletes the most recent commit on init, verifying
// that reopening at the prior commit preserves all earlier documents.
type deleteLastCommitPolicy struct{}

func (deleteLastCommitPolicy) OnInit(commits []*index.IndexCommit) error {
	if len(commits) == 0 {
		return nil
	}
	return commits[len(commits)-1].Delete()
}

func (deleteLastCommitPolicy) OnCommit(commits []*index.IndexCommit) error { return nil }
func (deleteLastCommitPolicy) Clone() index.IndexDeletionPolicy            { return deleteLastCommitPolicy{} }

// TestTransactionRollback_RepeatedRollBacks ports testRepeatedRollBacks().
func TestTransactionRollback_RepeatedRollBacks(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	transactionBuildIndex(t, dir)

	for last := 100; last > 10; last -= 10 {
		transactionRollBackLast(t, dir, last)
		transactionCheckExpecteds(t, dir, last)
	}
}

// TestTransactionRollback_RollbackDeletionPolicy ports testRollbackDeletionPolicy().
func TestTransactionRollback_RollbackDeletionPolicy(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	transactionBuildIndex(t, dir)

	for i := 0; i < 2; i++ {
		cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		cfg.SetIndexDeletionPolicy(deleteLastCommitPolicy{})
		w, err := index.NewIndexWriter(dir, cfg)
		if err != nil {
			t.Fatalf("NewIndexWriter iteration %d: %v", i, err)
		}
		// Gocene's Commit() only writes a new generation when there are pending
		// mutations or changed live commit data.  The Java test relies on
		// commitOnClose writing a fresh commit even when nothing changed.  We
		// emulate that by stamping distinct live commit data before closing.
		w.SetLiveCommitData(map[string]string{"iteration": fmt.Sprintf("%d", i)})
		if err := w.Close(); err != nil {
			t.Fatalf("Close iteration %d: %v", i, err)
		}

		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("OpenDirectoryReader iteration %d: %v", i, err)
		}
		if got := reader.NumDocs(); got != 100 {
			t.Fatalf("iteration %d: NumDocs = %d, want 100", i, got)
		}
		reader.Close()
	}
}
