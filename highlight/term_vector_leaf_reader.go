// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

import (
	"fmt"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// termVectorFields renders the anonymous Fields subclass built by the
// TermVectorLeafReader constructor (TermVectorLeafReader.java:62).
type termVectorFields struct {
	spi.FieldsBase
	field string
	terms spi.Terms
}

// termVectorFieldIterator renders the java.util.Iterator<String> returned by
// `Collections.singletonList(field).iterator()`.
type termVectorFieldIterator struct {
	field string
	done  bool
}

func (it *termVectorFieldIterator) HasNext() bool { return !it.done }

func (it *termVectorFieldIterator) Next() (string, error) {
	if it.done {
		return "", nil
	}
	it.done = true
	return it.field, nil
}

// Iterator renders `public Iterator<String> iterator()`
// (TermVectorLeafReader.java:65).
func (f *termVectorFields) Iterator() (spi.FieldIterator, error) {
	return &termVectorFieldIterator{field: f.field}, nil
}

// Terms renders `public Terms terms(String fld)` (TermVectorLeafReader.java:70).
func (f *termVectorFields) Terms(fld string) (spi.Terms, error) {
	if f.field != fld {
		return nil, nil
	}
	return f.terms, nil
}

// Size renders `public int size()` (TermVectorLeafReader.java:78).
func (f *termVectorFields) Size() int { return 1 }

// TermVectorLeafReader wraps a Terms with a LeafReader, typically from term
// vectors.
//
// This is the Go port of org.apache.lucene.search.highlight.TermVectorLeafReader
// (Apache Lucene 10.5.0,
// lucene/highlighter/src/java/org/apache/lucene/search/highlight/TermVectorLeafReader.java).
type TermVectorLeafReader struct {
	fields     *termVectorFields
	fieldInfos *spi.FieldInfos

	// readerContext, refs and closed carry the state that Java inherits from
	// IndexReader; Go has no class inheritance, so the concrete reader holds
	// it. refs starts at 1, as IndexReader's refCount does.
	readerContext *spi.LeafReaderContext
	refs          atomic.Int32
	closed        atomic.Bool
}

var _ index.LeafReader = (*TermVectorLeafReader)(nil)

// NewTermVectorLeafReader renders
// `public TermVectorLeafReader(String field, Terms terms)`
// (TermVectorLeafReader.java:60).
func NewTermVectorLeafReader(field string, terms spi.Terms) *TermVectorLeafReader {
	r := &TermVectorLeafReader{}
	r.fields = &termVectorFields{field: field, terms: terms}

	var indexOptions spi.IndexOptions
	if !terms.HasFreqs() {
		indexOptions = spi.IndexOptionsDocs
	} else if !terms.HasPositions() {
		indexOptions = spi.IndexOptionsDocsAndFreqs
	} else if !terms.HasOffsets() {
		indexOptions = spi.IndexOptionsDocsAndFreqsAndPositions
	} else {
		indexOptions = spi.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	}
	fieldInfo := spi.NewFieldInfo(field, 0, spi.FieldInfoOptions{
		StoreTermVectors:         true,
		OmitNorms:                true,
		IndexOptions:             indexOptions,
		DocValuesType:            spi.DocValuesTypeNone,
		DocValuesSkipIndexType:   spi.DocValuesSkipIndexTypeNone,
		DocValuesGen:             -1,
		PointDimensionCount:      0,
		PointIndexDimensionCount: 0,
		PointNumBytes:            0,
		VectorDimension:          0,
		VectorEncoding:           util.VectorEncodingFloat32,
		VectorSimilarityFunction: util.GetSimilarityFunction(util.VectorSimilarityIDEuclidean),
		IsSoftDeletesField:       false,
		IsParentField:            false,
	})
	// Java's fifth constructor argument is storePayloads, fed from
	// terms.hasPayloads(). spi.FieldInfoOptions carries no such member, so the
	// flag is set through the dedicated mutator, which only ever sets it to
	// true — false is the value NewFieldInfo already leaves behind.
	if terms.HasPayloads() {
		fieldInfo.SetStorePayloads()
	}
	r.fieldInfos = spi.NewFieldInfos(fieldInfo)
	r.refs.Store(1)
	r.readerContext = spi.NewLeafReaderContextForReader(r)
	return r
}

// doClose renders `protected void doClose()` (TermVectorLeafReader.java:112).
func (r *TermVectorLeafReader) doClose() {}

// Terms renders `public Terms terms(String field)` (TermVectorLeafReader.java:115).
func (r *TermVectorLeafReader) Terms(field string) (spi.Terms, error) {
	return r.fields.Terms(field)
}

// GetNumericDocValues renders `public NumericDocValues getNumericDocValues(String field)`
// (TermVectorLeafReader.java:120).
func (r *TermVectorLeafReader) GetNumericDocValues(field string) (spi.NumericDocValues, error) {
	return nil, nil
}

// GetBinaryDocValues renders `public BinaryDocValues getBinaryDocValues(String field)`
// (TermVectorLeafReader.java:125).
func (r *TermVectorLeafReader) GetBinaryDocValues(field string) (spi.BinaryDocValues, error) {
	return nil, nil
}

// GetSortedDocValues renders `public SortedDocValues getSortedDocValues(String field)`
// (TermVectorLeafReader.java:130).
func (r *TermVectorLeafReader) GetSortedDocValues(field string) (spi.SortedDocValues, error) {
	return nil, nil
}

// GetSortedNumericDocValues renders
// `public SortedNumericDocValues getSortedNumericDocValues(String field)`
// (TermVectorLeafReader.java:135).
func (r *TermVectorLeafReader) GetSortedNumericDocValues(field string) (spi.SortedNumericDocValues, error) {
	return nil, nil
}

// GetSortedSetDocValues renders `public SortedSetDocValues getSortedSetDocValues(String field)`
// (TermVectorLeafReader.java:140).
func (r *TermVectorLeafReader) GetSortedSetDocValues(field string) (spi.SortedSetDocValues, error) {
	return nil, nil
}

// GetDocValuesSkipper renders `public DocValuesSkipper getDocValuesSkipper(String field)`
// (TermVectorLeafReader.java:145).
func (r *TermVectorLeafReader) GetDocValuesSkipper(field string) (spi.DocValuesSkipper, error) {
	return nil, nil
}

// GetNormValues renders `public NumericDocValues getNormValues(String field)`
// (TermVectorLeafReader.java:150).
func (r *TermVectorLeafReader) GetNormValues(field string) (spi.NumericDocValues, error) {
	return nil, nil // Is this needed?  See MemoryIndex for a way to do it.
}

// GetFieldInfos renders `public FieldInfos getFieldInfos()`
// (TermVectorLeafReader.java:155).
func (r *TermVectorLeafReader) GetFieldInfos() *spi.FieldInfos { return r.fieldInfos }

// GetLiveDocs renders `public Bits getLiveDocs()` (TermVectorLeafReader.java:160).
func (r *TermVectorLeafReader) GetLiveDocs() util.Bits { return nil }

// GetPointValues renders `public PointValues getPointValues(String fieldName)`
// (TermVectorLeafReader.java:165).
func (r *TermVectorLeafReader) GetPointValues(fieldName string) (spi.PointValues, error) {
	return nil, nil
}

// GetFloatVectorValues renders `public FloatVectorValues getFloatVectorValues(String fieldName)`
// (TermVectorLeafReader.java:170).
func (r *TermVectorLeafReader) GetFloatVectorValues(fieldName string) (spi.FloatVectorValues, error) {
	return nil, nil
}

// GetByteVectorValues renders `public ByteVectorValues getByteVectorValues(String fieldName)`
// (TermVectorLeafReader.java:175).
func (r *TermVectorLeafReader) GetByteVectorValues(fieldName string) (spi.ByteVectorValues, error) {
	return nil, nil
}

// SearchNearestVectorsCollector renders
// `public void searchNearestVectors(String field, float[] target, KnnCollector knnCollector, AcceptDocs acceptDocs)`
// (TermVectorLeafReader.java:180), whose body is empty.
func (r *TermVectorLeafReader) SearchNearestVectorsCollector(field string, target []float32, knnCollector spi.KnnCollector, acceptDocs util.Bits) error {
	return nil
}

// SearchNearestVectorsByteCollector renders
// `public void searchNearestVectors(String field, byte[] target, KnnCollector knnCollector, AcceptDocs acceptDocs)`
// (TermVectorLeafReader.java:184), whose body is empty.
func (r *TermVectorLeafReader) SearchNearestVectorsByteCollector(field string, target []byte, knnCollector spi.KnnCollector, acceptDocs util.Bits) error {
	return nil
}

// SearchNearestVectors renders the final convenience overload
// LeafReader.searchNearestVectors(String, float[], int, AcceptDocs, int),
// which Java inherits. Since the abstract overload above collects nothing,
// the convenience overload returns no hits.
func (r *TermVectorLeafReader) SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits, visitedLimit int) (spi.TopDocs, error) {
	return spi.TopDocs{}, nil
}

// CheckIntegrity renders `public void checkIntegrity()`
// (TermVectorLeafReader.java:187).
func (r *TermVectorLeafReader) CheckIntegrity() error { return nil }

// termVectorTermVectors renders the anonymous TermVectors of
// `public TermVectors termVectors()` (TermVectorLeafReader.java:191).
type termVectorTermVectors struct {
	fields *termVectorFields
}

// Get renders `public Fields get(int docID)` (TermVectorLeafReader.java:193).
func (tv *termVectorTermVectors) Get(docID int) (spi.Fields, error) {
	if docID != 0 {
		return nil, nil
	}
	return tv.fields, nil
}

// GetField returns the term vector of one field. Lucene's TermVectors declares
// `Terms get(int docID, String field)` with a default body that calls
// get(docID) and then Fields.terms(field); Gocene names it GetField.
func (tv *termVectorTermVectors) GetField(docID int, field string) (spi.Terms, error) {
	fields, err := tv.Get(docID)
	if err != nil || fields == nil {
		return nil, err
	}
	return fields.Terms(field)
}

// Prefetch renders the default body of TermVectors.prefetch(int), which is a
// no-op for an in-memory reader.
func (tv *termVectorTermVectors) Prefetch(docIDs []int) error { return nil }

// TermVectors renders `public TermVectors termVectors()`
// (TermVectorLeafReader.java:190).
func (r *TermVectorLeafReader) TermVectors() (spi.TermVectors, error) {
	return &termVectorTermVectors{fields: r.fields}, nil
}

// NumDocs renders `public int numDocs()` (TermVectorLeafReader.java:209).
func (r *TermVectorLeafReader) NumDocs() int { return 1 }

// MaxDoc renders `public int maxDoc()` (TermVectorLeafReader.java:214).
func (r *TermVectorLeafReader) MaxDoc() int { return 1 }

// termVectorStoredFields renders the anonymous StoredFields of
// `public StoredFields storedFields()` (TermVectorLeafReader.java:220).
type termVectorStoredFields struct{}

// Document renders `public void document(int docID, StoredFieldVisitor visitor)`
// (TermVectorLeafReader.java:222), whose body is empty.
func (sf *termVectorStoredFields) Document(docID int, visitor spi.StoredFieldVisitor) error {
	return nil
}

// Prefetch renders the default body of StoredFields.prefetch(int), which is a
// no-op for an in-memory reader.
func (sf *termVectorStoredFields) Prefetch(docIDs []int) error { return nil }

// StoredFields renders `public StoredFields storedFields()`
// (TermVectorLeafReader.java:219).
func (r *TermVectorLeafReader) StoredFields() (spi.StoredFields, error) {
	return &termVectorStoredFields{}, nil
}

// GetMetaData renders `public LeafMetaData getMetaData()`
// (TermVectorLeafReader.java:227), which Java answers with
// `new LeafMetaData(Version.LATEST.major, null, null, false)`. Gocene's
// spi.IndexReaderMetaData carries the deletion and document counts instead of
// the created-version, the minimum version and the index sort, so those are
// the members reported here.
func (r *TermVectorLeafReader) GetMetaData() *spi.IndexReaderMetaData {
	return &spi.IndexReaderMetaData{HasDeletions: false, NumDocs: 1, MaxDoc: 1}
}

// GetCoreCacheHelper renders `public CacheHelper getCoreCacheHelper()`
// (TermVectorLeafReader.java:232).
func (r *TermVectorLeafReader) GetCoreCacheHelper() spi.CacheHelper { return nil }

// GetReaderCacheHelper renders `public CacheHelper getReaderCacheHelper()`
// (TermVectorLeafReader.java:237).
func (r *TermVectorLeafReader) GetReaderCacheHelper() spi.CacheHelper { return nil }

// ---------------------------------------------------------------------------
// Members TermVectorLeafReader inherits from org.apache.lucene.index.LeafReader
// and org.apache.lucene.index.IndexReader. Go has no class inheritance, so the
// concrete bodies of the two Java base classes are carried here.
// ---------------------------------------------------------------------------

// DocFreq renders the concrete `public final int docFreq(Term term)` of
// org.apache.lucene.index.LeafReader.
func (r *TermVectorLeafReader) DocFreq(term spi.Term) (int, error) {
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
func (r *TermVectorLeafReader) TotalTermFreq(term spi.Term) (int64, error) {
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
func (r *TermVectorLeafReader) Postings(term spi.Term, flags int) (spi.PostingsEnum, error) {
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
// org.apache.lucene.index.LeafReader does not declare. This reader exposes
// exactly one document, whose identifier is 0.
func (r *TermVectorLeafReader) DocID() int { return 0 }

// DocCount renders the document count of this leaf, which holds one document.
func (r *TermVectorLeafReader) DocCount() int { return 1 }

// HasDeletions renders `public boolean hasDeletions()` of
// org.apache.lucene.index.IndexReader, whose body asks each leaf whether it
// carries live docs. This leaf never does.
func (r *TermVectorLeafReader) HasDeletions() bool { return false }

// NumDeletedDocs renders `public final int numDeletedDocs()` of
// org.apache.lucene.index.IndexReader: maxDoc() - numDocs().
func (r *TermVectorLeafReader) NumDeletedDocs() int { return r.MaxDoc() - r.NumDocs() }

// EnsureOpen renders `protected final void ensureOpen()` of
// org.apache.lucene.index.IndexReader.
func (r *TermVectorLeafReader) EnsureOpen() error {
	if r.refs.Load() <= 0 {
		return fmt.Errorf("this IndexReader is closed")
	}
	return nil
}

// Close renders `public final synchronized void close()` of
// org.apache.lucene.index.IndexReader.
func (r *TermVectorLeafReader) Close() error {
	if r.closed.CompareAndSwap(false, true) {
		return r.DecRef()
	}
	return nil
}

// IncRef renders `public final void incRef()` of
// org.apache.lucene.index.IndexReader.
func (r *TermVectorLeafReader) IncRef() error {
	if !r.TryIncRef() {
		return fmt.Errorf("this IndexReader is closed")
	}
	return nil
}

// DecRef renders `public final void decRef()` of
// org.apache.lucene.index.IndexReader.
func (r *TermVectorLeafReader) DecRef() error {
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
func (r *TermVectorLeafReader) TryIncRef() bool {
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
func (r *TermVectorLeafReader) GetRefCount() int32 { return r.refs.Load() }

// GetContext renders `public final LeafReaderContext getContext()` of
// org.apache.lucene.index.LeafReader.
func (r *TermVectorLeafReader) GetContext() (spi.IndexReaderContext, error) {
	if err := r.EnsureOpen(); err != nil {
		return nil, err
	}
	return r.readerContext, nil
}

// Leaves renders `public final List<LeafReaderContext> leaves()` of
// org.apache.lucene.index.IndexReader, which delegates to the top-level
// context. A leaf reader's own context is top level here.
func (r *TermVectorLeafReader) Leaves() ([]*spi.LeafReaderContext, error) {
	if err := r.EnsureOpen(); err != nil {
		return nil, err
	}
	return []*spi.LeafReaderContext{r.readerContext}, nil
}
