package uhighlight

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// OffsetSource enumerates the mechanisms a FieldOffsetStrategy can use to
// locate offset positions within a document. Mirrors the inner enum
// org.apache.lucene.search.uhighlight.UnifiedHighlighter.OffsetSource.
type OffsetSource int

const (
	// OffsetSourcePostings reads offsets from indexed postings.
	OffsetSourcePostings OffsetSource = iota
	// OffsetSourceTermVectors reads offsets from stored term vectors.
	OffsetSourceTermVectors
	// OffsetSourceAnalysis re-runs the field analyzer to derive offsets.
	OffsetSourceAnalysis
	// OffsetSourcePostingsWithTermVectors reads from postings and falls back
	// to term vectors.
	OffsetSourcePostingsWithTermVectors
	// OffsetSourceNoneNeeded signals that no offset extraction is needed.
	OffsetSourceNoneNeeded
)

// FieldOffsetStrategy ultimately returns an OffsetsEnum yielding potentially
// highlightable words in the text. It needs information about the query up
// front.
//
// This is the Go port of the abstract class
// org.apache.lucene.search.uhighlight.FieldOffsetStrategy from Apache Lucene
// 10.5.0. Concrete implementations embed BaseFieldOffsetStrategy to inherit
// the components plumbing and the four protected offsets-enum builders.
type FieldOffsetStrategy interface {
	// Field renders FieldOffsetStrategy.getField()
	// (FieldOffsetStrategy.java:48).
	Field() string

	// GetOffsetSource renders the abstract
	// FieldOffsetStrategy.getOffsetSource() (FieldOffsetStrategy.java:52).
	GetOffsetSource() OffsetSource

	// GetOffsetsEnum is the primary method -- return offsets for
	// highlightable words in the specified document. Callers are expected to
	// close the returned OffsetsEnum when it has been finished with.
	//
	// Renders the abstract
	// FieldOffsetStrategy.getOffsetsEnum(LeafReader, int, String)
	// (FieldOffsetStrategy.java:59).
	GetOffsetsEnum(reader index.LeafReader, docID int, content string) (OffsetsEnum, error)
}

// BaseFieldOffsetStrategy carries the protected `components` field of the Java
// abstract class (FieldOffsetStrategy.java:41) together with the concrete
// bodies every subclass inherits from it.
type BaseFieldOffsetStrategy struct {
	components *UHComponents
}

// NewBaseFieldOffsetStrategy renders `FieldOffsetStrategy(UHComponents
// components)` (FieldOffsetStrategy.java:43).
func NewBaseFieldOffsetStrategy(components *UHComponents) BaseFieldOffsetStrategy {
	return BaseFieldOffsetStrategy{components: components}
}

// Components returns the UHComponents this strategy was built with. It renders
// access to the protected `components` field of the Java abstract class, which
// subclasses read directly.
func (s BaseFieldOffsetStrategy) Components() *UHComponents { return s.components }

// Field renders FieldOffsetStrategy.getField() (FieldOffsetStrategy.java:48).
func (s BaseFieldOffsetStrategy) Field() string { return s.components.Field }

// CreateOffsetsEnumFromReader renders the protected
// FieldOffsetStrategy.createOffsetsEnumFromReader(LeafReader, int)
// (FieldOffsetStrategy.java:62).
func (s BaseFieldOffsetStrategy) CreateOffsetsEnumFromReader(leafReader index.LeafReader, doc int) (OffsetsEnum, error) {
	termsIndex, err := leafReader.Terms(s.Field())
	if err != nil {
		return nil, err
	}
	if termsIndex == nil {
		return OffsetsEnumEMPTY, nil
	}

	offsetsEnums := make([]OffsetsEnum, 0)

	// Handle Weight.matches approach
	if _, weightMatches := s.components.HighlightFlags[HighlightFlagWeightMatches]; weightMatches {

		if err := s.CreateOffsetsEnumsWeightMatcher(leafReader, doc, &offsetsEnums); err != nil {
			return nil, err
		}

	} else { // classic approach

		// Handle position insensitive terms (a subset of this.terms field):
		var insensitiveTerms []*util.BytesRef
		phraseHelper := s.components.PhraseHelper
		terms := s.components.Terms
		if phraseHelper.HasPositionSensitivity() {
			insensitiveTerms = phraseHelper.GetAllPositionInsensitiveTerms()
			if len(insensitiveTerms) > len(terms) {
				return nil, fmt.Errorf("uhighlight: insensitive terms should be smaller set of all terms")
			}
		} else {
			insensitiveTerms = terms
		}
		if len(insensitiveTerms) > 0 {
			if err := s.CreateOffsetsEnumsForTerms(insensitiveTerms, termsIndex, doc, &offsetsEnums); err != nil {
				return nil, err
			}
		}

		// Handle spans
		if phraseHelper.HasPositionSensitivity() {
			if err := phraseHelper.CreateOffsetsEnumsForSpans(leafReader, doc, &offsetsEnums); err != nil {
				return nil, err
			}
		}

		// Handle automata
		if len(s.components.Automata) > 0 {
			if err := s.CreateOffsetsEnumsForAutomata(termsIndex, doc, &offsetsEnums); err != nil {
				return nil, err
			}
		}
	}

	switch len(offsetsEnums) {
	case 0:
		return OffsetsEnumEMPTY, nil
	case 1:
		return offsetsEnums[0], nil
	default:
		return NewMultiOffsetsEnum(offsetsEnums)
	}
}

// CreateOffsetsEnumsWeightMatcher renders the protected
// FieldOffsetStrategy.createOffsetsEnumsWeightMatcher(LeafReader, int,
// List<OffsetsEnum>) (FieldOffsetStrategy.java:113). Java appends to the
// caller's List; Go slices are values, so results is a pointer to the
// caller's slice.
func (s BaseFieldOffsetStrategy) CreateOffsetsEnumsWeightMatcher(underlying index.LeafReader, docID int, results *[]OffsetsEnum) error {
	// remap fieldMatcher/requireFieldMatch fields to the field we are highlighting
	leafReader := newFieldMatcherRemappingLeafReader(underlying, s.components)
	indexSearcher := search.NewIndexSearcher(leafReader)
	indexSearcher.SetQueryCache(nil)
	rewritten, err := indexSearcher.Rewrite(s.components.Query)
	if err != nil {
		return err
	}
	weight, err := indexSearcher.CreateWeight(rewritten, search.ScoreModeCompleteNoScores, 1.0)
	if err != nil {
		return err
	}
	readerContext, err := leafReader.GetContext()
	if err != nil {
		return err
	}
	leafContext, ok := readerContext.(*index.LeafReaderContext)
	if !ok {
		return fmt.Errorf("uhighlight: expected a LeafReaderContext, got %T", readerContext)
	}
	matches, err := weight.Matches(leafContext, docID)
	if err != nil {
		return err
	}
	if matches == nil {
		return nil // doc doesn't match
	}
	for it := matches.Iterator(); it.HasNext(); {
		field := it.Next()
		if s.components.FieldMatcher(field) {
			iterator, err := matches.GetMatches(field)
			if err != nil {
				return err
			}
			if iterator == nil {
				continue
			}
			*results = append(*results, NewOfMatchesIteratorWithSubs(iterator))
		}
	}
	return nil
}

// CreateOffsetsEnumsForTerms renders the protected
// FieldOffsetStrategy.createOffsetsEnumsForTerms(BytesRef[], Terms, int,
// List<OffsetsEnum>) (FieldOffsetStrategy.java:158).
func (s BaseFieldOffsetStrategy) CreateOffsetsEnumsForTerms(sourceTerms []*util.BytesRef, termsIndex index.Terms, doc int, results *[]OffsetsEnum) error {
	termsEnum, err := termsIndex.Iterator() // does not return null
	if err != nil {
		return err
	}
	for _, term := range sourceTerms {
		found, err := termsEnum.SeekExact(&index.Term{Field: s.Field(), Bytes: term})
		if err != nil {
			return err
		}
		if found {
			postingsEnum, err := termsEnum.Postings(index.PostingsFlagOffsets)
			if err != nil {
				return err
			}
			if postingsEnum == nil {
				// no offsets or positions available
				return fmt.Errorf("uhighlight: field '%s' was indexed without offsets, cannot highlight", s.Field())
			}
			advanced, err := postingsEnum.Advance(doc)
			if err != nil {
				return err
			}
			if doc == advanced { // now it's positioned, although may be exhausted
				oe, err := NewOfPostingsFromPostings(bytesRefValue(term), postingsEnum)
				if err != nil {
					return err
				}
				*results = append(*results, oe)
			}
		}
	}
	return nil
}

// CreateOffsetsEnumsForAutomata renders the protected
// FieldOffsetStrategy.createOffsetsEnumsForAutomata(Terms, int,
// List<OffsetsEnum>) (FieldOffsetStrategy.java:176).
func (s BaseFieldOffsetStrategy) CreateOffsetsEnumsForAutomata(termsIndex index.Terms, doc int, results *[]OffsetsEnum) error {
	automata := s.components.Automata
	automataPostings := make([][]index.PostingsEnum, 0, len(automata))
	for i := 0; i < len(automata); i++ {
		automataPostings = append(automataPostings, []index.PostingsEnum{})
	}

	termsEnum, err := termsIndex.Iterator()
	if err != nil {
		return err
	}

	refBuilder := util.NewCharsRefBuilder()
	for {
		term, err := termsEnum.Next()
		if err != nil {
			return err
		}
		if term == nil {
			break
		}
		for i := 0; i < len(automata); i++ {
			automaton := automata[i]
			refBuilder.CopyUTF8BytesRef(term.Bytes)
			if charArrayMatcherMatchCharsRef(automaton, refBuilder.Get()) {
				postings, err := termsEnum.Postings(index.PostingsFlagOffsets)
				if err != nil {
					return err
				}
				advanced, err := postings.Advance(doc)
				if err != nil {
					return err
				}
				if doc == advanced {
					automataPostings[i] = append(automataPostings[i], postings)
				}
			}
		}
	}

	for i := 0; i < len(automata); i++ {
		automaton := automata[i]
		postingsEnums := automataPostings[i]
		if len(postingsEnums) == 0 {
			continue
		}
		// Build one OffsetsEnum exposing the automaton label as the term, and the sum of freq
		wildcardTerm := []byte(automaton.GetLabel())
		sumFreq := 0
		for _, postingsEnum := range postingsEnums {
			freq, err := postingsEnum.Freq()
			if err != nil {
				return err
			}
			sumFreq += freq
		}
		for _, postingsEnum := range postingsEnums {
			oe, err := NewOfPostings(wildcardTerm, sumFreq, postingsEnum)
			if err != nil {
				return err
			}
			*results = append(*results, oe)
		}
	}
	return nil
}

// bytesRefValue renders the bytes a Java BytesRef stands for. Unlike
// util.BytesRef.ValidBytes it never collapses a zero-length reference to nil,
// because Java's BytesRef is never null once constructed and OffsetsEnum's
// compareTo gives a nil term a distinct meaning.
func bytesRefValue(br *util.BytesRef) []byte {
	if br == nil || br.Bytes == nil {
		return []byte{}
	}
	return br.Bytes[br.Offset : br.Offset+br.Length]
}

// fieldMatcherRemappingLeafReader renders the anonymous FilterLeafReader
// subclass of createOffsetsEnumsWeightMatcher (FieldOffsetStrategy.java:117),
// which remaps every field the fieldMatcher accepts onto the field being
// highlighted.
type fieldMatcherRemappingLeafReader struct {
	*index.FilterLeafReader

	components    *UHComponents
	readerContext *index.LeafReaderContext
}

// newFieldMatcherRemappingLeafReader builds the wrapper and, as Java's
// LeafReader constructor does, its own LeafReaderContext.
func newFieldMatcherRemappingLeafReader(in index.LeafReader, components *UHComponents) *fieldMatcherRemappingLeafReader {
	r := &fieldMatcherRemappingLeafReader{
		FilterLeafReader: index.NewFilterLeafReader(in),
		components:       components,
	}
	r.readerContext = index.NewLeafReaderContext(r, nil, 0, 0)
	return r
}

// Terms renders the anonymous subclass's terms(String) override.
func (r *fieldMatcherRemappingLeafReader) Terms(field string) (index.Terms, error) {
	if r.components.FieldMatcher(field) {
		return r.FilterLeafReader.Terms(r.components.Field)
	}
	return r.FilterLeafReader.Terms(field)
}

// GetContext renders LeafReader.getContext(), which returns the reader's own
// LeafReaderContext rather than the wrapped reader's.
func (r *fieldMatcherRemappingLeafReader) GetContext() (index.IndexReaderContext, error) {
	return r.readerContext, nil
}

// GetCoreCacheHelper renders the anonymous subclass's getCoreCacheHelper()
// override, which returns null. So many subclasses do this! These ought to be
// a default or added via some intermediary like "FilterTransientLeafReader"
// (exception on close).
func (r *fieldMatcherRemappingLeafReader) GetCoreCacheHelper() index.CacheHelper { return nil }

// GetReaderCacheHelper renders the anonymous subclass's
// getReaderCacheHelper() override, which returns null.
func (r *fieldMatcherRemappingLeafReader) GetReaderCacheHelper() index.CacheHelper { return nil }

var _ index.LeafReader = (*fieldMatcherRemappingLeafReader)(nil)
