package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// mockBitSetProducer2 is a mock BitSetProducer for testing
type mockBitSetProducer2 struct {
	bitSet *FixedBitSet
}

func (m *mockBitSetProducer2) GetBitSet(context *index.LeafReaderContext) (*FixedBitSet, error) {
	return m.bitSet, nil
}

// MockFieldComparator is a mock FieldComparator for testing
type MockFieldComparator struct {
	compareFunc       func(doc1, doc2 int) int
	setBottomFunc     func(doc int)
	compareBottomFunc func(doc int) int
	copyFunc          func(slot int, doc int)
	setScorerFunc     func(scorer search.Scorer)
}

func (m *MockFieldComparator) Compare(doc1, doc2 int) int {
	if m.compareFunc != nil {
		return m.compareFunc(doc1, doc2)
	}
	return 0
}

func (m *MockFieldComparator) SetBottom(doc int) {
	if m.setBottomFunc != nil {
		m.setBottomFunc(doc)
	}
}

func (m *MockFieldComparator) CompareBottom(doc int) int {
	if m.compareBottomFunc != nil {
		return m.compareBottomFunc(doc)
	}
	return 0
}

func (m *MockFieldComparator) Copy(slot int, doc int) {
	if m.copyFunc != nil {
		m.copyFunc(slot, doc)
	}
}

func (m *MockFieldComparator) SetScorer(scorer search.Scorer) {
	if m.setScorerFunc != nil {
		m.setScorerFunc(scorer)
	}
}

func TestNewBlockJoinComparatorSource(t *testing.T) {
	mockComparator := &MockFieldComparator{}
	parentsFilter := &mockBitSetProducer2{bitSet: NewFixedBitSet(100)}

	source := NewBlockJoinComparatorSource(mockComparator, parentsFilter)

	if source == nil {
		t.Fatal("Expected BlockJoinComparatorSource to be created")
	}

	if source.parentComparator != mockComparator {
		t.Error("Expected parent comparator to match")
	}
}

func TestBlockJoinComparatorSourceNewComparator(t *testing.T) {
	mockComparator := &MockFieldComparator{}
	parentsFilter := &mockBitSetProducer2{bitSet: NewFixedBitSet(100)}

	source := NewBlockJoinComparatorSource(mockComparator, parentsFilter)

	sortField := search.NewSortField("field", search.SortFieldTypeString)
	comparator := source.NewComparator(sortField, 10)

	if comparator == nil {
		t.Fatal("Expected comparator to be created")
	}

	blockJoinComp, ok := comparator.(*BlockJoinComparator)
	if !ok {
		t.Fatal("Expected *BlockJoinComparator")
	}

	if blockJoinComp.parentComparator != mockComparator {
		t.Error("Expected parent comparator to match")
	}
}

func TestNewBlockJoinComparator(t *testing.T) {
	mockComparator := &MockFieldComparator{}
	parentsFilter := &mockBitSetProducer2{bitSet: NewFixedBitSet(100)}

	comparator := NewBlockJoinComparator(mockComparator, parentsFilter, 10)

	if comparator == nil {
		t.Fatal("Expected BlockJoinComparator to be created")
	}

	if comparator.parentComparator != mockComparator {
		t.Error("Expected parent comparator to match")
	}

	if comparator.parentsFilter != parentsFilter {
		t.Error("Expected parents filter to match")
	}

	if len(comparator.parentDocs) != 10 {
		t.Errorf("Expected parentDocs length 10, got %d", len(comparator.parentDocs))
	}
}

// newParentsProducer creates a mock BitSetProducer with parent bits set at
// the given doc ids over a bitset of capacity size.
func newParentsProducer(size int, parents ...int) *mockBitSetProducer2 {
	bs := NewFixedBitSet(size)
	for _, p := range parents {
		bs.Set(p)
	}
	return &mockBitSetProducer2{bitSet: bs}
}

func TestBlockJoinComparatorCompare(t *testing.T) {
	mockComparator := &MockFieldComparator{
		compareFunc: func(doc1, doc2 int) int {
			if doc1 < doc2 {
				return -1
			} else if doc1 > doc2 {
				return 1
			}
			return 0
		},
	}
	// Parents at doc 6 (children 0-5) and doc 11 (children 7-10).
	parentsFilter := newParentsProducer(100, 6, 11)

	comparator := NewBlockJoinComparator(mockComparator, parentsFilter, 10)
	if err := comparator.SetContext(&index.LeafReaderContext{}); err != nil {
		t.Fatalf("SetContext failed: %v", err)
	}

	// child doc 5 -> parent 6, child doc 10 -> parent 11; 6 < 11.
	if result := comparator.Compare(5, 10); result != -1 {
		t.Errorf("Expected Compare(child=5, child=10) = -1 (parents 6 vs 11), got %d", result)
	}
	// Same child collapses to same parent: 6 == 6.
	if result := comparator.Compare(5, 5); result != 0 {
		t.Errorf("Expected Compare(5, 5) = 0, got %d", result)
	}
	// Reversed: parent 11 > parent 6.
	if result := comparator.Compare(10, 5); result != 1 {
		t.Errorf("Expected Compare(child=10, child=5) = 1 (parents 11 vs 6), got %d", result)
	}
}

func TestBlockJoinComparatorSetBottom(t *testing.T) {
	var bottomDoc int
	mockComparator := &MockFieldComparator{
		setBottomFunc: func(doc int) {
			bottomDoc = doc
		},
	}
	// Parent at doc 6 -> children 0..5 resolve to parent 6.
	parentsFilter := newParentsProducer(100, 6)

	comparator := NewBlockJoinComparator(mockComparator, parentsFilter, 10)
	if err := comparator.SetContext(&index.LeafReaderContext{}); err != nil {
		t.Fatalf("SetContext failed: %v", err)
	}

	comparator.SetBottom(5)

	if bottomDoc != 6 {
		t.Errorf("Expected SetBottom(child=5) to set parent doc 6, got %d", bottomDoc)
	}
}

func TestBlockJoinComparatorCompareBottom(t *testing.T) {
	var compareDoc int
	mockComparator := &MockFieldComparator{
		compareBottomFunc: func(doc int) int {
			compareDoc = doc
			return 1
		},
	}
	parentsFilter := newParentsProducer(100, 6)

	comparator := NewBlockJoinComparator(mockComparator, parentsFilter, 10)
	if err := comparator.SetContext(&index.LeafReaderContext{}); err != nil {
		t.Fatalf("SetContext failed: %v", err)
	}

	result := comparator.CompareBottom(5)
	if compareDoc != 6 {
		t.Errorf("Expected CompareBottom(child=5) to compare parent doc 6, got %d", compareDoc)
	}
	if result != 1 {
		t.Errorf("Expected CompareBottom to return 1, got %d", result)
	}
}

func TestBlockJoinComparatorCopy(t *testing.T) {
	var copiedSlot, copiedDoc int
	mockComparator := &MockFieldComparator{
		copyFunc: func(slot int, doc int) {
			copiedSlot = slot
			copiedDoc = doc
		},
	}
	parentsFilter := newParentsProducer(100, 6)

	comparator := NewBlockJoinComparator(mockComparator, parentsFilter, 10)
	if err := comparator.SetContext(&index.LeafReaderContext{}); err != nil {
		t.Fatalf("SetContext failed: %v", err)
	}

	comparator.Copy(2, 5)

	if comparator.parentDocs[2] != 6 {
		t.Errorf("Expected parentDocs[2] = 6, got %d", comparator.parentDocs[2])
	}
	if copiedSlot != 2 {
		t.Errorf("Expected slot 2, got %d", copiedSlot)
	}
	if copiedDoc != 6 {
		t.Errorf("Expected doc 6, got %d", copiedDoc)
	}
}

func TestBlockJoinComparatorSetScorer(t *testing.T) {
	var scorerSet bool
	mockComparator := &MockFieldComparator{
		setScorerFunc: func(scorer search.Scorer) {
			scorerSet = true
		},
	}
	parentsFilter := &mockBitSetProducer2{bitSet: NewFixedBitSet(100)}

	comparator := NewBlockJoinComparator(mockComparator, parentsFilter, 10)

	// Set scorer (nil is ok for this test)
	comparator.SetScorer(nil)

	if !scorerSet {
		t.Error("Expected SetScorer to be called on parent comparator")
	}
}

func TestBlockJoinComparatorGetParentDoc(t *testing.T) {
	mockComparator := &MockFieldComparator{}
	// Parents at doc 1 (for child 0), doc 6 (for children 2..5), doc 50.
	parentsFilter := newParentsProducer(100, 1, 6, 50)

	comparator := NewBlockJoinComparator(mockComparator, parentsFilter, 10)
	if err := comparator.SetContext(&index.LeafReaderContext{}); err != nil {
		t.Fatalf("SetContext failed: %v", err)
	}

	if parent := comparator.getParentDoc(5); parent != 6 {
		t.Errorf("Expected getParentDoc(5) = 6, got %d", parent)
	}
	if parent := comparator.getParentDoc(0); parent != 1 {
		t.Errorf("Expected getParentDoc(0) = 1, got %d", parent)
	}
	// A child past the last parent has no parent: getParentDoc degrades to
	// the child doc id so the comparator still produces a stable ordering.
	if parent := comparator.getParentDoc(99); parent != 99 {
		t.Errorf("Expected getParentDoc(99) (no parent past 50) = 99, got %d", parent)
	}
	// A doc that *is* a parent returns itself.
	if parent := comparator.getParentDoc(6); parent != 6 {
		t.Errorf("Expected getParentDoc(6) = 6 (doc is a parent), got %d", parent)
	}
}

func TestBlockJoinComparatorWithoutContextDegrades(t *testing.T) {
	// Without SetContext, the comparator must not blow up; it should fall
	// back to comparing raw doc ids (parentsBits is nil).
	mockComparator := &MockFieldComparator{}
	parentsFilter := newParentsProducer(100, 6)

	comparator := NewBlockJoinComparator(mockComparator, parentsFilter, 10)

	if parent := comparator.getParentDoc(5); parent != 5 {
		t.Errorf("Expected getParentDoc(5) to degrade to 5 without SetContext, got %d", parent)
	}
}

func TestBlockJoinComparatorImplementsInterface(t *testing.T) {
	mockComparator := &MockFieldComparator{}
	parentsFilter := &mockBitSetProducer2{bitSet: NewFixedBitSet(100)}

	// This should compile if BlockJoinComparator implements FieldComparator
	var _ search.FieldComparator = NewBlockJoinComparator(mockComparator, parentsFilter, 10)
}

func TestBlockJoinComparatorSourceImplementsInterface(t *testing.T) {
	mockComparator := &MockFieldComparator{}
	parentsFilter := &mockBitSetProducer2{bitSet: NewFixedBitSet(100)}

	// This should compile if BlockJoinComparatorSource implements FieldComparatorSource
	var _ search.FieldComparatorSource = NewBlockJoinComparatorSource(mockComparator, parentsFilter)
}
