package uhighlight

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestPassage(t *testing.T) {
	p := NewPassage()
	p.SetStartOffset(0)
	p.SetEndOffset(10)
	p.AddMatch(2, 5, util.BytesRef("test"), 1)
	p.AddMatch(6, 8, util.BytesRef("foo"), 2)

	if p.NumMatches() != 2 {
		t.Errorf("expected 2 matches, got %d", p.NumMatches())
	}
	if p.StartOffset() != 0 || p.EndOffset() != 10 {
		t.Errorf("offsets wrong: %d-%d", p.StartOffset(), p.EndOffset())
	}
}

func TestPassageScorer(t *testing.T) {
	ps := NewPassageScorer()
	p := NewPassage()
	p.SetStartOffset(0)
	p.SetEndOffset(10)
	p.AddMatch(2, 5, util.BytesRef("test"), 1)

	score := ps.Score(p, 100)
	if score <= 0 {
		t.Errorf("expected positive score, got %f", score)
	}
}

func TestDefaultPassageFormatter(t *testing.T) {
	f := NewDefaultPassageFormatter()
	p := NewPassage()
	p.SetStartOffset(0)
	p.SetEndOffset(10)
	p.AddMatch(2, 5, util.BytesRef("test"), 1)

	passages := []*Passage{p}
	content := "hello test world"
	res := f.Format(passages, content)
	if res == "" {
		t.Error("expected non-empty result")
	}
}
