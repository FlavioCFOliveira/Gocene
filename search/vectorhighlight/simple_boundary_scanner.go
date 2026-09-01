package vectorhighlight

// DefaultMaxScan is the default maximum number of characters to scan
// for a boundary. Mirrors SimpleBoundaryScanner.DEFAULT_MAX_SCAN.
const DefaultMaxScan = 20

// DefaultBoundaryChars are the default characters that trigger a boundary.
// Mirrors SimpleBoundaryScanner.DEFAULT_BOUNDARY_CHARS.
var DefaultBoundaryChars = []rune{'.', ',', '!', '?', ' ', '\t', '\n'}

// SimpleBoundaryScanner is a boundary scanner implementation that divides
// fragments based on a set of separator characters.
// Mirrors org.apache.lucene.search.vectorhighlight.SimpleBoundaryScanner.
type SimpleBoundaryScanner struct {
	maxScan       int
	boundaryRunes map[rune]bool
}

// NewSimpleBoundaryScanner creates a new SimpleBoundaryScanner.
// If maxScan < 1, DefaultMaxScan is used.
// If boundaryChars is empty, DefaultBoundaryChars is used.
func NewSimpleBoundaryScanner(maxScan int, boundaryChars []rune) *SimpleBoundaryScanner {
	if maxScan < 1 {
		maxScan = DefaultMaxScan
	}

	runes := boundaryChars
	if len(runes) == 0 {
		runes = DefaultBoundaryChars
	}

	set := make(map[rune]bool, len(runes))
	for _, r := range runes {
		set[r] = true
	}

	return &SimpleBoundaryScanner{
		maxScan:       maxScan,
		boundaryRunes: set,
	}
}

// FindStartOffset scans backward from start to find a boundary character.
// Mirrors org.apache.lucene.search.vectorhighlight.SimpleBoundaryScanner.findStartOffset.
func (s *SimpleBoundaryScanner) FindStartOffset(text string, start int) int {
	runes := []rune(text)
	if start > len(runes) || start < 1 {
		return start
	}

	offset := start
	count := s.maxScan
	for offset > 0 && count > 0 {
		// found?
		if s.boundaryRunes[runes[offset-1]] {
			return offset
		}
		offset--
		count--
	}

	// if we scanned up to the start of the text, return it, it's a "boundary"
	if offset == 0 {
		return 0
	}

	// not found
	return start
}

// FindEndOffset scans forward from start to find a boundary character.
// Mirrors org.apache.lucene.search.vectorhighlight.SimpleBoundaryScanner.findEndOffset.
func (s *SimpleBoundaryScanner) FindEndOffset(text string, start int) int {
	runes := []rune(text)
	if start > len(runes) || start < 0 {
		return start
	}

	offset := start
	count := s.maxScan
	for offset < len(runes) && count > 0 {
		// found?
		if s.boundaryRunes[runes[offset]] {
			return offset
		}
		offset++
		count--
	}

	// not found
	return start
}

var _ BoundaryScanner = (*SimpleBoundaryScanner)(nil)
