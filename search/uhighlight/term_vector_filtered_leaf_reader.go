package uhighlight

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// TermVectorFilteredLeafReader is a filtered LeafReader that only includes the
// terms that are also in a provided set of terms. Certain methods may be
// unimplemented or cause large operations on the underlying reader and be
// slow.
//
// This is the Go port of
// org.apache.lucene.search.uhighlight.TermVectorFilteredLeafReader from Apache
// Lucene 10.5.0, which extends FilterLeafReader. NOTE: super ("in") is
// baseLeafReader.
type TermVectorFilteredLeafReader struct {
	*index.FilterLeafReader

	in          index.LeafReader
	filterTerms index.Terms
	fieldFilter string
}

// NewTermVectorFilteredLeafReader constructs a FilterLeafReader based on the
// specified base reader. Renders
// `TermVectorFilteredLeafReader(LeafReader baseLeafReader, Terms filterTerms,
// String fieldFilter)` (TermVectorFilteredLeafReader.java:51).
//
// Note that base reader is closed if this FilterLeafReader is closed.
//
// baseLeafReader is the full/original reader; filterTerms is the set of terms
// to filter by -- probably from a TermVector or MemoryIndex; fieldFilter is
// the field to do this on.
func NewTermVectorFilteredLeafReader(baseLeafReader index.LeafReader, filterTerms index.Terms, fieldFilter string) *TermVectorFilteredLeafReader {
	return &TermVectorFilteredLeafReader{
		FilterLeafReader: index.NewFilterLeafReader(baseLeafReader),
		in:               baseLeafReader,
		filterTerms:      filterTerms,
		fieldFilter:      fieldFilter,
	}
}

// Terms renders `public Terms terms(String field)`
// (TermVectorFilteredLeafReader.java:58).
func (r *TermVectorFilteredLeafReader) Terms(field string) (index.Terms, error) {
	if field != r.fieldFilter {
		// proceed like normal for fields we're not interested in
		return r.FilterLeafReader.Terms(field)
	}
	terms, err := r.in.Terms(field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}
	return newTermsFilteredTerms(terms, r.filterTerms), nil
}

// GetCoreCacheHelper renders `public CacheHelper getCoreCacheHelper()`
// (TermVectorFilteredLeafReader.java:120), which returns null.
func (r *TermVectorFilteredLeafReader) GetCoreCacheHelper() index.CacheHelper { return nil }

// GetReaderCacheHelper renders `public CacheHelper getReaderCacheHelper()`
// (TermVectorFilteredLeafReader.java:125), which returns null.
func (r *TermVectorFilteredLeafReader) GetReaderCacheHelper() index.CacheHelper { return nil }

var _ index.LeafReader = (*TermVectorFilteredLeafReader)(nil)

// termsFilteredTerms renders the private static final class
// TermVectorFilteredLeafReader.TermsFilteredTerms
// (TermVectorFilteredLeafReader.java:68). NOTE: super ("in") is the baseTerms.
type termsFilteredTerms struct {
	*index.FilterTerms

	in          index.Terms
	filterTerms index.Terms
}

// newTermsFilteredTerms renders `TermsFilteredTerms(Terms baseTerms, Terms
// filterTerms)` (TermVectorFilteredLeafReader.java:73).
func newTermsFilteredTerms(baseTerms, filterTerms index.Terms) *termsFilteredTerms {
	return &termsFilteredTerms{
		FilterTerms: index.NewFilterTerms(baseTerms),
		in:          baseTerms,
		filterTerms: filterTerms,
	}
}

// TODO (from Java) delegate size()
// TODO (from Java) delegate getMin, getMax to filterTerms

// Iterator renders `public TermsEnum iterator()`
// (TermVectorFilteredLeafReader.java:83).
func (t *termsFilteredTerms) Iterator() (index.TermsEnum, error) {
	baseTermsEnum, err := t.in.Iterator()
	if err != nil {
		return nil, err
	}
	filteredTermsEnum, err := t.filterTerms.Iterator()
	if err != nil {
		return nil, err
	}
	return newTermVectorFilteredTermsEnum(baseTermsEnum, filteredTermsEnum), nil
}

// Intersect renders `public TermsEnum intersect(CompiledAutomaton compiled,
// BytesRef startTerm)` (TermVectorFilteredLeafReader.java:88).
func (t *termsFilteredTerms) Intersect(compiled *automaton.CompiledAutomaton, startTerm *index.Term) (index.TermsEnum, error) {
	baseTermsEnum, err := t.in.Iterator()
	if err != nil {
		return nil, err
	}
	filteredTermsEnum, err := t.filterTerms.Intersect(compiled, startTerm)
	if err != nil {
		return nil, err
	}
	return newTermVectorFilteredTermsEnum(baseTermsEnum, filteredTermsEnum), nil
}

var _ index.Terms = (*termsFilteredTerms)(nil)

// termVectorFilteredTermsEnum renders the private static final class
// TermVectorFilteredLeafReader.TermVectorFilteredTermsEnum
// (TermVectorFilteredLeafReader.java:95). NOTE: super ("in") is the
// filteredTermsEnum. This is different than the wrappers above because we
// navigate the terms using the filter.
type termVectorFilteredTermsEnum struct {
	*index.FilterTermsEnum

	in            index.TermsEnum // the filtered terms enum
	baseTermsEnum index.TermsEnum
}

// newTermVectorFilteredTermsEnum renders
// `TermVectorFilteredTermsEnum(TermsEnum baseTermsEnum, TermsEnum
// filteredTermsEnum)` (TermVectorFilteredLeafReader.java:103); note this is
// reversed from the constructors above.
func newTermVectorFilteredTermsEnum(baseTermsEnum, filteredTermsEnum index.TermsEnum) *termVectorFilteredTermsEnum {
	return &termVectorFilteredTermsEnum{
		FilterTermsEnum: index.NewFilterTermsEnum(filteredTermsEnum),
		in:              filteredTermsEnum,
		baseTermsEnum:   baseTermsEnum,
	}
}

// TODO (from Java) delegate docFreq & ttf (moveToCurrentTerm() then call on full?

// Postings renders `public PostingsEnum postings(PostingsEnum reuse, int
// flags)` (TermVectorFilteredLeafReader.java:111).
func (e *termVectorFilteredTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	if err := e.moveToCurrentTerm(); err != nil {
		return nil, err
	}
	return e.baseTermsEnum.Postings(flags)
}

// PostingsWithLiveDocs renders TermsEnum.postings(PostingsEnum, int) reached
// through the live-docs overload Gocene's TermsEnum interface declares; the
// term alignment it performs first is the same.
func (e *termVectorFilteredTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	if err := e.moveToCurrentTerm(); err != nil {
		return nil, err
	}
	return e.baseTermsEnum.PostingsWithLiveDocs(liveDocs, flags)
}

// moveToCurrentTerm renders the package-private
// TermVectorFilteredTermsEnum.moveToCurrentTerm()
// (TermVectorFilteredLeafReader.java:117), whose IllegalStateException is
// rendered as an error.
func (e *termVectorFilteredTermsEnum) moveToCurrentTerm() error {
	currentTerm := e.in.Term() // from filteredTermsEnum
	termInBothTermsEnum, err := e.baseTermsEnum.SeekExact(currentTerm)
	if err != nil {
		return err
	}
	if !termInBothTermsEnum {
		text := ""
		if currentTerm != nil && currentTerm.Bytes != nil {
			text = string(currentTerm.Bytes.ValidBytes())
		}
		return fmt.Errorf("uhighlight: term vector term '%s' does not appear in full index", text)
	}
	return nil
}

var _ index.TermsEnum = (*termVectorFilteredTermsEnum)(nil)
