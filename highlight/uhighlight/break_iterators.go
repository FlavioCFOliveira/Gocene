package uhighlight

import "unicode"

// BreakIterator is the minimal contract the uhighlight package needs from a
// text segmenter: given a text and a starting position, return the boundary
// of the next segment. Mirrors java.text.BreakIterator slice used by
// uhighlight.
type BreakIterator interface {
	// Following returns the index of the boundary strictly after pos.
	// Returns -1 when no further boundary exists.
	Following(text string, pos int) int

	// Preceding returns the index of the boundary strictly before pos, or
	// 0 when none exists.
	Preceding(text string, pos int) int
}

// CustomSeparatorBreakIterator splits text on a fixed separator character.
// Mirrors org.apache.lucene.search.uhighlight.CustomSeparatorBreakIterator.
type CustomSeparatorBreakIterator struct {
	Sep rune
}

// NewCustomSeparatorBreakIterator builds the iterator.
func NewCustomSeparatorBreakIterator(sep rune) *CustomSeparatorBreakIterator {
	return &CustomSeparatorBreakIterator{Sep: sep}
}

// Following walks forward from pos until Sep is found.
func (b *CustomSeparatorBreakIterator) Following(text string, pos int) int {
	for i, r := range text {
		if i <= pos {
			continue
		}
		if r == b.Sep {
			return i
		}
	}
	return -1
}

// Preceding walks backward from pos until Sep is found.
func (b *CustomSeparatorBreakIterator) Preceding(text string, pos int) int {
	last := 0
	for i, r := range text {
		if i >= pos {
			break
		}
		if r == b.Sep {
			last = i + 1
		}
	}
	return last
}

var _ BreakIterator = (*CustomSeparatorBreakIterator)(nil)

// LengthGoalBreakIterator wraps another BreakIterator and stretches every
// segment until it reaches the configured character length. Mirrors
// org.apache.lucene.search.uhighlight.LengthGoalBreakIterator.
type LengthGoalBreakIterator struct {
	Inner      BreakIterator
	LengthGoal int
}

// NewLengthGoalBreakIterator builds the wrapper.
func NewLengthGoalBreakIterator(inner BreakIterator, lengthGoal int) *LengthGoalBreakIterator {
	if lengthGoal < 1 {
		lengthGoal = 1
	}
	return &LengthGoalBreakIterator{Inner: inner, LengthGoal: lengthGoal}
}

// Following advances the inner iterator until the cumulative segment length
// reaches LengthGoal.
func (b *LengthGoalBreakIterator) Following(text string, pos int) int {
	target := pos + b.LengthGoal
	next := b.Inner.Following(text, pos)
	for next > 0 && next < target {
		nn := b.Inner.Following(text, next)
		if nn < 0 {
			break
		}
		next = nn
	}
	return next
}

// Preceding delegates to the inner iterator.
func (b *LengthGoalBreakIterator) Preceding(text string, pos int) int {
	return b.Inner.Preceding(text, pos)
}

var _ BreakIterator = (*LengthGoalBreakIterator)(nil)

// SplittingBreakIterator is a basic BreakIterator that breaks on any whitespace.
// Mirrors org.apache.lucene.search.uhighlight.SplittingBreakIterator.
type SplittingBreakIterator struct{}

// Following walks forward from pos to the next whitespace boundary.
func (SplittingBreakIterator) Following(text string, pos int) int {
	for i, r := range text {
		if i <= pos {
			continue
		}
		if unicode.IsSpace(r) {
			return i
		}
	}
	return -1
}

// Preceding walks backward from pos to the previous whitespace boundary.
func (SplittingBreakIterator) Preceding(text string, pos int) int {
	last := 0
	for i, r := range text {
		if i >= pos {
			break
		}
		if unicode.IsSpace(r) {
			last = i + 1
		}
	}
	return last
}

var _ BreakIterator = SplittingBreakIterator{}
