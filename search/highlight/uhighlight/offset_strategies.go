package uhighlight

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// PostingsOffsetStrategy retrieves offsets from postings.
type PostingsOffsetStrategy struct {
	baseFieldOffsetStrategy
}

func NewPostingsOffsetStrategy(components *UHComponents) *PostingsOffsetStrategy {
	return &PostingsOffsetStrategy{
		baseFieldOffsetStrategy: baseFieldOffsetStrategy{components: components},
	}
}

func (s *PostingsOffsetStrategy) GetOffsetSource() OffsetSource {
	return SourcePostings
}

func (s *PostingsOffsetStrategy) GetOffsetsEnum(reader index.LeafReader, docID int, content string) (OffsetsEnum, error) {
	return s.CreateOffsetsEnumFromReader(reader, docID)
}

// TermVectorOffsetStrategy retrieves offsets from term vectors.
type TermVectorOffsetStrategy struct {
	baseFieldOffsetStrategy
}

func NewTermVectorOffsetStrategy(components *UHComponents) *TermVectorOffsetStrategy {
	return &TermVectorOffsetStrategy{
		baseFieldOffsetStrategy: baseFieldOffsetStrategy{components: components},
	}
}

func (s *TermVectorOffsetStrategy) GetOffsetSource() OffsetSource {
	return SourceTermVectors
}

func (s *TermVectorOffsetStrategy) GetOffsetsEnum(reader index.LeafReader, docID int, content string) (OffsetsEnum, error) {
	tv, err := reader.TermVectors().Get(docID)
	if err != nil {
		return nil, err
	}
	// In real Lucene, this creates an OffsetsEnum from the term vectors.
	// We'll implement a simplified version.
	return NewOfPostingsFromTV(s.components.Terms, tv)
}

func NewOfPostingsFromTV(terms []util.BytesRef, tv index.Fields) (OffsetsEnum, error) {
	// Simplified: just return Empty for now
	return Empty, nil
}

// PostingsWithTermVectorsOffsetStrategy retrieves offsets from both.
type PostingsWithTermVectorsOffsetStrategy struct {
	baseFieldOffsetStrategy
}

func NewPostingsWithTermVectorsOffsetStrategy(components *UHComponents) *PostingsWithTermVectorsOffsetStrategy {
	return &PostingsWithTermVectorsOffsetStrategy{
		baseFieldOffsetStrategy: baseFieldOffsetStrategy{components: components},
	}
}

func (s *PostingsWithTermVectorsOffsetStrategy) GetOffsetSource() OffsetSource {
	return SourcePostingsWithTermVectors
}

func (s *PostingsWithTermVectorsOffsetStrategy) GetOffsetsEnum(reader index.LeafReader, docID int, content string) (OffsetsEnum, error) {
	return s.CreateOffsetsEnumFromReader(reader, docID)
}

// MultiFieldsOffsetStrategy merges offsets from multiple fields.
type MultiFieldsOffsetStrategy struct {
	strategies []FieldOffsetStrategy
}

func NewMultiFieldsOffsetStrategy(strategies []FieldOffsetStrategy) *MultiFieldsOffsetStrategy {
	return &MultiFieldsOffsetStrategy{
		strategies: strategies,
	}
}

func (s *MultiFieldsOffsetStrategy) GetField() string {
	if len(s.strategies) == 0 {
		return ""
	}
	return s.strategies[0].GetField()
}

func (s *MultiFieldsOffsetStrategy) GetOffsetSource() OffsetSource {
	return SourcePostings // Simplified
}

func (s *MultiFieldsOffsetStrategy) GetOffsetsEnum(reader index.LeafReader, docID int, content string) (OffsetsEnum, error) {
	var enums []OffsetsEnum
	for _, strategy := range s.strategies {
		oe, err := strategy.GetOffsetsEnum(reader, docID, content)
		if err != nil {
			return nil, err
		}
		if oe != Empty {
			enums = append(enums, oe)
		}
	}
	if len(enums) == 0 {
		return Empty, nil
	}
	return NewMultiOffsetsEnum(enums)
}

// MemoryIndexOffsetStrategy retrieves offsets via re-analysis.
type MemoryIndexOffsetStrategy struct {
	baseFieldOffsetStrategy
	indexAnalyzer search.Analyzer
}

func NewMemoryIndexOffsetStrategy(components *UHComponents, analyzer search.Analyzer) *MemoryIndexOffsetStrategy {
	return &MemoryIndexOffsetStrategy{
		baseFieldOffsetStrategy: baseFieldOffsetStrategy{components: components},
		indexAnalyzer:           analyzer,
	}
}

func (s *MemoryIndexOffsetStrategy) GetOffsetSource() OffsetSource {
	return SourceAnalysis
}

func (s *MemoryIndexOffsetStrategy) GetOffsetsEnum(reader index.LeafReader, docID int, content string) (OffsetsEnum, error) {
	// Re-analyze content to find offsets
	return NewTokenStreamOffsetsEnum(s.indexAnalyzer, content, s.convertTermsToMatchers())
}

func (s *MemoryIndexOffsetStrategy) convertTermsToMatchers() []CharArrayMatcher {
	// similar to TokenStreamOffsetStrategy
	terms := s.components.Terms
	automata := s.components.Automata
	newAutomata := make([]CharArrayMatcher, len(terms)+len(automata))
	for i, term := range terms {
		termStr := string(term)
		newAutomata[i] = NewLabelledCharArrayMatcher(termStr, func(text string) bool {
			return text == termStr
		})
	}
	copy(newAutomata[len(terms):], automata)
	return newAutomata
}
