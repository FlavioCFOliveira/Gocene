package grouping

// TestBlockGrouping is the Go counterpart of
// org.apache.lucene.search.grouping.TestBlockGrouping from Lucene 10.4.0.
//
// The Java original tests GroupingSearch against a RandomIndexWriter where
// documents are indexed in blocks: each block represents a book, with its
// chapters as child documents and the final chapter marked as "blockEnd".
// The integration tests (testSimple, testTopLevelSort, testWithinGroupSort)
// require RandomIndexWriter + IndexSearcher + GroupingSearch.search() which
// depend on the Lucene search stack not yet wired in Gocene.
//
// What is exercised here:
//   - Block structure creation (createBlock, createBlocks)
//   - BlockGroupingCollector: collect parent/child docs, group assignment
//   - BlockGroup shape assertions (groupValue, children, parent doc)

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/join"
)

// chapter is a simple in-memory representation of a book chapter.
type chapter struct {
	book       string
	chapter    string
	text       string
	length     int
	isBlockEnd bool
}

// createBlock returns a slice of chapters for a single book (one block).
// The last chapter is marked as the blockEnd, mirroring
// TestBlockGrouping.createRandomBlock.
func createBlock(bookName string, chapterCount int) []chapter {
	chapters := make([]chapter, chapterCount)
	for j := 0; j < chapterCount; j++ {
		chapters[j] = chapter{
			book:       bookName,
			chapter:    "chapter" + itoa(j),
			text:       sampleText(j),
			length:     10 + j,
			isBlockEnd: j == chapterCount-1,
		}
	}
	return chapters
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func sampleText(seed int) string {
	texts := []string{
		"It was the day my grandmother exploded",
		"It was the best of times, it was the worst of times",
		"It was a bright cold morning in April",
		"It is a truth universally acknowledged",
	}
	return texts[seed%len(texts)]
}

// TestBlockGrouping_CreateBlock verifies that createBlock produces the correct
// structure: the right number of chapters, correct book name, and only the
// last chapter marked as blockEnd.
func TestBlockGrouping_CreateBlock(t *testing.T) {
	const bookName = "book1"
	const chapterCount = 5

	chapters := createBlock(bookName, chapterCount)

	if len(chapters) != chapterCount {
		t.Fatalf("expected %d chapters, got %d", chapterCount, len(chapters))
	}
	for i, ch := range chapters {
		if ch.book != bookName {
			t.Errorf("[%d] book = %q, want %q", i, ch.book, bookName)
		}
		wantBlockEnd := i == chapterCount-1
		if ch.isBlockEnd != wantBlockEnd {
			t.Errorf("[%d] isBlockEnd = %v, want %v", i, ch.isBlockEnd, wantBlockEnd)
		}
	}
}

// TestBlockGrouping_BlockGroupingCollectorAssignment verifies that
// BlockGroupingCollector correctly assigns child docs to the parent group
// and tracks group values, mirroring the intent of testSimple.
func TestBlockGrouping_BlockGroupingCollectorAssignment(t *testing.T) {
	// Build a FixedBitSet marking doc IDs 2 and 5 as parent (blockEnd) docs.
	// Block 0: docs 0,1 (children) + doc 2 (parent/blockEnd)
	// Block 1: docs 3,4 (children) + doc 5 (parent/blockEnd)
	const numDocs = 6
	parentBits := join.NewFixedBitSet(numDocs)
	parentBits.Set(2)
	parentBits.Set(5)

	// GroupSelector maps each doc to its book name via a lookup table.
	books := map[int]string{0: "bookA", 1: "bookA", 2: "bookA", 3: "bookB", 4: "bookB", 5: "bookB"}
	sel := &mapGroupSelector{books: books}

	bgc := NewBlockGroupingCollector(sel, parentBits)

	// Feed documents in order.
	for doc := 0; doc < numDocs; doc++ {
		score := float32(doc) + 1.0
		if err := bgc.Collect(doc, score); err != nil {
			t.Fatalf("Collect(%d): %v", doc, err)
		}
	}
	// Finalise the last block.
	bgc.Finish()

	groups := bgc.GetGroups()
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// Both groups should have 2 child docs (the non-parent docs).
	for _, g := range groups {
		if len(g.ChildDocs) < 2 {
			t.Errorf("group %v: expected at least 2 child docs, got %d", g.GroupValue, len(g.ChildDocs))
		}
	}
}

// TestBlockGrouping_BlockGroupSortByParentScore verifies that groups can be
// ordered by parent score, mirroring the top-level sort behaviour of
// testTopLevelSort.
func TestBlockGrouping_BlockGroupSortByParentScore(t *testing.T) {
	const numDocs = 6
	parentBits := join.NewFixedBitSet(numDocs)
	parentBits.Set(2)
	parentBits.Set(5)

	books := map[int]string{0: "A", 1: "A", 2: "A", 3: "B", 4: "B", 5: "B"}
	sel := &mapGroupSelector{books: books}
	bgc := NewBlockGroupingCollector(sel, parentBits)

	// Book B's parent (doc 5) gets a higher score than Book A's parent (doc 2).
	scores := []float32{0.5, 0.3, 0.4, 0.9, 0.8, 1.2}
	for doc := 0; doc < numDocs; doc++ {
		if err := bgc.Collect(doc, scores[doc]); err != nil {
			t.Fatalf("Collect(%d): %v", doc, err)
		}
	}
	bgc.Finish()

	groups := bgc.GetGroups()
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// Find the group scores to verify ordering potential.
	var scoreA, scoreB float32
	for _, g := range groups {
		if g.GroupValue == "A" {
			scoreA = g.ParentScore
		} else if g.GroupValue == "B" {
			scoreB = g.ParentScore
		}
	}
	if scoreB <= scoreA {
		t.Errorf("book B parent score (%f) should exceed book A parent score (%f)", scoreB, scoreA)
	}
}

// mapGroupSelector is a test-only GroupSelector backed by a doc→group map.
type mapGroupSelector struct {
	books map[int]string
}

func (m *mapGroupSelector) Select(doc int) interface{} {
	return m.books[doc]
}
