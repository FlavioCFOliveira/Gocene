// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/join/src/test/org/apache/lucene/search/join/TestCheckJoinIndex.java
// (Apache Lucene 10.5.0).

package join

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// expectIllegalStateCheck renders
// expectThrows(IllegalStateException.class, () -> CheckJoinIndex.check(reader, parentsFilter)).
func expectIllegalStateCheck(t *testing.T, reader index.IndexReaderInterface, parentsFilter BitSetProducer) {
	t.Helper()
	if err := Check(reader, parentsFilter); err == nil {
		t.Fatal("expected IllegalStateException")
	}
}

func TestCheckJoinIndexNoParent(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	numDocs := nextInt(1, 3)
	for i := 0; i < numDocs; i++ {
		mustAddDocument(t, w, document.NewDocument())
	}
	reader := mustGetReader(t, w)
	mustClose(t, w)
	parentsFilter := NewQueryBitSetProducer(search.MatchNoDocsQueryInstance)
	defer mustClose(t, reader, dir)
	expectIllegalStateCheck(t, reader, parentsFilter)
}

func TestCheckJoinIndexOrphans(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	{
		// Add a first valid block
		var block []*document.Document
		numChildren := nextInt(0, 3)
		for i := 0; i < numChildren; i++ {
			block = append(block, document.NewDocument())
		}
		parent := document.NewDocument()
		parent.Add(newStringField(t, "parent", "true", false))
		block = append(block, parent)
		if _, err := w.AddDocuments(block); err != nil {
			t.Fatal(err)
		}
	}

	{
		// Then a block with no parent
		var block []*document.Document
		numChildren := nextInt(1, 3)
		for i := 0; i < numChildren; i++ {
			block = append(block, document.NewDocument())
		}
		if _, err := w.AddDocuments(block); err != nil {
			t.Fatal(err)
		}
	}

	reader := mustGetReader(t, w)
	mustClose(t, w)
	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("parent", "true")))
	defer mustClose(t, reader, dir)
	expectIllegalStateCheck(t, reader, parentsFilter)
}

func TestCheckJoinIndexInconsistentDeletes(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(index.NewNoMergePolicy()) // NoMergePolicy.INSTANCE, so that deletions don't trigger merges
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	var block []*document.Document
	numChildren := nextInt(1, 3)
	for i := 0; i < numChildren; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "child", strconv.Itoa(i), false))
		block = append(block, doc)
	}
	parent := document.NewDocument()
	parent.Add(newStringField(t, "parent", "true", false))
	block = append(block, parent)
	if _, err := w.AddDocuments(block); err != nil {
		t.Fatal(err)
	}

	var err error
	if random().Intn(2) == 0 {
		_, err = w.DeleteDocuments(index.NewTerm("parent", "true"))
	} else {
		// delete any of the children
		_, err = w.DeleteDocuments(index.NewTerm("child", strconv.Itoa(random().Intn(numChildren))))
	}
	if err != nil {
		t.Fatal(err)
	}

	reader := mustGetReader(t, w)
	mustClose(t, w)

	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("parent", "true")))
	defer mustClose(t, reader, dir)
	expectIllegalStateCheck(t, reader, parentsFilter)
}
