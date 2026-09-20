// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index/column"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// IndexingChain is the default general-purpose indexing chain, which handles
// indexing all types of fields. It is the Go port of Apache Lucene 10.4.0's
// org.apache.lucene.index.IndexingChain.
//
// PORTING NOTE:
//
// The constructor builds every consumer the chain owns, exactly as Lucene's
// does; see NewIndexingChain. The remaining collaborators below are modelled
// as narrow interfaces declared in this file rather than as concrete types,
// because package search imports package index and so cannot be imported
// back. When the real implementations land, swap the interface for the
// concrete type; the orchestration logic does not change.
//
//   - StoredFieldsConsumer  -> StoredFieldsConsumerHandle (the interface is
//     the Go rendering of Lucene's StoredFieldsConsumer-typed field, which
//     holds either a StoredFieldsConsumer or a SortingStoredFieldsConsumer;
//     both concrete Gocene types satisfy it).
//   - VectorValuesConsumer  -> VectorValuesConsumerHandle (satisfied by the
//     package-private vectorValuesConsumer).
//   - FieldInfos.Builder    -> FieldInfosBuilderHandle (Gocene's existing
//     FieldInfosBuilder is a simple fluent builder lacking add()-with-global-
//     consistency-check, finish(), getSoftDeletesFieldName(),
//     getParentFieldName()).
//   - org.apache.lucene.search.similarities.Similarity -> SimilarityHandle
//     (the real Similarity lives in package search, which imports index).
//   - IndexableField     -> IndexingChainField (a readability alias; the
//     underlying document.IndexableField already carries the full surface).
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
	// sharedIndexingScratch holds the lazily-allocated scratch buffers
	// handed to per-field writers during indexing.
	sharedIndexingScratch *sharedIndexingScratch
	// storedFieldsConsumer writes stored fields.
	storedFieldsConsumer StoredFieldsConsumerHandle
	vectorValuesConsumer VectorValuesConsumerHandle
	// termVectorsWriter is the term-vectors consumer. When the segment is
	// sorted it is the TermVectorsConsumer embedded in the
	// SortingTermVectorsConsumer -- the same object Java's
	// TermVectorsConsumer-typed field holds, reached through the base
	// pointer because Go has no upcast. The chain link that carries the
	// polymorphic calls is the nextTermsHash inside termsHash.
	termVectorsWriter *TermVectorsConsumer

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

	indexWriterConfig         *LiveIndexWriterConfig
	indexCreatedVersionMajor  int
	abortingExceptionConsumer func(error)

	// parentPf and parentField carry the configured parent field, which
	// the constructor pre-registers. Mirrors IndexingChain's final
	// PerField parentPf / NumericDocValuesField parentField.
	parentPf    *indexingPerField
	parentField *document.NumericDocValuesField

	hasHitAbortingException bool
}

// IndexingChainConfig is the subset of Lucene's LiveIndexWriterConfig that the
// indexing chain consumes. See the PORTING NOTE on IndexingChain.
type IndexingChainConfig interface {
	// GetIndexSort returns the configured index sort, or nil when the index
	// is unsorted. Lucene: LiveIndexWriterConfig.getIndexSort().
	GetIndexSort() any
	// GetSimilarity returns the similarity used to compute norms.
	// Lucene: LiveIndexWriterConfig.getSimilarity().
	GetSimilarity() Similarity
	// GetSoftDeletesField returns the configured soft-deletes field name, or
	// "" when soft deletes are disabled.
	// Lucene: LiveIndexWriterConfig.getSoftDeletesField().
	GetSoftDeletesField() string
	// GetParentField returns the configured parent field name, or "" when no
	// parent field is configured.
	// Lucene: LiveIndexWriterConfig.getParentField().
	GetParentField() string
}

// SimilarityHandle is the similarity contract the indexing chain consumes to
// compute per-field norms. index.Similarity already exposes exactly
// Similarity.computeNorm(FieldInvertState), so this is a readability alias.
type SimilarityHandle = Similarity

// FieldInfosBuilderHandle is the subset of Lucene's FieldInfos.Builder that the
// indexing chain consumes. Gocene's existing FieldInfosBuilder does not expose
// these semantics, so the chain depends on this interface instead.
type FieldInfosBuilderHandle interface {
	// Add registers (or merges) a FieldInfo and returns the canonical
	// FieldInfo for the segment, after global-consistency checks.
	// Lucene: FieldInfos.Builder.add(FieldInfo).
	Add(fi *FieldInfo) *FieldInfo
	// Build materialises the final FieldInfos for the segment.
	// Lucene: FieldInfos.Builder.finish().
	Build() *FieldInfos
}

// StoredFieldsConsumerHandle is the subset of Lucene's StoredFieldsConsumer
// consumed by the indexing chain.
type StoredFieldsConsumerHandle interface {
	StartDocument(docID int) error
	WriteField(fi *FieldInfo, value *StoredValue) error
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

// IndexingChainField is the field contract consumed by the indexing chain.
//
// Lucene's IndexingChain consumes org.apache.lucene.index.IndexableField
// directly. index.IndexableField already carries the full upstream surface
// (FieldType, BinaryValue, InvertableType, TokenStream, StoredValue), so the
// name is kept only as a readability alias for the chain's own signatures.
type IndexingChainField = IndexableField

// NewIndexingChain constructs an IndexingChain and, with it, every consumer
// the chain owns. It is the Go port of
//
//	IndexingChain(int indexCreatedVersionMajor,
//	              SegmentInfo segmentInfo,
//	              Directory directory,
//	              FieldInfos.Builder fieldInfos,
//	              LiveIndexWriterConfig indexWriterConfig,
//	              Consumer<Throwable> abortingExceptionConsumer)
//
// and follows its body statement for statement:
//
//   - the byte blocks come from a ByteBlockPool.DirectTrackingAllocator and
//     the int blocks from the nested IntBlockAllocator, both charged to the
//     chain's own bytesUsed counter;
//   - the vector consumer is always the plain VectorValuesConsumer;
//   - when segmentInfo.getIndexSort() is null the chain takes the plain
//     StoredFieldsConsumer and TermVectorsConsumer; otherwise it takes
//     SortingStoredFieldsConsumer and SortingTermVectorsConsumer;
//   - the terms hash is always a FreqProxTermsWriter wrapping the chosen
//     term-vectors consumer;
//   - docValuesBytePool shares the tracking byte allocator;
//   - sharedIndexingScratch is charged to the same counter;
//   - when a parent field is configured the chain pre-registers its PerField
//     and schema.
//
// Lucene asserts segmentInfo.getIndexSort() == indexWriterConfig.getIndexSort();
// the port returns that as an error instead, since Go has no assertions.
func NewIndexingChain(
	indexCreatedVersionMajor int,
	segmentInfo *SegmentInfo,
	directory store.Directory,
	fieldInfos FieldInfosBuilderHandle,
	indexWriterConfig *LiveIndexWriterConfig,
	abortingExceptionConsumer func(error),
) (*IndexingChain, error) {
	if abortingExceptionConsumer == nil {
		return nil, fmt.Errorf("indexing chain: abortingExceptionConsumer must not be nil")
	}
	if fieldInfos == nil {
		return nil, fmt.Errorf("indexing chain: fieldInfos must not be nil")
	}
	if segmentInfo == nil {
		return nil, fmt.Errorf("indexing chain: segmentInfo must not be nil")
	}
	if indexWriterConfig == nil {
		return nil, fmt.Errorf("indexing chain: indexWriterConfig must not be nil")
	}

	c := &IndexingChain{
		bytesUsed:                 util.NewCounter(),
		fieldInfos:                fieldInfos,
		indexWriterConfig:         indexWriterConfig,
		indexCreatedVersionMajor:  indexCreatedVersionMajor,
		abortingExceptionConsumer: abortingExceptionConsumer,
		fieldHash:                 make([]*indexingPerField, 2),
		hashMask:                  1,
		fields:                    make([]*indexingPerField, 1),
		docFields:                 make([]*indexingPerField, 2),
	}

	byteBlockAllocator := util.NewDirectTrackingAllocator(c.bytesUsed)
	intBlockAllocator := newIntBlockAllocator(c.bytesUsed)

	// assert segmentInfo.getIndexSort() == indexWriterConfig.getIndexSort();
	//
	// LiveIndexWriterConfig.GetIndexSort returns any (search.Sort lives in a
	// package that imports index), so the configured value is narrowed to
	// *spi.Sort first: comparing a typed nil *spi.Sort against a nil any
	// directly is never equal in Go and would reject every unsorted segment.
	configuredSort, _ := indexWriterConfig.GetIndexSort().(*spi.Sort)
	if segmentInfo.IndexSort() != configuredSort {
		return nil, fmt.Errorf("indexing chain: segment index sort differs from the configured index sort")
	}

	c.vectorValuesConsumer = newVectorValuesConsumer(
		indexWriterConfig.GetCodec(), directory, segmentInfo, indexWriterConfig.GetInfoStream())

	// termVectors is the chain link Java passes to FreqProxTermsWriter; it
	// carries the dynamic type so flush/abort/addField dispatch to the
	// sorting override when there is one.
	var termVectors TermsHash
	codec := indexWriterConfig.GetCodec()
	if segmentInfo.IndexSort() == nil {
		c.storedFieldsConsumer = NewStoredFieldsConsumer(codec, directory, segmentInfo)
		plain := NewTermVectorsConsumer(
			intBlockAllocator, byteBlockAllocator, directory, segmentInfo, codec)
		c.termVectorsWriter = plain
		termVectors = plain
	} else {
		c.storedFieldsConsumer = NewSortingStoredFieldsConsumer(codec, directory, segmentInfo)
		sorting := NewSortingTermVectorsConsumer(
			intBlockAllocator, byteBlockAllocator, directory, segmentInfo, codec)
		c.termVectorsWriter = sorting.TermVectorsConsumer
		termVectors = sorting
	}

	termsHash, err := NewFreqProxTermsWriter(
		intBlockAllocator, byteBlockAllocator, c.bytesUsed, termVectors)
	if err != nil {
		return nil, err
	}
	c.termsHash = termsHash

	c.docValuesBytePool = util.NewByteBlockPool(byteBlockAllocator)
	c.sharedIndexingScratch = newSharedIndexingScratch(c.bytesUsed)

	if indexWriterConfig.GetParentField() != "" {
		parentField, err := document.NewNumericDocValuesField(indexWriterConfig.GetParentField(), -1)
		if err != nil {
			return nil, fmt.Errorf("indexing chain: parent field: %w", err)
		}
		c.parentField = parentField
		c.parentPf = c.getOrAddPerField(parentField.Name())
		if err := updateDocFieldSchema(parentField.Name(), c.parentPf.schema, parentField.FieldType()); err != nil {
			return nil, fmt.Errorf("indexing chain: parent field schema: %w", err)
		}
	}

	// The nil checks below hold by construction; they stay because they are
	// the contract every caller of this constructor relies on.
	if c.termsHash == nil {
		return nil, fmt.Errorf("indexing chain: termsHash must not be nil")
	}
	if c.storedFieldsConsumer == nil {
		return nil, fmt.Errorf("indexing chain: storedFieldsConsumer must not be nil")
	}
	if c.vectorValuesConsumer == nil {
		return nil, fmt.Errorf("indexing chain: vectorValuesConsumer must not be nil")
	}
	return c, nil
}

// intBlockAllocator is the Go port of the private static nested class
// IndexingChain.IntBlockAllocator: an IntBlockPool allocator that charges
// every block it hands out, and credits every block it takes back, to the
// chain's shared bytes counter.
type intBlockAllocator struct {
	bytesUsed *util.Counter
}

// newIntBlockAllocator mirrors IntBlockAllocator(Counter bytesUsed), whose
// super call fixes the block size at IntBlockPool.INT_BLOCK_SIZE.
func newIntBlockAllocator(bytesUsed *util.Counter) *intBlockAllocator {
	return &intBlockAllocator{bytesUsed: bytesUsed}
}

// GetIntBlock allocates another int block from the shared pool. Mirrors
// IntBlockAllocator.getIntBlock().
func (a *intBlockAllocator) GetIntBlock() []int32 {
	b := make([]int32, util.IntBlockSize)
	a.bytesUsed.AddAndGet(int64(util.IntBlockSize) * 4)
	return b
}

// RecycleIntBlocks credits the recycled blocks back to the counter.
//
// Mirrors IntBlockAllocator.recycleIntBlocks literally, including its
// quirk: the abstract contract is recycleIntBlocks(blocks, start, end) --
// IntBlockPool.reset calls it with (offset, 1 + bufferUpto) -- but the
// override names the second bound "length" and credits
// -(length * INT_BLOCK_SIZE * Integer.BYTES), i.e. it charges back the
// END bound, not the count of recycled blocks. Lucene's arithmetic is
// reproduced as written; "fixing" it here would be a divergence.
func (a *intBlockAllocator) RecycleIntBlocks(blocks [][]int32, start, end int) {
	a.bytesUsed.AddAndGet(-(int64(end) * int64(util.IntBlockSize) * 4))
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
	if c.indexWriterConfig == nil || c.indexWriterConfig.GetIndexSort() == nil {
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
		fieldType := field.FieldType()
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

	if ierr := c.initAndValidateFields(fieldCount); ierr != nil {
		return ierr
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

// initAndValidateFields initialises the FieldInfo of every field seen for the
// first time in this segment and, for fields already known, verifies that the
// schema accumulated for the current document (or batch) matches the schema
// recorded in the index. Mirrors IndexingChain.initAndValidateFields(int).
func (c *IndexingChain) initAndValidateFields(fieldCount int) error {
	for i := 0; i < fieldCount; i++ {
		pf := c.fields[i]
		if pf.fieldInfo == nil {
			if err := c.initializeFieldInfo(pf); err != nil {
				return err
			}
		} else if err := pf.schema.assertSameSchema(pf.fieldInfo); err != nil {
			return err
		}
	}
	return nil
}

// ProcessBatch processes a column-oriented batch of documents: it iterates the
// batch's columns, validates each column against its field type, accumulates
// each field's schema and initialises or verifies the corresponding FieldInfo.
//
// baseDocID is the segment-level doc id of the first document in the batch, so
// batch-local doc 0 maps to baseDocID.
//
// Port of IndexingChain.processBatch(int, ColumnBatch) from Apache Lucene
// 10.5.0.
//
// GAP: the two value-bearing passes of the Java original — the row-oriented
// pass (stored fields and term inversion, driven by ColumnFieldAdapter) and the
// column-oriented pass (doc values, points and vectors, driven by the per-column
// cursors) — cannot be ported yet: Gocene's index/column package declares the
// Column hierarchy but none of the value accessors Lucene reads through
// (LongColumn.tuples/values, BinaryColumn.values, DictionaryColumn.ordinals,
// VectorColumn.vectors, TokenStreamColumn.tokenStreams) nor the
// ColumnFieldAdapter / LongValuesCursor / BytesRefValuesCursor /
// ObjectTupleCursor / OrdinalsTupleCursor types they return. Until those land,
// a batch that carries any indexing feature is refused with an explicit error
// rather than silently indexing nothing.
func (c *IndexingChain) ProcessBatch(baseDocID int, columnBatch *column.ColumnBatch) error {
	if columnBatch == nil {
		return fmt.Errorf("indexing chain: columnBatch must not be nil")
	}
	hasRowColumns := false
	batchGen := c.nextFieldGen
	c.nextFieldGen++

	// Iterate the columns in field-name order. Lucene iterates
	// ColumnBatch.columns() in insertion order; Gocene's ColumnBatch keys its
	// columns by field name in a map, whose iteration order is randomised, so
	// the names are sorted to keep the pass deterministic. That keying also
	// makes a batch structurally incapable of carrying two columns for the same
	// field name, so the per-field feature-overlap check below can only ever
	// fire for a field that repeats across calls within the same batch
	// generation; it is kept to mirror the Java contract exactly.
	fieldNames := make([]string, 0, len(columnBatch.Columns))
	for fieldName := range columnBatch.Columns {
		fieldNames = append(fieldNames, fieldName)
	}
	sort.Strings(fieldNames)

	columnIdx := 0
	uniqueFieldCount := 0
	for _, fieldName := range fieldNames {
		col := columnBatch.Columns[fieldName]
		fieldType := col.FieldType()

		column.ValidateColumnHasIndexingFeature(col.Name(), fieldType)

		switch typed := col.(type) {
		case column.BinaryColumn:
			column.ValidateBinaryColumn(typed, fieldType)
		case column.LongColumn:
			column.ValidateLongColumn(typed, fieldType)
		case column.DictionaryColumn:
			column.ValidateDictionaryColumn(typed, fieldType)
		case column.VectorColumn:
			column.ValidateVectorColumn(typed, fieldType)
		case column.TokenStreamColumn:
			column.ValidateTokenStreamColumn(typed, fieldType)
		default:
			return fmt.Errorf("indexing chain: unknown column type: %T", col)
		}

		if fieldType.Stored() || fieldType.IndexOptions() != IndexOptionsNone {
			hasRowColumns = true
		}

		pf := c.getOrAddPerField(col.Name())
		if columnIdx >= len(c.docFields) {
			c.oversizeDocFields()
		}
		c.docFields[columnIdx] = pf
		columnIdx++

		columnFeatures := column.FeatureMask(fieldType)
		if pf.fieldGen != batchGen {
			// First column for this field name in this batch: start a fresh
			// schema and feature set, and collect the field once so its
			// FieldInfo is initialised/validated after the loop.
			pf.fieldGen = batchGen
			pf.columnFeatures = columnFeatures
			pf.schema.reset(baseDocID)
			// getOrAddPerField already grows c.fields to hold totalFieldCount
			// entries, so the slot is guaranteed to exist — the same
			// invariant Lucene relies on.
			c.fields[uniqueFieldCount] = pf
			uniqueFieldCount++
		} else {
			// Each indexing feature must come from a single column for a given
			// field name.
			overlap := pf.columnFeatures & columnFeatures
			if overlap != 0 {
				return fmt.Errorf(
					"indexing chain: ColumnBatch has multiple columns for field %q claiming the same indexing feature %s; each feature may appear in at most one column",
					col.Name(), column.FeatureNames(overlap))
			}
			pf.columnFeatures |= columnFeatures
		}

		if err := updateDocFieldSchema(col.Name(), pf.schema, fieldType); err != nil {
			return err
		}
	}

	// Initialise field infos / validate schemas once per unique field name in
	// the batch.
	if uniqueFieldCount > 0 {
		if err := c.initAndValidateFields(uniqueFieldCount); err != nil {
			return err
		}
	}

	// GAP (see the doc comment): the row-oriented and column-oriented value
	// passes need the unported index/column cursor and adapter cluster. Refuse
	// loudly instead of dropping the batch's values on the floor.
	if uniqueFieldCount > 0 {
		return fmt.Errorf(
			"indexing chain: column batch of %d docs at baseDocID=%d carries %d field(s) (rowOriented=%v) but batch value indexing is not yet supported (GAP: index/column value cursors and ColumnFieldAdapter unported)",
			columnBatch.NumDocs, baseDocID, uniqueFieldCount, hasRowColumns)
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
	// codec.KnnVectorsFormat().GetMaxDimensions(name); the wide
	// spi.KnnVectorsFormat interface now lifted by rmp #4707 does not yet
	// declare GetMaxDimensions, so the upper-bound check is still skipped.

	opts := DefaultFieldInfoOptions()
	opts.IndexOptions = s.indexOptions
	opts.DocValuesType = s.docValuesType
	// index.DocValuesSkipIndexType and schema.DocValuesSkipIndexType are two
	// declarations of the same Lucene enum (identical ordinals NONE=0,
	// RANGE=1, both pinned to org.apache.lucene.index.DocValuesSkipIndexType),
	// so the ordinal-preserving conversion is exact.
	opts.DocValuesSkipIndexType = spi.DocValuesSkipIndexType(s.docValuesSkipIndex)
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
	// Lucene reads these off FieldInfos.Builder (getSoftDeletesFieldName /
	// getParentFieldName), which simply forwards the two names the
	// FieldNumbers registry was constructed with — namely
	// LiveIndexWriterConfig.getSoftDeletesField() and getParentField() (see
	// IndexWriter.getFieldNumberMap). Gocene's spi.FieldInfosBuilder does not
	// expose those forwarders yet, so the chain reads the same two values
	// straight off the live config it already holds.
	if c.indexWriterConfig != nil {
		opts.IsSoftDeletesField = pf.fieldName != "" && pf.fieldName == c.indexWriterConfig.GetSoftDeletesField()
		opts.IsParentField = pf.fieldName != "" && pf.fieldName == c.indexWriterConfig.GetParentField()
	}

	fi := NewFieldInfo(pf.fieldName, -1, opts)
	for k, v := range s.attributes {
		fi.PutAttribute(k, v)
	}
	registered := c.fieldInfos.Add(fi)
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
	fieldType := field.FieldType()
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
		// Mirrors Lucene: StoredValue storedValue = field.storedValue().
		if err := c.storedFieldsConsumer.WriteField(pf.fieldInfo, field.StoredValue()); err != nil {
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
		if err := pf.pointValuesWriter.AddPackedValue(docID, util.NewBytesRef(field.BinaryValue())); err != nil {
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

// getPerField returns the PerField for name, or nil if this field has not been
// seen in the current segment. Mirrors IndexingChain.getPerField(String).
func (c *IndexingChain) getPerField(name string) *indexingPerField {
	hashPos := stringHashCode(name) & c.hashMask
	for fp := c.fieldHash[hashPos]; fp != nil; fp = fp.next {
		if fp.fieldName == name {
			return fp
		}
	}
	return nil
}

// GetHasDocValues returns the doc-values iterator for name, or nil.
func (c *IndexingChain) GetHasDocValues(fieldName string) util.DocIdSetIterator {
	pf := c.getPerField(fieldName)
	if pf == nil || pf.docValuesWriter == nil {
		return nil
	}
	// GAP: The docValuesWriter should provide a DocIdSetIterator of docs that have values.
	// For now, we return nil or a dummy, as the real implementation is deferred.
	return nil
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
		return fp.docValuesWriter.addBinary(docID, util.NewBytesRef(field.BinaryValue()))
	default:
		return fmt.Errorf("indexing chain: unrecognized DocValues type: %v", dvType)
	}
}

// indexVectorValue indexes one field's vector value.
//
// The document-level KNN field types (KnnFloatVectorField /
// KnnByteVectorField) store the vector as a binary value: a BYTE field holds
// the raw bytes, a FLOAT32 field holds the little-endian IEEE-754 encoding
// (document.encodeFloat32Vector). The codec's per-field KnnVectorsWriter
// expects the decoded element type — []float32 for FLOAT32, []byte for BYTE
// — so this method dispatches on the field's VectorEncoding and decodes the
// binary value accordingly, mirroring the BYTE/FLOAT32 split in Lucene's
// IndexingChain.PerField.indexVectorValue.
func (c *IndexingChain) indexVectorValue(docID int, pf *indexingPerField, field IndexingChainField) error {
	raw := field.BinaryValue()
	switch pf.fieldInfo.VectorEncoding() {
	case VectorEncodingFloat32:
		vec, err := decodeFloat32Vector(raw)
		if err != nil {
			return fmt.Errorf("index: field %q: decode float vector: %w", pf.fieldName, err)
		}
		return pf.knnFieldVectorsWriter.AddValue(docID, vec)
	case VectorEncodingByte:
		// BYTE vectors are stored verbatim; forward a defensive copy so the
		// writer owns immutable storage.
		cp := make([]byte, len(raw))
		copy(cp, raw)
		return pf.knnFieldVectorsWriter.AddValue(docID, cp)
	default:
		return fmt.Errorf("index: field %q: unsupported vector encoding %v",
			pf.fieldName, pf.fieldInfo.VectorEncoding())
	}
}

// decodeFloat32Vector decodes a little-endian IEEE-754 float32 vector from
// its binary encoding (the inverse of document.encodeFloat32Vector). It
// errors when the byte length is not a multiple of 4.
func decodeFloat32Vector(b []byte) ([]float32, error) {
	if len(b)%4 != 0 {
		return nil, fmt.Errorf("float vector byte length %d is not a multiple of 4", len(b))
	}
	out := make([]float32, len(b)/4)
	for i := range out {
		bits := uint32(b[i*4]) |
			uint32(b[i*4+1])<<8 |
			uint32(b[i*4+2])<<16 |
			uint32(b[i*4+3])<<24
		out[i] = math.Float32frombits(bits)
	}
	return out, nil
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

	// columnFeatures is the union of the indexing features already claimed for
	// this field by the columns of the batch currently being processed. Mirrors
	// IndexingChain.PerField.columnFeatures; only meaningful while fieldGen
	// equals the current batch generation.
	columnFeatures int

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
		pf.similarity = cfg.GetSimilarity()
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
		if pf.invertState.length == 0 {
			// Field present in the doc but with no indexed tokens: norm is 0.
			normValue = 0
		} else {
			if pf.similarity == nil {
				return fmt.Errorf("indexing chain: no similarity configured for non-empty field %q", pf.fieldName)
			}
			normValue = pf.similarity.ComputeNormFromInvertState(pf.invertState)
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
		pf.invertState.reset()
	}
	// Non-tokenized fields (e.g. StringField) report InvertableTypeTokenStream
	// through the base Field.InvertableType() but must be indexed as a single
	// binary term via invertTerm rather than going through the stubbed
	// invertTokenStream path (which requires the unported analysis bridge).
	// This mirrors Lucene where a non-tokenized field whose value is a string
	// produces a single-token TokenStream internally; Gocene routes it through
	// the binary path instead.
	if field.FieldType().Tokenized() {
		return pf.invertTokenStream(docID, field, first)
	}
	return pf.invertTerm(docID, field, first)
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
	binaryValue := field.BinaryValue()
	if binaryValue == nil {
		return fmt.Errorf("indexing chain: field %s returns BINARY for invertableType and nil for binaryValue, which is illegal",
			field.Name())
	}
	ft := field.FieldType()
	if ft.Tokenized() ||
		ft.IndexOptions() > IndexOptionsDocsAndFreqs ||
		ft.StoreTermVectorPositions() ||
		ft.StoreTermVectorOffsets() ||
		ft.StoreTermVectorPayloads() {
		return fmt.Errorf("indexing chain: fields that are tokenized or index proximity data must produce a non-null TokenStream, but %s did not",
			field.Name())
	}
	pf.invertState.position++
	pf.invertState.length++
	pf.termsHashPerField.Start(field, first)
	newLen, err := addExact(pf.invertState.length, 1)
	if err != nil {
		return fmt.Errorf("indexing chain: too many tokens for field %q: %w", field.Name(), err)
	}
	pf.invertState.SetLength(newLen)

	// Enforce MAX_TERM_LENGTH: refuse terms longer than the maximum allowed
	// bytes, matching Lucene's IndexWriter.  32766 bytes is the default.
	if len(binaryValue) > MAX_TERM_LENGTH {
		return fmt.Errorf("field %q: immense term: bytes can be at most %d in length; got %d",
			field.Name(), MAX_TERM_LENGTH, len(binaryValue))
	}

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
	docValuesSkipIndex       spi.DocValuesSkipIndexType
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
		docValuesSkipIndex:       spi.DocValuesSkipIndexTypeNone,
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

func (s *fieldSchema) setDocValues(newDocValuesType DocValuesType, newDocValuesSkipIndex spi.DocValuesSkipIndexType) error {
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
	if fi.DocValuesSkipIndexType() != spi.DocValuesSkipIndexType(s.docValuesSkipIndex) {
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
func updateDocFieldSchema(fieldName string, schema *fieldSchema, fieldType spi.IndexableFieldType) error {
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
	} else if fieldType.DocValuesSkipIndexType() != spi.DocValuesSkipIndexTypeNone {
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
func verifyUnIndexedFieldType(name string, ft spi.IndexableFieldType) error {
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
