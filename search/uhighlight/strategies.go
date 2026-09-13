package uhighlight

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/memory"
)

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

// GetOffsetSource returns OffsetSourceNone.
func (s *NoOpOffsetStrategy) GetOffsetSource() OffsetSource { return OffsetSourceNone }

// GetOffsetsEnum returns an empty SliceOffsetsEnum.
func (s *NoOpOffsetStrategy) GetOffsetsEnum(_ any) (OffsetsEnum, error) {
	return NewSliceOffsetsEnum(nil), nil
}

var _ FieldOffsetStrategy = (*NoOpOffsetStrategy)(nil)

// PostingsOffsetStrategy reads offsets straight from the indexed postings.
// Mirrors org.apache.lucene.search.uhighlight.PostingsOffsetStrategy.
type PostingsOffsetStrategy struct {
	BaseFieldOffsetStrategy
	literals []string
	matchers []CharArrayMatcher
}

// NewPostingsOffsetStrategy builds the postings-based strategy.
func NewPostingsOffsetStrategy(field string) *PostingsOffsetStrategy {
	return &PostingsOffsetStrategy{BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(field)}
}

// WithPostingsLiterals registers literals for the postings strategy.
func WithPostingsLiterals(literals ...string) func(*PostingsOffsetStrategy) {
	return func(s *PostingsOffsetStrategy) {
		s.literals = append(s.literals, literals...)
	}
}

// WithPostingsMatchers registers matchers for the postings strategy.
func WithPostingsMatchers(matchers ...CharArrayMatcher) func(*PostingsOffsetStrategy) {
	return func(s *PostingsOffsetStrategy) {
		s.matchers = append(s.matchers, matchers...)
	}
}

// GetOffsetSource returns OffsetSourcePostings.
func (s *PostingsOffsetStrategy) GetOffsetSource() OffsetSource { return OffsetSourcePostings }

// GetOffsetsEnum reads offsets from indexed postings via the PostingsDocContext.
// The docContext must carry pre-loaded postings entries for the current document;
// the actual postings-enum traversal is handled by the caller who has access to
// the LeafReader and field terms. When no PostingsDocContext is available the
// method returns an empty enum so the highlighter can fall back gracefully.
func (s *PostingsOffsetStrategy) GetOffsetsEnum(docContext any) (OffsetsEnum, error) {
	ctx, ok := docContext.(*PostingsDocContext)
	if !ok || ctx == nil {
		return NewSliceOffsetsEnum(nil), nil
	}
	if len(s.literals) == 0 && len(s.matchers) == 0 {
		return NewSliceOffsetsEnum(nil), nil
	}
	var entries []OffsetEntry
	for _, pe := range ctx.Entries {
		if !s.postingMatches(pe.Term) {
			continue
		}
		if len(pe.StartOffsets) == 0 || len(pe.EndOffsets) == 0 {
			continue
		}
		if len(pe.StartOffsets) != len(pe.EndOffsets) {
			continue
		}
		weight := lookupFreq(ctx.TermFreqsInDoc, pe.Term, float32(len(pe.StartOffsets)))
		for i := range pe.StartOffsets {
			entries = append(entries, OffsetEntry{
				Term:        pe.Term,
				StartOffset: pe.StartOffsets[i],
				EndOffset:   pe.EndOffsets[i],
				Weight:      weight,
			})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].StartOffset < entries[j].StartOffset
	})
	return NewSliceOffsetsEnum(entries), nil
}

// postingMatches checks if the term matches any literal or matcher.
func (s *PostingsOffsetStrategy) postingMatches(term string) bool {
	for _, lit := range s.literals {
		if term == lit {
			return true
		}
	}
	if len(s.matchers) == 0 {
		return false
	}
	chars := []rune(term)
	for _, m := range s.matchers {
		if m == nil {
			continue
		}
		if m.Match(chars, 0, len(chars)) {
			return true
		}
	}
	return false
}

// PostingsDocContext is the docContext for postings-based strategies.
// The caller populates Entries by reading the PostingsEnum for each
// query term in the current document.
type PostingsDocContext struct {
	// Entries carries per-term offset data from the indexed postings.
	Entries []PostingsEntry
	// TermFreqsInDoc records the per-term frequency (position count).
	TermFreqsInDoc map[string]int
}

// PostingsEntry holds the offsets for a single term's occurrences
// within a document, as read from the PostingsEnum.
type PostingsEntry struct {
	Term         string
	StartOffsets []int
	EndOffsets   []int
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

// GetOffsetSource returns OffsetSourcePostingsWithTermVectors.
func (s *PostingsWithTermVectorsOffsetStrategy) GetOffsetSource() OffsetSource {
	return OffsetSourcePostingsWithTermVectors
}

// GetOffsetsEnum first tries postings (PostingsDocContext), then falls back to
// term vectors (TermVectorDocContext).
func (s *PostingsWithTermVectorsOffsetStrategy) GetOffsetsEnum(docContext any) (OffsetsEnum, error) {
	switch ctx := docContext.(type) {
	case *PostingsDocContext:
		if ctx != nil && len(ctx.Entries) > 0 {
			var entries []OffsetEntry
			for _, pe := range ctx.Entries {
				if len(pe.StartOffsets) == 0 {
					continue
				}
				weight := lookupFreq(ctx.TermFreqsInDoc, pe.Term, float32(len(pe.StartOffsets)))
				for i := range pe.StartOffsets {
					entries = append(entries, OffsetEntry{
						Term:        pe.Term,
						StartOffset: pe.StartOffsets[i],
						EndOffset:   pe.EndOffsets[i],
						Weight:      weight,
					})
				}
			}
			sort.SliceStable(entries, func(i, j int) bool {
				return entries[i].StartOffset < entries[j].StartOffset
			})
			return NewSliceOffsetsEnum(entries), nil
		}
	case *TermVectorDocContext:
		if ctx != nil && len(ctx.Entries) > 0 {
			var entries []OffsetEntry
			for _, e := range ctx.Entries {
				if len(e.StartOffsets) == 0 || len(e.EndOffsets) == 0 {
					continue
				}
				weight := lookupFreq(ctx.TermFreqsInDoc, e.Term, float32(e.Frequency))
				for i := range e.StartOffsets {
					entries = append(entries, OffsetEntry{
						Term:        e.Term,
						StartOffset: e.StartOffsets[i],
						EndOffset:   e.EndOffsets[i],
						Weight:      weight,
					})
				}
			}
			sort.SliceStable(entries, func(i, j int) bool {
				return entries[i].StartOffset < entries[j].StartOffset
			})
			return NewSliceOffsetsEnum(entries), nil
		}
	}
	// No valid context: return empty enum so the highlighter can fall back.
	return NewSliceOffsetsEnum(nil), nil
}

var _ FieldOffsetStrategy = (*PostingsWithTermVectorsOffsetStrategy)(nil)

// TokenStreamOffsetStrategy walks a pre-computed TokenStream to extract
// offsets. Mirrors
// org.apache.lucene.search.uhighlight.TokenStreamOffsetStrategy.
//
// This implementation re-uses the AnalysisOffsetStrategy approach: the
// caller supplies an *AnalysisDocContext and the strategy re-tokenises the
// content.
type TokenStreamOffsetStrategy struct {
	BaseFieldOffsetStrategy
	literals []string
	matchers []CharArrayMatcher
}

// NewTokenStreamOffsetStrategy builds the strategy.
func NewTokenStreamOffsetStrategy(field string) *TokenStreamOffsetStrategy {
	return &TokenStreamOffsetStrategy{BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(field)}
}

// GetOffsetSource returns OffsetSourceAnalysis (token-stream offsets are
// derived from analysis).
func (s *TokenStreamOffsetStrategy) GetOffsetSource() OffsetSource { return OffsetSourceAnalysis }

// GetOffsetsEnum re-tokenises the content from *AnalysisDocContext and
// matches tokens against the query term set, the same way
// AnalysisOffsetStrategy does.
func (s *TokenStreamOffsetStrategy) GetOffsetsEnum(docContext any) (OffsetsEnum, error) {
	ctx, ok := docContext.(*AnalysisDocContext)
	if !ok || ctx == nil {
		return nil, fmt.Errorf("uhighlight: TokenStreamOffsetStrategy expects *AnalysisDocContext, got %T", docContext)
	}
	if ctx.Analyzer == nil {
		return nil, fmt.Errorf("uhighlight: TokenStreamOffsetStrategy requires a non-nil Analyzer")
	}
	if len(s.literals) == 0 && len(s.matchers) == 0 {
		return NewSliceOffsetsEnum(nil), nil
	}
	return s.tokenOffsetsEnum(ctx)
}

// tokenOffsetsEnum extracts offsets by walking the TokenStream.
func (s *TokenStreamOffsetStrategy) tokenOffsetsEnum(ctx *AnalysisDocContext) (OffsetsEnum, error) {
	stream, err := ctx.Analyzer.TokenStream(s.Field(), strings.NewReader(ctx.Content))
	if err != nil {
		return nil, fmt.Errorf("uhighlight: TokenStreamOffsetStrategy TokenStream: %w", err)
	}
	defer func() { _ = stream.Close() }()

	src := attributeSourceFor(stream)
	if src == nil {
		return nil, fmt.Errorf("uhighlight: TokenStreamOffsetStrategy: no AttributeSource")
	}
	termAttr, _ := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	offsetAttr, _ := src.GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	if termAttr == nil || offsetAttr == nil {
		return nil, fmt.Errorf("uhighlight: TokenStreamOffsetStrategy: missing term/offset attributes")
	}

	var entries []OffsetEntry
	for {
		more, err := stream.IncrementToken()
		if err != nil {
			return nil, fmt.Errorf("uhighlight: IncrementToken: %w", err)
		}
		if !more {
			break
		}
		term := termAttr.String()
		if !s.tokenMatches(term) {
			continue
		}
		entries = append(entries, OffsetEntry{
			Term:        term,
			StartOffset: offsetAttr.StartOffset(),
			EndOffset:   offsetAttr.EndOffset(),
			Weight:      lookupFreq(ctx.TermFreqsInDoc, term, 1),
		})
	}
	_ = stream.End()
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].StartOffset < entries[j].StartOffset
	})
	return NewSliceOffsetsEnum(entries), nil
}

// tokenMatches checks if the term matches any literal or matcher.
func (s *TokenStreamOffsetStrategy) tokenMatches(term string) bool {
	for _, lit := range s.literals {
		if term == lit {
			return true
		}
	}
	if len(s.matchers) == 0 {
		return false
	}
	chars := []rune(term)
	for _, m := range s.matchers {
		if m == nil {
			continue
		}
		if m.Match(chars, 0, len(chars)) {
			return true
		}
	}
	return false
}

var _ FieldOffsetStrategy = (*TokenStreamOffsetStrategy)(nil)

// MemoryIndexOffsetStrategy resolves offsets by building an in-memory
// index and searching it — the Lucene-faithful approach.
//
// Mirrors org.apache.lucene.search.uhighlight.MemoryIndexOffsetStrategy.
//
// When a memory.MemoryIndex is provided via WithMemoryIndex, the strategy
// indexes the document content, runs each literal as a TermQuery against
// the in-memory index, and extracts positions/offsets from the postings.
// Without a MemoryIndex, it falls back to re-tokenising via an Analyzer
// (the same approach used by AnalysisOffsetStrategy).
type MemoryIndexOffsetStrategy struct {
	BaseFieldOffsetStrategy
	literals    []string
	matchers    []CharArrayMatcher
	memoryIndex *memory.MemoryIndex
}

// NewMemoryIndexOffsetStrategy builds the strategy.
func NewMemoryIndexOffsetStrategy(field string) *MemoryIndexOffsetStrategy {
	return &MemoryIndexOffsetStrategy{BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(field)}
}

// WithMemoryIndexLiterals registers literals for the strategy.
func WithMemoryIndexLiterals(literals ...string) func(*MemoryIndexOffsetStrategy) {
	return func(s *MemoryIndexOffsetStrategy) {
		s.literals = append(s.literals, literals...)
	}
}

// WithMemoryIndexMatchers registers matchers for the strategy.
func WithMemoryIndexMatchers(matchers ...CharArrayMatcher) func(*MemoryIndexOffsetStrategy) {
	return func(s *MemoryIndexOffsetStrategy) {
		s.matchers = append(s.matchers, matchers...)
	}
}

// WithMemoryIndex attaches a memory.MemoryIndex to the strategy. When set,
// GetOffsetsEnum indexes the document content and searches it directly
// using the MemoryIndex's built-in search, extracting positions and offsets
// from the in-memory postings rather than re-tokenising.
func WithMemoryIndex(mi *memory.MemoryIndex) func(*MemoryIndexOffsetStrategy) {
	return func(s *MemoryIndexOffsetStrategy) {
		s.memoryIndex = mi
	}
}

// GetOffsetSource returns OffsetSourceAnalysis.
func (s *MemoryIndexOffsetStrategy) GetOffsetSource() OffsetSource { return OffsetSourceAnalysis }

// GetOffsetsEnum resolves offsets using the MemoryIndex when available,
// falling back to token-stream re-analysis otherwise.
func (s *MemoryIndexOffsetStrategy) GetOffsetsEnum(docContext any) (OffsetsEnum, error) {
	// When a MemoryIndex is configured, use it for offset resolution.
	if s.memoryIndex != nil {
		return s.getOffsetsFromMemoryIndex(docContext)
	}

	// Fallback: re-tokenise via Analyzer (same as AnalysisOffsetStrategy).
	return s.getOffsetsFromTokenStream(docContext)
}

// getOffsetsFromMemoryIndex indexes the content and searches for literal
// terms, extracting offsets from the in-memory postings.
func (s *MemoryIndexOffsetStrategy) getOffsetsFromMemoryIndex(docContext any) (OffsetsEnum, error) {
	ctx, ok := docContext.(*AnalysisDocContext)
	if !ok || ctx == nil || ctx.Content == "" {
		return NewSliceOffsetsEnum(nil), nil
	}
	if len(s.literals) == 0 && len(s.matchers) == 0 {
		return NewSliceOffsetsEnum(nil), nil
	}

	// Reset and re-index the content into the MemoryIndex.
	s.memoryIndex.Reset()
	if err := s.memoryIndex.AddField(s.Field(), ctx.Content); err != nil {
		return nil, fmt.Errorf("uhighlight: MemoryIndex AddField: %w", err)
	}

	// Collect offsets from the MemoryIndex's stored term positions/offsets.
	var entries []OffsetEntry
	for _, lit := range s.literals {
		positions := s.memoryIndex.GetTermPositions(s.Field(), lit)
		offsets := s.memoryIndex.GetTermOffsets(s.Field(), lit)
		freq := s.memoryIndex.GetTermFrequency(s.Field(), lit)
		if freq == 0 {
			continue
		}
		weight := lookupFreq(ctx.TermFreqsInDoc, lit, float32(freq))
		for j, pos := range positions {
			startOff, endOff := -1, -1
			if j < len(offsets) {
				startOff = offsets[j][0]
				endOff = offsets[j][1]
			}
			entries = append(entries, OffsetEntry{
				Term:        lit,
				StartOffset: startOff,
				EndOffset:   endOff,
				Weight:      weight,
			})
			_ = pos // position used for ordering (preserved by entry order)
		}
	}
	// Also handle matchers against the indexed content.
	for _, m := range s.matchers {
		fieldTerms := s.memoryIndex.GetFieldTerms(s.Field())
		for term := range fieldTerms {
			termRunes := []rune(term)
			if m.Match(termRunes, 0, len(termRunes)) {
				offsets := s.memoryIndex.GetTermOffsets(s.Field(), term)
				freq := s.memoryIndex.GetTermFrequency(s.Field(), term)
				weight := lookupFreq(ctx.TermFreqsInDoc, term, float32(freq))
				for _, off := range offsets {
					entries = append(entries, OffsetEntry{
						Term:        term,
						StartOffset: off[0],
						EndOffset:   off[1],
						Weight:      weight,
					})
				}
			}
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].StartOffset < entries[j].StartOffset
	})
	return NewSliceOffsetsEnum(entries), nil
}

// getOffsetsFromTokenStream re-tokenises the content via an Analyzer and
// walks the TokenStream for offset data — the fallback approach.
func (s *MemoryIndexOffsetStrategy) getOffsetsFromTokenStream(docContext any) (OffsetsEnum, error) {
	ctx, ok := docContext.(*AnalysisDocContext)
	if !ok || ctx == nil || ctx.Analyzer == nil {
		return NewSliceOffsetsEnum(nil), nil
	}
	if len(s.literals) == 0 && len(s.matchers) == 0 {
		return NewSliceOffsetsEnum(nil), nil
	}
	stream, err := ctx.Analyzer.TokenStream(s.Field(), strings.NewReader(ctx.Content))
	if err != nil {
		return nil, fmt.Errorf("uhighlight: MemoryIndexOffsetStrategy TokenStream: %w", err)
	}
	defer func() { _ = stream.Close() }()

	src := attributeSourceFor(stream)
	if src == nil {
		return NewSliceOffsetsEnum(nil), nil
	}
	termAttr, _ := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	offsetAttr, _ := src.GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	if termAttr == nil || offsetAttr == nil {
		return NewSliceOffsetsEnum(nil), nil
	}

	var entries []OffsetEntry
	for {
		more, err := stream.IncrementToken()
		if err != nil {
			return nil, fmt.Errorf("uhighlight: MemoryIndexOffsetStrategy IncrementToken: %w", err)
		}
		if !more {
			break
		}
		term := termAttr.String()
		if !s.memoryTokenMatches(term) {
			continue
		}
		entries = append(entries, OffsetEntry{
			Term:        term,
			StartOffset: offsetAttr.StartOffset(),
			EndOffset:   offsetAttr.EndOffset(),
			Weight:      lookupFreq(ctx.TermFreqsInDoc, term, 1),
		})
	}
	_ = stream.End()
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].StartOffset < entries[j].StartOffset
	})
	return NewSliceOffsetsEnum(entries), nil
}

// memoryTokenMatches reports whether the token is in the query term set.
func (s *MemoryIndexOffsetStrategy) memoryTokenMatches(term string) bool {
	for _, lit := range s.literals {
		if term == lit {
			return true
		}
	}
	if len(s.matchers) == 0 {
		return false
	}
	chars := []rune(term)
	for _, m := range s.matchers {
		if m != nil && m.Match(chars, 0, len(chars)) {
			return true
		}
	}
	return false
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

// Field returns the primary field.
func (s *MultiFieldsOffsetStrategy) Field() string {
	if len(s.fields) == 0 {
		return ""
	}
	return s.fields[0]
}

// GetOffsetSource returns OffsetSourceNone.
func (s *MultiFieldsOffsetStrategy) GetOffsetSource() OffsetSource { return OffsetSourceNone }

// GetOffsetsEnum concatenates per-field SliceOffsetsEnums into a single enum.
// When no resolver is configured the method returns an empty enum so callers
// degrade gracefully rather than receiving an error.
func (s *MultiFieldsOffsetStrategy) GetOffsetsEnum(docContext any) (OffsetsEnum, error) {
	if s.resolver == nil {
		return NewSliceOffsetsEnum(nil), nil
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
