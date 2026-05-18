package highlight

import "github.com/FlavioCFOliveira/Gocene/analysis"

// TokenStreamFromTermVector materialises a TokenStream from a
// TermVectorLeafReader so the highlighter can replay the indexed token
// stream of a document without re-tokenising it. Mirrors
// org.apache.lucene.search.highlight.TokenStreamFromTermVector.
type TokenStreamFromTermVector struct {
	tokens []string
	idx    int
}

// NewTokenStreamFromTermVector flattens reader's term vector into a sorted
// position-major sequence of tokens.
func NewTokenStreamFromTermVector(reader *TermVectorLeafReader) *TokenStreamFromTermVector {
	maxPos := -1
	for _, e := range reader.Terms() {
		for _, p := range e.Positions {
			if p > maxPos {
				maxPos = p
			}
		}
	}
	if maxPos < 0 {
		return &TokenStreamFromTermVector{}
	}
	out := make([]string, maxPos+1)
	for _, e := range reader.Terms() {
		for _, p := range e.Positions {
			out[p] = e.Term
		}
	}
	return &TokenStreamFromTermVector{tokens: out, idx: -1}
}

// IncrementToken advances to the next token.
func (s *TokenStreamFromTermVector) IncrementToken() (bool, error) {
	s.idx++
	if s.idx >= len(s.tokens) {
		return false, nil
	}
	return true, nil
}

// CurrentToken returns the active token text (or "" when before-start /
// past-end).
func (s *TokenStreamFromTermVector) CurrentToken() string {
	if s.idx < 0 || s.idx >= len(s.tokens) {
		return ""
	}
	return s.tokens[s.idx]
}

// End is a no-op.
func (s *TokenStreamFromTermVector) End() error { return nil }

// Close is a no-op.
func (s *TokenStreamFromTermVector) Close() error { return nil }

var _ analysis.TokenStream = (*TokenStreamFromTermVector)(nil)
