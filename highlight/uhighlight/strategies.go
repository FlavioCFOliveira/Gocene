package uhighlight

import "errors"

// errNotImplemented is the placeholder error returned by the offset
// strategies whose backing primitives (postings reader, term vectors,
// MemoryIndex, multi-fields fan-out) are tracked in their own sprints. The
// implementations expose the right contract so callers can wire them up;
// concrete extraction logic is added once the upstream primitives ship.
var errNotImplemented = errors.New("uhighlight: strategy needs upstream primitives")

// NoOpOffsetStrategy returns an empty OffsetsEnum for the supplied field.
// Mirrors org.apache.lucene.search.uhighlight.NoOpOffsetStrategy.
type NoOpOffsetStrategy struct{ BaseFieldOffsetStrategy }

// NewNoOpOffsetStrategy builds the no-op.
func NewNoOpOffsetStrategy(field string) *NoOpOffsetStrategy {
	return &NoOpOffsetStrategy{BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(field)}
}

// GetOffsetsEnum returns an empty SliceOffsetsEnum.
func (s *NoOpOffsetStrategy) GetOffsetsEnum(_ any) (OffsetsEnum, error) {
	return NewSliceOffsetsEnum(nil), nil
}

var _ FieldOffsetStrategy = (*NoOpOffsetStrategy)(nil)

// AnalysisOffsetStrategy re-runs the field analyzer to produce offsets.
// Mirrors org.apache.lucene.search.uhighlight.AnalysisOffsetStrategy.
type AnalysisOffsetStrategy struct{ BaseFieldOffsetStrategy }

// NewAnalysisOffsetStrategy builds the analysis-based strategy.
func NewAnalysisOffsetStrategy(field string) *AnalysisOffsetStrategy {
	return &AnalysisOffsetStrategy{BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(field)}
}

// GetOffsetsEnum requires an analyzer + the field value; the doc-context
// helper must carry both. The Go port defers concrete extraction to the
// analyzer infrastructure landed in later sprints.
func (s *AnalysisOffsetStrategy) GetOffsetsEnum(_ any) (OffsetsEnum, error) {
	return nil, errNotImplemented
}

var _ FieldOffsetStrategy = (*AnalysisOffsetStrategy)(nil)

// PostingsOffsetStrategy reads offsets straight from the indexed postings.
// Mirrors org.apache.lucene.search.uhighlight.PostingsOffsetStrategy.
type PostingsOffsetStrategy struct{ BaseFieldOffsetStrategy }

// NewPostingsOffsetStrategy builds the postings-based strategy.
func NewPostingsOffsetStrategy(field string) *PostingsOffsetStrategy {
	return &PostingsOffsetStrategy{BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(field)}
}

// GetOffsetsEnum requires PostingsEnum-with-offsets support; deferred.
func (s *PostingsOffsetStrategy) GetOffsetsEnum(_ any) (OffsetsEnum, error) {
	return nil, errNotImplemented
}

var _ FieldOffsetStrategy = (*PostingsOffsetStrategy)(nil)

// PostingsWithTermVectorsOffsetStrategy falls back to term vectors when
// postings lack offsets. Mirrors
// org.apache.lucene.search.uhighlight.PostingsWithTermVectorsOffsetStrategy.
type PostingsWithTermVectorsOffsetStrategy struct{ BaseFieldOffsetStrategy }

// NewPostingsWithTermVectorsOffsetStrategy builds the strategy.
func NewPostingsWithTermVectorsOffsetStrategy(field string) *PostingsWithTermVectorsOffsetStrategy {
	return &PostingsWithTermVectorsOffsetStrategy{BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(field)}
}

// GetOffsetsEnum is deferred to the term-vector + postings infrastructure.
func (s *PostingsWithTermVectorsOffsetStrategy) GetOffsetsEnum(_ any) (OffsetsEnum, error) {
	return nil, errNotImplemented
}

var _ FieldOffsetStrategy = (*PostingsWithTermVectorsOffsetStrategy)(nil)

// TermVectorOffsetStrategy reads offsets from term vectors only.
// Mirrors org.apache.lucene.search.uhighlight.TermVectorOffsetStrategy.
type TermVectorOffsetStrategy struct{ BaseFieldOffsetStrategy }

// NewTermVectorOffsetStrategy builds the strategy.
func NewTermVectorOffsetStrategy(field string) *TermVectorOffsetStrategy {
	return &TermVectorOffsetStrategy{BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(field)}
}

// GetOffsetsEnum is deferred to the term-vector infrastructure.
func (s *TermVectorOffsetStrategy) GetOffsetsEnum(_ any) (OffsetsEnum, error) {
	return nil, errNotImplemented
}

var _ FieldOffsetStrategy = (*TermVectorOffsetStrategy)(nil)

// TokenStreamOffsetStrategy walks a pre-computed TokenStream to extract
// offsets. Mirrors
// org.apache.lucene.search.uhighlight.TokenStreamOffsetStrategy.
type TokenStreamOffsetStrategy struct{ BaseFieldOffsetStrategy }

// NewTokenStreamOffsetStrategy builds the strategy.
func NewTokenStreamOffsetStrategy(field string) *TokenStreamOffsetStrategy {
	return &TokenStreamOffsetStrategy{BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(field)}
}

// GetOffsetsEnum is deferred to the token-stream pipeline (which the
// caller supplies inside docContext).
func (s *TokenStreamOffsetStrategy) GetOffsetsEnum(_ any) (OffsetsEnum, error) {
	return nil, errNotImplemented
}

var _ FieldOffsetStrategy = (*TokenStreamOffsetStrategy)(nil)

// MemoryIndexOffsetStrategy spins up a MemoryIndex per highlight call to
// resolve offsets when neither postings nor term vectors are available.
// Mirrors org.apache.lucene.search.uhighlight.MemoryIndexOffsetStrategy.
type MemoryIndexOffsetStrategy struct{ BaseFieldOffsetStrategy }

// NewMemoryIndexOffsetStrategy builds the strategy.
func NewMemoryIndexOffsetStrategy(field string) *MemoryIndexOffsetStrategy {
	return &MemoryIndexOffsetStrategy{BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(field)}
}

// GetOffsetsEnum is deferred to the memory module.
func (s *MemoryIndexOffsetStrategy) GetOffsetsEnum(_ any) (OffsetsEnum, error) {
	return nil, errNotImplemented
}

var _ FieldOffsetStrategy = (*MemoryIndexOffsetStrategy)(nil)

// MultiFieldsOffsetStrategy fans the offset-resolution request out across
// several fields. Mirrors
// org.apache.lucene.search.uhighlight.MultiFieldsOffsetStrategy.
type MultiFieldsOffsetStrategy struct {
	fields   []string
	resolver func(field string) FieldOffsetStrategy
}

// NewMultiFieldsOffsetStrategy builds the fan-out strategy.
func NewMultiFieldsOffsetStrategy(fields []string, resolver func(field string) FieldOffsetStrategy) *MultiFieldsOffsetStrategy {
	return &MultiFieldsOffsetStrategy{fields: append([]string(nil), fields...), resolver: resolver}
}

// Field returns the primary field (the first in the configured list) so the
// FieldOffsetStrategy contract is satisfied.
func (s *MultiFieldsOffsetStrategy) Field() string {
	if len(s.fields) == 0 {
		return ""
	}
	return s.fields[0]
}

// GetOffsetsEnum concatenates the per-field SliceOffsetsEnums into a single
// virtual enum.
func (s *MultiFieldsOffsetStrategy) GetOffsetsEnum(docContext any) (OffsetsEnum, error) {
	if s.resolver == nil {
		return nil, errNotImplemented
	}
	var merged []OffsetEntry
	for _, f := range s.fields {
		strat := s.resolver(f)
		if strat == nil {
			continue
		}
		enum, err := strat.GetOffsetsEnum(docContext)
		if err != nil {
			return nil, err
		}
		for enum.Next() {
			merged = append(merged, OffsetEntry{
				Term:        enum.Term(),
				StartOffset: enum.StartOffset(),
				EndOffset:   enum.EndOffset(),
				Weight:      enum.Weight(),
			})
		}
		_ = enum.Close()
	}
	return NewSliceOffsetsEnum(merged), nil
}

var _ FieldOffsetStrategy = (*MultiFieldsOffsetStrategy)(nil)
