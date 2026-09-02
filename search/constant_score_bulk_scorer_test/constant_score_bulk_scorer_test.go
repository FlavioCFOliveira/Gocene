package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

type mockDocIdSetIterator struct {
	docs []int
	pos  int
}

func newMockDocIdSetIterator(docs []int) *mockDocIdSetIterator {
	return &mockDocIdSetIterator{docs: docs, pos: -1}
}

func (it *mockDocIdSetIterator) DocID() int {
	if it.pos < 0 || it.pos >= len(it.docs) {
		return search.NO_MORE_DOCS
	}
	return it.docs[it.pos]
}

func (it *mockDocIdSetIterator) NextDoc() (int, error) {
	it.pos++
	return it.DocID(), nil
}

func (it *mockDocIdSetIterator) Advance(target int) (int, error) {
	for it.pos < len(it.docs)-1 && it.docs[it.pos+1] < target {
		it.pos++
	}
	if it.pos < len(it.docs)-1 && it.docs[it.pos+1] >= target {
		it.pos++
	} else if it.pos == -1 {
		it.pos = 0
		if it.DocID() < target {
			return it.Advance(target)
		}
	} else {
		if it.DocID() < target {
			it.pos++
			return it.DocID(), nil
		}
	}
	return it.DocID(), nil
}

func (it *mockDocIdSetIterator) DocIDRunEnd() int {
	if it.pos < 0 || it.pos >= len(it.docs) {
		return search.NO_MORE_DOCS
	}
	return it.DocID() + 1
}

func (it *mockDocIdSetIterator) Cost() int64 {
	return int64(len(it.docs))
}

type mockTwoPhaseIterator struct {
	approx   search.DocIdSetIterator
	matches map[int]bool
}

func newMockTwoPhaseIterator(approx search.DocIdSetIterator, matches []int) *mockTwoPhaseIterator {
	m := make(map[int]bool)
	for _, doc := range matches {
		m[doc] = true
	}
	return &mockTwoPhaseIterator{approx: approx, matches: m}
}

func (tpi *mockTwoPhaseIterator) Approximation() search.DocIdSetIterator { return tpi.approx }

func (tpi *mockTwoPhaseIterator) Matches() (bool, error) {
	doc := tpi.approx.DocID()
	return tpi.matches[doc], nil
}

func (tpi *mockTwoPhaseIterator) MatchCost() float32 { return 1.0 }

func (tpi *mockTwoPhaseIterator) DocIDRunEnd() (int, error) {
	return tpi.approx.DocID(), nil
}

func (tpi *mockTwoPhaseIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return search.DefaultIntoBitSet(tpi, upTo, bitSet, offset)
}

type mockLeafCollector struct {
	collected []int
	scorer    search.Scorable
	compIter  search.DocIdSetIterator
}

func (c *mockLeafCollector) GetLeafCollector(context interface{}) (search.LeafCollector, error) {
	return c, nil
}

func (c *mockLeafCollector) ScoreMode() search.ScoreMode { return search.TOP_SCORES }

func (c *mockLeafCollector) SetWeight(weight search.Weight) {}

func (c *mockLeafCollector) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	return nil
}

func (c *mockLeafCollector) Collect(doc int) error {
	c.collected = append(c.collected, doc)
	return nil
}

func (c *mockLeafCollector) CollectRange(min, max int) error {
	for i := min; i < max; i++ {
		c.collected = append(c.collected, i)
	}
	return nil
}

func (c *mockLeafCollector) CollectStream(stream search.DocIdStream) error {
	for doc := stream.DocID(); doc != search.NO_MORE_DOCS; {
		c.collected = append(c.collected, doc)
		doc, _ = stream.NextDoc()
	}
	return nil
}

func (c *mockLeafCollector) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return c.compIter, nil
}

func (c *mockLeafCollector) Finish() error { return nil
}

func TestConstantScoreBulkScorer_Constructor(t *testing.T) {
	iter := newMockDocIdSetIterator([]int{1, 2, 3})

	t.Run("Valid", func(t *testing.T) {
		_, err := search.NewConstantScoreBulkScorer(1.0, search.TOP_SCORES, iter)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("WrapsTwoPhase", func(t *testing.T) {
		tpi := newMockTwoPhaseIterator(iter, []int{1, 2})
		wrapped := search.AsDocIdSetIterator(tpi)
		_, err := search.NewConstantScoreBulkScorer(1.0, search.TOP_SCORES, wrapped)
		if err == nil {
			t.Fatal("expected error when iterator wraps TwoPhaseIterator without providing the TwoPhaseIterator explicitly")
		}
	})
}

func TestConstantScoreBulkScorer_Score_NonCompetitive(t *testing.T) {
	tests := []struct {
		name           string
		docs           []int
		twoPhaseMatches []int
		acceptDocs     []int
		min            int
		max            int
		expected       []int
	}{
		{
			name:           "Simple",
			docs:           []int{10, 20, 30},
			twoPhaseMatches: nil,
			acceptDocs:     nil,
			min:            0,
			max:            100,
			expected:       []int{10, 20, 30},
		},
		{
			name:           "WithMinMax",
			docs:           []int{10, 20, 30, 40},
			twoPhaseMatches: nil,
			acceptDocs:     nil,
			min:            15,
			max:            35,
			expected:       []int{20, 30},
		},
		{
			name:           "WithTwoPhase",
			docs:           []int{10, 20, 30},
			twoPhaseMatches: []int{10, 30},
			acceptDocs:     nil,
			min:            0,
			max:            100,
			expected:       []int{10, 30},
		},
		{
			name:           "WithAcceptDocs",
			docs:           []int{10, 20, 30},
			twoPhaseMatches: nil,
			acceptDocs:     []int{10, 30},
			min:            0,
			max:            100,
			expected:       []int{10, 30},
		},
		{
			name:           "TwoPhaseAndAcceptDocs",
			docs:           []int{10, 20, 30},
			twoPhaseMatches: []int{10, 20},
			acceptDocs:     []int{20, 30},
			min:            0,
			max:            100,
			expected:       []int{20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iter := newMockDocIdSetIterator(tt.docs)
			var tpi *mockTwoPhaseIterator
			if tt.twoPhaseMatches != nil {
				tpi = newMockTwoPhaseIterator(iter, tt.twoPhaseMatches)
			}

			var bs *search.ConstantScoreBulkScorer
			var err error
			if tpi != nil {
				bs, err = search.NewConstantScoreBulkScorerFromTwoPhase(1.0, search.TOP_SCORES, tpi)
			} else {
				bs, err = search.NewConstantScoreBulkScorer(1.0, search.TOP_SCORES, iter)
			}
			if err != nil {
				t.Fatalf("failed to create scorer: %v", err)
			}

			var acceptDocs util.Bits
			if tt.acceptDocs != nil {
				bs_bits, _ := util.NewFixedBitSet(100)
				for _, d := range tt.acceptDocs {
					bs_bits.Set(d)
				}
				acceptDocs = bs_bits
			}

			collector := &mockLeafCollector{}
			_, err = bs.Score(collector, acceptDocs, tt.min, tt.max)
			if err != nil {
				t.Fatalf("score failed: %v", err)
			}

			if len(collector.collected) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, collector.collected)
			} else {
				for i := range tt.expected {
					if collector.collected[i] != tt.expected[i] {
						t.Errorf("at index %d: expected %d, got %d", i, tt.expected[i], collector.collected[i])
					}
				}
			}
		})
	}
}

func TestConstantScoreBulkScorer_Score_Competitive(t *testing.T) {
	tests := []struct {
		name           string
		docs           []int
		compDocs       []int
		twoPhaseMatches []int
		acceptDocs     []int
		min            int
		max            int
		expected       []int
	}{
		{
			name:           "SimpleCompetitive",
			docs:           []int{10, 20, 30},
			compDocs:       []int{15, 25, 35},
			twoPhaseMatches: nil,
			acceptDocs:     nil,
			min:            0,
			max:            100,
			expected:       []int{}, // No overlap
		},
		{
			name:           "OverlapCompetitive",
			docs:           []int{10, 20, 30},
			compDocs:       []int{10, 25, 30},
			twoPhaseMatches: nil,
			acceptDocs:     nil,
			min:            0,
			max:            100,
			expected:       []int{10, 30},
		},
		{
			name:           "CompetitiveWithTwoPhase",
			docs:           []int{10, 20, 30},
			compDocs:       []int{10, 20, 30},
			twoPhaseMatches: []int{10, 30},
			acceptDocs:     nil,
			min:            0,
			max:            100,
			expected:       []int{10, 30},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iter := newMockDocIdSetIterator(tt.docs)
			compIter := newMockDocIdSetIterator(tt.compDocs)
			var tpi *mockTwoPhaseIterator
			if tt.twoPhaseMatches != nil {
				tpi = newMockTwoPhaseIterator(iter, tt.twoPhaseMatches)
			}

			var bs *search.ConstantScoreBulkScorer
			var err error
			if tpi != nil {
				bs, err = search.NewConstantScoreBulkScorerFromTwoPhase(1.0, search.TOP_SCORES, tpi)
			} else {
				bs, err = search.NewConstantScoreBulkScorer(1.0, search.TOP_SCORES, iter)
			}
			if err != nil {
				t.Fatalf("failed to create scorer: %v", err)
			}

			var acceptDocs util.Bits
			if tt.acceptDocs != nil {
				bs_bits, _ := util.NewFixedBitSet(100)
				for _, d := range tt.acceptDocs {
					bs_bits.Set(d)
				}
				acceptDocs = bs_bits
			}

			collector := &mockLeafCollector{compIter: compIter}
			_, err = bs.Score(collector, acceptDocs, tt.min, tt.max)
			if err != nil {
				t.Fatalf("score failed: %v", err)
			}

			if len(collector.collected) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, collector.collected)
			} else {
				for i := range tt.expected {
					if collector.collected[i] != tt.expected[i] {
						t.Errorf("at index %d: expected %d, got %d", i, tt.expected[i], collector.collected[i])
					}
				}
			}
		})
	}
}

func TestConstantScoreBulkScorer_Windowing(t *testing.T) {
	// Test that it works across multiple windows.
	// WindowSize is 4096.
	docs := []int{10, 4100, 8200}
	iter := newMockDocIdSetIterator(docs)
	bs, _ := search.NewConstantScoreBulkScorer(1.0, search.TOP_SCORES, iter)
	collector := &mockLeafCollector{}
	_, err := bs.Score(collector, nil, 0, 10000)
	if err != nil {
		t.Fatalf("score failed: %v", err)
	}
	expected := []int{10, 4100, 8200}
	if len(collector.collected) != len(expected) {
		t.Errorf("expected %v, got %v", expected, collector.collected)
	}
}
