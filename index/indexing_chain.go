// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// IndexingChain is the default general-purpose indexing chain, which handles
// indexing all types of fields. It is the Go port of Apache Lucene 10.4.0's
// org.apache.lucene.index.IndexingChain.
//
// PORTING NOTE (Sprint 55, option c — large gaps expected):
//
// Lucene's IndexingChain depends on a cluster of types that are not yet ported
// to Gocene, several of which cannot be imported here at all because they
// would form an import cycle (package document and package search both import
// package index). To keep this port self-contained and compilable, the
// following collaborators are modelled as narrow interfaces declared in this
// file rather than concrete types. When the real implementations land, swap
// the interface for the concrete type; the orchestration logic does not change.
//
//   - StoredFieldsConsumer  -> StoredFieldsConsumerHandle
//   - VectorValuesConsumer  -> VectorValuesConsumerHandle
//   - LiveIndexWriterConfig -> IndexingChainConfig
//   - FieldInfos.Builder    -> FieldInfosBuilderHandle (Gocene's existing
//     FieldInfosBuilder is a simple fluent builder lacking add()-with-global-
//     consistency-check, finish(), getSoftDeletesFieldName(),
//     getParentFieldName()).
//   - org.apache.lucene.search.similarities.Similarity -> SimilarityHandle
//     (the real Similarity lives in package search, which imports index).
//   - IndexableField     -> IndexingChainField (Gocene's index.IndexableField
//     is intentionally minimal and omits invertableType()/binaryValue()/the
//     rich fieldType()).
//
// DEFERRED (gaps left for later sprints, each marked GAP in the code):
//   - maybeSortSegment / validateIndexSortDVType: index sorting needs the
//     IndexSorter / Sort / SortField cluster (package search). A configured
//     index sort makes Flush fail loudly.
//   - writeNorms / writeDocValues / writePoints codec wiring: the codec format
//     SPI for norms, doc values and points is not modelled on index.Codec.
//     The per-field iteration is ported; consumer acquisition fails loudly.
//   - termsHash.Flush NormsProducer merge-instance: the NormsProducer SPI is
//     not modelled; Flush passes nil.
//   - field-infos write at end of Flush: needs codec.FieldInfosFormat wiring
//     against the live FieldInfos.
//   - invertTokenStream: the analysis TokenStream / attribute pipeline is not
//     wired into the chain; token-stream inversion fails loudly. The
//     single-valued binary path (invertTerm) is fully ported.
//   - markAsReserved / ReservedField: depend on DocumentsWriterPerThread setup.
type IndexingChain struct {
	bytesUsed  *util.Counter
	fieldInfos FieldInfosBuilderHandle

	// termsHash writes postings and term vectors.
	termsHash TermsHash
	// docValuesBytePool is the shared pool for doc-value terms.
	docValuesBytePool *util.ByteBlockPool
	// storedFieldsConsumer writes stored fields.
	storedFieldsConsumer StoredFieldsConsumerHandle
	vectorValuesConsumer VectorValuesConsumerHandle
	termVectorsWriter    *TermVectorsConsumer

	// fieldHash is an open-addressed chained hash of PerField, keyed by name.
	// Lucene benchmarked this to be ~2% faster than a HashMap.
	fieldHash []*indexingPerField
	hashMask  int

	totalFieldCount int
	nextFieldGen    int64

	// fields holds the unique fields seen in the current document.
	fields []*indexingPerField
	// docFields holds one slot per field instance in the current document.
	docFields []*indexingPerField

	indexWriterConfig         IndexingChainConfig
	indexCreatedVersionMajor  int
	abortingExceptionConsumer func(error)
	hasHitAbortingException   bool
}

// IndexingChainConfig is the subset of Lucene's LiveIndexWriterConfig that the
// indexing chain consumes. See the PORTING NOTE on IndexingChain.
type IndexingChainConfig interface {
	// HasIndexSort reports whether an index sort is configured.
	HasIndexSort() bool
	// Similarity returns the similarity used to compute norms.
	Similarity() SimilarityHandle
}

// SimilarityHandle is the subset of Lucene's Similarity consumed by the
// indexing chain: it computes the per-field norm from the inversion state.
//
// GAP: the real Similarity lives in package search, which imports package
// index; it cannot be imported here. search.LucenePerFieldSimilarityWrapper
// already exposes ComputeNormFromInvertState(*FieldInvertState) int64 with a
// matching shape, so a thin adapter in package search will satisfy this.
type SimilarityHandle interface {
	// ComputeNorm returns the norm value for a field given its inversion
	// state. Must return a non-zero value for a non-empty field.
	ComputeNorm(state *FieldInvertState) int64
}

// FieldInfosBuilderHandle is the subset of Lucene's FieldInfos.Builder that the
// indexing chain consumes. Gocene's existing FieldInfosBuilder does not expose
// these semantics, so the chain depends on this interface instead.
type FieldInfosBuilderHandle interface {
	// Add registers (or merges) a FieldInfo and returns the canonical
	// FieldInfo for the segment, after global-consistency checks.
	Add(fi *FieldInfo) (*FieldInfo, error)
	// Finish materialises the final FieldInfos for the segment.
	Finish() *FieldInfos
	// SoftDeletesFieldName returns the configured soft-deletes field name.
	SoftDeletesFieldName() string
	// ParentFieldName returns the configured parent field name.
	ParentFieldName() string
}

// StoredFieldsConsumerHandle is the subset of Lucene's StoredFieldsConsumer
// consumed by the indexing chain.
type StoredFieldsConsumerHandle interface {
	StartDocument(docID int) error
	WriteField(fi *FieldInfo, field IndexingChainField) error
	FinishDocument() error
	Finish(maxDoc int) error
	Flush(state *SegmentWriteState, sortMap SorterDocMap) error
	Abort()
	RamBytesUsed() int64
}

// VectorValuesConsumerHandle is the subset of Lucene's VectorValuesConsumer
// consumed by the indexing chain.
type VectorValuesConsumerHandle interface {
	AddField(fi *FieldInfo) (KnnFieldVectorsWriterHandle, error)
	Flush(state *SegmentWriteState, sortMap SorterDocMap) error
	Abort()
	RamBytesUsed() int64
}

// KnnFieldVectorsWriterHandle is the opaque per-field vector writer returned by
// the vector values consumer. It accepts one vector value per document.
//
// GAP: Lucene's KnnFieldVectorsWriter<T> is generic over byte[] / float[].
// Gocene erases the element type to any until the codec vectors SPI is ported.
type KnnFieldVectorsWriterHandle interface {
	AddValue(docID int, value any) error
}

// IndexingChainField is the field contract consumed by the indexing chain. It
// is the Go port of the surface Lucene's IndexingChain uses on
// org.apache.lucene.index.IndexableField.
//
// PORTING NOTE: Gocene's index.IndexableField is intentionally minimal and
// omits InvertableType(), BinaryValue() and the rich IndexableFieldType. The
// indexing chain needs all of those, so it depends on this wider interface.
// It embeds IndexableField so a concrete field still flows into
// TermsHashPerField.Start, which expects the narrow type.
type IndexingChainField interface {
	IndexableField

	// IndexableFieldType returns the rich field-type contract used to drive
	// FieldInfo construction. (Named to avoid colliding with the embedded
	// IndexableField.FieldType, which returns the minimal FieldTypeInterface.)
	IndexableFieldType() IndexableFieldType

	// BinaryValueBytes returns the binary value of the field, or nil.
	// (BinaryValue is already provided by the embedded IndexableField as
	// []byte; this alias keeps the porting intent explicit.)
	BinaryValueBytes() []byte

	// InvertableType describes how the field is inverted: as a single binary
	// term or through a token stream.
	InvertableType() InvertableType
}

// InvertableType describes how an IndexableField is inverted (indexed). It is
// the index-package mirror of Lucene's
// org.apache.lucene.document.InvertableType (which cannot be imported here
// without an import cycle). The ordinals match Lucene: BINARY=0,
// TOKEN_STREAM=1.
type InvertableType int

const (
	// InvertableTypeBinary inverts the field as a single binary term.
	InvertableTypeBinary InvertableType = iota
	// InvertableTypeTokenStream inverts the field through its TokenStream.
	InvertableTypeTokenStream
)

// String returns the canonical Lucene name for the InvertableType.
func (it InvertableType) String() string {
	switch it {
	case InvertableTypeBinary:
		return "BINARY"
	case InvertableTypeTokenStream:
		return "TOKEN_STREAM"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(it))
	}
}

// NewIndexingChain constructs an IndexingChain.
//
// GAP: Lucene wires the concrete StoredFieldsConsumer / TermVectorsConsumer /
// VectorValuesConsumer (and their sorting variants) here from the codec and
// directory. Gocene cannot construct those generically yet, so they are
// injected by the caller (DocumentsWriterPerThread, once ported). termsHash is
// likewise injected because FreqProxTermsWriter's Gocene constructor has a
// divergent signature (see freq_prox_terms_writer.go).
func NewIndexingChain(
	indexCreatedVersionMajor int,
	fieldInfos FieldInfosBuilderHandle,
	indexWriterConfig IndexingChainConfig,
	abortingExceptionConsumer func(error),
	termsHash TermsHash,
	storedFieldsConsumer StoredFieldsConsumerHandle,
	vectorValuesConsumer VectorValuesConsumerHandle,
	termVectorsWriter *TermVectorsConsumer,
) (*IndexingChain, error) {
	if abortingExceptionConsumer == nil {
		return nil, fmt.Errorf("indexing chain: abortingExceptionConsumer must not be nil")
	}
	if fieldInfos == nil {
		return nil, fmt.Errorf("indexing chain: fieldInfos must not be nil")
	}
	if termsHash == nil {
		return nil, fmt.Errorf("indexing chain: termsHash must not be nil")
	}
	if storedFieldsConsumer == nil {
		return nil, fmt.Errorf("indexing chain: storedFieldsConsumer must not be nil")
	}
	if vectorValuesConsumer == nil {
		return nil, fmt.Errorf("indexing chain: vectorValuesConsumer must not be nil")
	}
	return &IndexingChain{
		bytesUsed:                 util.NewCounter(),
		fieldInfos:                fieldInfos,
		indexWriterConfig:         indexWriterConfig,
		indexCreatedVersionMajor:  indexCreatedVersionMajor,
		abortingExceptionConsumer: abortingExceptionConsumer,
		termsHash:                 termsHash,
		storedFieldsConsumer:      storedFieldsConsumer,
		vectorValuesConsumer:      vectorValuesConsumer,
		termVectorsWriter:         termVectorsWriter,
		fieldHash:                 make([]*indexingPerField, 2),
		hashMask:                  1,
		fields:                    make([]*indexingPerField, 1),
		docFields:                 make([]*indexingPerField, 2),
		docValuesBytePool:         util.NewByteBlockPool(util.NewDirectAllocator()),
	}, nil
}

func (c *IndexingChain) onAbortingException(err error) {
	c.hasHitAbortingException = true
	c.abortingExceptionConsumer(err)
}

// Flush writes all buffered state for the segment and returns the sort map
// (nil when the segment is unsorted or already sorted).
//
// NOTE: the caller (DocumentsWriterPerThread) handles aborting on any error
// from this method.
func (c *IndexingChain) Flush(state *SegmentWriteState) (SorterDocMap, error) {
	sortMap, err := c.maybeSortSegment(state)
	if err != nil {
		return nil, err
	}
	maxDoc := state.SegmentInfo.DocCount()

	if err := c.writeNorms(state, sortMap); err != nil {
		return nil, err
	}
	if err := c.writeDocValues(state, sortMap); err != nil {
		return nil, err
	}
	if err := c.writePoints(state, sortMap); err != nil {
		return nil, err
	}
	if err := c.vectorValuesConsumer.Flush(state, sortMap); err != nil {
		return nil, err
	}

	// It's possible all docs hit non-aborting exceptions.
	if err := c.storedFieldsConsumer.Finish(maxDoc); err != nil {
		return nil, err
	}
	if err := c.storedFieldsConsumer.Flush(state, sortMap); err != nil {
		return nil, err
	}

	fieldsToFlush := make(map[string]*TermsHashPerField)
	for _, perField := range c.fieldHash {
		for pf := perField; pf != nil; pf = pf.next {
			if pf.invertState != nil {
				fieldsToFlush[pf.fieldInfo.Name()] = pf.termsHashPerField
			}
		}
	}

	// GAP: Lucene opens a NormsProducer here and passes its merge instance to
	// termsHash.flush so postings reuse a single IndexInput for norms. The
	// NormsProducer SPI is not ported yet, so nil is passed.
	if err := c.termsHash.Flush(fieldsToFlush, state, sortMap, nil); err != nil {
		return nil, err
	}

	// GAP: Lucene writes field infos here, after the consumers flush, so a
	// consumer (e.g. FreqProxTermsWriter) can still alter FieldInfo (e.g.
	// storePayloads). This needs codec.FieldInfosFormat().Write(...) wired
	// against the live FieldInfos; deferred.

	return sortMap, nil
}

// maybeSortSegment computes the document sort map when the segment uses index
// sorting.
//
// GAP: index sorting depends on the IndexSorter / Sort / SortField cluster
// (package search) which is not ported. This returns nil (unsorted), failing
// loudly only if a sort is configured so the gap cannot pass silently.
func (c *IndexingChain) maybeSortSegment(_ *SegmentWriteState) (SorterDocMap, error) {
	if c.indexWriterConfig == nil || !c.indexWriterConfig.HasIndexSort() {
		return nil, nil
	}
	return nil, fmt.Errorf("indexing chain: index sorting not yet supported (GAP: IndexSorter cluster unported)")
}

// writePoints writes all buffered points.
func (c *IndexingChain) writePoints(state *SegmentWriteState, sortMap SorterDocMap) error {
	var pointsWriter BufferedPointsCodecWriter
	for _, perField := range c.fieldHash {
		for pf := perField; pf != nil; pf = pf.next {
			if pf.pointValuesWriter == nil {
				continue
			}
			// pointValuesWriter may be initialised but never have written a doc.
			if pf.fieldInfo.PointDimensionCount() > 0 {
				if pointsWriter == nil {
					// GAP: lazy init of PointsWriter via
					// codec.PointsFormat().FieldsWriter(state). The codec
					// points format SPI is not modelled on index.Codec.
					return fmt.Errorf("indexing chain: points flush not yet wired to codec (GAP: PointsFormat SPI)")
				}
				if err := pf.pointValuesWriter.Flush(state, sortMap, pointsWriter); err != nil {
					return err
				}
			}
			pf.pointValuesWriter = nil
		}
	}
	return nil
}

// writeDocValues writes all buffered doc values.
func (c *IndexingChain) writeDocValues(state *SegmentWriteState, _ SorterDocMap) error {
	for _, perField := range c.fieldHash {
		for pf := perField; pf != nil; pf = pf.next {
			if pf.docValuesWriter != nil {
				if pf.fieldInfo.DocValuesType() == DocValuesTypeNone {
					return fmt.Errorf("indexing chain: segment=%v field=%q has no docValues but wrote them",
						state.SegmentInfo, pf.fieldInfo.Name())
				}
				// GAP: lazy init of DocValuesConsumer via
				// codec.DocValuesFormat().FieldsConsumer(state). The codec
				// doc-values format SPI is not modelled on index.Codec.
				return fmt.Errorf("indexing chain: docValues flush not yet wired to codec (GAP: DocValuesFormat SPI)")
			}
			if pf.fieldInfo != nil && pf.fieldInfo.DocValuesType() != DocValuesTypeNone {
				return fmt.Errorf("indexing chain: segment=%v field=%q has docValues but did not write them",
					state.SegmentInfo, pf.fieldInfo.Name())
			}
		}
	}
	return nil
}

// writeNorms writes all buffered norms.
func (c *IndexingChain) writeNorms(state *SegmentWriteState, _ SorterDocMap) error {
	if state.FieldInfos == nil || !state.FieldInfos.HasNorms() {
		return nil
	}
	// GAP: requires codec.NormsFormat().NormsConsumer(state). The codec norms
	// format SPI is not modelled on index.Codec. The per-field iteration and
	// the omitNorms re-check (a field's omitNorms can change after it is first
	// added) belong here; deferred until the SPI lands.
	return fmt.Errorf("indexing chain: norms flush not yet wired to codec (GAP: NormsFormat SPI)")
}

// Abort releases buffered resources after an unrecoverable error.
func (c *IndexingChain) Abort() {
	defer func() {
		// Finalizer: closes any open files in the term vectors writer.
		c.termsHash.Abort()
	}()
	c.storedFieldsConsumer.Abort()
	c.vectorValuesConsumer.Abort()
	for i := range c.fieldHash {
		c.fieldHash[i] = nil
	}
}

// rehash doubles the field hash table and re-buckets every PerField.
func (c *IndexingChain) rehash() {
	newHashSize := len(c.fieldHash) * 2
	newHashArray := make([]*indexingPerField, newHashSize)
	newHashMask := newHashSize - 1
	for j := range c.fieldHash {
		fp0 := c.fieldHash[j]
		for fp0 != nil {
			hashPos2 := stringHashCode(fp0.fieldName) & newHashMask
			nextFP0 := fp0.next
			fp0.next = newHashArray[hashPos2]
			newHashArray[hashPos2] = fp0
			fp0 = nextFP0
		}
	}
	c.fieldHash = newHashArray
	c.hashMask = newHashMask
}

// startStoredFields calls StoredFieldsWriter.startDocument, aborting the
// segment on any error.
func (c *IndexingChain) startStoredFields(docID int) error {
	if err := c.storedFieldsConsumer.StartDocument(docID); err != nil {
		c.onAbortingException(err)
		return err
	}
	return nil
}

// finishStoredFields calls StoredFieldsWriter.finishDocument, aborting the
// segment on any error.
func (c *IndexingChain) finishStoredFields() error {
	if err := c.storedFieldsConsumer.FinishDocument(); err != nil {
		c.onAbortingException(err)
		return err
	}
	return nil
}

// ProcessDocument indexes one document. docID is the in-segment document id.
func (c *IndexingChain) ProcessDocument(docID int, doc []IndexingChainField) (err error) {
	fieldCount := 0
	indexedFieldCount := 0 // number of unique fields indexed with postings
	fieldGen := c.nextFieldGen
	c.nextFieldGen++
	docFieldIdx := 0

	// Two passes are required: a multi-valued field must be fully processed at
	// once because the analyzer is free to reuse a TokenStream across fields.
	if err = c.termsHash.StartDocument(); err != nil {
		return err
	}
	if err = c.startStoredFields(docID); err != nil {
		return err
	}

	defer func() {
		if c.hasHitAbortingException {
			return
		}
		// Finish each indexed field name seen in the document.
		for i := 0; i < indexedFieldCount; i++ {
			if ferr := c.fields[i].finish(docID); ferr != nil && err == nil {
				err = ferr
			}
		}
		if ferr := c.finishStoredFields(); ferr != nil && err == nil {
			err = ferr
		}
		// TODO: for broken docs, optimize termsHash.finishDocument.
		if ferr := c.termsHash.FinishDocument(docID); ferr != nil {
			// Must abort: on-disk term vectors may now be corrupt.
			c.abortingExceptionConsumer(ferr)
			if err == nil {
				err = ferr
			}
		}
	}()

	// 1st pass: verify the doc schema matches the index schema and build the
	// per-field schema for every unique field in the document.
	for _, field := range doc {
		fieldType := field.IndexableFieldType()
		pf := c.getOrAddPerField(field.Name())
		if pf.fieldGen != fieldGen { // first time we see this field in this document
			c.fields[fieldCount] = pf
			fieldCount++
			pf.fieldGen = fieldGen
			pf.reset(docID)
		}
		if docFieldIdx >= len(c.docFields) {
			c.oversizeDocFields()
		}
		c.docFields[docFieldIdx] = pf
		docFieldIdx++
		if uerr := updateDocFieldSchema(field.Name(), pf.schema, fieldType); uerr != nil {
			return uerr
		}
	}

	// For each field, initialize its FieldInfo on first sight in the segment,
	// otherwise verify the in-doc schema matches the segment schema.
	for i := 0; i < fieldCount; i++ {
		pf := c.fields[i]
		if pf.fieldInfo == nil {
			if ierr := c.initializeFieldInfo(pf); ierr != nil {
				return ierr
			}
		} else if serr := pf.schema.assertSameSchema(pf.fieldInfo); serr != nil {
			return serr
		}
	}

	// 2nd pass: index each field, counting unique fields indexed with postings.
	docFieldIdx = 0
	for _, field := range doc {
		indexed, perr := c.processField(docID, field, c.docFields[docFieldIdx])
		if perr != nil {
			return perr
		}
		if indexed {
			c.fields[indexedFieldCount] = c.docFields[docFieldIdx]
			indexedFieldCount++
		}
		docFieldIdx++
	}
	return nil
}

func (c *IndexingChain) oversizeDocFields() {
	newSize := util.Oversize(len(c.docFields)+1, util.NumBytesObjectRef)
	newDocFields := make([]*indexingPerField, newSize)
	copy(newDocFields, c.docFields)
	c.docFields = newDocFields
}

// initializeFieldInfo creates and registers a new FieldInfo for a field seen
// for the first time in this segment, and wires its per-field writers.
func (c *IndexingChain) initializeFieldInfo(pf *indexingPerField) error {
	s := pf.schema

	// GAP: validateIndexSortDVType requires the IndexSorter cluster; index
	// sorting is rejected up-front by maybeSortSegment, so it is omitted here.
	// GAP: validateMaxVectorDimension requires
	// codec.KnnVectorsFormat().GetMaxDimensions(name); the codec vectors SPI
	// is not modelled on index.Codec, so the upper-bound check is skipped.

	opts := DefaultFieldInfoOptions()
	opts.IndexOptions = s.indexOptions
	opts.DocValuesType = s.docValuesType
	opts.DocValuesSkipIndexType = s.docValuesSkipIndex
	opts.DocValuesGen = -1
	opts.OmitNorms = s.omitNorms
	opts.StoreTermVectors = s.storeTermVector
	// storePayloads is set during indexing if payloads are seen; left false.
	opts.PointDimensionCount = s.pointDimensionCount
	opts.PointIndexDimensionCount = s.pointIndexDimensionCount
	opts.PointNumBytes = s.pointNumBytes
	opts.VectorDimension = s.vectorDimension
	opts.VectorEncoding = s.vectorEncoding
	opts.VectorSimilarityFunction = s.vectorSimilarityFunction
	opts.IsSoftDeletesField = pf.fieldName == c.fieldInfos.SoftDeletesFieldName()
	opts.IsParentField = pf.fieldName == c.fieldInfos.ParentFieldName()

	fi := NewFieldInfo(pf.fieldName, -1, opts)
	for k, v := range s.attributes {
		fi.PutAttribute(k, v)
	}
	registered, err := c.fieldInfos.Add(fi)
	if err != nil {
		return err
	}
	pf.setFieldInfo(registered)

	if registered.IndexOptions() != IndexOptionsNone {
		if err := pf.setInvertState(c); err != nil {
			return err
		}
	}

	switch registered.DocValuesType() {
	case DocValuesTypeNone:
		// nothing to do
	case DocValuesTypeNumeric:
		pf.docValuesWriter = newDVWNumeric(NewNumericDocValuesWriter(registered, c.bytesUsed))
	case DocValuesTypeBinary:
		w, err := NewBinaryDocValuesWriter(registered, c.bytesUsed)
		if err != nil {
			return err
		}
		pf.docValuesWriter = newDVWBinary(w)
	case DocValuesTypeSorted:
		pf.docValuesWriter = newDVWSorted(NewSortedDocValuesWriter(registered, c.bytesUsed, c.docValuesBytePool))
	case DocValuesTypeSortedNumeric:
		pf.docValuesWriter = newDVWSortedNumeric(NewSortedNumericDocValuesWriter(registered, c.bytesUsed))
	case DocValuesTypeSortedSet:
		pf.docValuesWriter = newDVWSortedSet(NewSortedSetDocValuesWriter(registered, c.bytesUsed, c.docValuesBytePool))
	default:
		return fmt.Errorf("indexing chain: unrecognized DocValues type: %v", registered.DocValuesType())
	}

	if registered.PointDimensionCount() != 0 {
		pw, err := NewPointsWriter(c.bytesUsed, registered)
		if err != nil {
			return err
		}
		pf.pointValuesWriter = pw
	}
	if registered.VectorDimension() != 0 {
		vw, err := c.vectorValuesConsumer.AddField(registered)
		if err != nil {
			c.onAbortingException(err)
			return err
		}
		pf.knnFieldVectorsWriter = vw
	}
	return nil
}

// processField indexes one field instance and reports whether it is the first
// (postings-indexed) instance of a unique field within the current document.
func (c *IndexingChain) processField(docID int, field IndexingChainField, pf *indexingPerField) (bool, error) {
	fieldType := field.IndexableFieldType()
	indexedField := false

	// Invert indexed fields.
	if fieldType.IndexOptions() != IndexOptionsNone {
		if pf.first { // first time we see this field in this doc
			if err := pf.invert(docID, field, true); err != nil {
				return false, err
			}
			pf.first = false
			indexedField = true
		} else if err := pf.invert(docID, field, false); err != nil {
			return false, err
		}
	}

	// Add stored fields.
	if fieldType.Stored() {
		if err := c.storedFieldsConsumer.WriteField(pf.fieldInfo, field); err != nil {
			c.onAbortingException(err)
			return false, err
		}
	}

	dvType := fieldType.DocValuesType()
	if dvType != DocValuesTypeNone {
		if err := c.indexDocValue(docID, pf, dvType, field); err != nil {
			return false, err
		}
	}
	if fieldType.PointDimensionCount() != 0 {
		if err := pf.pointValuesWriter.AddPackedValue(docID, util.NewBytesRef(field.BinaryValueBytes())); err != nil {
			return false, err
		}
	}
	if fieldType.VectorDimension() != 0 {
		if err := c.indexVectorValue(docID, pf, field); err != nil {
			return false, err
		}
	}
	return indexedField, nil
}

// getOrAddPerField returns the PerField for fieldName, creating it on first
// sight in the segment.
//
// PORTING NOTE: Lucene's getOrAddPerField has a `reserved` parameter used by
// markAsReserved. ReservedField support depends on DocumentsWriterPerThread
// (not ported), so the parameter is dropped; reserved is always false.
func (c *IndexingChain) getOrAddPerField(fieldName string) *indexingPerField {
	hashPos := stringHashCode(fieldName) & c.hashMask
	pf := c.fieldHash[hashPos]
	for pf != nil && pf.fieldName != fieldName {
		pf = pf.next
	}
	if pf == nil {
		schema := newFieldSchema(fieldName)
		pf = newIndexingPerField(fieldName, c.indexCreatedVersionMajor, schema, c.indexWriterConfig)
		pf.next = c.fieldHash[hashPos]
		c.fieldHash[hashPos] = pf
		c.totalFieldCount++
		// At most 50% load factor.
		if c.totalFieldCount >= len(c.fieldHash)/2 {
			c.rehash()
		}
		if c.totalFieldCount > len(c.fields) {
			newFields := make([]*indexingPerField, util.Oversize(c.totalFieldCount, util.NumBytesObjectRef))
			copy(newFields, c.fields)
			c.fields = newFields
		}
	}
	return pf
}

// getPerField returns the PerField for name, or nil if unseen.
func (c *IndexingChain) getPerField(name string) *indexingPerField {
	hashPos := stringHashCode(name) & c.hashMask
	fp := c.fieldHash[hashPos]
	for fp != nil && fp.fieldName != name {
		fp = fp.next
	}
	return fp
}

// indexDocValue indexes one field's doc value.
func (c *IndexingChain) indexDocValue(docID int, fp *indexingPerField, dvType DocValuesType, field IndexingChainField) error {
	switch dvType {
	case DocValuesTypeNumeric, DocValuesTypeSortedNumeric:
		nv := field.NumericValue()
		if nv == nil {
			return fmt.Errorf("indexing chain: field=%q: null value not allowed", fp.fieldInfo.Name())
		}
		return fp.docValuesWriter.addNumeric(docID, toInt64(nv))
	case DocValuesTypeBinary, DocValuesTypeSorted, DocValuesTypeSortedSet:
		return fp.docValuesWriter.addBinary(docID, util.NewBytesRef(field.BinaryValueBytes()))
	default:
		return fmt.Errorf("indexing chain: unrecognized DocValues type: %v", dvType)
	}
}

// indexVectorValue indexes one field's vector value.
func (c *IndexingChain) indexVectorValue(docID int, pf *indexingPerField, field IndexingChainField) error {
	// GAP: Lucene dispatches on VectorEncoding (BYTE -> []byte, FLOAT32 ->
	// []float32) and unwraps the concrete KnnByteVectorField /
	// KnnFloatVectorField. Gocene has no concrete KNN field types yet, so the
	// raw binary value is forwarded; the handle is type-erased (see
	// KnnFieldVectorsWriterHandle).
	return pf.knnFieldVectorsWriter.AddValue(docID, field.BinaryValueBytes())
}

// RamBytesUsed reports the bytes held by the chain and its child consumers.
func (c *IndexingChain) RamBytesUsed() int64 {
	total := c.bytesUsed.Get()
	if c.storedFieldsConsumer != nil {
		total += c.storedFieldsConsumer.RamBytesUsed()
	}
	if c.termVectorsWriter != nil {
		total += c.termVectorsWriter.RamBytesUsed()
	}
	if c.vectorValuesConsumer != nil {
		total += c.vectorValuesConsumer.RamBytesUsed()
	}
	return total
}

// ---------------------------------------------------------------------------
// PerField
// ---------------------------------------------------------------------------

// indexingPerField holds the indexing state for a single field name within a
// segment. It is the Go port of IndexingChain.PerField.
//
// NOTE: not standalone — it reaches back into the owning IndexingChain for the
// shared termsHash and bytesUsed counter.
type indexingPerField struct {
	fieldName                string
	indexCreatedVersionMajor int
	schema                   *fieldSchema
	fieldInfo                *FieldInfo
	similarity               SimilarityHandle

	invertState       *FieldInvertState
	termsHashPerField *TermsHashPerField

	// docValuesWriter is non-nil if this field ever had doc values.
	docValuesWriter *dvWriterBox
	// pointValuesWriter is non-nil if this field ever had points.
	pointValuesWriter *PointValuesWriter
	// knnFieldVectorsWriter is non-nil if this field ever had vectors.
	knnFieldVectorsWriter KnnFieldVectorsWriterHandle

	// fieldGen tracks when a PerField was first seen in the current document.
	fieldGen int64

	// next chains PerField within a fieldHash bucket.
	next *indexingPerField

	// norms is lazily initialised.
	norms *NormValuesWriter

	first bool // first instance in a document
}

func newIndexingPerField(
	fieldName string,
	indexCreatedVersionMajor int,
	schema *fieldSchema,
	cfg IndexingChainConfig,
) *indexingPerField {
	pf := &indexingPerField{
		fieldName:                fieldName,
		indexCreatedVersionMajor: indexCreatedVersionMajor,
		schema:                   schema,
		fieldGen:                 -1,
	}
	if cfg != nil {
		pf.similarity = cfg.Similarity()
	}
	return pf
}

func (pf *indexingPerField) reset(docID int) {
	pf.first = true
	pf.schema.reset(docID)
}

func (pf *indexingPerField) setFieldInfo(fi *FieldInfo) {
	pf.fieldInfo = fi
}

func (pf *indexingPerField) setInvertState(c *IndexingChain) error {
	pf.invertState = NewFieldInvertState(
		pf.indexCreatedVersionMajor, pf.fieldInfo.Name(), pf.fieldInfo.IndexOptions())
	pf.termsHashPerField = c.termsHash.AddField(pf.invertState, pf.fieldInfo)
	if !pf.fieldInfo.OmitNorms() {
		// Even if no document sets a norm, norms are still written for the segment.
		nw, err := NewNormValuesWriter(pf.fieldInfo, c.bytesUsed)
		if err != nil {
			return err
		}
		pf.norms = nw
	}
	if pf.fieldInfo.HasTermVectors() && c.termVectorsWriter != nil {
		c.termVectorsWriter.SetHasVectors()
	}
	return nil
}

// finish completes the field for one document: computes and stores the norm,
// then finishes the postings.
func (pf *indexingPerField) finish(docID int) error {
	if !pf.fieldInfo.OmitNorms() {
		var normValue int64
		if pf.invertState.Length() == 0 {
			// Field present in the doc but with no indexed tokens: norm is 0.
			normValue = 0
		} else {
			if pf.similarity == nil {
				return fmt.Errorf("indexing chain: no similarity configured for non-empty field %q", pf.fieldName)
			}
			normValue = pf.similarity.ComputeNorm(pf.invertState)
			if normValue == 0 {
				return fmt.Errorf("indexing chain: similarity returned 0 for non-empty field %q", pf.fieldName)
			}
		}
		if err := pf.norms.AddValue(docID, normValue); err != nil {
			return err
		}
	}
	return pf.termsHashPerField.Finish()
}

// invert inverts one field instance for one document. first is true the first
// time this field name is seen in the document.
func (pf *indexingPerField) invert(docID int, field IndexingChainField, first bool) error {
	if first {
		// First time we see this indexed field in this document: reset the
		// inversion counters. The TermsHashPerField holds this same
		// *FieldInvertState pointer, so it must be mutated in place rather
		// than replaced.
		//
		// GAP: Gocene's FieldInvertState has no Reset() and exposes no
		// lastPosition / lastStartOffset / attributeSource fields (the
		// token-stream accounting that invertTokenStream needs). The subset
		// that the ported binary path touches is reset here.
		resetFieldInvertState(pf.invertState)
	}
	switch field.InvertableType() {
	case InvertableTypeBinary:
		return pf.invertTerm(docID, field, first)
	case InvertableTypeTokenStream:
		return pf.invertTokenStream(docID, field, first)
	default:
		return fmt.Errorf("indexing chain: unrecognized invertable type %v", field.InvertableType())
	}
}

// invertTokenStream inverts a tokenized field through its TokenStream.
//
// GAP: Gocene's index.IndexableField does not expose TokenStream(), and the
// analysis TokenStream / attribute pipeline is not wired into the indexing
// chain. The faithful per-token loop — position/offset accounting,
// MAX_POSITION checks, immense-term handling, termsHashPerField.add — is
// deferred to the sprint that ports the analysis bridge. This stub keeps the
// chain compilable and fails loudly so the gap cannot pass silently.
func (pf *indexingPerField) invertTokenStream(_ int, _ IndexingChainField, _ bool) error {
	return fmt.Errorf("indexing chain: token-stream inversion not yet supported (GAP: analysis TokenStream bridge unported)")
}

// invertTerm inverts a single-valued binary field (InvertableType BINARY).
func (pf *indexingPerField) invertTerm(docID int, field IndexingChainField, first bool) error {
	binaryValue := field.BinaryValueBytes()
	if binaryValue == nil {
		return fmt.Errorf("indexing chain: field %s returns BINARY for invertableType and nil for binaryValue, which is illegal",
			field.Name())
	}
	ft := field.IndexableFieldType()
	if ft.Tokenized() ||
		ft.IndexOptions() > IndexOptionsDocsAndFreqs ||
		ft.StoreTermVectorPositions() ||
		ft.StoreTermVectorOffsets() ||
		ft.StoreTermVectorPayloads() {
		return fmt.Errorf("indexing chain: fields that are tokenized or index proximity data must produce a non-null TokenStream, but %s did not",
			field.Name())
	}
	pf.invertState.SetPosition(pf.invertState.Position() + 1)
	pf.invertState.SetLength(pf.invertState.Length() + 1)
	pf.termsHashPerField.Start(field, first)
	newLen, err := addExact(pf.invertState.Length(), 1)
	if err != nil {
		return fmt.Errorf("indexing chain: too many tokens for field %q: %w", field.Name(), err)
	}
	pf.invertState.SetLength(newLen)
	if err := pf.termsHashPerField.Add(util.NewBytesRef(binaryValue), docID); err != nil {
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// FieldSchema
// ---------------------------------------------------------------------------

// fieldSchema is the schema of a field within the current document. It is
// reset per document and, once built, compared against the segment's FieldInfo
// to ensure a field has identical data structures across all documents. It is
// the Go port of IndexingChain.FieldSchema.
type fieldSchema struct {
	name                     string
	docID                    int
	attributes               map[string]string
	omitNorms                bool
	storeTermVector          bool
	indexOptions             IndexOptions
	docValuesType            DocValuesType
	docValuesSkipIndex       DocValuesSkipIndexType
	pointDimensionCount      int
	pointIndexDimensionCount int
	pointNumBytes            int
	vectorDimension          int
	vectorEncoding           VectorEncoding
	vectorSimilarityFunction VectorSimilarityFunction
}

const fieldSchemaErrMsg = "Inconsistency of field data structures across documents for field "

func newFieldSchema(name string) *fieldSchema {
	return &fieldSchema{
		name:                     name,
		attributes:               make(map[string]string),
		indexOptions:             IndexOptionsNone,
		docValuesType:            DocValuesTypeNone,
		docValuesSkipIndex:       DocValuesSkipIndexTypeNone,
		vectorEncoding:           VectorEncodingFloat32,
		vectorSimilarityFunction: VectorSimilarityFunctionEuclidean,
	}
}

func (s *fieldSchema) raiseNotSame(label string, expected, given any) error {
	return fmt.Errorf("%s[%s] of doc [%d]. %s: expected '%v', but it has '%v'.",
		fieldSchemaErrMsg, s.name, s.docID, label, expected, given)
}

func (s *fieldSchema) assertSameBool(label string, expected, given bool) error {
	if expected != given {
		return s.raiseNotSame(label, expected, given)
	}
	return nil
}

func (s *fieldSchema) assertSameInt(label string, expected, given int) error {
	if expected != given {
		return s.raiseNotSame(label, expected, given)
	}
	return nil
}

func (s *fieldSchema) updateAttributes(attrs map[string]string) {
	for k, v := range attrs {
		s.attributes[k] = v
	}
}

func (s *fieldSchema) setIndexOptions(newIndexOptions IndexOptions, newOmitNorms, newStoreTermVector bool) error {
	if s.indexOptions == IndexOptionsNone {
		s.indexOptions = newIndexOptions
		s.omitNorms = newOmitNorms
		s.storeTermVector = newStoreTermVector
		return nil
	}
	if s.indexOptions != newIndexOptions {
		return s.raiseNotSame("index options", s.indexOptions, newIndexOptions)
	}
	if err := s.assertSameBool("omit norms", s.omitNorms, newOmitNorms); err != nil {
		return err
	}
	return s.assertSameBool("store term vector", s.storeTermVector, newStoreTermVector)
}

func (s *fieldSchema) setDocValues(newDocValuesType DocValuesType, newDocValuesSkipIndex DocValuesSkipIndexType) error {
	if s.docValuesType == DocValuesTypeNone {
		s.docValuesType = newDocValuesType
		s.docValuesSkipIndex = newDocValuesSkipIndex
		return nil
	}
	if s.docValuesType != newDocValuesType {
		return s.raiseNotSame("doc values type", s.docValuesType, newDocValuesType)
	}
	if s.docValuesSkipIndex != newDocValuesSkipIndex {
		return s.raiseNotSame("doc values skip index type", s.docValuesSkipIndex, newDocValuesSkipIndex)
	}
	return nil
}

func (s *fieldSchema) setPoints(dimensionCount, indexDimensionCount, numBytes int) error {
	if s.pointIndexDimensionCount == 0 {
		s.pointDimensionCount = dimensionCount
		s.pointIndexDimensionCount = indexDimensionCount
		s.pointNumBytes = numBytes
		return nil
	}
	if err := s.assertSameInt("point dimension", s.pointDimensionCount, dimensionCount); err != nil {
		return err
	}
	if err := s.assertSameInt("point index dimension", s.pointIndexDimensionCount, indexDimensionCount); err != nil {
		return err
	}
	return s.assertSameInt("point num bytes", s.pointNumBytes, numBytes)
}

func (s *fieldSchema) setVectors(encoding VectorEncoding, similarityFunction VectorSimilarityFunction, dimension int) error {
	if s.vectorDimension == 0 {
		s.vectorEncoding = encoding
		s.vectorSimilarityFunction = similarityFunction
		s.vectorDimension = dimension
		return nil
	}
	if s.vectorEncoding != encoding {
		return s.raiseNotSame("vector encoding", s.vectorEncoding, encoding)
	}
	if s.vectorSimilarityFunction != similarityFunction {
		return s.raiseNotSame("vector similarity function", s.vectorSimilarityFunction, similarityFunction)
	}
	return s.assertSameInt("vector dimension", s.vectorDimension, dimension)
}

func (s *fieldSchema) reset(doc int) {
	s.docID = doc
	s.omitNorms = false
	s.storeTermVector = false
	s.indexOptions = IndexOptionsNone
	s.docValuesType = DocValuesTypeNone
	s.pointDimensionCount = 0
	s.pointIndexDimensionCount = 0
	s.pointNumBytes = 0
	s.vectorDimension = 0
	s.vectorEncoding = VectorEncodingFloat32
	s.vectorSimilarityFunction = VectorSimilarityFunctionEuclidean
}

func (s *fieldSchema) assertSameSchema(fi *FieldInfo) error {
	if fi.IndexOptions() != s.indexOptions {
		return s.raiseNotSame("index options", fi.IndexOptions(), s.indexOptions)
	}
	if err := s.assertSameBool("omit norms", fi.OmitNorms(), s.omitNorms); err != nil {
		return err
	}
	if err := s.assertSameBool("store term vector", fi.HasTermVectors(), s.storeTermVector); err != nil {
		return err
	}
	if fi.DocValuesType() != s.docValuesType {
		return s.raiseNotSame("doc values type", fi.DocValuesType(), s.docValuesType)
	}
	if fi.DocValuesSkipIndexType() != s.docValuesSkipIndex {
		return s.raiseNotSame("doc values skip index type", fi.DocValuesSkipIndexType(), s.docValuesSkipIndex)
	}
	if fi.VectorSimilarityFunction() != s.vectorSimilarityFunction {
		return s.raiseNotSame("vector similarity function", fi.VectorSimilarityFunction(), s.vectorSimilarityFunction)
	}
	if fi.VectorEncoding() != s.vectorEncoding {
		return s.raiseNotSame("vector encoding", fi.VectorEncoding(), s.vectorEncoding)
	}
	if err := s.assertSameInt("vector dimension", fi.VectorDimension(), s.vectorDimension); err != nil {
		return err
	}
	if err := s.assertSameInt("point dimension", fi.PointDimensionCount(), s.pointDimensionCount); err != nil {
		return err
	}
	if err := s.assertSameInt("point index dimension", fi.PointIndexDimensionCount(), s.pointIndexDimensionCount); err != nil {
		return err
	}
	return s.assertSameInt("point num bytes", fi.PointNumBytes(), s.pointNumBytes)
}

// updateDocFieldSchema updates a field schema with the options seen in one
// document's instance of the field.
func updateDocFieldSchema(fieldName string, schema *fieldSchema, fieldType IndexableFieldType) error {
	if fieldType.IndexOptions() != IndexOptionsNone {
		if err := schema.setIndexOptions(
			fieldType.IndexOptions(), fieldType.OmitNorms(), fieldType.StoreTermVectors()); err != nil {
			return err
		}
	} else if err := verifyUnIndexedFieldType(fieldName, fieldType); err != nil {
		return err
	}
	if fieldType.DocValuesType() != DocValuesTypeNone {
		if err := schema.setDocValues(
			fieldType.DocValuesType(), fieldType.DocValuesSkipIndexType()); err != nil {
			return err
		}
	} else if fieldType.DocValuesSkipIndexType() != DocValuesSkipIndexTypeNone {
		return fmt.Errorf("indexing chain: field '%s' cannot have docValuesSkipIndexType=%v without doc values",
			schema.name, fieldType.DocValuesSkipIndexType())
	}
	if fieldType.PointDimensionCount() != 0 {
		if err := schema.setPoints(
			fieldType.PointDimensionCount(),
			fieldType.PointIndexDimensionCount(),
			fieldType.PointNumBytes()); err != nil {
			return err
		}
	}
	if fieldType.VectorDimension() != 0 {
		if err := schema.setVectors(
			fieldType.VectorEncoding(),
			fieldType.VectorSimilarityFunction(),
			fieldType.VectorDimension()); err != nil {
			return err
		}
	}
	if attrs := fieldType.GetAttributes(); len(attrs) != 0 {
		schema.updateAttributes(attrs)
	}
	return nil
}

// verifyUnIndexedFieldType rejects term-vector options on an unindexed field.
func verifyUnIndexedFieldType(name string, ft IndexableFieldType) error {
	if ft.StoreTermVectors() {
		return fmt.Errorf("indexing chain: cannot store term vectors for a field that is not indexed (field=%q)", name)
	}
	if ft.StoreTermVectorPositions() {
		return fmt.Errorf("indexing chain: cannot store term vector positions for a field that is not indexed (field=%q)", name)
	}
	if ft.StoreTermVectorOffsets() {
		return fmt.Errorf("indexing chain: cannot store term vector offsets for a field that is not indexed (field=%q)", name)
	}
	if ft.StoreTermVectorPayloads() {
		return fmt.Errorf("indexing chain: cannot store term vector payloads for a field that is not indexed (field=%q)", name)
	}
	return nil
}

// ---------------------------------------------------------------------------
// dvWriterBox: a tagged union over the five concrete DocValues writers.
//
// PORTING NOTE: Lucene casts a single DocValuesWriter<?> reference. Gocene's
// five concrete DV writers do NOT share a common Go interface (their
// GetDocValues signatures diverge — some return an error, some do not — see
// numeric_doc_values_writer.go vs binary_doc_values_writer.go). A tagged box
// keeps the indexing chain typed without forcing a premature interface
// refactor of the writers, which is out of scope for GOC-3392.
// ---------------------------------------------------------------------------

type dvWriterBox struct {
	numeric       *NumericDocValuesWriter
	binary        *BinaryDocValuesWriter
	sorted        *SortedDocValuesWriter
	sortedNumeric *SortedNumericDocValuesWriter
	sortedSet     *SortedSetDocValuesWriter
}

func newDVWNumeric(w *NumericDocValuesWriter) *dvWriterBox { return &dvWriterBox{numeric: w} }
func newDVWBinary(w *BinaryDocValuesWriter) *dvWriterBox   { return &dvWriterBox{binary: w} }
func newDVWSorted(w *SortedDocValuesWriter) *dvWriterBox   { return &dvWriterBox{sorted: w} }
func newDVWSortedNumeric(w *SortedNumericDocValuesWriter) *dvWriterBox {
	return &dvWriterBox{sortedNumeric: w}
}
func newDVWSortedSet(w *SortedSetDocValuesWriter) *dvWriterBox { return &dvWriterBox{sortedSet: w} }

func (b *dvWriterBox) addNumeric(docID int, value int64) error {
	switch {
	case b.numeric != nil:
		return b.numeric.AddValue(docID, value)
	case b.sortedNumeric != nil:
		return b.sortedNumeric.AddValue(docID, value)
	}
	return nil
}

func (b *dvWriterBox) addBinary(docID int, value *util.BytesRef) error {
	switch {
	case b.binary != nil:
		return b.binary.AddValue(docID, value)
	case b.sorted != nil:
		return b.sorted.AddValue(docID, value)
	case b.sortedSet != nil:
		return b.sortedSet.AddValue(docID, value)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

// stringHashCode reproduces java.lang.String.hashCode so field bucketing
// matches Lucene exactly.
func stringHashCode(s string) int {
	var h int32
	for _, r := range s {
		h = 31*h + int32(r)
	}
	return int(h)
}

// resetFieldInvertState zeroes the inversion counters of a FieldInvertState,
// mirroring FieldInvertState.reset() for the subset of fields Gocene exposes.
//
// GAP: Lucene's reset() also clears lastPosition, lastStartOffset and the
// attributeSource. Gocene's FieldInvertState does not model those yet, so they
// are not reset; they are only needed by invertTokenStream, which is deferred.
func resetFieldInvertState(s *FieldInvertState) {
	s.SetPosition(0)
	s.SetLength(0)
	s.SetNumOverlap(0)
	s.SetOffset(0)
	s.SetMaxTermFrequency(0)
	s.SetUniqueTermCount(0)
}

// toInt64 converts a numeric IndexableField value to int64, mirroring
// java.lang.Number.longValue().
func toInt64(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	case float32:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

// sortPerFieldsByName sorts PerField slices by field name. PerField is
// Comparable<PerField> in Lucene; Gocene exposes it as a helper since Go has
// no built-in ordering of struct pointers.
func sortPerFieldsByName(pfs []*indexingPerField) {
	sort.Slice(pfs, func(i, j int) bool { return pfs[i].fieldName < pfs[j].fieldName })
}
