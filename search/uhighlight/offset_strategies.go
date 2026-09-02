package uhighlight

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FieldOffsetStrategy is the base for offset retrieval strategies.
type FieldOffsetStrategy interface {
	GetField() string
	GetOffsetSource() OffsetSource
	GetOffsetsEnum(reader index.LeafReader, docID int, content string) (OffsetsEnum, error)
}

type baseFieldOffsetStrategy struct {
	components *UHComponents
}

func (b *baseFieldOffsetStrategy) GetField() string {
	return b.components.Field
}

func (b *baseFieldOffsetStrategy) CreateOffsetsEnumFromReader(leafReader index.LeafReader, doc int) (OffsetsEnum, error) {
	termsIndex := leafReader.Terms(b.GetField())
	if termsIndex == nil {
		return Empty, nil
	}

	var offsetsEnums []OffsetsEnum

	// Handle Weight.matches approach
	if b.components.Flags.Contains(FlagWeightMatches) {
		// This requires a complex FilterLeafReader and IndexSearcher setup.
		// For now, we leave this as a TODO or implement a simplified version.
		// In Lucene, it uses a FilterLeafReader to remap field names.
		// We will implement this in a separate task or as part of the complex strategies.
		return nil, fmt.Errorf("WeightMatches not yet implemented in base strategy")
	} else {
		// classic approach
		var insensitiveTerms []util.BytesRef
		phraseHelper := b.components.PhraseHelper
		terms := b.components.Terms

		if phraseHelper != nil && phraseHelper.HasPositionSensitivity() {
			insensitiveTerms = phraseHelper.GetAllPositionInsensitiveTerms()
		} else {
			insensitiveTerms = terms
		}

		if len(insensitiveTerms) > 0 {
			for _, term := range insensitiveTerms {
				if termsIndex.SeekExact(term) {
					postingsEnum, err := termsIndex.Postings(nil, index.PostingsEnumOffsets)
					if err != nil {
						return nil, err
					}
					if doc == postingsEnum.Advance(doc) {
						oe, err := NewOfPostings(term, postingsEnum)
						if err != nil {
							return nil, err
						}
						offsetsEnums = append(offsetsEnums, oe)
					}
				}
			}
		}

		// Handle spans
		if phraseHelper != nil && phraseHelper.HasPositionSensitivity() {
			spans, err := phraseHelper.CreateOffsetsEnumsForSpans(leafReader, doc)
			if err != nil {
				return nil, err
			}
			offsetsEnums = append(offsetsEnums, spans...)
		}

		// Handle automata
		if len(b.components.Automata) > 0 {
			automataEnums, err := b.createOffsetsEnumsForAutomata(termsIndex, doc)
			if err != nil {
				return nil, err
			}
			offsetsEnums = append(offsetsEnums, automataEnums...)
		}
	}

	switch len(offsetsEnums) {
	case 0:
		return Empty, nil
	case 1:
		return offsetsEnums[0], nil
	default:
		return NewMultiOffsetsEnum(offsetsEnums)
	}
}

func (b *baseFieldOffsetStrategy) createOffsetsK_terms(termsIndex index.Terms, doc int) ([]OffsetsEnum, error) {
	// This is a helper for automata
	return nil, nil
}

func (b *baseFieldOffsetStrategy) createOffsetsEnumsForAutomata(termsIndex index.Terms, doc int) ([]OffsetsEnum, error) {
	automata := b.components.Automata
	var results []OffsetsEnum

	termsEnum := termsIndex.Iterator()
	for {
		term, err := termsEnum.Next()
		if err != nil || term == nil {
			break
		}

		for _, automaton := range automata {
			if automaton.Match(string(term)) {
				postings, err := termsEnum.Postings(nil, index.PostingsEnumOffsets)
				if err != nil {
					return nil, err
				}
				if doc == postings.Advance(doc) {
					oe, err := NewOfPostingsWithFreq(util.BytesRef(automaton.Label()), postings.Freq(), postings)
					if err != nil {
						return nil, err
					}
					results = append(results, oe)
				}
			}
		}
	}
	return results, nil
}

// NoOpOffsetStrategy never returns offsets.
type NoOpOffsetStrategy struct {
	baseFieldOffsetStrategy
}

func NewNoOpOffsetStrategy() *NoOpOffsetStrategy {
	return &NoOpOffsetStrategy{
		baseFieldOffsetStrategy: baseFieldOffsetStrategy{
			components: &UHComponents{
				Field: "_ignored_",
				FieldMatcher: func(s string) bool { return false },
				Query: search.MatchNoDocsQuery{},
				Terms: []util.BytesRef{},
				PhraseHelper: nil,
				Automata: []CharArrayMatcher{},
				HasUnrecognizedQueryPart: false,
				Flags: FlagSet{},
			},
		},
	}
}

func (s *NoOpOffsetStrategy) GetOffsetSource() OffsetSource {
	return SourceNoneNeeded
}

func (s *NoOpOffsetStrategy) GetOffsetsEnum(reader index.LeafReader, docID int, content string) (OffsetsEnum, error) {
	return Empty, nil
}

var NoOpInstance = NewNoOpOffsetStrategy()

// TokenStreamOffsetStrategy analyzes the text to produce offsets.
type TokenStreamOffsetStrategy struct {
	baseFieldOffsetStrategy
	indexAnalyzer search.Analyzer
	combinedAutomata []CharArrayMatcher
}

func NewTokenStreamOffsetStrategy(components *UHComponents, analyzer search.Analyzer) *TokenStreamOffsetStrategy {
	s := &TokenStreamOffsetStrategy{
		baseFieldOffsetStrategy: baseFieldOffsetStrategy{components: components},
		indexAnalyzer:           analyzer,
	}
	s.combinedAutomata = s.convertTermsToMatchers(components.Terms, components.Automata)
	return s
}

func (s *TokenStreamOffsetStrategy) convertTermsToMatchers(terms []util.BytesRef, matchers []CharArrayMatcher) []CharArrayMatcher {
	newAutomata := make([]CharArrayMatcher, len(terms)+len(matchers))
	for i, term := range terms {
		termStr := string(term)
		newAutomata[i] = NewLabelledCharArrayMatcher(termStr, func(text string) bool {
			return text == termStr // Simplified: in real Lucene this uses an automaton
		})
	}
	copy(newAutomata[len(terms):], matchers)
	return newAutomata
}

func (s *TokenStreamOffsetStrategy) GetOffsetSource() OffsetSource {
	return SourceAnalysis
}

func (s *TokenStreamOffsetStrategy) GetOffsetsEnum(reader index.LeafReader, docID int, content string) (OffsetsEnum, error) {
	return NewTokenStreamOffsetsEnum(s.indexAnalyzer, content, s.combinedAutomata)
}

type tokenStreamOffsetsEnum struct {
	stream           search.TokenStream
	matchers         []CharArrayMatcher
	currentMatch     int
	matchDescriptions []util.BytesRef
}

func NewTokenStreamOffsetsEnum(analyzer search.Analyzer, content string, matchers []CharArrayMatcher) (*tokenStreamOffsetsEnum, error) {
	ts, err := analyzer.TokenStream(content)
	if err != nil {
		return nil, err
	}
	return &tokenStreamOffsetsEnum{
		stream:           ts,
		matchers:         matchers,
		currentMatch:     -1,
		matchDescriptions: make([]util.BytesRef, len(matchers)),
	}, nil
}

func (oe *tokenStreamOffsetsEnum) NextPosition() (bool, error) {
	if oe.stream == nil {
		return false, nil
	}
	for {
		token, err := oe.stream.IncrementToken()
		if err != nil {
			return false, err
		}
		if token == nil {
			oe.stream.End()
			oe.Close()
			return false, nil
		}

		for i, matcher := range oe.matchers {
			if matcher.Match(token.Term) {
				oe.currentMatch = i
				return true, nil
			}
		}
	}
}

func (oe *tokenStreamOffsetsEnum) Freq() (int, error) {
	return 2147483647, nil // Integer.MAX_VALUE
}

func (oe *tokenStreamOffsetsEnum) Term() (util.BytesRef, error) {
	if oe.matchDescriptions[oe.currentMatch] == nil {
		oe.matchDescriptions[oe.currentMatch] = util.BytesRef(oe.matchers[oe.currentMatch].Label())
	}
	return oe.matchDescriptions[oe.currentMatch], nil
}

func (oe *tokenStreamOffsetsEnum) StartOffset() (int, error) {
	// This requires access to the current token's offset.
	// In Gocene, TokenStream.IncrementToken should return a token with offsets.
	return 0, fmt.Errorf("startOffset not implemented")
}

func (oe *tokenStreamOffsetsEnum) EndOffset() (int, error) {
	return 0, fmt.Errorf("endOffset not implemented")
}

func (oe *tokenStreamOffsetsEnum) Close() error {
	if oe.stream != nil {
		oe.stream.Close()
		oe.stream = nil
	}
	return nil
}
