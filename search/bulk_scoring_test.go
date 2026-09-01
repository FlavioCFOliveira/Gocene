package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

type mockBulkScorer struct {
	docs   []int
	scores []float32
}

func (m *mockBulkScorer) Score(collector LeafCollector, acceptDocs util.Bits, min, max int) (int, error) {
	for _, doc := range m.docs {
		if doc >= min && doc < max {
			// Need to set the scorer on the collector to mirror real behavior
			// In this mock, we just call Collect.
			if err := collector.Collect(doc); err != nil {
				return 0, err
			}
		}
	}
	return NO_MORE_DOCS, nil
}

func (m *mockBulkScorer) Cost() int64 {
	return int64(len(m.docs))
}

type mockLeafCollector struct {
	collected []int
	scores    []float32
	scorable  Scorable
}

func (m *mockLeafCollector) SetScorer(scorer Scorable) error {
	m.scorable = scorer
	return nil
}

func (m *mockLeafCollector) Collect(doc int) error {
	m.collected = append(m.collected, doc)
	if m.scorable != nil {
		s, _ := m.scorable.Score()
		m.scores = append(m.scores, s)
	}
	return nil
}

func (m *mockLeafCollector) CollectRange(min, max int) error { return nil }
func (m *mockLeafCollector) CollectStream(stream DocIdStream) error { return nil }
func (m *mockLeafCollector) CompetitiveIterator() (DocIdSetIterator, error) { return nil, nil }
func (m *mockLeafCollector) Finish() error { return nil }

type mockScorable struct {
	score float32
}

func (m *mockScorable) Score() (float32, error) { return m.score, nil }
func (m *mockScorable) SmoothingScore(docID int) (float32, error) { return 0, nil }
func (m *mockScorable) SetMinCompetitiveScore(minScore float32) error { return nil }
func (m *mockScorable) GetChildren() ([]ChildScorable, error) { return nil, nil }

func TestBulkScorerWrapperScorer(t *testing.T) {
	bs := &mockBulkScorer{
		docs:   []int{10, 20, 30},
		scores: []float32{1.0, 2.0, 3.0},
	}

	// We need a BulkScorer that actually sets the scorer on the collector.
	// Our mockBulkScorer doesn't do it, but the real DefaultBulkScorer does.
	// For the WrapperScorer test, the key is the refill and the capture of scores.

	// Since BulkScorerWrapperScorer's collector calls scorable.Score(),
	// we need the BulkScorer.Score to set a scorable that returns the correct score.

	// Let's use a real DefaultBulkScorer with a mock Scorer.
	scorer := &mockScorable{score: 1.5}
	dbs := NewDefaultBulkScorer(scorer)

	wrapper := NewBulkScorerWrapperScorer(dbs, 10)

	doc, err := wrapper.Advance(0)
	if err != nil {
		t.Fatalf("Advance failed: %v", err)
	}
	if doc != 0 { // DefaultBulkScorer uses RangeDocIdSetIterator(0, maxDoc)
		t.Errorf("Expected doc 0, got %d", doc)
	}

	if wrapper.Score() != 1.5 {
		t.Errorf("Expected score 1.5, got %f", wrapper.Score())
	}
}
