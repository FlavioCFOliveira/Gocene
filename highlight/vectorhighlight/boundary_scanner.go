// Package vectorhighlight implements
// org.apache.lucene.search.vectorhighlight: the term-vector-driven
// fast-vector highlighter.
package vectorhighlight

import "unicode"

// BoundaryScanner widens highlight fragments so they break on natural text
// boundaries instead of mid-token. Mirrors
// org.apache.lucene.search.vectorhighlight.BoundaryScanner.
type BoundaryScanner interface {
	// FindStartOffset walks backward from start until a boundary is found.
	FindStartOffset(text string, start int) int
	// FindEndOffset walks forward from end until a boundary is found.
	FindEndOffset(text string, end int) int
}

// SimpleBoundaryScanner walks character by character looking for one of the
// configured boundary characters within MaxScan steps. Mirrors
// org.apache.lucene.search.vectorhighlight.SimpleBoundaryScanner.
type SimpleBoundaryScanner struct {
	MaxScan        int
	BoundaryRunes  map[rune]bool
}

// NewSimpleBoundaryScanner builds the scanner.
func NewSimpleBoundaryScanner(maxScan int, boundaryChars []rune) *SimpleBoundaryScanner {
	if maxScan < 1 {
		maxScan = 20
	}
	set := make(map[rune]bool, len(boundaryChars))
	for _, r := range boundaryChars {
		set[r] = true
	}
	if len(set) == 0 {
		set[' '] = true
		set['\t'] = true
		set['\n'] = true
		set['\r'] = true
		set['.'] = true
		set[','] = true
		set['!'] = true
		set['?'] = true
	}
	return &SimpleBoundaryScanner{MaxScan: maxScan, BoundaryRunes: set}
}

// FindStartOffset walks backward from start.
func (s *SimpleBoundaryScanner) FindStartOffset(text string, start int) int {
	if start < 0 {
		return 0
	}
	if start > len(text) {
		start = len(text)
	}
	limit := start - s.MaxScan
	if limit < 0 {
		limit = 0
	}
	for i := start; i > limit; i-- {
		if i <= 0 {
			return 0
		}
		r := rune(text[i-1])
		if s.BoundaryRunes[r] {
			return i
		}
	}
	return limit
}

// FindEndOffset walks forward from end.
func (s *SimpleBoundaryScanner) FindEndOffset(text string, end int) int {
	if end >= len(text) {
		return len(text)
	}
	if end < 0 {
		end = 0
	}
	limit := end + s.MaxScan
	if limit > len(text) {
		limit = len(text)
	}
	for i := end; i < limit; i++ {
		r := rune(text[i])
		if s.BoundaryRunes[r] {
			return i
		}
	}
	return limit
}

var _ BoundaryScanner = (*SimpleBoundaryScanner)(nil)

// BreakIteratorBoundaryScanner uses an external sentence-detection function
// to choose boundaries. Mirrors
// org.apache.lucene.search.vectorhighlight.BreakIteratorBoundaryScanner.
type BreakIteratorBoundaryScanner struct {
	SentenceBoundary func(text string, pos int) (start, end int)
}

// NewBreakIteratorBoundaryScanner builds the scanner.
func NewBreakIteratorBoundaryScanner(boundary func(string, int) (int, int)) *BreakIteratorBoundaryScanner {
	return &BreakIteratorBoundaryScanner{SentenceBoundary: boundary}
}

// FindStartOffset walks back to the start of the surrounding sentence.
func (s *BreakIteratorBoundaryScanner) FindStartOffset(text string, start int) int {
	if s.SentenceBoundary == nil {
		return defaultBoundaryStart(text, start)
	}
	a, _ := s.SentenceBoundary(text, start)
	return a
}

// FindEndOffset walks forward to the end of the surrounding sentence.
func (s *BreakIteratorBoundaryScanner) FindEndOffset(text string, end int) int {
	if s.SentenceBoundary == nil {
		return defaultBoundaryEnd(text, end)
	}
	_, b := s.SentenceBoundary(text, end)
	return b
}

var _ BoundaryScanner = (*BreakIteratorBoundaryScanner)(nil)

func defaultBoundaryStart(text string, pos int) int {
	if pos <= 0 {
		return 0
	}
	for i := pos; i > 0; i-- {
		if unicode.IsSpace(rune(text[i-1])) {
			return i
		}
	}
	return 0
}

func defaultBoundaryEnd(text string, pos int) int {
	if pos >= len(text) {
		return len(text)
	}
	for i := pos; i < len(text); i++ {
		if unicode.IsSpace(rune(text[i])) {
			return i
		}
	}
	return len(text)
}
