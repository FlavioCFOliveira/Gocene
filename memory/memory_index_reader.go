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

// --- In-memory doc values implementations ---

// memNumericDV implements index.NumericDocValues for a single stored value.
type memNumericDV struct {
	value     int64
	docID     int
	exhausted bool
}

func (v *memNumericDV) DocID() int { return v.docID }

func (v *memNumericDV) NextDoc() (int, error) {
	if v.exhausted {
		return schema.NO_MORE_DOCS, nil
	}
	v.docID = 0
	v.exhausted = true
	return 0, nil
}

func (v *memNumericDV) Advance(target int) (int, error) {
	if target <= 0 && !v.exhausted {
		v.docID = 0
		v.exhausted = true
		return 0, nil
	}
	return schema.NO_MORE_DOCS, nil
}

func (v *memNumericDV) AdvanceExact(target int) (bool, error) {
	if target == 0 && !v.exhausted {
		v.docID = 0
		v.exhausted = true
		return true, nil
	}
	return false, nil
}

func (v *memNumericDV) LongValue() (int64, error) { return v.value, nil }
func (v *memNumericDV) Cost() int64               { return 1 }

// memBinaryDV implements index.BinaryDocValues for a single stored value.
type memBinaryDV struct {
	value     []byte
	docID     int
	exhausted bool
}

func (v *memBinaryDV) DocID() int { return v.docID }

func (v *memBinaryDV) NextDoc() (int, error) {
	if v.exhausted {
		return schema.NO_MORE_DOCS, nil
	}
	v.docID = 0
	v.exhausted = true
	return 0, nil
}

func (v *memBinaryDV) Advance(target int) (int, error) {
	if target <= 0 && !v.exhausted {
		v.docID = 0
		v.exhausted = true
		return 0, nil
	}
	return schema.NO_MORE_DOCS, nil
}

func (v *memBinaryDV) AdvanceExact(target int) (bool, error) {
	if target == 0 && !v.exhausted {
		v.docID = 0
		v.exhausted = true
		return true, nil
	}
	return false, nil
}

func (v *memBinaryDV) BinaryValue() ([]byte, error) { return v.value, nil }
func (v *memBinaryDV) Cost() int64                  { return 1 }

// memSortedDV implements index.SortedDocValues for a single stored value.
type memSortedDV struct {
	memNumericDV
	term       []byte
	valueCount int
}

func (v *memSortedDV) OrdValue() (int, error) { return 0, nil }
func (v *memSortedDV) LookupOrd(ord int) ([]byte, error) {
	if ord == 0 {
		return v.term, nil
	}
	return nil, nil
}
func (v *memSortedDV) GetValueCount() int { return v.valueCount }

// memSortedNumericDV implements index.SortedNumericDocValues for stored values.
type memSortedNumericDV struct {
	memNumericDV
	values []int64
	idx    int
}

func (v *memSortedNumericDV) NextValue() (int64, error) {
	if v.idx >= len(v.values) {
		return 0, nil
	}
	val := v.values[v.idx]
	v.idx++
	return val, nil
}

func (v *memSortedNumericDV) DocValueCount() (int, error) {
	return len(v.values), nil
}

// memSortedSetDV implements index.SortedSetDocValues for stored values.
type memSortedSetDV struct {
	docID        int
	exhausted    bool
	ordExhausted bool
	terms        [][]byte
	idx          int
}

func (v *memSortedSetDV) DocID() int { return v.docID }

func (v *memSortedSetDV) NextDoc() (int, error) {
	if v.exhausted {
		return schema.NO_MORE_DOCS, nil
	}
	v.docID = 0
	v.exhausted = true
	v.idx = 0
	v.ordExhausted = false
	return 0, nil
}

func (v *memSortedSetDV) Advance(target int) (int, error) {
	if target <= 0 && !v.exhausted {
		v.docID = 0
		v.exhausted = true
		v.idx = 0
		v.ordExhausted = false
		return 0, nil
	}
	return schema.NO_MORE_DOCS, nil
}

func (v *memSortedSetDV) AdvanceExact(target int) (bool, error) {
	if target == 0 && !v.exhausted {
		v.docID = 0
		v.exhausted = true
		v.idx = 0
		v.ordExhausted = false
		return true, nil
	}
	return false, nil
}

func (v *memSortedSetDV) NextOrd() (int, error) {
	if v.ordExhausted || v.idx >= len(v.terms) {
		v.ordExhausted = true
		return -1, nil
	}
	ord := v.idx
	v.idx++
	return ord, nil
}

func (v *memSortedSetDV) LookupOrd(ord int) ([]byte, error) {
	if ord >= 0 && ord < len(v.terms) {
		return v.terms[ord], nil
	}
	return nil, nil
}

func (v *memSortedSetDV) GetValueCount() int { return len(v.terms) }
func (v *memSortedSetDV) Cost() int64        { return 1 }

// --- In-memory point values ---

// memPointValues implements index.PointValues for a single-document in-memory store.
type memPointValues struct {
	meta *pointFieldMeta
}

func (pv *memPointValues) GetDocCount() int {
	if len(pv.meta.values) > 0 {
		return 1
	}
	return 0
}

func (pv *memPointValues) GetDocCountWithValue() int64 {
	if len(pv.meta.values) > 0 {
		return 1
	}
	return 0
}

func (pv *memPointValues) GetValueCount() int64 {
	return int64(len(pv.meta.values))
}

func (pv *memPointValues) GetMinPackedValue() ([]byte, error) {
	if len(pv.meta.values) == 0 {
		return nil, nil
	}
	min := make([]byte, len(pv.meta.values[0].packedValue))
	copy(min, pv.meta.values[0].packedValue)
	for _, pv := range pv.meta.values[1:] {
		for i, b := range pv.packedValue {
			if b < min[i] {
				min[i] = b
			}
		}
	}
	return min, nil
}

func (pv *memPointValues) GetMaxPackedValue() ([]byte, error) {
	if len(pv.meta.values) == 0 {
		return nil, nil
	}
	max := make([]byte, len(pv.meta.values[0].packedValue))
	copy(max, pv.meta.values[0].packedValue)
	for _, pv := range pv.meta.values[1:] {
		for i, b := range pv.packedValue {
			if b > max[i] {
				max[i] = b
			}
		}
	}
	return max, nil
}

func (pv *memPointValues) GetNumDimensions() int    { return pv.meta.numDims }
func (pv *memPointValues) GetBytesPerDimension() int { return pv.meta.bytesPerDim }

// --- memoryTermVectors ---

// memoryTermVectors implements index.TermVectors over a MemoryIndex's fields.
type memoryTermVectors struct {
	mi *MemoryIndex
}

func (tv *memoryTermVectors) Prefetch(_ []int) error { return nil }

func (tv *memoryTermVectors) Get(docID int) (index.Fields, error) {
	if docID != 0 {
		return nil, fmt.Errorf("document ID %d out of range (single-doc MemoryIndex)", docID)
	}
	tv.mi.mu.RLock()
	defer tv.mi.mu.RUnlock()
	fields := schema.NewMemoryFields()
	for name, mf := range tv.mi.fields {
		fields.AddField(name, newMemoryTerms(name, mf))
	}
	return fields, nil
}

func (tv *memoryTermVectors) GetField(docID int, field string) (schema.Terms, error) {
	if docID != 0 {
		return nil, fmt.Errorf("document ID %d out of range (single-doc MemoryIndex)", docID)
	}
	tv.mi.mu.RLock()
	defer tv.mi.mu.RUnlock()
	mf, ok := tv.mi.fields[field]
	if !ok || len(mf.terms) == 0 {
		return nil, nil
	}
	return newMemoryTerms(field, mf), nil
}

// --- index.IndexReaderInterface ---

func (r *memoryIndexReader) DocCount() int     { return 1 }
func (r *memoryIndexReader) NumDocs() int      { return 1 }
func (r *memoryIndexReader) MaxDoc() int       { return 1 }
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
	return &memoryTermVectors{mi: r.mi}, nil
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

// GetNumericDocValues returns numeric doc values for the given field.
func (r *memoryIndexReader) GetNumericDocValues(field string) (index.NumericDocValues, error) {
	r.mi.mu.RLock()
	defer r.mi.mu.RUnlock()
	dv, ok := r.mi.docValues[field]
	if !ok || dv.numeric == nil {
		return nil, nil
	}
	return &memNumericDV{value: *dv.numeric}, nil
}

// GetBinaryDocValues returns binary doc values for the given field.
func (r *memoryIndexReader) GetBinaryDocValues(field string) (index.BinaryDocValues, error) {
	r.mi.mu.RLock()
	defer r.mi.mu.RUnlock()
	dv, ok := r.mi.docValues[field]
	if !ok || dv.binary == nil {
		return nil, nil
	}
	return &memBinaryDV{value: dv.binary}, nil
}

// GetSortedDocValues returns sorted doc values for the given field.
func (r *memoryIndexReader) GetSortedDocValues(field string) (index.SortedDocValues, error) {
	r.mi.mu.RLock()
	defer r.mi.mu.RUnlock()
	dv, ok := r.mi.docValues[field]
	if !ok || dv.sorted == nil {
		return nil, nil
	}
	return &memSortedDV{
		memNumericDV: memNumericDV{value: 0},
		term:         dv.sorted,
		valueCount:   1,
	}, nil
}

// GetSortedNumericDocValues returns sorted numeric doc values for the given field.
func (r *memoryIndexReader) GetSortedNumericDocValues(field string) (index.SortedNumericDocValues, error) {
	r.mi.mu.RLock()
	defer r.mi.mu.RUnlock()
	dv, ok := r.mi.docValues[field]
	if !ok || dv.sortedNumeric == nil {
		return nil, nil
	}
	return &memSortedNumericDV{values: dv.sortedNumeric}, nil
}

// GetSortedSetDocValues returns sorted set doc values for the given field.
func (r *memoryIndexReader) GetSortedSetDocValues(field string) (index.SortedSetDocValues, error) {
	r.mi.mu.RLock()
	defer r.mi.mu.RUnlock()
	dv, ok := r.mi.docValues[field]
	if !ok || dv.sortedSet == nil {
		return nil, nil
	}
	return &memSortedSetDV{terms: dv.sortedSet}, nil
}

// GetNormValues returns norms for the given field.
func (r *memoryIndexReader) GetNormValues(field string) (index.NumericDocValues, error) {
	r.mi.mu.RLock()
	defer r.mi.mu.RUnlock()
	mf, ok := r.mi.fields[field]
	if !ok || len(mf.terms) == 0 {
		return nil, nil
	}
	// Compute norm from field's token count, matching the
	// DefaultSimilarity encoding used by IndexWriter:
	// norm = IntToByte4(numTerms)
	numTerms := len(mf.positions)
	if numTerms <= 0 {
		for _, freq := range mf.terms {
			numTerms += freq
		}
	}
	if numTerms <= 0 {
		return nil, nil
	}
	normByte, err := util.IntToByte4(numTerms)
	if err != nil {
		return nil, err
	}
	return &memNumericDV{value: int64(normByte)}, nil
}

// GetPointValues returns point values for the given field.
func (r *memoryIndexReader) GetPointValues(field string) (index.PointValues, error) {
	r.mi.mu.RLock()
	defer r.mi.mu.RUnlock()
	pfm, ok := r.mi.pointFields[field]
	if !ok || len(pfm.values) == 0 {
		return nil, nil
	}
	return &memPointValues{meta: pfm}, nil
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

func (mt *memoryTerms) HasFreqs() bool     { return true }
func (mt *memoryTerms) HasOffsets() bool   { return len(mt.offsets) > 0 }
func (mt *memoryTerms) HasPositions() bool { return len(mt.positions) > 0 }
func (mt *memoryTerms) HasPayloads() bool  { return false }

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
	freq       int
	positions  []int
	offsets    [][2]int
	posIdx     int
	positioned bool
}

func newMemoryPostingsEnum(freq int, positions []int, offsets [][2]int) *memoryPostingsEnum {
	return &memoryPostingsEnum{
		freq:             freq,
		positions:        positions,
		offsets:          offsets,
		posIdx:           -1,
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
