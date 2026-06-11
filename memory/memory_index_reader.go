// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package memory

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// memoryIndexReader implements a minimal index.IndexReaderInterface over an
// in-memory MemoryIndex. It exposes a single document (doc 0) and provides
// term dictionary access so that search.IndexSearcher can execute queries
// directly against the in-memory data.
//
// This is the Go port of Lucene's MemoryIndex.MemoryIndexReader (an internal
// LeafReader subclass in org.apache.lucene.index.memory.MemoryIndex).
type memoryIndexReader struct {
	mi     *MemoryIndex
	closed atomic.Bool
	refs   atomic.Int32
}

// Ensure memoryIndexReader implements index.LeafReaderInterface.
var _ index.LeafReaderInterface = (*memoryIndexReader)(nil)

// --- index.IndexReaderInterface ---

func (r *memoryIndexReader) DocCount() int    { return 1 }
func (r *memoryIndexReader) NumDocs() int     { return 1 }
func (r *memoryIndexReader) MaxDoc() int      { return 1 }
func (r *memoryIndexReader) HasDeletions() bool { return false }
func (r *memoryIndexReader) NumDeletedDocs() int { return 0 }

func (r *memoryIndexReader) Close() error {
	r.closed.Store(true)
	return nil
}

func (r *memoryIndexReader) EnsureOpen() error {
	if r.closed.Load() {
		return fmt.Errorf("memoryIndexReader: already closed")
	}
	return nil
}

func (r *memoryIndexReader) IncRef() error {
	r.refs.Add(1)
	return nil
}

func (r *memoryIndexReader) DecRef() error {
	if r.refs.Add(-1) <= 0 {
		return r.Close()
	}
	return nil
}

func (r *memoryIndexReader) TryIncRef() bool {
	if r.closed.Load() {
		return false
	}
	r.refs.Add(1)
	return true
}

func (r *memoryIndexReader) GetRefCount() int32 { return r.refs.Load() }

func (r *memoryIndexReader) GetContext() (index.IndexReaderContext, error) {
	return index.NewLeafReaderContext(r, nil, 0, 0), nil
}

func (r *memoryIndexReader) Leaves() ([]*index.LeafReaderContext, error) {
	ctx, err := r.GetContext()
	if err != nil {
		return nil, err
	}
	if lctx, ok := ctx.(*index.LeafReaderContext); ok {
		return []*index.LeafReaderContext{lctx}, nil
	}
	return nil, fmt.Errorf("memoryIndexReader: unexpected context type")
}

func (r *memoryIndexReader) StoredFields() (index.StoredFields, error) {
	return nil, fmt.Errorf("memoryIndexReader: stored fields not supported")
}

func (r *memoryIndexReader) TermVectors() (index.TermVectors, error) {
	return nil, fmt.Errorf("memoryIndexReader: term vectors not supported")
}

// Terms returns the Terms for a given field, or nil if the field does not exist.
// This is the bridge that allows search.IndexSearcher to find and iterate over terms.
func (r *memoryIndexReader) Terms(field string) (schema.Terms, error) {
	r.mi.mu.RLock()
	defer r.mi.mu.RUnlock()

	mf, ok := r.mi.fields[field]
	if !ok || len(mf.terms) == 0 {
		return nil, nil
	}
	return newMemoryTerms(field, mf), nil
}

// GetCoreCacheKey returns a cache key for this reader.
func (r *memoryIndexReader) GetCoreCacheKey() interface{} { return r }

// GetTermVectors returns term vectors for the given document.
// MemoryIndex does not store term vectors, so this returns nil.
func (r *memoryIndexReader) GetTermVectors(docID int) (index.Fields, error) {
	return nil, nil
}

// GetLiveDocs returns nil (no deletions in a MemoryIndex).
func (r *memoryIndexReader) GetLiveDocs() util.Bits { return nil }

// newMemoryIndexReader creates a new reader wrapping the given MemoryIndex.
func newMemoryIndexReader(mi *MemoryIndex) *memoryIndexReader {
	r := &memoryIndexReader{mi: mi}
	r.refs.Store(1)
	return r
}

// --- memoryTerms: schema.Terms implementation ---

// memoryTerms implements schema.Terms over a single field's in-memory term data.
type memoryTerms struct {
	schema.TermsBase
	field     string
	terms     []string       // sorted list of term texts
	freqs     map[string]int // term -> frequency
	positions map[string][]int
	offsets   map[string][][2]int
}

func newMemoryTerms(field string, mf *memoryField) *memoryTerms {
	terms := make([]string, 0, len(mf.terms))
	for t := range mf.terms {
		terms = append(terms, t)
	}
	sort.Strings(terms)

	return &memoryTerms{
		field:     field,
		terms:     terms,
		freqs:     mf.terms,
		positions: mf.termPositions,
		offsets:   mf.termOffsets,
	}
}

func (mt *memoryTerms) Size() int64 { return int64(len(mt.terms)) }

func (mt *memoryTerms) GetDocCount() (int, error) { return 1, nil }

func (mt *memoryTerms) GetSumDocFreq() (int64, error) {
	return int64(len(mt.terms)), nil
}

func (mt *memoryTerms) GetSumTotalTermFreq() (int64, error) {
	var total int64
	for _, f := range mt.freqs {
		total += int64(f)
	}
	return total, nil
}

func (mt *memoryTerms) HasFreqs() bool    { return true }
func (mt *memoryTerms) HasOffsets() bool  { return len(mt.offsets) > 0 }
func (mt *memoryTerms) HasPositions() bool { return len(mt.positions) > 0 }
func (mt *memoryTerms) HasPayloads() bool { return false }

func (mt *memoryTerms) GetMin() (*schema.Term, error) {
	if len(mt.terms) == 0 {
		return nil, nil
	}
	return schema.NewTerm(mt.field, mt.terms[0]), nil
}

func (mt *memoryTerms) GetMax() (*schema.Term, error) {
	if len(mt.terms) == 0 {
		return nil, nil
	}
	return schema.NewTerm(mt.field, mt.terms[len(mt.terms)-1]), nil
}

func (mt *memoryTerms) GetIterator() (schema.TermsEnum, error) {
	return newMemoryTermsEnum(mt), nil
}

func (mt *memoryTerms) GetIteratorWithSeek(seekTerm *schema.Term) (schema.TermsEnum, error) {
	enum := newMemoryTermsEnum(mt)
	if seekTerm != nil && seekTerm.Field == mt.field {
		_, _ = enum.SeekCeil(seekTerm)
	}
	return enum, nil
}

func (mt *memoryTerms) GetPostingsReader(termText string, flags int) (schema.PostingsEnum, error) {
	freq, ok := mt.freqs[termText]
	if !ok {
		return nil, nil
	}

	var positions []int
	var offsets [][2]int
	if flags&schema.PostingsFlagPositions != 0 {
		positions = mt.positions[termText]
	}
	if flags&schema.PostingsFlagOffsets != 0 {
		offsets = mt.offsets[termText]
	}

	return newMemoryPostingsEnum(freq, positions, offsets), nil
}

// --- memoryTermsEnum: schema.TermsEnum implementation ---

type memoryTermsEnum struct {
	schema.TermsEnumBase
	mt      *memoryTerms
	pos     int
	started bool
}

func newMemoryTermsEnum(mt *memoryTerms) *memoryTermsEnum {
	return &memoryTermsEnum{mt: mt, pos: -1}
}

func (e *memoryTermsEnum) Next() (*schema.Term, error) {
	e.pos++
	if e.pos >= len(e.mt.terms) {
		return nil, nil
	}
	term := schema.NewTerm(e.mt.field, e.mt.terms[e.pos])
	e.SetCurrentTerm(term)
	return term, nil
}

func (e *memoryTermsEnum) SeekCeil(seek *schema.Term) (*schema.Term, error) {
	if seek == nil || seek.Field != e.mt.field {
		e.pos = -1
		return e.Next()
	}

	idx := sort.SearchStrings(e.mt.terms, seek.Text())
	e.pos = idx - 1
	return e.Next()
}

func (e *memoryTermsEnum) SeekExact(seek *schema.Term) (bool, error) {
	if seek == nil || seek.Field != e.mt.field {
		return false, nil
	}
	idx := sort.SearchStrings(e.mt.terms, seek.Text())
	if idx < len(e.mt.terms) && e.mt.terms[idx] == seek.Text() {
		e.pos = idx
		e.SetCurrentTerm(seek)
		return true, nil
	}
	return false, nil
}

func (e *memoryTermsEnum) DocFreq() (int, error) {
	if e.pos < 0 || e.pos >= len(e.mt.terms) {
		return 0, nil
	}
	return 1, nil // single doc, so docFreq is always 1
}

func (e *memoryTermsEnum) TotalTermFreq() (int64, error) {
	if e.pos < 0 || e.pos >= len(e.mt.terms) {
		return 0, nil
	}
	return int64(e.mt.freqs[e.mt.terms[e.pos]]), nil
}

func (e *memoryTermsEnum) Postings(flags int) (schema.PostingsEnum, error) {
	if e.pos < 0 || e.pos >= len(e.mt.terms) {
		return &schema.EmptyPostingsEnum{}, nil
	}
	termText := e.mt.terms[e.pos]
	return e.mt.GetPostingsReader(termText, flags)
}

func (e *memoryTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (schema.PostingsEnum, error) {
	return e.Postings(flags)
}

// --- memoryPostingsEnum: schema.PostingsEnum implementation ---

// memoryPostingsEnum implements schema.PostingsEnum for a single-document
// in-memory index. It returns doc 0 with the correct term frequency,
// positions, and offsets.
type memoryPostingsEnum struct {
	schema.PostingsEnumBase
	freq        int
	positions   []int
	offsets     [][2]int
	posIdx      int
	positioned  bool
}

func newMemoryPostingsEnum(freq int, positions []int, offsets [][2]int) *memoryPostingsEnum {
	return &memoryPostingsEnum{
		freq:            freq,
		positions:       positions,
		offsets:         offsets,
		posIdx:          -1,
		PostingsEnumBase: schema.NewPostingsEnumBase(-1),
	}
}

func (p *memoryPostingsEnum) NextDoc() (int, error) {
	if !p.positioned {
		p.positioned = true
		p.CurrentDoc = 0
		return 0, nil
	}
	p.CurrentDoc = schema.NO_MORE_DOCS
	return schema.NO_MORE_DOCS, nil
}

func (p *memoryPostingsEnum) Advance(target int) (int, error) {
	if !p.positioned && target <= 0 {
		p.positioned = true
		p.CurrentDoc = 0
		return 0, nil
	}
	p.positioned = true
	p.CurrentDoc = schema.NO_MORE_DOCS
	return schema.NO_MORE_DOCS, nil
}

func (p *memoryPostingsEnum) Freq() (int, error) {
	if !p.positioned || p.CurrentDoc == schema.NO_MORE_DOCS {
		return 0, nil
	}
	return p.freq, nil
}

func (p *memoryPostingsEnum) NextPosition() (int, error) {
	p.posIdx++
	if p.posIdx >= len(p.positions) {
		return schema.NO_MORE_POSITIONS, nil
	}
	return p.positions[p.posIdx], nil
}

func (p *memoryPostingsEnum) StartOffset() (int, error) {
	if p.posIdx < 0 || p.posIdx >= len(p.offsets) {
		return -1, nil
	}
	return p.offsets[p.posIdx][0], nil
}

func (p *memoryPostingsEnum) EndOffset() (int, error) {
	if p.posIdx < 0 || p.posIdx >= len(p.offsets) {
		return -1, nil
	}
	return p.offsets[p.posIdx][1], nil
}

func (p *memoryPostingsEnum) GetPayload() ([]byte, error) {
	return nil, nil
}

func (p *memoryPostingsEnum) Cost() int64 {
	return 1
}

// --- MemoryIndex search methods ---

// CreateSearcher returns a search.IndexSearcher that can query this in-memory index.
// This is the Go port of Lucene's MemoryIndex.createSearcher().
//
// The returned searcher wraps a minimal IndexReaderInterface that exposes the
// in-memory term dictionary. It supports term-based queries (TermQuery,
// BooleanQuery, PhraseQuery, prefix, regex, etc.) but not stored-field
// retrieval.
func (mi *MemoryIndex) CreateSearcher() (*search.IndexSearcher, error) {
	reader := newMemoryIndexReader(mi)
	return search.NewIndexSearcher(reader), nil
}

// Search runs a query against this in-memory index and returns the top results.
// Since MemoryIndex contains a single document, results contain at most one hit
// (doc 0) if the query matches.
//
// This is a convenience wrapper around CreateSearcher() + Search().
func (mi *MemoryIndex) Search(query search.Query, n int) (*search.TopDocs, error) {
	searcher, err := mi.CreateSearcher()
	if err != nil {
		return nil, err
	}
	return searcher.Search(query, n)
}

// String returns a compact summary of this reader.
func (r *memoryIndexReader) String() string {
	var sb strings.Builder
	sb.WriteString("memoryIndexReader{fields:")
	r.mi.mu.RLock()
	defer r.mi.mu.RUnlock()
	first := true
	for name := range r.mi.fields {
		if !first {
			sb.WriteString(",")
		}
		sb.WriteString(name)
		first = false
	}
	sb.WriteString("}")
	return sb.String()
}
