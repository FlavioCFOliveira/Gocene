// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

// TextFragment is one slice of the source text that the highlighter has
// chosen to emit. Mirrors org.apache.lucene.search.highlight.TextFragment.
type TextFragment struct {
	MarkedUpText  string
	FragNum       int
	TextStartPos  int
	TextEndPos    int
	Score         float32
}

// NewTextFragment builds a TextFragment.
func NewTextFragment(markedUp string, fragNum, start, end int) *TextFragment {
	return &TextFragment{
		MarkedUpText: markedUp,
		FragNum:      fragNum,
		TextStartPos: start,
		TextEndPos:   end,
	}
}

// GetScore returns the fragment's score.
func (f *TextFragment) GetScore() float32 { return f.Score }

// SetScore stamps a score on the fragment.
func (f *TextFragment) SetScore(s float32) { f.Score = s }

// GetFragNum returns the fragment number.
func (f *TextFragment) GetFragNum() int { return f.FragNum }

// Follows reports whether this fragment lies after other in the source text.
func (f *TextFragment) Follows(other *TextFragment) bool {
	if other == nil {
		return true
	}
	return f.TextStartPos >= other.TextEndPos
}

// String returns the MarkedUpText.
func (f *TextFragment) String() string { return f.MarkedUpText }
