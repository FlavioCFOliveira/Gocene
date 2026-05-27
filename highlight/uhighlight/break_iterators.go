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
