package vectorhighlight

import (
	"github.com/FlavioCFOliveira/Gocene/highlight"
)

// BreakIteratorBoundaryScanner is a BoundaryScanner implementation that uses a BreakIterator to find boundaries in the text.
// Mirrors org.apache.lucene.search.vectorhighlight.BreakIteratorBoundaryScanner.
type BreakIteratorBoundaryScanner struct {
	bi highlight.BreakIterator
}

// NewBreakIteratorBoundaryScanner builds the scanner.
// Mirrors org.apache.lucene.search.vectorhighlight.BreakIteratorBoundaryScanner(BreakIterator bi).
func NewBreakIteratorBoundaryScanner(bi highlight.BreakIterator) *BreakIteratorBoundaryScanner {
	return &BreakIteratorBoundaryScanner{
		bi: bi,
	}
}

// FindStartOffset scans backward to find the start offset.
// Mirrors org.apache.lucene.search.vectorhighlight.BreakIteratorBoundaryScanner.findStartOffset.
func (s *BreakIteratorBoundaryScanner) FindStartOffset(text string, start int) int {
	// avoid illegal start offset
	if start > len(text) || start < 1 {
		return start
	}

	// Mirrors:
	// bi.setText(buffer.substring(0, start));
	// bi.last();
	// return bi.previous();
	s.bi.SetText(text[:start])
	s.bi.Last()
	return s.bi.Previous()
}

// FindEndOffset scans forward to find the end offset.
// Mirrors org.apache.lucene.search.vectorhighlight.BreakIteratorBoundaryScanner.findEndOffset.
func (s *BreakIteratorBoundaryScanner) FindEndOffset(text string, start int) int {
	// avoid illegal start offset
	if start > len(text) || start < 0 {
		return start
	}

	// Mirrors:
	// bi.setText(buffer.substring(start));
	// return bi.next() + start;
	s.bi.SetText(text[start:])
	return s.bi.Next() + start
}

var _ BoundaryScanner = (*BreakIteratorBoundaryScanner)(nil)
