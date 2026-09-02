package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

type mockPostingsEnum struct {
	docID    int
	freq     int
	positions []int
	curPos   int
}

func (m *mockPostingsEnum) NextDoc() (int, error) {
	if m.docID == -1 {
		return 0, nil
	}
	return -1, nil
}

func (m *mockPostingsEnum) Advance(target int) (int, error) {
	if m.docID < target {
		m.docID = target
	}
	return m.docID, nil
}

func (m *mockPostingsEnum) DocID() int { return m.docID }
func (m *mockPostingsEnum) Freq() (int, error) { return m.freq, nil }
func (m *mockPostingsEnum) NextPosition() (int, error) {
	if m.curPos >= len(m.positions) {
		return -1, nil
	}
	pos := m.positions[m.curPos]
	m.curPos++
	return pos, nil
}
func (m *mockPostingsEnum) StartOffset() (int, error) { return 0, nil }
func (m *mockPostingsEnum) EndOffset() (int, error) { return 0, nil }
func (m *mockPostingsEnum) GetPayload() ([]byte, error) { return nil, nil }
func (m *mockPostingsEnum) Cost() int64 { return 1 }
func (m *mockPostingsEnum) AdvanceShallow(target int) error { return nil }
func (m *mockPostingsEnum) GetImpacts() (index.Impacts, error) {
	return &mockImpacts{freq: m.freq, norm: 1}, nil
}

type mockImpacts struct {
	freq int
	norm int64
}

func (m *mockImpacts) NumLevels() int { return 1 }
func (m *mockImpacts) GetDocIDUpTo(level int) int { return 100 }
func (m *mockImpacts) GetImpacts(level int) *index.FreqAndNormBuffer {
	buf := index.NewFreqAndNormBuffer()
	buf.Add(int32(m.freq), m.norm)
	return buf
}

func TestExactPhraseMatcher_NextMatch(t *testing.T) {
	// Phrase: "hello world"
	// Term 1 (hello) at pos 2, 5
	// Term 2 (world) at pos 3, 8
	p1 := &mockPostingsEnum{docID: 0, freq: 2, positions: []int{2, 5}}
	p2 := &mockPostingsEnum{docID: 0, freq: 2, positions: []int{3, 8}}

	matcher := NewExactPhraseMatcher([]struct {
		postings index.PostingsEnum
		offset   int
	}{
		{postings: p1, offset: 0},
		{postings: p2, offset: 1},
	}, ScoreModeTopScores, nil, 1.0)

	matcher.ResetPositions()

	// First match: hello at 2, world at 3
	ok, err := matcher.NextMatch()
	if err != nil {
		t.Fatalf("NextMatch error: %v", err)
	}
	if !ok {
		t.Fatal("Expected first match")
	}
	if matcher.StartPosition() != 2 {
		t.Errorf("Expected start position 2, got %d", matcher.StartPosition())
	}
	if matcher.EndPosition() != 3 {
		t.Errorf("Expected end position 3, got %d", matcher.EndPosition())
	}

	// Second match: hello at 5, world at 8 -> No match because 5+1 != 8
	ok, err = matcher.NextMatch()
	if err != nil {
		t.Fatalf("NextMatch error: %v", err)
	}
	if ok {
		t.Fatal("Expected no more matches")
	}
}

func TestExactPhraseMatcher_MaxFreq(t *testing.T) {
	p1 := &mockPostingsEnum{docID: 0, freq: 5}
	p2 := &mockPostingsEnum{docID: 0, freq: 3}

	matcher := NewExactPhraseMatcher([]struct {
		postings index.PostingsEnum
		offset   int
	}{
		{postings: p1, offset: 0},
		{postings: p2, offset: 1},
	}, ScoreModeTopScores, nil, 1.0)

	maxFreq, err := matcher.MaxFreq()
	if err != nil {
		t.Fatalf("MaxFreq error: %v", err)
	}
	if maxFreq != 3.0 {
		t.Errorf("Expected maxFreq 3.0, got %f", maxFreq)
	}
}

type flexibleImpacts struct {
	freqs []int
	norms []int64
}

func (f *flexibleImpacts) NumLevels() int { return 1 }
func (f *flexibleImpacts) GetDocIDUpTo(level int) int { return 100 }
func (f *flexibleImpacts) GetImpacts(level int) *index.FreqAndNormBuffer {
	buf := index.NewFreqAndNormBuffer()
	for i := 0; i < len(f.freqs); i++ {
		buf.Add(int32(f.freqs[i]), f.norms[i])
	}
	return buf
}

type flexibleImpactsEnum struct {
	mockPostingsEnum
	impacts *flexibleImpacts
}

func (f *flexibleImpactsEnum) GetImpacts() (index.Impacts, error) {
	return f.impacts, nil
}

func TestMergeImpacts(t *testing.T) {
	ie1 := &flexibleImpactsEnum{
		mockPostingsEnum: mockPostingsEnum{docID: 0, freq: 4},
		impacts: &flexibleImpacts{
			freqs: []int{2, 4},
			norms: []int64{10, 12},
		},
	}
	ie2 := &flexibleImpactsEnum{
		mockPostingsEnum: mockPostingsEnum{docID: 0, freq: 3},
		impacts: &flexibleImpacts{
			freqs: []int{1, 3},
			norms: []int64{15, 11},
		},
	}

	source := mergeImpacts([]index.ImpactsEnum{ie1, ie2}, nil)
	impacts, err := source.GetImpacts()
	if err != nil {
		t.Fatalf("GetImpacts error: %v", err)
	}

	merged := impacts.GetImpacts(0)
	// Expected merged impacts:
	// Freq 1: norm 15 (from ie2)
	// Freq 2: norm 10 (from ie1)
	// Freq 3: norm 11 (from ie2)
	// Freq 4: norm 12 (from ie1)
	// But mergeImpacts in Lucene merges by freq and tracks the best norm.
	// Let's check the actual result.
	if merged.Size != 4 {
		t.Errorf("Expected merged size 4, got %d", merged.Size)
	}
}
