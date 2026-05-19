// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// SlowCompositeCodecReaderWrapper exposes a merged, CodecReader-shaped view
// over multiple per-segment CodecReaders. It mirrors the package-private
// org.apache.lucene.index.SlowCompositeCodecReaderWrapper from Apache Lucene
// 10.4.0. The view targets merging, not searching: per-doc accessors fan out
// to the owning leaf by doc-id binary search; per-field accessors flatten
// across leaves via Multi* helpers when available.
//
// # Deviations from Lucene 10.4.0
//
// The Lucene class is exceptionally entangled with codec-side abstractions
// that Gocene has only partially ported. Where a Lucene contract is not yet
// representable, this port documents the gap inline and either:
//
//   - returns an explicit ErrSlowCompositeNotPorted, or
//   - delegates to the closest Gocene equivalent, or
//   - returns a zero/nil value matching Lucene's "not available" sentinel.
//
// Known gaps (see also the per-method comments):
//
//   - LeafMetaData on the wrapper composes from the first reader (no merged
//     LeafMetaData factory exists yet). Lucene aggregates createdVersionMajor,
//     minVersion, and hasBlocks across leaves.
//   - getFieldsReader / getTermVectorsReader / getNormsReader / getPointsReader
//     / getVectorReader stub the underlying wrappers because Gocene's
//     CodecReader returns interface{} for several producers and the
//     PointValues / KnnVectorValues / DocValuesSkipper surfaces lack the
//     PointTree, vectorValue(ord), and DocIndexIterator contracts that Lucene
//     relies on here.
//   - MultiBits.GetLiveDocs(MultiReader) is replaced by MultiBits over the
//     individual reader liveDocs and the per-segment docStarts.
//   - FieldInfos.GetMergedFieldInfos is replaced by a name-keyed union built
//     locally; conflict reconciliation is deferred (Lucene resolves via
//     FieldInfos.Builder, which is not yet ported).
//
// These deviations are intentional for Sprint 55 task GOC-3364 (option c:
// "muitos gaps esperados"). A follow-up sprint will close them once the
// missing infrastructure (KnnVectorsReader.search, AcceptDocs, PointTree,
// MultiBits factory, FieldInfos.Builder, KnnVectorValues random access) lands.
type SlowCompositeCodecReaderWrapper struct {
	codecReaders []*CodecReader
	docStarts    []int
	fieldInfos   *FieldInfos
	liveDocs     util.Bits
	meta         *LeafMetaData

	numDocsMu sync.Mutex
	numDocs   int // -1 until lazily computed
}

// ErrSlowCompositeNotPorted is returned by accessors whose Lucene-side
// dependencies are not yet representable in Gocene.
var ErrSlowCompositeNotPorted = errors.New("SlowCompositeCodecReaderWrapper: surface depends on unported Lucene types (see file header)")

// WrapSlowCompositeCodecReader returns a CodecReader-shaped view over the
// supplied per-segment CodecReaders. Mirrors the package-private
// SlowCompositeCodecReaderWrapper.wrap static factory:
//
//   - len(readers) == 0 yields an error (Lucene throws IllegalArgumentException).
//   - len(readers) == 1 returns the single reader unchanged.
//   - otherwise a fresh wrapper is constructed.
func WrapSlowCompositeCodecReader(readers []*CodecReader) (*SlowCompositeCodecReaderWrapper, *CodecReader, error) {
	switch len(readers) {
	case 0:
		return nil, nil, errors.New("SlowCompositeCodecReaderWrapper: must take at least one reader, got 0")
	case 1:
		return nil, readers[0], nil
	}
	w, err := newSlowCompositeCodecReaderWrapper(readers)
	if err != nil {
		return nil, nil, err
	}
	return w, nil, nil
}

func newSlowCompositeCodecReaderWrapper(codecReaders []*CodecReader) (*SlowCompositeCodecReaderWrapper, error) {
	w := &SlowCompositeCodecReaderWrapper{
		codecReaders: append([]*CodecReader(nil), codecReaders...),
		docStarts:    make([]int, len(codecReaders)+1),
		numDocs:      -1,
	}
	docStart := 0
	for i, reader := range codecReaders {
		docStart += reader.MaxDoc()
		w.docStarts[i+1] = docStart
	}

	// Deviation: Lucene aggregates LeafMetaData{createdVersionMajor, minVersion,
	// hasBlocks} across leaves and rejects mismatched major versions. Gocene
	// CodecReader does not yet expose LeafMetaData (only IndexReaderMetaData
	// without version fields). We construct a permissive LeafMetaData from the
	// current Lucene major version; the validation and per-leaf min-version
	// fold-in are deferred until CodecReader exposes per-leaf LeafMetaData.
	meta, err := NewLeafMetaData(util.LuceneVersionMajor, util.LuceneVersion, nil, false)
	if err != nil {
		return nil, fmt.Errorf("SlowCompositeCodecReaderWrapper: build LeafMetaData: %w", err)
	}
	w.meta = meta

	w.fieldInfos = mergeFieldInfosByName(codecReaders)

	subs := make([]util.Bits, len(codecReaders))
	for i, r := range codecReaders {
		subs[i] = r.GetLiveDocs()
	}
	// All-nil sub-bits means no leaf has deletions; Lucene also returns nil here.
	if allNilBits(subs) {
		w.liveDocs = nil
	} else {
		w.liveDocs = NewMultiBits(subs, w.docStarts)
	}
	return w, nil
}

// mergeFieldInfosByName is a name-keyed first-wins union over the leaf
// FieldInfos. It stands in for org.apache.lucene.index.FieldInfos.getMergedFieldInfos
// until FieldInfos.Builder is ported. Conflict reconciliation (e.g. mismatched
// doc-values type for the same field across leaves) is deferred.
func mergeFieldInfosByName(readers []*CodecReader) *FieldInfos {
	merged := NewFieldInfos()
	for _, r := range readers {
		fi := r.GetFieldInfos()
		if fi == nil && r.LeafReader != nil && r.LeafReader.IndexReader != nil {
			// Fallback: pre-segment-bound CodecReader stubs (and any tests)
			// publish FieldInfos through the embedded IndexReader rather
			// than via coreReaders. The Lucene path always has a populated
			// coreReaders, so this branch only matters for the in-test
			// construction path.
			fi = r.LeafReader.IndexReader.GetFieldInfos()
		}
		if fi == nil {
			continue
		}
		for _, name := range fi.Names() {
			if merged.GetByName(name) != nil {
				continue
			}
			if leaf := fi.GetByName(name); leaf != nil {
				// Add ignores errors only when the collection is frozen; ours is not.
				_ = merged.Add(leaf)
			}
		}
	}
	return merged
}

func allNilBits(subs []util.Bits) bool {
	for _, b := range subs {
		if b != nil {
			return false
		}
	}
	return true
}

// docIDToReaderID maps a global doc-id into the index of the owning leaf.
// Mirrors SlowCompositeCodecReaderWrapper.docIdToReaderId: a binary search
// over docStarts, with the Lucene -2-pos adjustment for non-exact hits.
func (w *SlowCompositeCodecReaderWrapper) docIDToReaderID(doc int) (int, error) {
	maxDoc := w.docStarts[len(w.docStarts)-1]
	if doc < 0 || doc >= maxDoc {
		return -1, fmt.Errorf("SlowCompositeCodecReaderWrapper: doc %d out of range [0, %d)", doc, maxDoc)
	}
	// sort.SearchInts returns the smallest i such that docStarts[i] >= doc.
	// Lucene's Arrays.binarySearch returns either an exact hit or -ip-1.
	// For our [docStart, docStart+maxDoc) ranges, the leaf index is:
	//   exact hit on docStarts[i] -> i (because docStarts[i] is itself the
	//                                   first doc of leaf i)
	//   non-exact (search returned i, doc < docStarts[i]) -> i-1
	i := sort.SearchInts(w.docStarts, doc)
	if i < len(w.docStarts) && w.docStarts[i] == doc {
		return i, nil
	}
	return i - 1, nil
}

// GetFieldInfos returns the merged FieldInfos across leaves.
func (w *SlowCompositeCodecReaderWrapper) GetFieldInfos() *FieldInfos { return w.fieldInfos }

// GetLiveDocs returns the merged liveDocs Bits or nil when no leaf has
// deletions.
func (w *SlowCompositeCodecReaderWrapper) GetLiveDocs() util.Bits { return w.liveDocs }

// GetMetaData returns the composite LeafMetaData. See file-header deviation
// note: the value is a permissive aggregate, not a strict version fold.
func (w *SlowCompositeCodecReaderWrapper) GetMetaData() *LeafMetaData { return w.meta }

// MaxDoc returns the sum of MaxDoc across all leaves.
func (w *SlowCompositeCodecReaderWrapper) MaxDoc() int {
	return w.docStarts[len(w.docStarts)-1]
}

// NumDocs lazily computes the sum of live docs across leaves on first call,
// mirroring Lucene's synchronized lazy field.
func (w *SlowCompositeCodecReaderWrapper) NumDocs() int {
	w.numDocsMu.Lock()
	defer w.numDocsMu.Unlock()
	if w.numDocs == -1 {
		n := 0
		for _, r := range w.codecReaders {
			n += r.NumDocs()
		}
		w.numDocs = n
	}
	return w.numDocs
}

// GetCoreCacheHelper mirrors Lucene's null return: a composite view is not a
// stable cache key.
func (w *SlowCompositeCodecReaderWrapper) GetCoreCacheHelper() interface{} { return nil }

// GetReaderCacheHelper mirrors Lucene's null return: see GetCoreCacheHelper.
func (w *SlowCompositeCodecReaderWrapper) GetReaderCacheHelper() interface{} { return nil }

// Close decRefs every wrapped CodecReader. Mirrors the implicit cleanup that
// callers perform after a merge: each codec reader was incRef'd when added,
// so a symmetric decRef releases its share.
func (w *SlowCompositeCodecReaderWrapper) Close() error {
	var first error
	for _, r := range w.codecReaders {
		if r == nil {
			continue
		}
		if err := r.DecRef(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// remap returns the merged-view FieldInfo for the leaf-side info, so visitors
// only ever observe the composite FieldInfos. Mirrors the private remap
// helper.
func (w *SlowCompositeCodecReaderWrapper) remap(info *FieldInfo) *FieldInfo {
	if info == nil {
		return nil
	}
	if merged := w.fieldInfos.GetByName(info.Name()); merged != nil {
		return merged
	}
	return info
}

// -----------------------------------------------------------------------------
// Stored fields
// -----------------------------------------------------------------------------

// SlowCompositeStoredFieldsReader fans StoredFieldsReader calls out to the
// owning leaf by doc-id. Visitor callbacks remap FieldInfo through the
// composite FieldInfos before forwarding.
type SlowCompositeStoredFieldsReader struct {
	readers   []StoredFieldsReader
	docStarts []int
	parent    *SlowCompositeCodecReaderWrapper
}

// GetFieldsReader returns the composite StoredFieldsReader.
func (w *SlowCompositeCodecReaderWrapper) GetFieldsReader() *SlowCompositeStoredFieldsReader {
	readers := make([]StoredFieldsReader, len(w.codecReaders))
	for i, r := range w.codecReaders {
		readers[i] = r.GetStoredFieldsReader()
	}
	return &SlowCompositeStoredFieldsReader{readers: readers, docStarts: w.docStarts, parent: w}
}

// Close releases every leaf reader, returning the first error.
func (r *SlowCompositeStoredFieldsReader) Close() error {
	closers := make([]io.Closer, 0, len(r.readers))
	for _, sr := range r.readers {
		if sr != nil {
			closers = append(closers, sr)
		}
	}
	return util.CloseAll(closers...)
}

// VisitDocument dispatches to the owning leaf and wraps the visitor so every
// field-name reported back goes through the composite FieldInfos.
func (r *SlowCompositeStoredFieldsReader) VisitDocument(docID int, visitor StoredFieldVisitor) error {
	readerID, err := r.parent.docIDToReaderID(docID)
	if err != nil {
		return err
	}
	leaf := r.readers[readerID]
	if leaf == nil {
		return nil
	}
	return leaf.VisitDocument(docID-r.docStarts[readerID], &remappingStoredFieldVisitor{
		parent:   r.parent,
		delegate: visitor,
	})
}

// remappingStoredFieldVisitor is the equivalent of Lucene's anonymous visitor
// wrapper inside document(): it consults the composite FieldInfos before
// dispatching.
type remappingStoredFieldVisitor struct {
	parent   *SlowCompositeCodecReaderWrapper
	delegate StoredFieldVisitor
}

func (v *remappingStoredFieldVisitor) StringField(field string, value string) {
	v.delegate.StringField(v.remapName(field), value)
}
func (v *remappingStoredFieldVisitor) BinaryField(field string, value []byte) {
	v.delegate.BinaryField(v.remapName(field), value)
}
func (v *remappingStoredFieldVisitor) IntField(field string, value int) {
	v.delegate.IntField(v.remapName(field), value)
}
func (v *remappingStoredFieldVisitor) LongField(field string, value int64) {
	v.delegate.LongField(v.remapName(field), value)
}
func (v *remappingStoredFieldVisitor) FloatField(field string, value float32) {
	v.delegate.FloatField(v.remapName(field), value)
}
func (v *remappingStoredFieldVisitor) DoubleField(field string, value float64) {
	v.delegate.DoubleField(v.remapName(field), value)
}

// remapName resolves the field through the composite FieldInfos when the
// merged view knows the field; otherwise the input name passes through. The
// composite FieldInfos is the authoritative naming context for downstream
// merge consumers.
func (v *remappingStoredFieldVisitor) remapName(field string) string {
	if fi := v.parent.fieldInfos.GetByName(field); fi != nil {
		return fi.Name()
	}
	return field
}

// -----------------------------------------------------------------------------
// Term vectors
// -----------------------------------------------------------------------------

// SlowCompositeTermVectorsReader fans TermVectorsReader.Get / GetField calls
// out to the owning leaf by doc-id.
type SlowCompositeTermVectorsReader struct {
	readers   []TermVectorsReader
	docStarts []int
	parent    *SlowCompositeCodecReaderWrapper
}

// GetTermVectorsReader returns the composite TermVectorsReader.
func (w *SlowCompositeCodecReaderWrapper) GetTermVectorsReader() *SlowCompositeTermVectorsReader {
	readers := make([]TermVectorsReader, len(w.codecReaders))
	for i, r := range w.codecReaders {
		readers[i] = r.GetTermVectorsReader()
	}
	return &SlowCompositeTermVectorsReader{readers: readers, docStarts: w.docStarts, parent: w}
}

// Close releases every leaf reader, returning the first error.
func (r *SlowCompositeTermVectorsReader) Close() error {
	closers := make([]io.Closer, 0, len(r.readers))
	for _, tv := range r.readers {
		if tv != nil {
			closers = append(closers, tv)
		}
	}
	return util.CloseAll(closers...)
}

// Get returns the term vectors for docID. Returns (nil, nil) when the owning
// leaf does not have term vectors, mirroring Lucene's null return.
func (r *SlowCompositeTermVectorsReader) Get(docID int) (Fields, error) {
	readerID, err := r.parent.docIDToReaderID(docID)
	if err != nil {
		return nil, err
	}
	leaf := r.readers[readerID]
	if leaf == nil {
		return nil, nil
	}
	return leaf.Get(docID - r.docStarts[readerID])
}

// GetField returns the term vector for one field of docID.
func (r *SlowCompositeTermVectorsReader) GetField(docID int, field string) (Terms, error) {
	readerID, err := r.parent.docIDToReaderID(docID)
	if err != nil {
		return nil, err
	}
	leaf := r.readers[readerID]
	if leaf == nil {
		return nil, nil
	}
	return leaf.GetField(docID-r.docStarts[readerID], field)
}

// -----------------------------------------------------------------------------
// Postings
// -----------------------------------------------------------------------------

// SlowCompositeFieldsProducer flattens the per-leaf postings via MultiFields.
type SlowCompositeFieldsProducer struct {
	producers []FieldsProducer
	fields    *MultiFields
}

// GetPostingsReader returns the composite postings producer. Mirrors
// SlowCompositeFieldsProducerWrapper: it sources every leaf's
// FieldsProducer, then builds a MultiFields over the non-nil ones.
//
// Deviation: Lucene also constructs ReaderSlice per leaf to feed MultiFields.
// Gocene's MultiFields presently consumes only the Fields slice (NewMultiFields(fields...));
// the slice information is captured but unused until MultiTermsEnum lands
// (backlog #2706).
func (w *SlowCompositeCodecReaderWrapper) GetPostingsReader() *SlowCompositeFieldsProducer {
	producers := make([]FieldsProducer, len(w.codecReaders))
	subs := make([]Fields, 0, len(w.codecReaders))
	for i, r := range w.codecReaders {
		p := r.GetPostingsReader()
		producers[i] = p
		if p != nil {
			subs = append(subs, fieldsProducerAsFields{p})
		}
	}
	return &SlowCompositeFieldsProducer{
		producers: producers,
		fields:    NewMultiFields(subs...),
	}
}

// fieldsProducerAsFields adapts FieldsProducer to the Fields interface that
// MultiFields expects, since the two share Terms()/Iterator() semantics but
// differ at the type level.
type fieldsProducerAsFields struct{ FieldsProducer }

func (a fieldsProducerAsFields) Iterator() (FieldIterator, error) {
	// Deviation: Gocene's FieldsProducer does not yet expose an iterator over
	// field names. Once exposed, this adapter will forward; for now the
	// MultiFields aggregate iterator falls back to combining per-segment views
	// at the SegmentReader boundary.
	return nil, ErrSlowCompositeNotPorted
}

func (a fieldsProducerAsFields) Size() int { return -1 }

// Close releases each underlying producer.
func (p *SlowCompositeFieldsProducer) Close() error {
	closers := make([]io.Closer, 0, len(p.producers))
	for _, fp := range p.producers {
		if fp != nil {
			closers = append(closers, fp)
		}
	}
	return util.CloseAll(closers...)
}

// Terms returns the MultiFields terms for the field.
func (p *SlowCompositeFieldsProducer) Terms(field string) (Terms, error) {
	return p.fields.Terms(field)
}

// Iterator returns the MultiFields iterator.
func (p *SlowCompositeFieldsProducer) Iterator() (FieldIterator, error) {
	return p.fields.Iterator()
}

// Size returns the aggregate field count.
func (p *SlowCompositeFieldsProducer) Size() int { return p.fields.Size() }

// -----------------------------------------------------------------------------
// Norms, doc values, points, vectors
//
// Deviation: Lucene returns concrete NormsProducer / DocValuesProducer /
// PointsReader / KnnVectorsReader instances. Gocene's CodecReader.Get*Reader
// for these four return interface{} (the typed surface lives in package
// codecs), and the per-field flattening uses MultiDocValues helpers that
// currently return ErrMultiDocValuesNotImplemented (backlog #2703). The
// stubs below preserve the wrapper-shape and document the gap.
// -----------------------------------------------------------------------------

// GetNormsReader is not yet ported; see file-header deviations and #2703.
func (w *SlowCompositeCodecReaderWrapper) GetNormsReader() (interface{}, error) {
	return nil, ErrSlowCompositeNotPorted
}

// GetDocValuesReader is not yet ported; see file-header deviations and #2703.
func (w *SlowCompositeCodecReaderWrapper) GetDocValuesReader() (interface{}, error) {
	return nil, ErrSlowCompositeNotPorted
}

// GetPointsReader is not yet ported; PointValues lacks PointTree/getNumIndexDimensions.
func (w *SlowCompositeCodecReaderWrapper) GetPointsReader() (interface{}, error) {
	return nil, ErrSlowCompositeNotPorted
}

// GetVectorReader is not yet ported; KnnVectorValues lacks the
// DocIndexIterator and random-access vectorValue(ord) contracts that the
// merged float/byte vector views rely on.
func (w *SlowCompositeCodecReaderWrapper) GetVectorReader() (interface{}, error) {
	return nil, ErrSlowCompositeNotPorted
}
