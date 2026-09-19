package uhighlight

import (
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// CharArrayMatcher matches a character array.
//
// This is the Go port of the interface
// org.apache.lucene.search.uhighlight.CharArrayMatcher from Apache Lucene
// 10.5.0. Java's char[] is rendered as []rune.
type CharArrayMatcher interface {
	// Match returns true if the passed-in character array matches. Mirrors
	// CharArrayMatcher.match(char[] s, int offset, int length).
	Match(chars []rune, start, length int) bool
}

// charArrayMatcherFunc adapts a plain function to CharArrayMatcher. Java
// declares CharArrayMatcher as a functional interface and supplies several of
// its implementations as lambdas (CharArrayMatcher.fromTerms,
// LabelledCharArrayMatcher.wrap, MemoryIndexOffsetStrategy's aggregate
// matcher); a Go interface admits no lambda, so those lambdas are rendered
// through this adapter.
type charArrayMatcherFunc func(chars []rune, start, length int) bool

// Match delegates to the wrapped function.
func (f charArrayMatcherFunc) Match(chars []rune, start, length int) bool {
	return f(chars, start, length)
}

// charArrayMatcherMatchCharsRef renders the default method
// `boolean match(CharsRef chars)` (CharArrayMatcher.java:35). Go interfaces
// carry no default bodies, so it is a free function taking the receiver.
func charArrayMatcherMatchCharsRef(m CharArrayMatcher, chars *util.CharsRef) bool {
	return m.Match(chars.Chars, chars.Offset, chars.Length)
}

// CharArrayMatcherFromTerms renders the static factory
// `CharArrayMatcher.fromTerms(List<BytesRef> terms)`
// (CharArrayMatcher.java:39). Go has no interface statics, so the owning
// interface is carried in the name.
func CharArrayMatcherFromTerms(terms []*util.BytesRef) CharArrayMatcher {
	a := automaton.NewCharacterRunAutomaton(automaton.MakeStringUnion(terms))
	return charArrayMatcherFunc(a.RunRunes)
}

// LabelledCharArrayMatcher associates a label with a CharArrayMatcher to
// distinguish different sources for terms in highlighting.
//
// This is the Go port of the interface
// org.apache.lucene.search.uhighlight.LabelledCharArrayMatcher from Apache
// Lucene 10.5.0; its single implementation in Lucene is the anonymous class
// returned by wrap, which is rendered here as the struct itself.
type LabelledCharArrayMatcher struct {
	Label string
	Inner CharArrayMatcher
}

// NewLabelledCharArrayMatcher renders the static factory
// `LabelledCharArrayMatcher.wrap(String label, CharArrayMatcher in)`
// (LabelledCharArrayMatcher.java:36).
func NewLabelledCharArrayMatcher(label string, inner CharArrayMatcher) *LabelledCharArrayMatcher {
	return &LabelledCharArrayMatcher{Label: label, Inner: inner}
}

// GetLabel renders LabelledCharArrayMatcher.getLabel()
// (LabelledCharArrayMatcher.java:33).
func (m *LabelledCharArrayMatcher) GetLabel() string { return m.Label }

// Match delegates to the wrapped matcher, as the anonymous subclass of
// LabelledCharArrayMatcher.wrap does.
func (m *LabelledCharArrayMatcher) Match(chars []rune, start, length int) bool {
	if m.Inner == nil {
		return false
	}
	return m.Inner.Match(chars, start, length)
}

var _ CharArrayMatcher = (*LabelledCharArrayMatcher)(nil)
