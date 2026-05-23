package uhighlight

import "fmt"

// TermSource is the minimal interface over a term-indexed field that
// TermVectorFilteredLeafReader uses as its "base" data source.  It mirrors the
// LeafReader/Terms/TermsEnum path in the Java original at the level of
// abstraction needed by the Go uhighlight package.
type TermSource interface {
	// TermEntries returns all TermVectorEntry items stored for the given field.
	TermEntries(field string) []TermVectorEntry
}

// TermVectorFilteredLeafReader wraps a base TermSource and exposes only the
// terms that are also present in a filter term set (typically from a term
// vector or MemoryIndex).  Term postings are read from the base source.
//
// Mirrors org.apache.lucene.search.uhighlight.TermVectorFilteredLeafReader.
type TermVectorFilteredLeafReader struct {
	base        TermSource
	filterTerms map[string]struct{} // terms present in the filter set
	fieldFilter string
}

// NewTermVectorFilteredLeafReader builds a reader that only exposes terms that
// appear in filterEntries for fieldFilter; base is the authoritative source of
// postings data.
func NewTermVectorFilteredLeafReader(base TermSource, filterEntries []TermVectorEntry, fieldFilter string) *TermVectorFilteredLeafReader {
	filter := make(map[string]struct{}, len(filterEntries))
	for _, e := range filterEntries {
		filter[e.Term] = struct{}{}
	}
	return &TermVectorFilteredLeafReader{
		base:        base,
		filterTerms: filter,
		fieldFilter: fieldFilter,
	}
}

// TermEntries returns TermVectorEntry items for field, filtered to those that
// also appear in the filter term set when the requested field matches
// fieldFilter.
func (r *TermVectorFilteredLeafReader) TermEntries(field string) []TermVectorEntry {
	all := r.base.TermEntries(field)
	if field != r.fieldFilter {
		return all
	}
	out := make([]TermVectorEntry, 0, len(all))
	for _, e := range all {
		if _, ok := r.filterTerms[e.Term]; ok {
			out = append(out, e)
		}
	}
	return out
}

// FilteredTermsIterator iterates over terms that are present in both the base
// and filter term enumerators.  Navigation is driven by the filter; postings
// are fetched from the base when the current filter term is confirmed to exist
// in the base.
//
// Mirrors the inner TermVectorFilteredTermsEnum in the Java original.
type FilteredTermsIterator struct {
	base   []TermVectorEntry // base term list (sorted by term)
	filter []TermVectorEntry // filter term list (sorted by term)
	pos    int               // position in filter list
	baseIdx map[string]int    // index into base by term
}

// NewFilteredTermsIterator builds an iterator.
func NewFilteredTermsIterator(base, filter []TermVectorEntry) *FilteredTermsIterator {
	idx := make(map[string]int, len(base))
	for i, e := range base {
		idx[e.Term] = i
	}
	return &FilteredTermsIterator{
		base:    base,
		filter:  filter,
		pos:     -1,
		baseIdx: idx,
	}
}

// Next advances to the next term in the filter list.
func (it *FilteredTermsIterator) Next() bool {
	it.pos++
	return it.pos < len(it.filter)
}

// Term returns the current term text.
func (it *FilteredTermsIterator) Term() string {
	if it.pos < 0 || it.pos >= len(it.filter) {
		return ""
	}
	return it.filter[it.pos].Term
}

// Entry returns the base TermVectorEntry for the current filter term.  An
// error is returned when the term does not appear in the base source, which
// mirrors the IllegalStateException thrown by the Java original.
func (it *FilteredTermsIterator) Entry() (TermVectorEntry, error) {
	term := it.Term()
	i, ok := it.baseIdx[term]
	if !ok {
		return TermVectorEntry{}, fmt.Errorf("uhighlight: term vector term %q does not appear in full index", term)
	}
	return it.base[i], nil
}
