// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
)

// Port of
// lucene/join/src/test/org/apache/lucene/search/join/TestParentsChildrenBlockJoinQuery.java
// (Apache Lucene 10.5.0).

func createParentsChildrenIndexWriter(t testing.TB, dir store.Directory) *testindex.RandomIndexWriter {
	t.Helper()
	// We need a merge policy that merges segments sequentially.
	// Most tests here merge down to a single segment and assume the order of documents in the
	// segment
	// matches the order in which they were added.
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newLogMergePolicy())
	return newRandomIndexWriterWithConfig(t, dir, iwc)
}

func TestParentsChildrenBlockJoinQueryEmptyIndex(t *testing.T) {
	// No documents to index, just test the query execution
	testParentsChildren(t, [][]*pcTestDoc{}, []int{}, 10)
}

func TestParentsChildrenBlockJoinQueryOnlyParentDocs(t *testing.T) {
	// Only parent documents, no children
	blocks := [][]*pcTestDoc{
		{newPCTestDoc("parent", true, 0)},
		{newPCTestDoc("parent", true, 1)},
		{newPCTestDoc("parent", true, 2)},
	}
	expectedDocIDs := []int{}
	testParentsChildren(t, blocks, expectedDocIDs, 10)
}

func TestParentsChildrenBlockJoinQueryFirstParentWithoutChild(t *testing.T) {
	// First parent has no children, but the second parent has two children
	blocks := [][]*pcTestDoc{
		{newPCTestDoc("parent", true, 0)},
		{newPCTestDoc("child", true, 1), newPCTestDoc("child", true, 2), newPCTestDoc("parent", true, 3)},
	}
	expectedDocIDs := []int{1, 2}
	testParentsChildren(t, blocks, expectedDocIDs, 10)
}

func TestParentsChildrenBlockJoinQueryWithRandomizedIndex(t *testing.T) {
	for i := 0; i < 10; i++ {
		// Run multiple iterations to ensure randomness
		if verbose {
			fmt.Printf("Running randomized test iteration: %d\n", i)
		}
		runParentsChildrenRandomizedTest(t)
	}
}

func runParentsChildrenRandomizedTest(t *testing.T) {
	// Random child limit between 1 and 10
	childLimitPerParent := 1 + random().Intn(10)

	// Create at least 100 parents
	numParents := atLeast(100)
	docID := 0

	// Array to store test documents: each parent and its children
	testDocs := make([][]*pcTestDoc, numParents)

	// List to store expected matching document IDs
	expectedMatches := make([]int, 0)
	for parentIdx := 0; parentIdx < numParents; parentIdx++ {
		// Randomly decide if parent matches
		matchingParent := random().Intn(2) == 0

		// Random number of children (0-19)
		numChildren := random().Intn(20)
		testDocs[parentIdx] = make([]*pcTestDoc, numChildren+1) // +1 for parent

		matchingChildrenCount := 0

		// Create children
		for childIdx := 0; childIdx < numChildren; childIdx++ {
			// Randomly decide if child matches
			matchingChild := random().Intn(2) == 0
			testDocs[parentIdx][childIdx] = newPCTestDoc("child", matchingChild, docID)

			// If both parent and child match, and we haven't exceeded child limit
			if matchingChild && matchingParent {
				matchingChildrenCount++
				if matchingChildrenCount <= childLimitPerParent {
					expectedMatches = append(expectedMatches, docID)
				}
			}
			docID++
		}

		// Add parent document
		testDocs[parentIdx][numChildren] = newPCTestDoc("parent", matchingParent, docID)
		docID++
	}

	// Convert expected matches to array and run test
	testParentsChildren(t, testDocs, expectedMatches, childLimitPerParent)
}

// indexParentsChildrenBlocks renders the shared indexing prologue of
// testAdvance, test, testExplain and testIntraSegmentConcurrencyNotSupported.
func indexParentsChildrenBlocks(t testing.TB, blocks [][]*pcTestDoc) (store.Directory, *index.DirectoryReader) {
	t.Helper()
	dir := newDirectory()
	writer := createParentsChildrenIndexWriter(t, dir)

	// Add documents
	docs := make([]*document.Document, 0)
	for _, block := range blocks {
		for _, doc := range block {
			docs = append(docs, doc.toDocument(t))
		}
		if _, err := writer.AddDocuments(docs); err != nil {
			t.Fatalf("addDocuments: %v", err)
		}
		docs = docs[:0]
	}
	mustCommit(t, writer)
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	reader := mustGetReader(t, writer)
	mustClose(t, writer)
	return dir, reader
}

// parentsChildrenQueries renders the parentFilter, parentQuery and childQuery
// every test of the class builds.
func parentsChildrenQueries(t testing.TB, reader index.IndexReaderInterface) (BitSetProducer, search.Query, search.Query) {
	t.Helper()
	parentFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("type", "parent")))
	if err := Check(reader, parentFilter); err != nil {
		t.Fatalf("CheckJoinIndex.check: %v", err)
	}
	parentQuery := search.NewBooleanQueryBuilder().
		Add(search.NewTermQuery(index.NewTerm("value", "match")), search.MUST).
		Add(search.NewTermQuery(index.NewTerm("type", "parent")), search.MUST).
		Build()
	childQuery := search.NewBooleanQueryBuilder().
		Add(search.NewTermQuery(index.NewTerm("value", "match")), search.MUST).
		Add(search.NewTermQuery(index.NewTerm("type", "child")), search.MUST).
		Build()
	return parentFilter, parentQuery, childQuery
}

func TestParentsChildrenBlockJoinQueryAdvance(t *testing.T) {
	// Create test blocks with specific structure to test advance()
	blocks := make([][]*pcTestDoc, 3)

	// Block 0: 2 children + 1 parent (parent matches)
	blocks[0] = []*pcTestDoc{
		newPCTestDoc("child", true, 0),  // docId=0
		newPCTestDoc("child", true, 1),  // docId=1
		newPCTestDoc("parent", true, 2), // docId=2
	}

	// Block 1: 3 children + 1 parent (parent matches)
	blocks[1] = []*pcTestDoc{
		newPCTestDoc("child", true, 3),  // docId=3
		newPCTestDoc("child", true, 4),  // docId=4
		newPCTestDoc("child", true, 5),  // docId=5
		newPCTestDoc("parent", true, 6), // docId=6
	}

	// Block 2: 2 children + 1 parent (parent matches)
	blocks[2] = []*pcTestDoc{
		newPCTestDoc("child", true, 7),  // docId=7
		newPCTestDoc("child", true, 8),  // docId=8
		newPCTestDoc("parent", true, 9), // docId=9
	}

	dir, reader := indexParentsChildrenBlocks(t, blocks)

	// Do not use Concurrency.INTRA_SEGMENT as that would break childLimitPerParent counting
	searcher := newSearcherWithConcurrency(t, reader, false, false)
	parentFilter, parentQuery, childQuery := parentsChildrenQueries(t, reader)

	query := NewParentsChildrenBlockJoinQuery(parentFilter, parentQuery, childQuery, 2)

	// Test advance() functionality
	weight := mustCreateWeight(t, searcher, mustRewrite(t, searcher, query), search.COMPLETE, 1)
	scorer := mustScorer(t, weight, mustLeaves(t, reader)[0])
	if scorer == nil {
		t.Fatal("scorer != null")
	}
	it := scorer.Iterator()

	// Test advance to doc 3 (should skip to first child of second parent)
	assertAdvance(t, it, 3, 3)

	// Test advance to doc 7 (should skip to first child of third parent)
	assertAdvance(t, it, 7, 7)

	// Test advance beyond last document
	assertAdvance(t, it, 10, search.NO_MORE_DOCS)

	mustClose(t, reader, dir)
}

func assertAdvance(t testing.TB, it search.DocIdSetIterator, target, expected int) {
	t.Helper()
	got, err := it.Advance(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != expected {
		t.Fatalf("advance(%d): expected %d, got %d", target, expected, got)
	}
}

func testParentsChildren(t *testing.T, blocks [][]*pcTestDoc, expectedDocIDs []int, childLimitPerParent int) {
	t.Helper()
	dir, reader := indexParentsChildrenBlocks(t, blocks)

	// Do not use Concurrency.INTRA_SEGMENT as that would break childLimitPerParent counting
	searcher := newSearcherWithConcurrency(t, reader, false, false)
	parentFilter, parentQuery, childQuery := parentsChildrenQueries(t, reader)

	query := NewParentsChildrenBlockJoinQuery(parentFilter, parentQuery, childQuery, childLimitPerParent)

	results := mustSearch(t, searcher, query, (len(expectedDocIDs)+1)*2)

	failed := func(format string, args ...any) {
		t.Helper()
		if verbose {
			fmt.Println("Test failed. Document structure:")
			fmt.Println(visualizeTestDocs(blocks))
			fmt.Printf("Child limit per parent: %d\n", childLimitPerParent)
			fmt.Printf("Expected docIds: %v\n", expectedDocIDs)
			fmt.Printf("Actual hits: %d\n", results.TotalHits.Value)
		}
		t.Fatalf(format, args...)
	}

	if int64(len(expectedDocIDs)) != results.TotalHits.Value {
		failed("totalHits: expected %d, got %d", len(expectedDocIDs), results.TotalHits.Value)
	}
	expectedDocIDSet := map[int]struct{}{}
	for _, id := range expectedDocIDs {
		expectedDocIDSet[id] = struct{}{}
	}

	// Verify the matching documents
	storedFields := mustStoredFields(t, reader)
	for _, scoreDoc := range results.ScoreDocs {
		doc := storedDocument(t, storedFields, scoreDoc.Doc)
		typeField := doc.Get("type")
		if typeField == nil {
			failed("doc %d has no type field", scoreDoc.Doc)
		}
		idField := doc.Get("ID")
		if idField == nil {
			failed("doc %d has no ID field", scoreDoc.Doc)
		}
		id := numericIntValue(t, idField.NumericValue())
		if typeField.StringValue() != "child" { // All results should be children
			failed("expected a child, got %q", typeField.StringValue())
		}
		if _, ok := expectedDocIDSet[id]; !ok {
			failed("unexpected docId %d", id)
		}
	}

	mustClose(t, reader, dir)
}

// numericIntValue renders Number.intValue() over the boxed stored numeric.
func numericIntValue(t testing.TB, v any) int {
	t.Helper()
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	}
	t.Fatalf("not a number: %T", v)
	return 0
}

func TestParentsChildrenBlockJoinQueryInvalidChildLimit(t *testing.T) {
	parentFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("type", "parent")))
	parentQuery := search.NewTermQuery(index.NewTerm("value", "parent"))
	childQuery := search.NewTermQuery(index.NewTerm("type", "child"))

	msg := expectThrowsPanic(t, func() {
		NewParentsChildrenBlockJoinQuery(parentFilter, parentQuery, childQuery, 0)
	})
	if !strings.Contains(msg, "childLimitPerParent must be > 0") {
		t.Fatalf("unexpected message: %q", msg)
	}
}

func TestParentsChildrenBlockJoinQueryExplain(t *testing.T) {
	// Create test blocks with specific structure to test explain()
	blocks := make([][]*pcTestDoc, 2)

	// Block 0: 2 children + 1 parent (parent matches)
	blocks[0] = []*pcTestDoc{
		newPCTestDoc("child", true, 0),  // docId=0
		newPCTestDoc("child", true, 1),  // docId=1
		newPCTestDoc("parent", true, 2), // docId=2
	}

	// Block 1: 2 children + 1 parent (parent matches)
	blocks[1] = []*pcTestDoc{
		newPCTestDoc("child", true, 3),  // docId=3
		newPCTestDoc("child", false, 4), // docId=4
		newPCTestDoc("parent", true, 5), // docId=5
	}

	dir, reader := indexParentsChildrenBlocks(t, blocks)

	// Do not use Concurrency.INTRA_SEGMENT as that would break childLimitPerParent counting
	searcher := newSearcherWithConcurrency(t, reader, false, false)
	parentFilter, parentQuery, childQuery := parentsChildrenQueries(t, reader)

	query := NewParentsChildrenBlockJoinQuery(parentFilter, parentQuery, childQuery, 2)

	// Test explain for a matching child document
	explanation := mustExplain(t, searcher, query, 0)
	if verbose {
		fmt.Println("Explanation for matching child document:")
		fmt.Println(explanation)
	}
	if !explanation.IsMatch() {
		t.Fatal("doc 0 must match")
	}

	// Test explain for a non-matching child document
	// Add a non-matching child

	explanation = mustExplain(t, searcher, query, 4)
	if explanation.IsMatch() {
		t.Fatal("doc 4 must not match")
	}

	mustClose(t, reader, dir)
}

func TestParentsChildrenBlockJoinQueryIntraSegmentConcurrencyNotSupported(t *testing.T) {
	// Create test blocks with specific structure - using a larger number of documents
	numParents := atLeast(1000) // Create at least 1000 parents to ensure large segment
	blocks := make([][]*pcTestDoc, numParents)

	docID := 0
	for parentIdx := 0; parentIdx < numParents; parentIdx++ {
		// Each parent has 5 children
		blocks[parentIdx] = make([]*pcTestDoc, 6) // 5 children + 1 parent

		// Add children
		for childIdx := 0; childIdx < 5; childIdx++ {
			blocks[parentIdx][childIdx] = newPCTestDoc("child", true, docID)
			docID++
		}
		// Add parent
		blocks[parentIdx][5] = newPCTestDoc("parent", true, docID)
		docID++
	}

	// Force merge to ensure we have one large segment
	dir, reader := indexParentsChildrenBlocks(t, blocks)

	parentFilter, parentQuery, childQuery := parentsChildrenQueries(t, reader)

	query := NewParentsChildrenBlockJoinQuery(parentFilter, parentQuery, childQuery, 2)

	for i := 0; i < 10; i++ {
		// Verify that searching with IntraSegment concurrency throws the expected exception
		// Note that there are still randomness on whether to slice a single segment, thus we are
		// running the test 10 times
		// And only assert for exception if there are multiple slices
		searcher := newSearcherWithConcurrency(t, reader, false, false)
		leafSlices := searcher.GetSlices()
		if len(leafSlices) > 1 {
			_, err := searcher.Search(query, 10)
			if err == nil {
				t.Fatal("expected IllegalStateException")
			}
			if !strings.Contains(err.Error(), "ParentsChildrenBlockJoinQuery does not support intraSegment concurrency.") {
				t.Fatalf("unexpected message: %v", err)
			}
		}
	}

	mustClose(t, reader, dir)
}

// pcTestDoc renders the private record TestDoc(String type, boolean isMatch,
// int docID).
type pcTestDoc struct {
	typ     string
	isMatch bool
	docID   int
}

func newPCTestDoc(typ string, isMatch bool, docID int) *pcTestDoc {
	return &pcTestDoc{typ: typ, isMatch: isMatch, docID: docID}
}

func (d *pcTestDoc) toDocument(t testing.TB) *document.Document {
	t.Helper()
	doc := document.NewDocument()
	doc.Add(newStringField(t, "type", d.typ, true))
	id, err := document.NewStoredFieldFromInt("ID", d.docID)
	if err != nil {
		t.Fatal(err)
	}
	doc.Add(id)
	if d.isMatch {
		doc.Add(newStringField(t, "value", "match", true))
	} else {
		doc.Add(newStringField(t, "value", "nomatch", true))
	}
	return doc
}

func (d *pcTestDoc) String() string {
	match := "nomatch"
	if d.isMatch {
		match = "match"
	}
	return d.typ + "(" + match + ")"
}

func visualizeTestDocs(blocks [][]*pcTestDoc) string {
	var sb strings.Builder
	sb.WriteString("Test Documents Structure:\n")
	docID := 0
	for i, block := range blocks {
		sb.WriteString("Block " + strconv.Itoa(i) + ":\n")
		for _, doc := range block {
			sb.WriteString("  docId=" + strconv.Itoa(docID) + ": " + doc.String() + "\n")
			docID++
		}
	}
	return sb.String()
}
