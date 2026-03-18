package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// mockBitSetProducer is a simple mock for testing
type mockBitSetProducer struct {
	bitSet *FixedBitSet
}

func (m *mockBitSetProducer) GetBitSet(context *index.LeafReaderContext) (*FixedBitSet, error) {
	return m.bitSet, nil
}

func TestNewBlockJoinCollector(t *testing.T) {
	parentFilter := NewFixedBitSet(100)
	childFilter := NewFixedBitSet(100)

	collector := NewBlockJoinCollector(parentFilter, childFilter, Max)

	if collector == nil {
		t.Fatal("Expected BlockJoinCollector to be created")
	}

	if collector.parentFilter != parentFilter {
		t.Error("Expected parent filter to match")
	}

	if collector.childFilter != childFilter {
		t.Error("Expected child filter to match")
	}

	if collector.scoreMode != Max {
		t.Errorf("Expected score mode Max, got %v", collector.scoreMode)
	}
}

func TestBlockJoinCollectorCollect(t *testing.T) {
	parentFilter := NewFixedBitSet(100)
	childFilter := NewFixedBitSet(100)

	collector := NewBlockJoinCollector(parentFilter, childFilter, Max)

	// Collect some documents
	err := collector.Collect(5)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	err = collector.Collect(10)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if collector.GetTotalHits() != 2 {
		t.Errorf("Expected 2 hits, got %d", collector.GetTotalHits())
	}

	docs := collector.GetCollectedDocs()
	if len(docs) != 2 {
		t.Errorf("Expected 2 collected docs, got %d", len(docs))
	}

	if docs[0] != 5 || docs[1] != 10 {
		t.Errorf("Expected collected docs [5, 10], got %v", docs)
	}
}

func TestBlockJoinCollectorCollectWithScore(t *testing.T) {
	parentFilter := NewFixedBitSet(100)
	childFilter := NewFixedBitSet(100)

	collector := NewBlockJoinCollector(parentFilter, childFilter, Max)

	// Collect documents with scores
	err := collector.CollectWithScore(5, 1.5)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	err = collector.CollectWithScore(10, 2.5)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if collector.GetTotalHits() != 2 {
		t.Errorf("Expected 2 hits, got %d", collector.GetTotalHits())
	}

	scores := collector.GetCollectedScores()
	if len(scores) != 2 {
		t.Errorf("Expected 2 collected scores, got %d", len(scores))
	}

	if scores[0] != 1.5 || scores[1] != 2.5 {
		t.Errorf("Expected collected scores [1.5, 2.5], got %v", scores)
	}
}

func TestBlockJoinCollectorIsParent(t *testing.T) {
	parentFilter := NewFixedBitSet(100)
	// Mark documents 9, 19, 29, ... as parents
	for i := 9; i < 100; i += 10 {
		parentFilter.Set(i)
	}

	childFilter := NewFixedBitSet(100)
	collector := NewBlockJoinCollector(parentFilter, childFilter, Max)

	// Test IsParent
	if !collector.IsParent(9) {
		t.Error("Expected doc 9 to be a parent")
	}

	if !collector.IsParent(19) {
		t.Error("Expected doc 19 to be a parent")
	}

	if collector.IsParent(5) {
		t.Error("Expected doc 5 to not be a parent")
	}

	if collector.IsParent(100) {
		t.Error("Expected doc 100 (out of bounds) to not be a parent")
	}
}

func TestBlockJoinCollectorIsChild(t *testing.T) {
	parentFilter := NewFixedBitSet(100)
	childFilter := NewFixedBitSet(100)

	// Mark documents 5, 15, 25, ... as children
	for i := 5; i < 100; i += 10 {
		childFilter.Set(i)
	}

	collector := NewBlockJoinCollector(parentFilter, childFilter, Max)

	// Test IsChild
	if !collector.IsChild(5) {
		t.Error("Expected doc 5 to be a child")
	}

	if !collector.IsChild(15) {
		t.Error("Expected doc 15 to be a child")
	}

	if collector.IsChild(9) {
		t.Error("Expected doc 9 to not be a child")
	}
}

func TestBlockJoinCollectorGetParentDoc(t *testing.T) {
	parentFilter := NewFixedBitSet(100)
	// Mark documents 9, 19, 29, ... as parents
	for i := 9; i < 100; i += 10 {
		parentFilter.Set(i)
	}

	childFilter := NewFixedBitSet(100)
	collector := NewBlockJoinCollector(parentFilter, childFilter, Max)

	// Test GetParentDoc - searches forward from the child
	parent := collector.GetParentDoc(5)
	if parent != 9 {
		t.Errorf("Expected parent of 5 to be 9, got %d", parent)
	}

	parent = collector.GetParentDoc(15)
	if parent != 19 {
		t.Errorf("Expected parent of 15 to be 19, got %d", parent)
	}

	// When doc is a parent, searches for next parent after it
	parent = collector.GetParentDoc(9)
	if parent != 19 {
		t.Errorf("Expected next parent after 9 to be 19, got %d", parent)
	}

	// Test with nil parent filter
	collector2 := NewBlockJoinCollector(nil, childFilter, Max)
	parent = collector2.GetParentDoc(5)
	if parent != -1 {
		t.Errorf("Expected -1 with nil parent filter, got %d", parent)
	}
}

func TestBlockJoinCollectorGetChildrenDocs(t *testing.T) {
	parentFilter := NewFixedBitSet(100)
	childFilter := NewFixedBitSet(100)

	// Mark documents 9, 19 as parents
	parentFilter.Set(9)
	parentFilter.Set(19)

	// Mark documents 5-8 as children of parent 9
	for i := 5; i < 9; i++ {
		childFilter.Set(i)
	}

	collector := NewBlockJoinCollector(parentFilter, childFilter, Max)

	// Test GetChildrenDocs
	children := collector.GetChildrenDocs(9)
	if len(children) != 4 {
		t.Errorf("Expected 4 children for parent 9, got %d: %v", len(children), children)
	}

	// Test with nil child filter
	collector2 := NewBlockJoinCollector(parentFilter, nil, Max)
	children = collector2.GetChildrenDocs(9)
	if len(children) != 0 {
		t.Errorf("Expected 0 children with nil child filter, got %d", len(children))
	}
}

func TestBlockJoinCollectorComputeParentScore(t *testing.T) {
	parentFilter := NewFixedBitSet(100)
	childFilter := NewFixedBitSet(100)

	collector := NewBlockJoinCollector(parentFilter, childFilter, Avg)

	// Collect children with scores
	collector.CollectWithScore(5, 10.0)
	collector.CollectWithScore(6, 20.0)
	collector.CollectWithScore(7, 30.0)

	// Compute score for these children
	children := []int{5, 6, 7}
	score := collector.ComputeParentScore(children)

	expectedScore := float32((10.0 + 20.0 + 30.0) / 3.0)
	if score != expectedScore {
		t.Errorf("Expected Avg score %.2f, got %.2f", expectedScore, score)
	}

	// Test Max score mode
	collectorMax := NewBlockJoinCollector(parentFilter, childFilter, Max)
	collectorMax.CollectWithScore(5, 10.0)
	collectorMax.CollectWithScore(6, 50.0)
	collectorMax.CollectWithScore(7, 30.0)

	score = collectorMax.ComputeParentScore(children)
	if score != 50.0 {
		t.Errorf("Expected Max score 50.0, got %.2f", score)
	}

	// Test Min score mode
	collectorMin := NewBlockJoinCollector(parentFilter, childFilter, Min)
	collectorMin.CollectWithScore(5, 10.0)
	collectorMin.CollectWithScore(6, 50.0)
	collectorMin.CollectWithScore(7, 30.0)

	score = collectorMin.ComputeParentScore(children)
	if score != 10.0 {
		t.Errorf("Expected Min score 10.0, got %.2f", score)
	}

	// Test Total score mode
	collectorTotal := NewBlockJoinCollector(parentFilter, childFilter, Total)
	collectorTotal.CollectWithScore(5, 10.0)
	collectorTotal.CollectWithScore(6, 20.0)
	collectorTotal.CollectWithScore(7, 30.0)

	score = collectorTotal.ComputeParentScore(children)
	expectedTotal := float32(10.0 + 20.0 + 30.0)
	if score != expectedTotal {
		t.Errorf("Expected Total score %.2f, got %.2f", expectedTotal, score)
	}

	// Test None score mode
	collectorNone := NewBlockJoinCollector(parentFilter, childFilter, None)
	score = collectorNone.ComputeParentScore(children)
	if score != 0 {
		t.Errorf("Expected None score 0, got %.2f", score)
	}

	// Test with empty children
	score = collector.ComputeParentScore([]int{})
	if score != 0 {
		t.Errorf("Expected score 0 for empty children, got %.2f", score)
	}
}

func TestBlockJoinCollectorReset(t *testing.T) {
	parentFilter := NewFixedBitSet(100)
	childFilter := NewFixedBitSet(100)

	collector := NewBlockJoinCollector(parentFilter, childFilter, Max)

	// Collect some documents
	collector.Collect(5)
	collector.CollectWithScore(10, 2.5)

	if collector.GetTotalHits() != 2 {
		t.Errorf("Expected 2 hits before reset, got %d", collector.GetTotalHits())
	}

	// Reset
	collector.Reset()

	if collector.GetTotalHits() != 0 {
		t.Errorf("Expected 0 hits after reset, got %d", collector.GetTotalHits())
	}

	if len(collector.GetCollectedDocs()) != 0 {
		t.Errorf("Expected 0 collected docs after reset, got %d", len(collector.GetCollectedDocs()))
	}

	if len(collector.GetCollectedScores()) != 0 {
		t.Errorf("Expected 0 collected scores after reset, got %d", len(collector.GetCollectedScores()))
	}
}

func TestBlockJoinCollectorManager(t *testing.T) {
	// Create parent and child filters
	parentFilter := &mockBitSetProducer{bitSet: NewFixedBitSet(100)}
	childFilter := &mockBitSetProducer{bitSet: NewFixedBitSet(100)}

	manager := NewBlockJoinCollectorManager(parentFilter, childFilter, Max)

	if manager == nil {
		t.Fatal("Expected BlockJoinCollectorManager to be created")
	}

	if manager.scoreMode != Max {
		t.Errorf("Expected score mode Max, got %v", manager.scoreMode)
	}

	// Test NewCollector - requires a LeafReaderContext which we can't easily create
	// So we'll just verify it doesn't panic with nil
	// collector, err := manager.NewCollector(nil)
	// In a real test, we would create a proper LeafReaderContext
}
