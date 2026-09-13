package uhighlight

// CharArrayMatcher tests whether a substring of a char array matches some
// criterion. Mirrors org.apache.lucene.search.uhighlight.CharArrayMatcher.
type CharArrayMatcher interface {
	// Match reports whether chars[start:start+length] matches.
	Match(chars []rune, start, length int) bool
}

// LiteralCharArrayMatcher matches a fixed string.
type LiteralCharArrayMatcher struct {
	Word []rune
}

// NewLiteralCharArrayMatcher builds the matcher.
func NewLiteralCharArrayMatcher(word string) *LiteralCharArrayMatcher {
	return &LiteralCharArrayMatcher{Word: []rune(word)}
}

// Match reports whether chars[start:start+length] equals Word.
func (m *LiteralCharArrayMatcher) Match(chars []rune, start, length int) bool {
	if length != len(m.Word) || start < 0 || start+length > len(chars) {
		return false
	}
	for i := 0; i < length; i++ {
		if chars[start+i] != m.Word[i] {
			return false
		}
	}
	return true
}

var _ CharArrayMatcher = (*LiteralCharArrayMatcher)(nil)

// LabelledCharArrayMatcher wraps a CharArrayMatcher with a human-readable
// label so callers can identify which matcher fired. Mirrors
// org.apache.lucene.search.uhighlight.LabelledCharArrayMatcher.
type LabelledCharArrayMatcher struct {
	Label string
	Inner CharArrayMatcher
}

// NewLabelledCharArrayMatcher builds the wrapper.
func NewLabelledCharArrayMatcher(label string, inner CharArrayMatcher) *LabelledCharArrayMatcher {
	return &LabelledCharArrayMatcher{Label: label, Inner: inner}
}

// Match delegates to the wrapped matcher.
func (m *LabelledCharArrayMatcher) Match(chars []rune, start, length int) bool {
	if m.Inner == nil {
		return false
	}
	return m.Inner.Match(chars, start, length)
}

var _ CharArrayMatcher = (*LabelledCharArrayMatcher)(nil)
