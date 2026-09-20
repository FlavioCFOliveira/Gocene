package uhighlight

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// OverlaySingleDocTermsLeafReader overlays a 2nd LeafReader for the terms of
// one field, otherwise the primary reader is consulted. The 2nd reader is
// assumed to have one document of 0 and we remap it to a target doc ID.
//
// This is the Go port of
// org.apache.lucene.search.uhighlight.OverlaySingleDocTermsLeafReader from
// Apache Lucene 10.5.0, which extends FilterLeafReader.
type OverlaySingleDocTermsLeafReader struct {
	*index.FilterLeafReader

	in             index.LeafReader
	in2            index.LeafReader
	in2Field       string
	in2TargetDocID int
}

// NewOverlaySingleDocTermsLeafReader renders
// `OverlaySingleDocTermsLeafReader(LeafReader in, LeafReader in2, String
// in2Field, int in2TargetDocId)` (OverlaySingleDocTermsLeafReader.java:41).
// Java asserts in2.maxDoc() == 1.
func NewOverlaySingleDocTermsLeafReader(in, in2 index.LeafReader, in2Field string, in2TargetDocID int) *OverlaySingleDocTermsLeafReader {
	return &OverlaySingleDocTermsLeafReader{
		FilterLeafReader: index.NewFilterLeafReader(in),
		in:               in,
		in2:              in2,
		in2Field:         in2Field,
		in2TargetDocID:   in2TargetDocID,
	}
}

// Terms renders `public Terms terms(String field)`
// (OverlaySingleDocTermsLeafReader.java:52).
func (r *OverlaySingleDocTermsLeafReader) Terms(field string) (index.Terms, error) {
	if r.in2Field != field {
		return r.in.Terms(field)
	}

	// Shifts leafReader in2 with only doc ID 0 to a target doc ID
	terms, err := r.in2.Terms(field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}
	if r.in2TargetDocID == 0 { // no doc ID remapping to do
		return terms, nil
	}
	return &overlayRemappedTerms{
		FilterTerms:    index.NewFilterTerms(terms),
		in:             terms,
		in2TargetDocID: r.in2TargetDocID,
	}, nil
}

// GetCoreCacheHelper renders `public CacheHelper getCoreCacheHelper()`
// (OverlaySingleDocTermsLeafReader.java:106), which returns null.
func (r *OverlaySingleDocTermsLeafReader) GetCoreCacheHelper() index.CacheHelper { return nil }

// GetReaderCacheHelper renders `public CacheHelper getReaderCacheHelper()`
// (OverlaySingleDocTermsLeafReader.java:111), which returns null.
func (r *OverlaySingleDocTermsLeafReader) GetReaderCacheHelper() index.CacheHelper { return nil }

var _ index.LeafReader = (*OverlaySingleDocTermsLeafReader)(nil)

// overlayRemappedTerms renders the anonymous FilterTerms subclass returned by
// OverlaySingleDocTermsLeafReader.terms (OverlaySingleDocTermsLeafReader.java:65).
type overlayRemappedTerms struct {
	*index.FilterTerms

	in             index.Terms
	in2TargetDocID int
}

// Iterator renders the anonymous subclass's iterator() override.
func (t *overlayRemappedTerms) Iterator() (index.TermsEnum, error) {
	termsEnum, err := t.in.Iterator()
	if err != nil {
		return nil, err
	}
	return t.filterTermsEnum(termsEnum), nil
}

// Intersect renders the anonymous subclass's intersect(CompiledAutomaton,
// BytesRef) override.
func (t *overlayRemappedTerms) Intersect(compiled *automaton.CompiledAutomaton, startTerm *index.Term) (index.TermsEnum, error) {
	termsEnum, err := t.in.Intersect(compiled, startTerm)
	if err != nil {
		return nil, err
	}
	return t.filterTermsEnum(termsEnum), nil
}

// filterTermsEnum renders the private helper of the same name
// (OverlaySingleDocTermsLeafReader.java:77).
func (t *overlayRemappedTerms) filterTermsEnum(termsEnum index.TermsEnum) index.TermsEnum {
	return &overlayRemappedTermsEnum{
		FilterTermsEnum: index.NewFilterTermsEnum(termsEnum),
		in:              termsEnum,
		in2TargetDocID:  t.in2TargetDocID,
	}
}

var _ index.Terms = (*overlayRemappedTerms)(nil)

// overlayRemappedTermsEnum renders the anonymous FilterTermsEnum subclass
// created by filterTermsEnum (OverlaySingleDocTermsLeafReader.java:78).
type overlayRemappedTermsEnum struct {
	*index.FilterTermsEnum

	in             index.TermsEnum
	in2TargetDocID int
}

// Postings renders the anonymous subclass's postings(PostingsEnum, int)
// override. TODO (from Java) 'reuse' will always fail to reuse unless we
// unwrap it.
func (e *overlayRemappedTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	postings, err := e.in.Postings(flags)
	if err != nil {
		return nil, err
	}
	if postings == nil {
		return nil, nil
	}
	return &overlayRemappedPostingsEnum{
		FilterPostingsEnum: index.NewFilterPostingsEnum(postings),
		in:                 postings,
		in2TargetDocID:     e.in2TargetDocID,
	}, nil
}

// PostingsWithLiveDocs renders TermsEnum.postings(PostingsEnum, int) reached
// through the live-docs overload Gocene's TermsEnum interface declares; the
// remapping the anonymous subclass applies is the same.
func (e *overlayRemappedTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	postings, err := e.in.PostingsWithLiveDocs(liveDocs, flags)
	if err != nil {
		return nil, err
	}
	if postings == nil {
		return nil, nil
	}
	return &overlayRemappedPostingsEnum{
		FilterPostingsEnum: index.NewFilterPostingsEnum(postings),
		in:                 postings,
		in2TargetDocID:     e.in2TargetDocID,
	}, nil
}

var _ index.TermsEnum = (*overlayRemappedTermsEnum)(nil)

// overlayRemappedPostingsEnum renders the anonymous FilterPostingsEnum
// subclass created inside postings (OverlaySingleDocTermsLeafReader.java:82),
// which remaps doc ID 0 onto the target doc ID.
type overlayRemappedPostingsEnum struct {
	*index.FilterPostingsEnum

	in             index.PostingsEnum
	in2TargetDocID int
}

// NextDoc renders the anonymous subclass's nextDoc() override.
func (p *overlayRemappedPostingsEnum) NextDoc() (int, error) {
	doc, err := p.in.NextDoc()
	if err != nil {
		return 0, err
	}
	if doc == 0 {
		return p.in2TargetDocID, nil
	}
	return doc, nil
}

// Advance renders the anonymous subclass's advance(int) override, which
// defers to the linear slowAdvance helper of DocIdSetIterator.
func (p *overlayRemappedPostingsEnum) Advance(target int) (int, error) {
	return util.SlowAdvance(p, target)
}

// DocID renders the anonymous subclass's docID() override.
func (p *overlayRemappedPostingsEnum) DocID() int {
	doc := p.in.DocID()
	if doc == 0 {
		return p.in2TargetDocID
	}
	return doc
}

var _ index.PostingsEnum = (*overlayRemappedPostingsEnum)(nil)
