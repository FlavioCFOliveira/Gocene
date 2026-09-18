package uhighlight

import (
	"strings"
)

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

// SplittingBreakIterator is a BreakIterator that "sees" one or more
// characters as the boundary of a slice of the text, and it delegates the
// boundaries inside each slice to a wrapped BreakIterator.
//
// This is the Go port of
// org.apache.lucene.search.uhighlight.SplittingBreakIterator from Apache
// Lucene 10.5.0.
//
// Java's class extends java.text.BreakIterator, a stateful cursor
// (setText/current/first/last/next/previous/isBoundary/clone) that the JVM
// supplies and Go does not. Gocene models a BreakIterator as the stateless
// pair Following/Preceding, which carries the text on every call, so the
// sliceStartIdx / sliceEndIdx bookkeeping Java keeps between calls is
// recomputed here from text and the offset, and baseIter.first() /
// baseIter.last() inside a slice are its two ends.
type SplittingBreakIterator struct {
	baseIter  BreakIterator
	sliceChar rune
}

// NewSplittingBreakIterator builds the iterator over baseIter, slicing the
// text at every occurrence of sliceChar.
func NewSplittingBreakIterator(baseIter BreakIterator, sliceChar rune) *SplittingBreakIterator {
	return &SplittingBreakIterator{baseIter: baseIter, sliceChar: sliceChar}
}

// sliceAt returns the [start, end) slice of text that contains offset, where
// the slice boundaries are the sliceChar occurrences around it. It renders the
// sliceStartIdx / sliceEndIdx computation of
// SplittingBreakIterator.following(int).
func (b *SplittingBreakIterator) sliceAt(text string, offset int) (int, int) {
	sliceStartIdx := lastIndexRuneBefore(text, b.sliceChar, offset) // no +1
	if sliceStartIdx == -1 {
		sliceStartIdx = 0
	} else {
		sliceStartIdx++ // move past separator
	}
	from := offset + 1
	if sliceStartIdx > from {
		from = sliceStartIdx
	}
	sliceEndIdx := indexRuneFrom(text, b.sliceChar, from)
	if sliceEndIdx == -1 {
		sliceEndIdx = len(text)
	}
	return sliceStartIdx, sliceEndIdx
}

// Following returns the first boundary after pos, or -1 when none exists.
// Mirrors SplittingBreakIterator.following(int).
func (b *SplittingBreakIterator) Following(text string, pos int) int {
	if pos >= len(text) { // DONE condition
		return -1
	}
	sliceStartIdx, sliceEndIdx := b.sliceAt(text, pos)

	// lookup following() in this slice:
	if sliceStartIdx == sliceEndIdx { // adjacent separator or separator at end
		return pos + 1
	}
	// note: following() can never be first() if the first character is a
	// boundary (it usually is). So we have to check if we should call first()
	// instead of following():
	if pos == sliceStartIdx-1 {
		// the first boundary following this offset is the very first boundary
		// in this slice
		return sliceStartIdx
	}
	following := b.baseIter.Following(text[sliceStartIdx:sliceEndIdx], pos-sliceStartIdx)
	if following == -1 {
		return -1
	}
	return sliceStartIdx + following
}

// Preceding returns the last boundary before pos, or 0 when none exists.
// Mirrors SplittingBreakIterator.preceding(int); Java reports "no boundary" as
// BreakIterator.DONE, while the Gocene BreakIterator contract reports it as 0.
func (b *SplittingBreakIterator) Preceding(text string, pos int) int {
	if pos <= 0 { // DONE condition
		return 0
	}
	sliceEndIdx := indexRuneFrom(text, b.sliceChar, pos) // no -1
	if sliceEndIdx == -1 {
		sliceEndIdx = len(text)
	}
	sliceStartIdx := lastIndexRuneBefore(text, b.sliceChar, pos-1)
	if sliceStartIdx == -1 {
		sliceStartIdx = 0
	} else {
		sliceStartIdx++
		if sliceStartIdx > sliceEndIdx {
			sliceStartIdx = sliceEndIdx
		}
	}

	// lookup preceding() in this slice:
	if sliceStartIdx == sliceEndIdx { // adjacent separator or separator at end
		return pos - 1
	}
	// note: preceding() can never be last() if the last character is a
	// boundary (it usually is). So we have to check if we should call last()
	// instead of preceding():
	if pos == sliceEndIdx+1 {
		// the last boundary preceding this offset is the very last boundary in
		// this slice
		return sliceEndIdx
	}
	return sliceStartIdx + b.baseIter.Preceding(text[sliceStartIdx:sliceEndIdx], pos-sliceStartIdx)
}

// indexRuneFrom renders String.indexOf(char, int): the index of the first r at
// or after from, or -1.
func indexRuneFrom(text string, r rune, from int) int {
	if from < 0 {
		from = 0
	}
	if from >= len(text) {
		return -1
	}
	i := strings.IndexRune(text[from:], r)
	if i == -1 {
		return -1
	}
	return from + i
}

// lastIndexRuneBefore renders String.lastIndexOf(char, int): the index of the
// last r at or before to, or -1.
func lastIndexRuneBefore(text string, r rune, to int) int {
	if to < 0 {
		return -1
	}
	if to >= len(text) {
		to = len(text) - 1
	}
	return strings.LastIndex(text[:to+1], string(r))
}

var _ BreakIterator = (*SplittingBreakIterator)(nil)

// WholeBreakIterator treats the entire input as a single segment.
// Mirrors org.apache.lucene.search.uhighlight.WholeBreakIterator.
type WholeBreakIterator struct{}

// Following returns the end of the text (or -1 once already at the end).
func (WholeBreakIterator) Following(text string, pos int) int {
	if pos >= len(text) {
		return -1
	}
	return len(text)
}

// Preceding returns 0 (the only valid preceding boundary) or 0 when pos
// is already at the start.
func (WholeBreakIterator) Preceding(text string, pos int) int {
	_ = text
	if pos <= 0 {
		return 0
	}
	return 0
}

var _ BreakIterator = WholeBreakIterator{}

// SentenceBreakIterator splits text at sentence boundaries (periods,
// question marks, exclamation marks) followed by whitespace. This is the
// Go port of the JDK BreakIterator.getSentenceInstance() default the
// Lucene UH uses when no explicit BreakIterator is configured.
//
// The implementation is intentionally lightweight: it matches the
// terminator + whitespace pattern in linear time and is suitable for
// English-style sentences. A full ICU-backed sentence iterator is a
// separate concern.
type SentenceBreakIterator struct{}

// Following returns the index immediately after the next sentence
// terminator + whitespace pair, or len(text) when the tail of the text
// is reached, or -1 when pos is already at the end.
func (SentenceBreakIterator) Following(text string, pos int) int {
	if pos >= len(text) {
		return -1
	}
	for i := pos + 1; i < len(text); i++ {
		c := text[i]
		if c != '.' && c != '!' && c != '?' {
			continue
		}
		// Lookahead: consume any run of repeated terminators (e.g. "!!!").
		j := i + 1
		for j < len(text) && (text[j] == '.' || text[j] == '!' || text[j] == '?') {
			j++
		}
		if j >= len(text) {
			return len(text)
		}
		if text[j] == ' ' || text[j] == '\t' || text[j] == '\n' || text[j] == '\r' {
			// Boundary lands AFTER the terminator+whitespace so the
			// next passage starts at the first non-space char.
			k := j + 1
			for k < len(text) && (text[k] == ' ' || text[k] == '\t' || text[k] == '\n' || text[k] == '\r') {
				k++
			}
			return k
		}
	}
	return len(text)
}

// Preceding returns the start of the sentence containing pos, derived by
// walking backwards until a terminator + whitespace boundary is found.
func (SentenceBreakIterator) Preceding(text string, pos int) int {
	if pos <= 0 {
		return 0
	}
	if pos > len(text) {
		pos = len(text)
	}
	for i := pos - 1; i > 0; i-- {
		c := text[i-1]
		if c != '.' && c != '!' && c != '?' {
			continue
		}
		// Next char must be whitespace for a true sentence boundary.
		if i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
			// Skip the whitespace run to land at the start of the next sentence.
			k := i + 1
			for k < pos && (text[k] == ' ' || text[k] == '\t' || text[k] == '\n' || text[k] == '\r') {
				k++
			}
			return k
		}
	}
	return 0
}

var _ BreakIterator = SentenceBreakIterator{}
