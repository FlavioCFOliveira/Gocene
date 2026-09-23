// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of the private inner classes of
// org.apache.lucene.index.memory.MemoryIndex that make up its search support:
// MemoryIndexReader, MemoryFields, MemoryTermsEnum, MemoryPostingsEnum,
// MemoryIndexPointValues, MemoryFloatVectorValues and MemoryByteVectorValues.

package memory

import (
	"fmt"
	"sort"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// memoryIndexReader provides search support for Lucene framework integration;
// it implements all methods required by the Lucene IndexReader contracts.
//
// Renders the private final inner class MemoryIndex.MemoryIndexReader
// (MemoryIndex.java:1571), which extends LeafReader.
type memoryIndexReader struct {
	mi           *MemoryIndex
	memoryFields *memoryFields
	fieldInfos   *spi.FieldInfos

	// readerContext, refs and closed carry the state that Java inherits from
	// IndexReader; Go has no class inheritance, so the concrete reader holds
	// it. refs starts at 1, as IndexReader's refCount does.
	readerContext *spi.LeafReaderContext
	refs          atomic.Int32
	closed        atomic.Bool
}

var _ index.LeafReader = (*memoryIndexReader)(nil)

// newMemoryIndexReader renders `private MemoryIndexReader()`
// (MemoryIndex.java:1576).
func newMemoryIndexReader(mi *MemoryIndex) *memoryIndexReader {
	r := &memoryIndexReader{mi: mi}
	r.memoryFields = &memoryFields{mi: mi, fields: mi.fields}

	names := mi.sortedFieldNames()
	fieldInfosArr := make([]*spi.FieldInfo, 0, len(names))
	for _, name := range names {
		inf := mi.fields[name]
		inf.prepareDocValuesAndPointValues()
		fieldInfosArr = append(fieldInfosArr, inf.fieldInfo)
	}

	r.fieldInfos = spi.NewFieldInfos(fieldInfosArr...)
	r.refs.Store(1)
	r.readerContext = spi.NewLeafReaderContextForReader(r)
	return r
}

// getInfoForExpectedDocValuesType renders
// `private Info getInfoForExpectedDocValuesType(String fieldName, DocValuesType expectedType)`
// (MemoryIndex.java:1590).
func (r *memoryIndexReader) getInfoForExpectedDocValuesType(fieldName string, expectedType spi.DocValuesType) *info {
	if expectedType == spi.DocValuesTypeNone {
		return nil
	}
	inf := r.mi.fields[fieldName]
	if inf == nil {
		return nil
	}
	if inf.fieldInfo.DocValuesType() != expectedType {
		return nil
	}
	return inf
}

// GetLiveDocs renders `public Bits getLiveDocs()` (MemoryIndex.java:1605).
func (r *memoryIndexReader) GetLiveDocs() util.Bits { return nil }

// GetFieldInfos renders `public FieldInfos getFieldInfos()` (MemoryIndex.java:1610).
func (r *memoryIndexReader) GetFieldInfos() *spi.FieldInfos { return r.fieldInfos }

// GetNumericDocValues renders `public NumericDocValues getNumericDocValues(String field)`
// (MemoryIndex.java:1615).
func (r *memoryIndexReader) GetNumericDocValues(field string) (spi.NumericDocValues, error) {
	inf := r.getInfoForExpectedDocValuesType(field, spi.DocValuesTypeNumeric)
	if inf == nil {
		return nil, nil
	}
	return newSingleValuedNumericDocValues(inf.numericProducer.dvLongValues[0]), nil
}

// binaryOverSortedDocValues renders the anonymous BinaryDocValues of
// `public BinaryDocValues getBinaryDocValues(String field)`
// (MemoryIndex.java:1624): it wraps a SortedDocValues and makes it look like
// it is binary.
type binaryOverSortedDocValues struct {
	in spi.SortedDocValues
}

func (d *binaryOverSortedDocValues) BinaryValue() ([]byte, error) {
	ord, err := d.in.OrdValue()
	if err != nil {
		return nil, err
	}
	return d.in.LookupOrd(ord)
}

func (d *binaryOverSortedDocValues) AdvanceExact(target int) (bool, error) {
	return d.in.AdvanceExact(target)
}

func (d *binaryOverSortedDocValues) DocID() int { return d.in.DocID() }

func (d *binaryOverSortedDocValues) NextDoc() (int, error) { return d.in.NextDoc() }

func (d *binaryOverSortedDocValues) Advance(target int) (int, error) { return d.in.Advance(target) }

func (d *binaryOverSortedDocValues) Cost() int64 { return d.in.Cost() }

func (d *binaryOverSortedDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(d, upTo, bitSet, offset)
}

func (d *binaryOverSortedDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(d)
}

// GetBinaryDocValues renders `public BinaryDocValues getBinaryDocValues(String field)`
// (MemoryIndex.java:1624).
func (r *memoryIndexReader) GetBinaryDocValues(field string) (spi.BinaryDocValues, error) {
	in := r.getSortedDocValuesOfType(field, spi.DocValuesTypeBinary)
	if in == nil {
		return nil, nil
	}
	return &binaryOverSortedDocValues{in: in}, nil
}

// GetSortedDocValues renders `public SortedDocValues getSortedDocValues(String field)`
// (MemoryIndex.java:1664).
func (r *memoryIndexReader) GetSortedDocValues(field string) (spi.SortedDocValues, error) {
	return r.getSortedDocValuesOfType(field, spi.DocValuesTypeSorted), nil
}

// getSortedDocValuesOfType renders
// `private SortedDocValues getSortedDocValues(String field, DocValuesType docValuesType)`
// (MemoryIndex.java:1668).
func (r *memoryIndexReader) getSortedDocValuesOfType(field string, docValuesType spi.DocValuesType) spi.SortedDocValues {
	inf := r.getInfoForExpectedDocValuesType(field, docValuesType)
	if inf != nil {
		return newSingleValuedSortedDocValues(inf.binaryProducer.dvBytesValuesSet)
	}
	return nil
}

// GetSortedNumericDocValues renders
// `public SortedNumericDocValues getSortedNumericDocValues(String field)`
// (MemoryIndex.java:1678).
func (r *memoryIndexReader) GetSortedNumericDocValues(field string) (spi.SortedNumericDocValues, error) {
	inf := r.getInfoForExpectedDocValuesType(field, spi.DocValuesTypeSortedNumeric)
	if inf != nil {
		return newSortedNumericDocValuesOverArray(inf.numericProducer.dvLongValues, inf.numericProducer.count), nil
	}
	return nil, nil
}

// GetSortedSetDocValues renders `public SortedSetDocValues getSortedSetDocValues(String field)`
// (MemoryIndex.java:1688).
func (r *memoryIndexReader) GetSortedSetDocValues(field string) (spi.SortedSetDocValues, error) {
	inf := r.getInfoForExpectedDocValuesType(field, spi.DocValuesTypeSortedSet)
	if inf != nil {
		return newBytesRefHashSortedSetDocValues(
			inf.bytesRefHashProducer.dvBytesRefHashValuesSet, inf.bytesRefHashProducer.bytesIds), nil
	}
	return nil, nil
}

// GetDocValuesSkipper renders `public DocValuesSkipper getDocValuesSkipper(String field)`
// (MemoryIndex.java:1699).
func (r *memoryIndexReader) GetDocValuesSkipper(field string) (spi.DocValuesSkipper, error) {
	// Skipping isn't needed on a 1-doc index.
	return nil, nil
}

// GetPointValues renders `public PointValues getPointValues(String fieldName)`
// (MemoryIndex.java:1705).
func (r *memoryIndexReader) GetPointValues(fieldName string) (spi.PointValues, error) {
	inf := r.mi.fields[fieldName]
	if inf == nil || inf.pointValues == nil {
		return nil, nil
	}
	return newMemoryIndexPointValues(inf)
}

// GetFloatVectorValues renders `public FloatVectorValues getFloatVectorValues(String fieldName)`
// (MemoryIndex.java:1714).
func (r *memoryIndexReader) GetFloatVectorValues(fieldName string) (spi.FloatVectorValues, error) {
	inf := r.mi.fields[fieldName]
	if inf == nil || inf.floatVectorValues == nil {
		return nil, nil
	}
	return &memoryFloatVectorValues{info: inf}, nil
}

// GetByteVectorValues renders `public ByteVectorValues getByteVectorValues(String fieldName)`
// (MemoryIndex.java:1723).
func (r *memoryIndexReader) GetByteVectorValues(fieldName string) (spi.ByteVectorValues, error) {
	inf := r.mi.fields[fieldName]
	if inf == nil || inf.byteVectorValues == nil {
		return nil, nil
	}
	return &memoryByteVectorValues{info: inf}, nil
}

// SearchNearestVectorsCollector renders
// `public void searchNearestVectors(String field, float[] target, KnnCollector knnCollector, AcceptDocs acceptDocs)`
// (MemoryIndex.java:1732), whose body is empty.
func (r *memoryIndexReader) SearchNearestVectorsCollector(field string, target []float32, knnCollector spi.KnnCollector, acceptDocs util.Bits) error {
	return nil
}

// SearchNearestVectorsByteCollector renders
// `public void searchNearestVectors(String field, byte[] target, KnnCollector knnCollector, AcceptDocs acceptDocs)`
// (MemoryIndex.java:1736), whose body is empty.
func (r *memoryIndexReader) SearchNearestVectorsByteCollector(field string, target []byte, knnCollector spi.KnnCollector, acceptDocs util.Bits) error {
	return nil
}

// SearchNearestVectors renders the final convenience overload
// LeafReader.searchNearestVectors(String, float[], int, AcceptDocs, int),
// which Java inherits. Since the abstract overload above collects nothing,
// the convenience overload returns no hits.
func (r *memoryIndexReader) SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits, visitedLimit int) (spi.TopDocs, error) {
	return spi.TopDocs{}, nil
}

// CheckIntegrity renders `public void checkIntegrity()` (MemoryIndex.java:1740).
func (r *memoryIndexReader) CheckIntegrity() error {
	// no-op
	return nil
}

// Terms renders `public Terms terms(String field)` (MemoryIndex.java:1745).
func (r *memoryIndexReader) Terms(field string) (spi.Terms, error) {
	return r.memoryFields.Terms(field)
}

// memoryFields renders the private inner class
// MemoryIndexReader.MemoryFields (MemoryIndex.java:1749), which extends Fields.
type memoryFields struct {
	spi.FieldsBase
	// mi is the enclosing MemoryIndex; Java reaches storeOffsets and
	// storePayloads through the chain of enclosing instances.
	mi     *MemoryIndex
	fields map[string]*info
}

// nonEmptyFieldNames returns the names of the fields with at least one token,
// in ascending order. It renders the stream filter shared by MemoryFields'
// iterator() and size().
func (f *memoryFields) nonEmptyFieldNames() []string {
	names := make([]string, 0, len(f.fields))
	for name, inf := range f.fields {
		if inf.numTokens > 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// memoryFieldIterator renders the java.util.Iterator<String> returned by
// MemoryFields.iterator().
type memoryFieldIterator struct {
	names []string
	pos   int
}

func (it *memoryFieldIterator) HasNext() bool { return it.pos < len(it.names) }

func (it *memoryFieldIterator) Next() (string, error) {
	if it.pos >= len(it.names) {
		return "", nil
	}
	name := it.names[it.pos]
	it.pos++
	return name, nil
}

// Iterator renders `public Iterator<String> iterator()` (MemoryIndex.java:1758).
func (f *memoryFields) Iterator() (spi.FieldIterator, error) {
	return &memoryFieldIterator{names: f.nonEmptyFieldNames()}, nil
}

// Terms renders `public Terms terms(final String field)` (MemoryIndex.java:1766).
func (f *memoryFields) Terms(field string) (spi.Terms, error) {
	inf := f.fields[field]
	if inf == nil || inf.numTokens <= 0 {
		return nil, nil
	}
	return &memoryFieldTerms{mi: f.mi, field: field, info: inf}, nil
}

// Size renders `public int size()` (MemoryIndex.java:1822).
func (f *memoryFields) Size() int { return len(f.nonEmptyFieldNames()) }

// memoryFieldTerms renders the anonymous Terms subclass returned by
// MemoryFields.terms(String) (MemoryIndex.java:1771).
type memoryFieldTerms struct {
	spi.TermsBase
	mi    *MemoryIndex
	field string
	info  *info
}

// Field returns the name of the field these Terms describe. Lucene's Terms
// declares no such member; Gocene's spi.Terms requires it.
func (t *memoryFieldTerms) Field() string { return t.field }

// Iterator renders `public TermsEnum iterator()` (MemoryIndex.java:1773).
func (t *memoryFieldTerms) Iterator() (spi.TermsEnum, error) {
	return newMemoryTermsEnum(t.mi, t.field, t.info), nil
}

// GetIteratorWithSeek returns an iterator positioned at or after seekTerm.
// Lucene's Terms declares no such member; Gocene's spi.Terms requires it, and
// the faithful body is iterator() followed by TermsEnum.seekCeil.
func (t *memoryFieldTerms) GetIteratorWithSeek(seekTerm *spi.Term) (spi.TermsEnum, error) {
	e, err := t.Iterator()
	if err != nil {
		return nil, err
	}
	if seekTerm != nil {
		if _, err := e.SeekCeil(seekTerm); err != nil {
			return nil, err
		}
	}
	return e, nil
}

// GetPostingsReader returns the postings of termText. Lucene's Terms declares
// no such member; Gocene's spi.Terms requires it, and the faithful body is
// iterator() followed by TermsEnum.seekExact and TermsEnum.postings.
func (t *memoryFieldTerms) GetPostingsReader(termText string, flags int) (spi.PostingsEnum, error) {
	e, err := t.Iterator()
	if err != nil {
		return nil, err
	}
	found, err := e.SeekExact(spi.NewTerm(t.field, termText))
	if err != nil || !found {
		return nil, err
	}
	return e.Postings(flags)
}

// GetMin renders the inherited default `public BytesRef getMin()` of
// org.apache.lucene.index.Terms: `return iterator().next()`.
func (t *memoryFieldTerms) GetMin() (*spi.Term, error) {
	e, err := t.Iterator()
	if err != nil {
		return nil, err
	}
	return e.Next()
}

// GetMax renders the inherited default `public BytesRef getMax()` of
// org.apache.lucene.index.Terms. MemoryTermsEnum supports seek-by-ordinal, so
// the seek-by-ord branch is the one Java takes here; the binary-search
// fallback is unreachable for this Terms.
func (t *memoryFieldTerms) GetMax() (*spi.Term, error) {
	size := t.Size()
	if size == 0 {
		// empty: only possible from a FilteredTermsEnum...
		return nil, nil
	}
	e := newMemoryTermsEnum(t.mi, t.field, t.info)
	if err := e.SeekExactOrd(size - 1); err != nil {
		return nil, err
	}
	return e.Term(), nil
}

// Size renders `public long size()` (MemoryIndex.java:1778).
func (t *memoryFieldTerms) Size() int64 { return int64(t.info.terms.Size()) }

// GetSumTotalTermFreq renders `public long getSumTotalTermFreq()`
// (MemoryIndex.java:1783).
func (t *memoryFieldTerms) GetSumTotalTermFreq() (int64, error) { return t.info.sumTotalTermFreq, nil }

// GetSumDocFreq renders `public long getSumDocFreq()` (MemoryIndex.java:1788):
// each term has df=1.
func (t *memoryFieldTerms) GetSumDocFreq() (int64, error) { return int64(t.info.terms.Size()), nil }

// GetDocCount renders `public int getDocCount()` (MemoryIndex.java:1794).
func (t *memoryFieldTerms) GetDocCount() (int, error) {
	if t.Size() > 0 {
		return 1, nil
	}
	return 0, nil
}

// HasFreqs renders `public boolean hasFreqs()` (MemoryIndex.java:1799).
func (t *memoryFieldTerms) HasFreqs() bool { return true }

// HasOffsets renders `public boolean hasOffsets()` (MemoryIndex.java:1804).
func (t *memoryFieldTerms) HasOffsets() bool { return t.mi.storeOffsets }

// HasPositions renders `public boolean hasPositions()` (MemoryIndex.java:1809).
func (t *memoryFieldTerms) HasPositions() bool { return true }

// HasPayloads renders `public boolean hasPayloads()` (MemoryIndex.java:1814).
func (t *memoryFieldTerms) HasPayloads() bool { return t.mi.storePayloads }

// memoryTermsEnum renders the private inner class
// MemoryIndexReader.MemoryTermsEnum (MemoryIndex.java:1828), which extends
// BaseTermsEnum.
type memoryTermsEnum struct {
	spi.TermsEnumBase
	mi    *MemoryIndex
	field string
	info  *info
	br    *util.BytesRef
	// current is the reusable spi.Term view over br. Gocene's TermsEnum
	// traffics in *spi.Term where Java's traffics in BytesRef; the single
	// instance preserves Java's reuse semantics.
	current  *spi.Term
	termUpto int
}

// newMemoryTermsEnum renders `public MemoryTermsEnum(Info info)`
// (MemoryIndex.java:1833).
func newMemoryTermsEnum(mi *MemoryIndex, field string, inf *info) *memoryTermsEnum {
	br := util.NewBytesRefEmpty()
	e := &memoryTermsEnum{
		mi:       mi,
		field:    field,
		info:     inf,
		br:       br,
		current:  &spi.Term{Field: field, Bytes: br},
		termUpto: -1,
	}
	inf.sortTerms()
	return e
}

// binarySearch renders
// `private final int binarySearch(BytesRef b, BytesRef bytesRef, int low, int high, BytesRefHash hash, int[] ords)`
// (MemoryIndex.java:1838).
func (e *memoryTermsEnum) binarySearch(b, bytesRef *util.BytesRef, low, high int, hash *util.BytesRefHash, ords []int) int {
	mid := 0
	for low <= high {
		mid = int(uint(low+high) >> 1)
		hash.Get(ords[mid], bytesRef)
		cmp := bytesRef.BytesRefCompareTo(b)
		if cmp < 0 {
			low = mid + 1
		} else if cmp > 0 {
			high = mid - 1
		} else {
			return mid
		}
	}
	return -(low + 1)
}

// SeekExact renders `public boolean seekExact(BytesRef text)`
// (MemoryIndex.java:1858).
func (e *memoryTermsEnum) SeekExact(text *spi.Term) (bool, error) {
	e.termUpto = e.binarySearch(text.Bytes, e.br, 0, e.info.terms.Size()-1, e.info.terms, e.info.sortedTerms)
	return e.termUpto >= 0, nil
}

// SeekCeil renders `public SeekStatus seekCeil(BytesRef text)`
// (MemoryIndex.java:1864). Gocene's spi.TermsEnum returns the positioned term
// rather than Java's SeekStatus: FOUND and NOT_FOUND both yield the current
// term, END yields nil.
func (e *memoryTermsEnum) SeekCeil(text *spi.Term) (*spi.Term, error) {
	e.termUpto = e.binarySearch(text.Bytes, e.br, 0, e.info.terms.Size()-1, e.info.terms, e.info.sortedTerms)
	if e.termUpto < 0 { // not found; choose successor
		e.termUpto = -e.termUpto - 1
		if e.termUpto >= e.info.terms.Size() {
			return nil, nil // SeekStatus.END
		}
		e.info.terms.Get(e.info.sortedTerms[e.termUpto], e.br)
		return e.current, nil // SeekStatus.NOT_FOUND
	}
	e.info.terms.Get(e.info.sortedTerms[e.termUpto], e.br)
	return e.current, nil // SeekStatus.FOUND
}

// SeekExactOrd renders `public void seekExact(long ord)`
// (MemoryIndex.java:1880). Go has no overloading, hence the Ord suffix.
// Gocene's spi.TermsEnum does not declare this member; callers reach it
// through the optional interface `interface{ SeekExactOrd(int64) error }`
// used across the repository (index.FilterTermsEnum, search.FuzzyTermsEnum),
// whose error result renders the IOException that TermsEnum.seekExact(long)
// declares. This implementation never fails.
func (e *memoryTermsEnum) SeekExactOrd(ord int64) error {
	e.termUpto = int(ord)
	e.info.terms.Get(e.info.sortedTerms[e.termUpto], e.br)
	return nil
}

// Next renders `public BytesRef next()` (MemoryIndex.java:1887).
func (e *memoryTermsEnum) Next() (*spi.Term, error) {
	e.termUpto++
	if e.termUpto >= e.info.terms.Size() {
		return nil, nil
	}
	e.info.terms.Get(e.info.sortedTerms[e.termUpto], e.br)
	return e.current, nil
}

// Term renders `public BytesRef term()` (MemoryIndex.java:1898).
func (e *memoryTermsEnum) Term() *spi.Term { return e.current }

// Ord renders `public long ord()` (MemoryIndex.java:1903).
func (e *memoryTermsEnum) Ord() int64 { return int64(e.termUpto) }

// DocFreq renders `public int docFreq()` (MemoryIndex.java:1908).
func (e *memoryTermsEnum) DocFreq() (int, error) { return 1, nil }

// TotalTermFreq renders `public long totalTermFreq()` (MemoryIndex.java:1913).
func (e *memoryTermsEnum) TotalTermFreq() (int64, error) {
	return int64(e.info.sliceArray.freq[e.info.sortedTerms[e.termUpto]]), nil
}

// Postings renders `public PostingsEnum postings(PostingsEnum reuse, int flags)`
// (MemoryIndex.java:1918). Gocene's spi.TermsEnum carries no reuse parameter,
// so the body is Java's reuse == null branch.
func (e *memoryTermsEnum) Postings(flags int) (spi.PostingsEnum, error) {
	reuse := newMemoryPostingsEnum(e.mi)
	ord := e.info.sortedTerms[e.termUpto]
	return reuse.reset(e.info.sliceArray.start[ord], e.info.sliceArray.end[ord], e.info.sliceArray.freq[ord]), nil
}

// PostingsWithLiveDocs returns the postings of the current term restricted to
// liveDocs. Lucene's 10.5.0 TermsEnum declares no such member; Gocene's
// spi.TermsEnum requires it. A MemoryIndex holds one, never-deleted document,
// so the answer is the unrestricted postings.
func (e *memoryTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (spi.PostingsEnum, error) {
	return e.Postings(flags)
}

// Impacts renders `public ImpactsEnum impacts(int flags)` (MemoryIndex.java:1928).
func (e *memoryTermsEnum) Impacts(flags int) (spi.ImpactsEnum, error) {
	postings, err := e.Postings(flags)
	if err != nil {
		return nil, err
	}
	return index.NewSlowImpactsEnum(postings), nil
}

// SeekExactWithState renders
// `public void seekExact(BytesRef term, TermState state)` (MemoryIndex.java:1933).
// Go has no overloading, hence the WithState suffix; Gocene's spi.TermsEnum
// does not declare this member.
func (e *memoryTermsEnum) SeekExactWithState(term *spi.Term, state index.TermState) error {
	if state == nil {
		return fmt.Errorf("state must not be null")
	}
	ordState, ok := state.(*index.OrdTermState)
	if !ok {
		return fmt.Errorf("memory: expected an *index.OrdTermState, got %T", state)
	}
	return e.SeekExactOrd(ordState.Ord)
}

// TermState renders `public TermState termState()` (MemoryIndex.java:1939).
// Gocene's spi.TermsEnum does not declare this member.
func (e *memoryTermsEnum) TermState() (index.TermState, error) {
	ts := index.NewOrdTermState()
	ts.Ord = int64(e.termUpto)
	return ts, nil
}

// memoryPostingsEnum renders the private inner class
// MemoryIndexReader.MemoryPostingsEnum (MemoryIndex.java:1946), which extends
// PostingsEnum.
type memoryPostingsEnum struct {
	mi           *MemoryIndex
	sliceReader  *SliceReader
	posUpto      int // for assert
	hasNext      bool
	doc          int
	freq         int
	startOffset  int
	endOffset    int
	payloadIndex int
	// payloadBuilder is only non-nil when storePayloads. Java holds a
	// BytesRefBuilder; util.BytesRefArray.Get fills a util.BytesRef, so that
	// is the reusable holder here.
	payloadBuilder *util.BytesRef
}

// newMemoryPostingsEnum renders `public MemoryPostingsEnum()`
// (MemoryIndex.java:1958).
func newMemoryPostingsEnum(mi *MemoryIndex) *memoryPostingsEnum {
	e := &memoryPostingsEnum{mi: mi, sliceReader: NewSliceReader(mi.slicedIntBlockPool), doc: -1}
	if mi.storePayloads {
		e.payloadBuilder = util.NewBytesRefEmpty()
	}
	return e
}

// reset renders `public PostingsEnum reset(int start, int end, int freq)`
// (MemoryIndex.java:1963).
func (e *memoryPostingsEnum) reset(start, end, freq int) spi.PostingsEnum {
	e.sliceReader.Reset(start, end)
	e.posUpto = 0 // for assert
	e.hasNext = true
	e.doc = -1
	e.freq = freq
	return e
}

// DocID renders `public int docID()` (MemoryIndex.java:1973).
func (e *memoryPostingsEnum) DocID() int { return e.doc }

// NextDoc renders `public int nextDoc()` (MemoryIndex.java:1978).
func (e *memoryPostingsEnum) NextDoc() (int, error) {
	if e.hasNext {
		e.hasNext = false
		e.doc = 0
		return e.doc, nil
	}
	e.doc = spi.NO_MORE_DOCS
	return e.doc, nil
}

// Advance renders `public int advance(int target)` (MemoryIndex.java:1988).
func (e *memoryPostingsEnum) Advance(target int) (int, error) {
	return util.SlowAdvance(e, target)
}

// Freq renders `public int freq()` (MemoryIndex.java:1993).
func (e *memoryPostingsEnum) Freq() (int, error) { return e.freq, nil }

// NextPosition renders `public int nextPosition()` (MemoryIndex.java:1998).
func (e *memoryPostingsEnum) NextPosition() (int, error) {
	e.posUpto++
	pos := int(e.sliceReader.ReadInt())
	if e.mi.storeOffsets {
		e.startOffset = int(e.sliceReader.ReadInt())
		e.endOffset = int(e.sliceReader.ReadInt())
	}
	if e.mi.storePayloads {
		e.payloadIndex = int(e.sliceReader.ReadInt())
	}
	return pos, nil
}

// StartOffset renders `public int startOffset()` (MemoryIndex.java:2015).
func (e *memoryPostingsEnum) StartOffset() (int, error) { return e.startOffset, nil }

// EndOffset renders `public int endOffset()` (MemoryIndex.java:2020).
func (e *memoryPostingsEnum) EndOffset() (int, error) { return e.endOffset, nil }

// GetPayload renders `public BytesRef getPayload()` (MemoryIndex.java:2025).
func (e *memoryPostingsEnum) GetPayload() ([]byte, error) {
	if e.payloadBuilder == nil || e.payloadIndex == -1 {
		return nil, nil
	}
	e.mi.payloadsBytesRefs.Get(e.payloadIndex, e.payloadBuilder)
	return e.payloadBuilder.ValidBytes(), nil
}

// Cost renders `public long cost()` (MemoryIndex.java:2033).
func (e *memoryPostingsEnum) Cost() int64 { return 1 }

func (e *memoryPostingsEnum) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(e, upTo, bitSet, offset)
}

func (e *memoryPostingsEnum) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(e)
}

// memoryIndexPointValues renders the private inner class
// MemoryIndexReader.MemoryIndexPointValues (MemoryIndex.java:2038), which
// extends PointValues.
type memoryIndexPointValues struct {
	*spi.BasePointValues
	info *info
}

// newMemoryIndexPointValues renders `MemoryIndexPointValues(Info info)`
// (MemoryIndex.java:2042).
func newMemoryIndexPointValues(inf *info) (spi.PointValues, error) {
	if inf == nil {
		return nil, fmt.Errorf("info must not be null")
	}
	if inf.pointValues == nil {
		return nil, fmt.Errorf("Field does not have points")
	}
	pv := &memoryIndexPointValues{info: inf}
	pv.BasePointValues = spi.NewBasePointValues(pv)
	return pv, nil
}

// memoryIndexPointTree renders the anonymous PointTree of
// `public PointTree getPointTree()` (MemoryIndex.java:2048).
type memoryIndexPointTree struct {
	info *info
}

func (t *memoryIndexPointTree) Clone() spi.PointTree { return t }

func (t *memoryIndexPointTree) MoveToChild() (bool, error) { return false, nil }

func (t *memoryIndexPointTree) MoveToSibling() (bool, error) { return false, nil }

func (t *memoryIndexPointTree) MoveToParent() (bool, error) { return false, nil }

func (t *memoryIndexPointTree) GetMinPackedValue() []byte { return t.info.minPackedValue }

func (t *memoryIndexPointTree) GetMaxPackedValue() []byte { return t.info.maxPackedValue }

func (t *memoryIndexPointTree) Size() int64 { return int64(t.info.pointValuesCount) }

func (t *memoryIndexPointTree) VisitDocIDs(visitor spi.IntersectVisitor) error {
	visitor.Grow(t.info.pointValuesCount)
	for i := 0; i < t.info.pointValuesCount; i++ {
		if err := visitor.Visit(0); err != nil {
			return err
		}
	}
	return nil
}

func (t *memoryIndexPointTree) VisitDocValues(visitor spi.IntersectVisitor) error {
	values := t.info.pointValues

	visitor.Grow(t.info.pointValuesCount)
	for i := 0; i < t.info.pointValuesCount; i++ {
		if err := visitor.VisitByPackedValue(0, values[i].Bytes); err != nil {
			return err
		}
	}
	return nil
}

// GetPointTree renders `public PointTree getPointTree()` (MemoryIndex.java:2048).
func (pv *memoryIndexPointValues) GetPointTree() (spi.PointTree, error) {
	return &memoryIndexPointTree{info: pv.info}, nil
}

// GetMinPackedValue renders `public byte[] getMinPackedValue()`
// (MemoryIndex.java:2106).
func (pv *memoryIndexPointValues) GetMinPackedValue() ([]byte, error) {
	return pv.info.minPackedValue, nil
}

// GetMaxPackedValue renders `public byte[] getMaxPackedValue()`
// (MemoryIndex.java:2111).
func (pv *memoryIndexPointValues) GetMaxPackedValue() ([]byte, error) {
	return pv.info.maxPackedValue, nil
}

// GetNumDimensions renders `public int getNumDimensions()`
// (MemoryIndex.java:2116).
func (pv *memoryIndexPointValues) GetNumDimensions() (int, error) {
	return pv.info.fieldInfo.PointDimensionCount(), nil
}

// GetNumIndexDimensions renders `public int getNumIndexDimensions()`
// (MemoryIndex.java:2121).
func (pv *memoryIndexPointValues) GetNumIndexDimensions() (int, error) {
	return pv.info.fieldInfo.PointDimensionCount(), nil
}

// GetBytesPerDimension renders `public int getBytesPerDimension()`
// (MemoryIndex.java:2126).
func (pv *memoryIndexPointValues) GetBytesPerDimension() (int, error) {
	return pv.info.fieldInfo.PointNumBytes(), nil
}

// Size renders `public long size()` (MemoryIndex.java:2131).
func (pv *memoryIndexPointValues) Size() int64 { return int64(pv.info.pointValuesCount) }

// GetDocCount renders `public int getDocCount()` (MemoryIndex.java:2136).
func (pv *memoryIndexPointValues) GetDocCount() int { return 1 }

// memoryIndexTermVectors renders the anonymous TermVectors of
// `public TermVectors termVectors()` (MemoryIndex.java:2142).
type memoryIndexTermVectors struct {
	memoryFields *memoryFields
}

// Get renders `public Fields get(int docID)` (MemoryIndex.java:2144).
func (tv *memoryIndexTermVectors) Get(docID int) (spi.Fields, error) {
	if docID == 0 {
		return tv.memoryFields, nil
	}
	return nil, nil
}

// GetField returns the term vector of one field. Lucene's TermVectors declares
// `Terms get(int docID, String field)` with a default body that calls
// get(docID) and then Fields.terms(field); Gocene names it GetField.
func (tv *memoryIndexTermVectors) GetField(docID int, field string) (spi.Terms, error) {
	fields, err := tv.Get(docID)
	if err != nil || fields == nil {
		return nil, err
	}
	return fields.Terms(field)
}

// Prefetch renders the default body of TermVectors.prefetch(int), which is a
// no-op for an in-memory reader.
func (tv *memoryIndexTermVectors) Prefetch(docIDs []int) error { return nil }

// TermVectors renders `public TermVectors termVectors()` (MemoryIndex.java:2142).
func (r *memoryIndexReader) TermVectors() (spi.TermVectors, error) {
	return &memoryIndexTermVectors{memoryFields: r.memoryFields}, nil
}

// NumDocs renders `public int numDocs()` (MemoryIndex.java:2156).
func (r *memoryIndexReader) NumDocs() int {
	if debug {
		fmt.Println("MemoryIndexReader.numDocs")
	}
	return 1
}

// MaxDoc renders `public int maxDoc()` (MemoryIndex.java:2162).
func (r *memoryIndexReader) MaxDoc() int {
	if debug {
		fmt.Println("MemoryIndexReader.maxDoc")
	}
	return 1
}

// memoryIndexStoredFields renders the anonymous StoredFields of
// `public StoredFields storedFields()` (MemoryIndex.java:2168).
type memoryIndexStoredFields struct {
	mi *MemoryIndex
}

// Document renders `public void document(int docID, StoredFieldVisitor visitor)`
// (MemoryIndex.java:2170).
func (sf *memoryIndexStoredFields) Document(docID int, visitor spi.StoredFieldVisitor) error {
	if debug {
		fmt.Println("MemoryIndexReader.document")
	}
	for _, name := range sf.mi.sortedFieldNames() {
		inf := sf.mi.fields[name]
		status, err := visitor.NeedsField(inf.fieldInfo)
		if err != nil {
			return err
		}
		if status == spi.StoredFieldVisitorStatusStop {
			return nil
		}
		if status == spi.StoredFieldVisitorStatusNo {
			continue
		}
		if inf.storedValues != nil {
			for _, value := range inf.storedValues {
				var err error
				switch v := value.(type) {
				case *util.BytesRef:
					err = visitor.BinaryField(inf.fieldInfo, util.BytesRefDeepCopyOf(v).Bytes)
				case float64:
					err = visitor.DoubleField(inf.fieldInfo, v)
				case float32:
					err = visitor.FloatField(inf.fieldInfo, v)
				case int64:
					err = visitor.LongField(inf.fieldInfo, v)
				case int:
					err = visitor.IntField(inf.fieldInfo, v)
				case int32:
					err = visitor.IntField(inf.fieldInfo, int(v))
				case string:
					err = visitor.StringField(inf.fieldInfo, v)
				}
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Prefetch renders the default body of StoredFields.prefetch(int), which is a
// no-op for an in-memory reader.
func (sf *memoryIndexStoredFields) Prefetch(docIDs []int) error { return nil }

// StoredFields renders `public StoredFields storedFields()`
// (MemoryIndex.java:2168).
func (r *memoryIndexReader) StoredFields() (spi.StoredFields, error) {
	return &memoryIndexStoredFields{mi: r.mi}, nil
}

// doClose renders `protected void doClose()` (MemoryIndex.java:2204).
func (r *memoryIndexReader) doClose() {
	if debug {
		fmt.Println("MemoryIndexReader.doClose")
	}
}

// GetNormValues renders `public NumericDocValues getNormValues(String field)`
// (MemoryIndex.java:2209).
func (r *memoryIndexReader) GetNormValues(field string) (spi.NumericDocValues, error) {
	inf := r.mi.fields[field]
	if inf == nil || inf.fieldInfo.OmitNorms() {
		return nil, nil
	}
	return inf.getNormDocValues(r.mi), nil
}

// GetMetaData renders `public LeafMetaData getMetaData()`
// (MemoryIndex.java:2218), which Java answers with
// `new LeafMetaData(Version.LATEST.major, Version.LATEST, null, false)`.
// Gocene's spi.IndexReaderMetaData carries the deletion and document counts
// instead of the created-version, the minimum version and the index sort, so
// those are the members reported here.
func (r *memoryIndexReader) GetMetaData() *spi.IndexReaderMetaData {
	return &spi.IndexReaderMetaData{HasDeletions: false, NumDocs: 1, MaxDoc: 1}
}

// GetCoreCacheHelper renders `public CacheHelper getCoreCacheHelper()`
// (MemoryIndex.java:2223).
func (r *memoryIndexReader) GetCoreCacheHelper() spi.CacheHelper { return nil }

// GetReaderCacheHelper renders `public CacheHelper getReaderCacheHelper()`
// (MemoryIndex.java:2228).
func (r *memoryIndexReader) GetReaderCacheHelper() spi.CacheHelper { return nil }

// ---------------------------------------------------------------------------
// Members MemoryIndexReader inherits from org.apache.lucene.index.LeafReader
// and org.apache.lucene.index.IndexReader. Go has no class inheritance, so the
// concrete bodies of the two Java base classes are carried here.
// ---------------------------------------------------------------------------

// DocFreq renders the concrete `public final int docFreq(Term term)` of
// org.apache.lucene.index.LeafReader.
func (r *memoryIndexReader) DocFreq(term spi.Term) (int, error) {
	terms, err := r.Terms(term.Field)
	if err != nil || terms == nil {
		return 0, err
	}
	termsEnum, err := terms.Iterator()
	if err != nil {
		return 0, err
	}
	found, err := termsEnum.SeekExact(&term)
	if err != nil || !found {
		return 0, err
	}
	return termsEnum.DocFreq()
}

// TotalTermFreq renders the concrete `public final long totalTermFreq(Term term)`
// of org.apache.lucene.index.LeafReader.
func (r *memoryIndexReader) TotalTermFreq(term spi.Term) (int64, error) {
	terms, err := r.Terms(term.Field)
	if err != nil || terms == nil {
		return 0, err
	}
	termsEnum, err := terms.Iterator()
	if err != nil {
		return 0, err
	}
	found, err := termsEnum.SeekExact(&term)
	if err != nil || !found {
		return 0, err
	}
	return termsEnum.TotalTermFreq()
}

// Postings renders the concrete `public final PostingsEnum postings(Term term, int flags)`
// of org.apache.lucene.index.LeafReader.
func (r *memoryIndexReader) Postings(term spi.Term, flags int) (spi.PostingsEnum, error) {
	terms, err := r.Terms(term.Field)
	if err != nil || terms == nil {
		return nil, err
	}
	termsEnum, err := terms.Iterator()
	if err != nil {
		return nil, err
	}
	found, err := termsEnum.SeekExact(&term)
	if err != nil || !found {
		return nil, err
	}
	return termsEnum.Postings(flags)
}

// DocID answers the member Gocene's spi.LeafReader requires and
// org.apache.lucene.index.LeafReader does not declare. A MemoryIndex holds
// exactly one document, whose identifier is 0.
func (r *memoryIndexReader) DocID() int { return 0 }

// DocCount renders the document count of this leaf; a MemoryIndex holds one
// document.
func (r *memoryIndexReader) DocCount() int { return 1 }

// HasDeletions renders `public boolean hasDeletions()` of
// org.apache.lucene.index.IndexReader, whose body asks each leaf whether it
// carries live docs. This leaf never does.
func (r *memoryIndexReader) HasDeletions() bool { return false }

// NumDeletedDocs renders `public final int numDeletedDocs()` of
// org.apache.lucene.index.IndexReader: maxDoc() - numDocs().
func (r *memoryIndexReader) NumDeletedDocs() int { return r.MaxDoc() - r.NumDocs() }

// EnsureOpen renders `protected final void ensureOpen()` of
// org.apache.lucene.index.IndexReader.
func (r *memoryIndexReader) EnsureOpen() error {
	if r.refs.Load() <= 0 {
		return fmt.Errorf("this IndexReader is closed")
	}
	return nil
}

// Close renders `public final synchronized void close()` of
// org.apache.lucene.index.IndexReader.
func (r *memoryIndexReader) Close() error {
	if r.closed.CompareAndSwap(false, true) {
		return r.DecRef()
	}
	return nil
}

// IncRef renders `public final void incRef()` of
// org.apache.lucene.index.IndexReader.
func (r *memoryIndexReader) IncRef() error {
	if !r.TryIncRef() {
		return fmt.Errorf("this IndexReader is closed")
	}
	return nil
}

// DecRef renders `public final void decRef()` of
// org.apache.lucene.index.IndexReader.
func (r *memoryIndexReader) DecRef() error {
	rc := r.refs.Add(-1)
	if rc == 0 {
		r.doClose()
	} else if rc < 0 {
		return fmt.Errorf("too many decRef calls: refCount is %d after decrement", rc)
	}
	return nil
}

// TryIncRef renders `public final boolean tryIncRef()` of
// org.apache.lucene.index.IndexReader.
func (r *memoryIndexReader) TryIncRef() bool {
	for {
		count := r.refs.Load()
		if count <= 0 {
			return false
		}
		if r.refs.CompareAndSwap(count, count+1) {
			return true
		}
	}
}

// GetRefCount renders `public int getRefCount()` of
// org.apache.lucene.index.IndexReader.
func (r *memoryIndexReader) GetRefCount() int32 { return r.refs.Load() }

// GetContext renders `public final LeafReaderContext getContext()` of
// org.apache.lucene.index.LeafReader.
func (r *memoryIndexReader) GetContext() (spi.IndexReaderContext, error) {
	if err := r.EnsureOpen(); err != nil {
		return nil, err
	}
	return r.readerContext, nil
}

// Leaves renders `public final List<LeafReaderContext> leaves()` of
// org.apache.lucene.index.IndexReader, which delegates to the top-level
// context. A leaf reader's own context is top level here.
func (r *memoryIndexReader) Leaves() ([]*spi.LeafReaderContext, error) {
	if err := r.EnsureOpen(); err != nil {
		return nil, err
	}
	return []*spi.LeafReaderContext{r.readerContext}, nil
}

// memoryFloatVectorValues renders the private static final class
// MemoryIndex.MemoryFloatVectorValues (MemoryIndex.java:2287), which extends
// FloatVectorValues.
type memoryFloatVectorValues struct {
	info *info
}

// Dimension renders `public int dimension()` (MemoryIndex.java:2294).
func (v *memoryFloatVectorValues) Dimension() int { return v.info.fieldInfo.VectorDimension() }

// Size renders `public int size()` (MemoryIndex.java:2299).
func (v *memoryFloatVectorValues) Size() int { return v.info.floatVectorCount }

// VectorValue renders `public float[] vectorValue(int ord)`
// (MemoryIndex.java:2304).
func (v *memoryFloatVectorValues) VectorValue(ord int) ([]float32, error) {
	if ord == 0 {
		return v.info.floatVectorValues[0], nil
	}
	return nil, nil
}

// Iterator renders `public DocIndexIterator iterator()` (MemoryIndex.java:2313).
func (v *memoryFloatVectorValues) Iterator() spi.DocIndexIterator {
	return spi.CreateDenseIterator(v)
}

// memoryFloatVectorScorer renders the anonymous VectorScorer of
// `public VectorScorer scorer(float[] query)` (MemoryIndex.java:2319).
type memoryFloatVectorScorer struct {
	info         *info
	vectorValues *memoryFloatVectorValues
	iterator     spi.DocIndexIterator
	query        []float32
}

func (s *memoryFloatVectorScorer) Score() (float32, error) {
	value, err := s.vectorValues.VectorValue(0)
	if err != nil {
		return 0, err
	}
	return s.info.fieldInfo.VectorSimilarityFunction().CompareFloat(value, s.query), nil
}

func (s *memoryFloatVectorScorer) Iterator() util.DocIdSetIterator { return s.iterator }

// Scorer renders `public VectorScorer scorer(float[] query)`
// (MemoryIndex.java:2318).
func (v *memoryFloatVectorValues) Scorer(query []float32) (util.VectorScorer, error) {
	if len(query) != v.info.fieldInfo.VectorDimension() {
		return nil, fmt.Errorf("query vector dimension %d does not match field dimension %d",
			len(query), v.info.fieldInfo.VectorDimension())
	}
	vectorValues := &memoryFloatVectorValues{info: v.info}
	iterator := vectorValues.Iterator()
	return &memoryFloatVectorScorer{info: v.info, vectorValues: vectorValues, iterator: iterator, query: query}, nil
}

// Rescorer renders the inherited default `public VectorScorer rescorer(float[] target)`
// of org.apache.lucene.index.FloatVectorValues: `return scorer(target)`.
func (v *memoryFloatVectorValues) Rescorer(target []float32) (util.VectorScorer, error) {
	return v.Scorer(target)
}

// CopyFloatVectorValues renders `public MemoryFloatVectorValues copy()`
// (MemoryIndex.java:2344).
func (v *memoryFloatVectorValues) CopyFloatVectorValues() (spi.FloatVectorValues, error) {
	return v, nil
}

// Copy renders `public MemoryFloatVectorValues copy()` (MemoryIndex.java:2344)
// through the KnnVectorValues contract.
func (v *memoryFloatVectorValues) Copy() (spi.KnnVectorValues, error) { return v, nil }

// OrdToDoc renders the inherited default `public int ordToDoc(int ord)` of
// org.apache.lucene.index.KnnVectorValues.
func (v *memoryFloatVectorValues) OrdToDoc(ord int) int { return ord }

// Prefetch renders the inherited default `public void prefetch(int[] ordsToPrefetch, int numOrds)`,
// whose body is empty.
func (v *memoryFloatVectorValues) Prefetch(ordsToPrefetch []int, numOrds int) error { return nil }

// GetVectorByteLength renders the inherited default
// `public int getVectorByteLength()`: dimension() * encoding byte size.
func (v *memoryFloatVectorValues) GetVectorByteLength() int {
	return v.Dimension() * index.VectorEncodingByteSize(v.GetEncoding())
}

// GetEncoding renders the inherited `public VectorEncoding getEncoding()` of
// org.apache.lucene.index.FloatVectorValues.
func (v *memoryFloatVectorValues) GetEncoding() spi.VectorEncoding {
	return util.VectorEncodingFloat32
}

// GetAcceptOrds renders the inherited default
// `public Bits getAcceptOrds(Bits acceptDocs)` of KnnVectorValues.
func (v *memoryFloatVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return spi.DefaultGetAcceptOrds(v, acceptDocs)
}

// memoryByteVectorValues renders the private static final class
// MemoryIndex.MemoryByteVectorValues (MemoryIndex.java:2351), which extends
// ByteVectorValues.
type memoryByteVectorValues struct {
	info *info
}

// Dimension renders `public int dimension()` (MemoryIndex.java:2358).
func (v *memoryByteVectorValues) Dimension() int { return v.info.fieldInfo.VectorDimension() }

// Size renders `public int size()` (MemoryIndex.java:2363).
func (v *memoryByteVectorValues) Size() int { return v.info.byteVectorCount }

// VectorValue renders `public byte[] vectorValue(int ord)`
// (MemoryIndex.java:2368).
func (v *memoryByteVectorValues) VectorValue(ord int) ([]byte, error) {
	if ord == 0 {
		return v.info.byteVectorValues[0], nil
	}
	return nil, nil
}

// Iterator renders `public DocIndexIterator iterator()` (MemoryIndex.java:2377).
func (v *memoryByteVectorValues) Iterator() spi.DocIndexIterator {
	return spi.CreateDenseIterator(v)
}

// memoryByteVectorScorer renders the anonymous VectorScorer of
// `public VectorScorer scorer(byte[] query)` (MemoryIndex.java:2383).
type memoryByteVectorScorer struct {
	info         *info
	vectorValues *memoryByteVectorValues
	iterator     spi.DocIndexIterator
	query        []byte
}

func (s *memoryByteVectorScorer) Score() (float32, error) {
	value, err := s.vectorValues.VectorValue(0)
	if err != nil {
		return 0, err
	}
	return s.info.fieldInfo.VectorSimilarityFunction().CompareBytes(value, s.query), nil
}

func (s *memoryByteVectorScorer) Iterator() util.DocIdSetIterator { return s.iterator }

// Scorer renders `public VectorScorer scorer(byte[] query)`
// (MemoryIndex.java:2382).
func (v *memoryByteVectorValues) Scorer(query []byte) (util.VectorScorer, error) {
	if len(query) != v.info.fieldInfo.VectorDimension() {
		return nil, fmt.Errorf("query vector dimension %d does not match field dimension %d",
			len(query), v.info.fieldInfo.VectorDimension())
	}
	vectorValues := &memoryByteVectorValues{info: v.info}
	iterator := vectorValues.Iterator()
	return &memoryByteVectorScorer{info: v.info, vectorValues: vectorValues, iterator: iterator, query: query}, nil
}

// Rescorer renders the inherited default `public VectorScorer rescorer(byte[] target)`
// of org.apache.lucene.index.ByteVectorValues: `return scorer(target)`.
func (v *memoryByteVectorValues) Rescorer(target []byte) (util.VectorScorer, error) {
	return v.Scorer(target)
}

// CopyByteVectorValues renders `public MemoryByteVectorValues copy()`
// (MemoryIndex.java:2408).
func (v *memoryByteVectorValues) CopyByteVectorValues() (spi.ByteVectorValues, error) {
	return v, nil
}

// Copy renders `public MemoryByteVectorValues copy()` (MemoryIndex.java:2408)
// through the KnnVectorValues contract.
func (v *memoryByteVectorValues) Copy() (spi.KnnVectorValues, error) { return v, nil }

// OrdToDoc renders the inherited default `public int ordToDoc(int ord)` of
// org.apache.lucene.index.KnnVectorValues.
func (v *memoryByteVectorValues) OrdToDoc(ord int) int { return ord }

// Prefetch renders the inherited default `public void prefetch(int[] ordsToPrefetch, int numOrds)`,
// whose body is empty.
func (v *memoryByteVectorValues) Prefetch(ordsToPrefetch []int, numOrds int) error { return nil }

// GetVectorByteLength renders the inherited default
// `public int getVectorByteLength()`: dimension() * encoding byte size.
func (v *memoryByteVectorValues) GetVectorByteLength() int {
	return v.Dimension() * index.VectorEncodingByteSize(v.GetEncoding())
}

// GetEncoding renders the inherited `public VectorEncoding getEncoding()` of
// org.apache.lucene.index.ByteVectorValues.
func (v *memoryByteVectorValues) GetEncoding() spi.VectorEncoding {
	return util.VectorEncodingByte
}

// GetAcceptOrds renders the inherited default
// `public Bits getAcceptOrds(Bits acceptDocs)` of KnnVectorValues.
func (v *memoryByteVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return spi.DefaultGetAcceptOrds(v, acceptDocs)
}
