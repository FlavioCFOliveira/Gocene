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
	compareFunc      func(doc1, doc2 int) int
	setBottomFunc    func(doc int)
	compareBottomFunc func(doc int) int
	copyFunc         func(slot int, doc int)
	setScorerFunc    func(scorer search.Scorer)
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

func TestBlockJoinComparatorCompare(t *testing.T) {
	// Mock comparator that simply compares doc IDs
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
	parentsFilter := &mockBitSetProducer2{bitSet: NewFixedBitSet(100)}

	comparator := NewBlockJoinComparator(mockComparator, parentsFilter, 10)

	// Compare documents
	// Note: getParentDoc currently returns childDoc + 1
	// So comparing doc 5 (parent 6) vs doc 10 (parent 11) should return -1
	result := comparator.Compare(5, 10)
	if result != -1 {
		t.Errorf("Expected Compare(5, 10) = -1, got %d", result)
	}

	// Compare equal docs
	result = comparator.Compare(5, 5)
	if result != 0 {
		t.Errorf("Expected Compare(5, 5) = 0, got %d", result)
	}

	// Compare reversed
	result = comparator.Compare(10, 5)
	if result != 1 {
		t.Errorf("Expected Compare(10, 5) = 1, got %d", result)
	}
}

func TestBlockJoinComparatorSetBottom(t *testing.T) {
	var bottomDoc int
	mockComparator := &MockFieldComparator{
		setBottomFunc: func(doc int) {
			bottomDoc = doc
		},
	}
	parentsFilter := &mockBitSetProducer2{bitSet: NewFixedBitSet(100)}

	comparator := NewBlockJoinComparator(mockComparator, parentsFilter, 10)

	// Set bottom for child doc 5 (parent should be 6)
	comparator.SetBottom(5)

	// Verify bottom was set to parent doc (5 + 1 = 6)
	if bottomDoc != 6 {
		t.Errorf("Expected SetBottom(5) to set parent doc 6, got %d", bottomDoc)
	}
}

func TestBlockJoinComparatorCompareBottom(t *testing.T) {
	var compareDoc int
	mockComparator := &MockFieldComparator{
		compareBottomFunc: func(doc int) int {
			compareDoc = doc
			return 1 // Always return 1
		},
	}
	parentsFilter := &mockBitSetProducer2{bitSet: NewFixedBitSet(100)}

	comparator := NewBlockJoinComparator(mockComparator, parentsFilter, 10)

	result := comparator.CompareBottom(5)

	if compareDoc != 6 {
		t.Errorf("Expected CompareBottom(5) to compare parent doc 6, got %d", compareDoc)
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
	parentsFilter := &mockBitSetProducer2{bitSet: NewFixedBitSet(100)}

	comparator := NewBlockJoinComparator(mockComparator, parentsFilter, 10)

	// Copy slot 2 with doc 5 (parent should be 6)
	comparator.Copy(2, 5)

	// Verify parent doc was cached
	if comparator.parentDocs[2] != 6 {
		t.Errorf("Expected parentDocs[2] = 6, got %d", comparator.parentDocs[2])
	}

	// Verify copy was called with parent doc
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
	parentsFilter := &mockBitSetProducer2{bitSet: NewFixedBitSet(100)}

	comparator := NewBlockJoinComparator(mockComparator, parentsFilter, 10)

	// Test getParentDoc
	// Current implementation returns childDoc + 1
	parent := comparator.getParentDoc(5)
	if parent != 6 {
		t.Errorf("Expected getParentDoc(5) = 6, got %d", parent)
	}

	parent = comparator.getParentDoc(0)
	if parent != 1 {
		t.Errorf("Expected getParentDoc(0) = 1, got %d", parent)
	}

	parent = comparator.getParentDoc(99)
	if parent != 100 {
		t.Errorf("Expected getParentDoc(99) = 100, got %d", parent)
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
