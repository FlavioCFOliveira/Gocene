package uhighlight

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// TokenStreamOffsetStrategy analyzes the text, producing a single OffsetsEnum
// wrapping the TokenStream filtered to terms in the query, including
// wildcards. It can't handle position-sensitive queries (phrases). Passage
// accuracy suffers because the freq() is unknown -- it's always
// Integer.MAX_VALUE instead.
//
// This is the Go port of
// org.apache.lucene.search.uhighlight.TokenStreamOffsetStrategy from Apache
// Lucene 10.5.0.
type TokenStreamOffsetStrategy struct {
	*AnalysisOffsetStrategy

	combinedAutomata []CharArrayMatcher
}

// NewTokenStreamOffsetStrategy renders
// `TokenStreamOffsetStrategy(UHComponents components, Analyzer indexAnalyzer)`
// (TokenStreamOffsetStrategy.java:41). Java asserts
// components.phraseHelper().hasPositionSensitivity() == false.
func NewTokenStreamOffsetStrategy(components *UHComponents, indexAnalyzer analysis.Analyzer) *TokenStreamOffsetStrategy {
	s := &TokenStreamOffsetStrategy{
		AnalysisOffsetStrategy: NewAnalysisOffsetStrategy(components, indexAnalyzer),
	}
	matchers := make([]CharArrayMatcher, 0, len(components.Automata))
	for _, a := range components.Automata {
		matchers = append(matchers, a)
	}
	s.combinedAutomata = convertTermsToMatchers(components.Terms, matchers)
	return s
}

// convertTermsToMatchers renders the private static
// TokenStreamOffsetStrategy.convertTermsToMatchers(BytesRef[],
// CharArrayMatcher[]) (TokenStreamOffsetStrategy.java:48).
//
// TODO (from Java) this is inefficient; instead build a union automata just
// for terms part.
func convertTermsToMatchers(terms []*util.BytesRef, matchers []CharArrayMatcher) []CharArrayMatcher {
	newAutomata := make([]CharArrayMatcher, len(terms)+len(matchers))
	for i := 0; i < len(terms); i++ {
		termString := string(bytesRefValue(terms[i]))
		a := automaton.NewCharacterRunAutomaton(automaton.MakeString(termString))
		newAutomata[i] = NewLabelledCharArrayMatcher(termString, charArrayMatcherFunc(a.RunRunes))
	}
	// Append existing automata (that which is used for MTQs)
	copy(newAutomata[len(terms):], matchers)
	return newAutomata
}

// GetOffsetsEnum renders
// TokenStreamOffsetStrategy.getOffsetsEnum(LeafReader, int, String)
// (TokenStreamOffsetStrategy.java:62).
func (s *TokenStreamOffsetStrategy) GetOffsetsEnum(_ index.LeafReader, _ int, content string) (OffsetsEnum, error) {
	tokenStream, err := s.TokenStream(content)
	if err != nil {
		return nil, err
	}
	return newTokenStreamOffsetsEnum(tokenStream, s.combinedAutomata)
}

// tokenStreamOffsetsEnum renders the private static class
// TokenStreamOffsetStrategy.TokenStreamOffsetsEnum
// (TokenStreamOffsetStrategy.java:67).
type tokenStreamOffsetsEnum struct {
	BaseOffsetsEnum

	stream      analysis.TokenStream // becomes nil when closed
	matchers    []CharArrayMatcher
	charTermAtt analysis.CharTermAttribute
	offsetAtt   analysis.OffsetAttribute

	currentMatch int

	matchDescriptions [][]byte
}

// newTokenStreamOffsetsEnum renders `TokenStreamOffsetsEnum(TokenStream ts,
// CharArrayMatcher[] matchers)` (TokenStreamOffsetStrategy.java:76).
func newTokenStreamOffsetsEnum(ts analysis.TokenStream, matchers []CharArrayMatcher) (*tokenStreamOffsetsEnum, error) {
	e := &tokenStreamOffsetsEnum{
		stream:            ts,
		matchers:          matchers,
		currentMatch:      -1,
		matchDescriptions: make([][]byte, len(matchers)),
	}
	src := attributeSourceFor(ts)
	if src == nil {
		return nil, fmt.Errorf("uhighlight: TokenStreamOffsetsEnum: the token stream exposes no AttributeSource")
	}
	e.charTermAtt, _ = src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	e.offsetAtt, _ = src.GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	if e.charTermAtt == nil || e.offsetAtt == nil {
		return nil, fmt.Errorf("uhighlight: TokenStreamOffsetsEnum: the token stream lacks a CharTermAttribute or OffsetAttribute")
	}
	if err := ts.Reset(); err != nil {
		return nil, err
	}
	return e, nil
}

// NextPosition renders TokenStreamOffsetsEnum.nextPosition().
func (e *tokenStreamOffsetsEnum) NextPosition() (bool, error) {
	if e.stream != nil {
		for {
			more, err := e.stream.IncrementToken()
			if err != nil {
				return false, err
			}
			if !more {
				break
			}
			chars := []rune(e.charTermAtt.String())
			for i := 0; i < len(e.matchers); i++ {
				if e.matchers[i].Match(chars, 0, len(chars)) {
					e.currentMatch = i
					return true, nil
				}
			}
		}
		if err := e.stream.End(); err != nil {
			return false, err
		}
		if err := e.Close(); err != nil {
			return false, err
		}
	}
	// exhausted
	return false, nil
}

// Freq renders TokenStreamOffsetsEnum.freq(), which lies with
// Integer.MAX_VALUE.
func (e *tokenStreamOffsetsEnum) Freq() (int, error) { return math.MaxInt32, nil }

// StartOffset renders TokenStreamOffsetsEnum.startOffset().
func (e *tokenStreamOffsetsEnum) StartOffset() (int, error) { return e.offsetAtt.StartOffset(), nil }

// EndOffset renders TokenStreamOffsetsEnum.endOffset().
func (e *tokenStreamOffsetsEnum) EndOffset() (int, error) { return e.offsetAtt.EndOffset(), nil }

// GetTerm renders TokenStreamOffsetsEnum.getTerm(). Java builds the
// description from the matcher's toString(); the matchers created by
// LabelledCharArrayMatcher.wrap do not override it, so Lucene's value is the
// default object string, which Go's %v renders.
func (e *tokenStreamOffsetsEnum) GetTerm() ([]byte, error) {
	if e.currentMatch < 0 || e.currentMatch >= len(e.matchDescriptions) {
		return nil, nil
	}
	if e.matchDescriptions[e.currentMatch] == nil {
		e.matchDescriptions[e.currentMatch] = []byte(fmt.Sprintf("%v", e.matchers[e.currentMatch]))
	}
	return e.matchDescriptions[e.currentMatch], nil
}

// Close renders TokenStreamOffsetsEnum.close().
func (e *tokenStreamOffsetsEnum) Close() error {
	if e.stream != nil {
		err := e.stream.Close()
		e.stream = nil
		return err
	}
	return nil
}

// String renders the inherited OffsetsEnum.toString().
func (e *tokenStreamOffsetsEnum) String() string { return offsetsEnumString(e) }

var _ OffsetsEnum = (*tokenStreamOffsetsEnum)(nil)

var _ FieldOffsetStrategy = (*TokenStreamOffsetStrategy)(nil)
