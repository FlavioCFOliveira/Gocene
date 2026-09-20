package uhighlight

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/memory"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// MemoryIndexOffsetStrategy uses an Analyzer on content to get offsets and
// then populates a MemoryIndex.
//
// This is the Go port of
// org.apache.lucene.search.uhighlight.MemoryIndexOffsetStrategy from Apache
// Lucene 10.5.0.
type MemoryIndexOffsetStrategy struct {
	*AnalysisOffsetStrategy

	memoryIndex                *memory.MemoryIndex
	memIndexLeafReader         index.LeafReader
	preMemIndexFilterAutomaton CharArrayMatcher
}

// NewMemoryIndexOffsetStrategy renders
// `MemoryIndexOffsetStrategy(UHComponents components, Analyzer analyzer)`
// (MemoryIndexOffsetStrategy.java:48).
func NewMemoryIndexOffsetStrategy(components *UHComponents, analyzer analysis.Analyzer) *MemoryIndexOffsetStrategy {
	s := &MemoryIndexOffsetStrategy{
		AnalysisOffsetStrategy: NewAnalysisOffsetStrategy(components, analyzer),
	}
	storePayloads := components.PhraseHelper.HasPositionSensitivity() // might be needed
	s.memoryIndex = memory.NewMemoryIndexWithOffsetsAndPayloads(true, storePayloads)
	// true==store offsets
	reader := s.memoryIndex.CreateSearcher().GetIndexReader() // appears to be re-usable
	leafReader, ok := reader.(index.LeafReader)
	if !ok {
		// Renders the ClassCastException of Java's `(LeafReader)` cast.
		panic(fmt.Sprintf("uhighlight: MemoryIndex reader is not a LeafReader, got %T", reader))
	}
	s.memIndexLeafReader = leafReader
	// preFilter for MemoryIndex
	s.preMemIndexFilterAutomaton = buildCombinedAutomaton(components)
	return s
}

// buildCombinedAutomaton builds one CharArrayMatcher matching any term the
// query might match. Renders the private static
// MemoryIndexOffsetStrategy.buildCombinedAutomaton(UHComponents)
// (MemoryIndexOffsetStrategy.java:60).
func buildCombinedAutomaton(components *UHComponents) CharArrayMatcher {
	// We don't know enough about the query to do this confidently
	if components.Terms == nil || components.Automata == nil {
		return nil
	}

	allAutomata := make([]CharArrayMatcher, 0)
	if len(components.Terms) > 0 {
		// Filter out any long terms that would otherwise cause exceptions if we tried
		// to build an automaton on them
		filteredTerms := make([]*util.BytesRef, 0, len(components.Terms))
		for _, b := range components.Terms {
			if b.Length < automaton.MaxStringUnionTermLength {
				filteredTerms = append(filteredTerms, b)
			}
		}
		allAutomata = append(allAutomata, CharArrayMatcherFromTerms(filteredTerms))
	}
	for _, a := range components.Automata {
		allAutomata = append(allAutomata, a)
	}
	for spanQuery := range components.PhraseHelper.GetSpanQueries() {
		for _, a := range ExtractAutomata(spanQuery, components.FieldMatcher, true) { // true==lookInSpan
			allAutomata = append(allAutomata, a)
		}
	}

	if len(allAutomata) == 1 {
		return allAutomata[0]
	}

	// TODO (from Java) it'd be nice if we could get at the underlying Automaton in
	//  CharacterRunAutomaton so that we could union them all. But it's not exposed, and
	//  sometimes the automaton is byte (not char) oriented

	// Return an aggregate CharArrayMatcher of others
	return charArrayMatcherFunc(func(chars []rune, offset, length int) bool {
		for i := 0; i < len(allAutomata); i++ {
			// don't use foreach to avoid Iterator allocation
			if allAutomata[i].Match(chars, offset, length) {
				return true
			}
		}
		return false
	})
}

// GetOffsetsEnum renders
// MemoryIndexOffsetStrategy.getOffsetsEnum(LeafReader, int, String)
// (MemoryIndexOffsetStrategy.java:108).
func (s *MemoryIndexOffsetStrategy) GetOffsetsEnum(reader index.LeafReader, docID int, content string) (OffsetsEnum, error) {
	// note: don't need LimitTokenOffsetFilter since content is already truncated to maxLength
	tokenStream, err := s.TokenStream(content)
	if err != nil {
		return nil, err
	}

	// Filter the tokenStream to applicable terms
	if s.preMemIndexFilterAutomaton != nil {
		tokenStream = newKeepWordFilter(tokenStream, s.preMemIndexFilterAutomaton)
	}
	s.memoryIndex.Reset()
	// note: calls tokenStream.reset() & close()
	if err := s.memoryIndex.AddField(s.Field(), tokenStream); err != nil {
		return nil, err
	}

	if reader == nil {
		return s.CreateOffsetsEnumFromReader(s.memIndexLeafReader, 0)
	}
	return s.CreateOffsetsEnumFromReader(
		NewOverlaySingleDocTermsLeafReader(reader, s.memIndexLeafReader, s.Field(), docID),
		docID)
}

// newKeepWordFilter renders the private static
// MemoryIndexOffsetStrategy.newKeepWordFilter(TokenStream, CharArrayMatcher)
// (MemoryIndexOffsetStrategy.java:131).
//
// It'd be nice to use KeepWordFilter but it demands a CharArraySet.
func newKeepWordFilter(tokenStream analysis.TokenStream, matcher CharArrayMatcher) analysis.TokenStream {
	var charAtt analysis.CharTermAttribute
	filter := analysis.NewFilteringTokenFilter(tokenStream, func() (bool, error) {
		if charAtt == nil {
			return false, nil
		}
		chars := []rune(charAtt.String())
		return matcher.Match(chars, 0, len(chars)), nil
	})
	if src := filter.GetAttributeSource(); src != nil {
		charAtt, _ = src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	}
	return filter
}

var _ FieldOffsetStrategy = (*MemoryIndexOffsetStrategy)(nil)
