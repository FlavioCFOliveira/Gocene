// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Port of
// lucene/join/src/test/org/apache/lucene/search/join/TestBlockJoinValidation.java
// (Apache Lucene 10.5.0). Its setUp/tearDown pair is rendered as
// setUpBlockJoinValidation, whose returned fixture every test closes.

const (
	amountOfSegments      = 5
	amountOfParentDocs    = 10
	amountOfChildDocs     = 5
	amountOfDocsInSegment = amountOfParentDocs + amountOfParentDocs*amountOfChildDocs
)

// blockJoinValidationFixture holds the instance fields set up by setUp().
type blockJoinValidationFixture struct {
	directory     store.Directory
	indexReader   index.IndexReaderInterface
	indexSearcher *search.IndexSearcher
	parentsFilter BitSetProducer
}

// setUpBlockJoinValidation renders setUp(); the returned fixture's tearDown
// is registered with t.Cleanup.
func setUpBlockJoinValidation(t *testing.T) *blockJoinValidationFixture {
	t.Helper()
	f := &blockJoinValidationFixture{}
	f.directory = newDirectory()
	config := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	config.SetMergePolicy(newMergePolicyNoMock(t))
	indexWriter := mustNewIndexWriter(t, f.directory, config)
	for i := 0; i < amountOfSegments; i++ {
		segmentDocs := createDocsForSegment(t, i)
		mustAddDocuments(t, indexWriter, segmentDocs...)
		mustCommit(t, indexWriter)
	}
	f.indexReader = mustOpenDirectoryReaderFromWriter(t, indexWriter)
	mustClose(t, indexWriter)
	f.indexSearcher = search.NewIndexSearcher(f.indexReader)
	f.parentsFilter = NewQueryBitSetProducer(search.NewWildcardQuery(index.NewTerm("parent", "*")))
	t.Cleanup(func() { f.tearDown(t) })
	return f
}

// tearDown renders tearDown().
func (f *blockJoinValidationFixture) tearDown(t *testing.T) {
	mustClose(t, f.indexReader, f.directory)
}

func TestBlockJoinValidationNextDocValidationForToParentBjq(t *testing.T) {
	f := setUpBlockJoinValidation(t)
	// TODO: This test is broken when score mode is None because BlockJoinScorer#scoreChildDocs does
	// not advance the child approximation. Adjust this test once that is fixed.
	validScoreModes := []ScoreMode{Avg, Max, Total, Min}
	parentQueryWithRandomChild := createChildrenQueryWithOneParent(getRandomChildNumber(0))
	blockJoinQuery := NewToParentBlockJoinQuery(parentQueryWithRandomChild, f.parentsFilter,
		validScoreModes[random().Intn(len(validScoreModes))])
	_, err := f.indexSearcher.Search(blockJoinQuery, 1)
	if err == nil {
		t.Fatal("expected IllegalStateException")
	}
	if !strings.Contains(err.Error(), "Child query must not match same docs with parent filter") {
		t.Fatalf("unexpected message: %v", err)
	}
}

func TestBlockJoinValidationNextDocValidationForToChildBjq(t *testing.T) {
	f := setUpBlockJoinValidation(t)
	parentQueryWithRandomChild := createParentsQueryWithOneChild(getRandomChildNumber(0))

	blockJoinQuery := NewToChildBlockJoinQuery(parentQueryWithRandomChild, f.parentsFilter)

	_, err := f.indexSearcher.Search(blockJoinQuery, 1)
	if err == nil {
		t.Fatal("expected IllegalStateException")
	}
	if !strings.Contains(err.Error(), invalidQueryMessage) {
		t.Fatalf("unexpected message: %v", err)
	}
}

func TestBlockJoinValidationAdvanceValidationForToChildBjq(t *testing.T) {
	f := setUpBlockJoinValidation(t)
	parentQuery := search.Instance
	blockJoinQuery := NewToChildBlockJoinQuery(parentQuery, f.parentsFilter)

	context := mustLeaves(t, f.indexSearcher.GetIndexReader())[0]
	weight := mustCreateWeight(t, f.indexSearcher, mustRewrite(t, f.indexSearcher, blockJoinQuery), search.COMPLETE, 1)
	scorer := mustScorer(t, weight, context)
	parentDocs, err := f.parentsFilter.GetBitSet(context)
	if err != nil {
		t.Fatal(err)
	}

	var target int
	for {
		// make the parent scorer advance to a doc ID which is not a parent
		target = nextInt(0, context.LeafReader().MaxDoc()-2)
		if !parentDocs.Get(target + 1) {
			break
		}
	}

	illegalTarget := target
	_, err = scorer.Iterator().Advance(illegalTarget)
	if err == nil {
		t.Fatal("expected IllegalStateException")
	}
	if !strings.Contains(err.Error(), invalidQueryMessage) {
		t.Fatalf("unexpected message: %v", err)
	}
}

func createDocsForSegment(t testing.TB, segmentNumber int) []*document.Document {
	blocks := make([][]*document.Document, 0, amountOfParentDocs)
	for i := 0; i < amountOfParentDocs; i++ {
		blocks = append(blocks, createParentDocWithChildren(t, segmentNumber, i))
	}
	result := make([]*document.Document, 0, amountOfDocsInSegment)
	for _, block := range blocks {
		result = append(result, block...)
	}
	return result
}

func createParentDocWithChildren(t testing.TB, segmentNumber, parentNumber int) []*document.Document {
	result := make([]*document.Document, 0, amountOfChildDocs+1)
	for i := 0; i < amountOfChildDocs; i++ {
		result = append(result, createChildDoc(t, segmentNumber, parentNumber, i))
	}
	result = append(result, createParentDoc(t, segmentNumber, parentNumber))
	return result
}

func createParentDoc(t testing.TB, segmentNumber, parentNumber int) *document.Document {
	return newTestDocument(
		newStringField(t, "id", createFieldValue(segmentNumber*amountOfParentDocs+parentNumber), true),
		newStringField(t, "parent", createFieldValue(parentNumber), false),
		newStringField(t, "common_field", "1", false))
}

func createChildDoc(t testing.TB, segmentNumber, parentNumber, childNumber int) *document.Document {
	return newTestDocument(
		newStringField(t, "id", createFieldValue(segmentNumber*amountOfParentDocs+parentNumber, childNumber), true),
		newStringField(t, "child", createFieldValue(childNumber), false),
		newStringField(t, "common_field", "1", false))
}

func createFieldValue(documentNumbers ...int) string {
	var sb strings.Builder
	for _, documentNumber := range documentNumbers {
		if sb.Len() > 0 {
			sb.WriteString("_")
		}
		sb.WriteString(strconv.Itoa(documentNumber))
	}
	return sb.String()
}

func createChildrenQueryWithOneParent(childNumber int) search.Query {
	childQuery := search.NewTermQuery(index.NewTerm("child", createFieldValue(childNumber)))
	randomParentQuery := search.NewTermQuery(index.NewTerm("id", createFieldValue(getRandomParentID())))
	childrenQueryWithRandomParent := search.NewBooleanQueryBuilder()
	childrenQueryWithRandomParent.Add(childQuery, search.SHOULD)
	childrenQueryWithRandomParent.Add(randomParentQuery, search.SHOULD)
	return childrenQueryWithRandomParent.Build()
}

func createParentsQueryWithOneChild(randomChildNumber int) search.Query {
	childQueryWithRandomParent := search.NewBooleanQueryBuilder()
	parentsQuery := search.NewTermQuery(index.NewTerm("parent", createFieldValue(getRandomParentNumber())))
	childQueryWithRandomParent.Add(parentsQuery, search.SHOULD)
	childQueryWithRandomParent.Add(validationRandomChildQuery(randomChildNumber), search.SHOULD)
	return childQueryWithRandomParent.Build()
}

func getRandomParentID() int {
	return random().Intn(amountOfParentDocs * amountOfSegments)
}

func getRandomParentNumber() int {
	return random().Intn(amountOfParentDocs)
}

// validationRandomChildQuery renders TestBlockJoinValidation.randomChildQuery(int).
func validationRandomChildQuery(randomChildNumber int) search.Query {
	return search.NewTermQuery(index.NewTerm("id", createFieldValue(getRandomParentID(), randomChildNumber)))
}

func getRandomChildNumber(notLessThan int) int {
	return notLessThan + random().Intn(amountOfChildDocs-notLessThan)
}
