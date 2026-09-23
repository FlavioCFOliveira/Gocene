package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

type mockPostings struct {
	docID     int
	freq      int
	positions []int
	offsets   []int
	curDoc    int
	curPos    int
}

func (m *mockPostings) NextDoc() (int, error) {
	if m.curDoc == -1 {
		m.curDoc = m.docID
		return m.curDoc, nil
	}
	m.curDoc = index.NO_MORE_DOCS
	return m.curDoc, nil
}

func (m *mockPostings) Advance(target int) (int, error) {
	if m.curDoc == -1 && m.docID >= target {
		m.curDoc = m.docID
		return m.curDoc, nil
	}
	m.curDoc = index.NO_MORE_DOCS
	return m.curDoc, nil
}

func (m *mockPostings) DocID() int         { return m.curDoc }
func (m *mockPostings) Freq() (int, error) { return m.freq, nil }
func (m *mockPostings) NextPosition() (int, error) {
	if m.curPos >= len(m.positions) {
		return index.NO_MORE_POSITIONS, nil
	}
	pos := m.positions[m.curPos]
	m.curPos++
	return pos, nil
}
func (m *mockPostings) StartOffset() (int, error) {
	if m.curPos == 0 {
		return m.offsets[0], nil
	}
	return m.offsets[m.curPos-1], nil
}
func (m *mockPostings) EndOffset() (int, error) {
	if m.curPos == 0 {
		return m.offsets[0] + 1, nil
	}
	return m.offsets[m.curPos-1] + 1, nil
}
func (m *mockPostings) GetPayload() ([]byte, error) { return nil, nil }
func (m *mockPostings) Cost() int64                 { return 1 }

// Also implement ImpactsSource to be an ImpactsEnum
func (m *mockPostings) AdvanceShallow(target int) error { return nil }
func (m *mockPostings) GetImpacts() (index.Impacts, error) {
	return &mockImpacts{postings: m}, nil
}

// DocIDRunEnd carries the default body Lucene gives PostingsEnum.DocIDRunEnd.
func (m *mockPostings) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(m)
}

// IntoBitSet carries the default body Lucene gives PostingsEnum.IntoBitSet.
func (m *mockPostings) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(m, upTo, bitSet, offset)
}

type mockImpacts struct {
	postings *mockPostings
}

func (m *mockImpacts) NumLevels() int             { return 1 }
func (m *mockImpacts) GetDocIDUpTo(level int) int { return m.postings.docID }
func (m *mockImpacts) GetImpacts(level int) *index.FreqAndNormBuffer {
	buf := index.NewFreqAndNormBuffer()
	buf.Add(m.postings.freq, 1)
	return buf
}

// toPostingsAndFreq renders each (postings, offset) pair as Lucene's
// PhraseQuery.PostingsAndFreq(postings, impacts, position): no impacts, the
// offset as the phrase position.
func toPostingsAndFreq(ps []struct {
	postings index.PostingsEnum
	offset   int
}) []*postingsAndFreq {
	out := make([]*postingsAndFreq, len(ps))
	for i, p := range ps {
		out[i] = NewPostingsAndFreq(p.postings, nil, p.offset)
	}
	return out
}

func TestExactPhraseMatcher_NextMatch(t *testing.T) {
	tests := []struct {
		name     string
		postings []struct {
			postings index.PostingsEnum
			offset   int
		}
		wantMatch bool
		wantStart int
		wantEnd   int
	}{
		{
			name: "Exact match",
			postings: []struct {
				postings index.PostingsEnum
				offset   int
			}{
				{postings: &mockPostings{docID: 1, freq: 1, positions: []int{10}, offsets: []int{100}, curDoc: -1, curPos: 0}, offset: 0},
				{postings: &mockPostings{docID: 1, freq: 1, positions: []int{11}, offsets: []int{110}, curDoc: -1, curPos: 0}, offset: 1},
			},
			wantMatch: true,
			wantStart: 10,
			wantEnd:   11,
		},
		{
			name: "Non-match (gap)",
			postings: []struct {
				postings index.PostingsEnum
				offset   int
			}{
				{postings: &mockPostings{docID: 1, freq: 1, positions: []int{10}, offsets: []int{100}, curDoc: -1, curPos: 0}, offset: 0},
				{postings: &mockPostings{docID: 1, freq: 1, positions: []int{12}, offsets: []int{120}, curDoc: -1, curPos: 0}, offset: 1},
			},
			wantMatch: false,
		},
		{
			name: "Exact match with multiple occurrences",
			postings: []struct {
				postings index.PostingsEnum
				offset   int
			}{
				{postings: &mockPostings{docID: 1, freq: 2, positions: []int{10, 20}, offsets: []int{100, 200}, curDoc: -1, curPos: 0}, offset: 0},
				{postings: &mockPostings{docID: 1, freq: 2, positions: []int{11, 21}, offsets: []int{110, 210}, curDoc: -1, curPos: 0}, offset: 1},
			},
			wantMatch: true,
			wantStart: 10,
			wantEnd:   11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewExactPhraseMatcher(toPostingsAndFreq(tt.postings), ScoreModeComplete, nil, 1.0)
			matcher.ResetPositions()
			match, err := matcher.NextMatch()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if match != tt.wantMatch {
				t.Errorf("NextMatch() = %v, want %v", match, tt.wantMatch)
			}
			if match && matcher.StartPosition() != tt.wantStart {
				t.Errorf("StartPosition() = %v, want %v", matcher.StartPosition(), tt.wantStart)
			}
			if match && matcher.EndPosition() != tt.wantEnd {
				t.Errorf("EndPosition() = %v, want %v", matcher.EndPosition(), tt.wantEnd)
			}
		})
	}
}

func TestExactPhraseMatcher_Offsets(t *testing.T) {
	postings := []struct {
		postings index.PostingsEnum
		offset   int
	}{
		{postings: &mockPostings{docID: 1, freq: 1, positions: []int{10}, offsets: []int{100}, curDoc: -1, curPos: 0}, offset: 0},
		{postings: &mockPostings{docID: 1, freq: 1, positions: []int{11}, offsets: []int{110}, curDoc: -1, curPos: 0}, offset: 1},
	}
	matcher := NewExactPhraseMatcher(toPostingsAndFreq(postings), ScoreModeComplete, nil, 1.0)
	matcher.ResetPositions()
	matcher.NextMatch()

	startOff, _ := matcher.StartOffset()
	if startOff != 100 {
		t.Errorf("StartOffset() = %v, want 100", startOff)
	}
	endOff, _ := matcher.EndOffset()
	if endOff != 110 {
		t.Errorf("EndOffset() = %v, want 110", endOff)
	}
}
