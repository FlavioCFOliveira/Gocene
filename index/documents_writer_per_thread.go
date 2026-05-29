// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocumentsWriterPerThread handles document processing for a single thread.
// Each thread gets its own DWPT to avoid contention during indexing.
//
// This is the Go port of Lucene's org.apache.lucene.index.DocumentsWriterPerThread.
type DocumentsWriterPerThread struct {
	mu sync.RWMutex

	// parent is the DocumentsWriter that owns this DWPT
	parent *DocumentsWriter

	// segmentInfo holds segment information for the segment being built
	segmentInfo *SegmentInfo

	// fieldInfosBuilder builds field info as documents are added
	fieldInfosBuilder *FieldInfosBuilder

	// numDocsInRAM tracks documents in memory for this DWPT
	numDocsInRAM int

	// invertedIndex holds the in-memory postings data
	invertedIndex *InvertedIndex

	// storedFields holds stored field data for each document
	storedFields *StoredFieldsBuffer

	// docValues holds doc values data per field
	docValues map[string]*DocValuesBuffer

	// termVectors holds term vectors per document (if enabled)
	termVectors *TermVectorsBuffer

	// vectorValues holds the buffered KNN vector values per field, in
	// document order. It is populated during ProcessDocument and replayed
	// into the codec's KnnVectorsWriter at flush time (flushKnnVectors).
	// Mirrors the per-field KnnFieldVectorsWriter buffer that Lucene's
	// IndexingChain accumulates before VectorValuesConsumer.flush.
	vectorValues map[string]*VectorValuesBuffer

	// pointValues holds the buffered multi-dimensional point (BKD) values
	// per field, in document order. Populated during ProcessDocument and
	// replayed into the codec's PointsWriter at flush time (flushPoints).
	// Mirrors the per-field PointValuesWriter buffer that Lucene's
	// IndexingChain accumulates before PointsWriter.writeField.
	pointValues map[string]*PointValuesBuffer

	// lastDocID is the last document ID assigned
	lastDocID int

	// flushPending indicates a flush is pending
	flushPending bool

	// bytesUsed estimates memory usage
	bytesUsed int64

	// deleteQueue holds pending delete operations
	pendingDeletes []*Term
}

// InvertedIndex holds the in-memory postings data structure.
// This maps terms to document IDs and positions.
type InvertedIndex struct {
	mu sync.RWMutex

	// fields maps field name to per-field postings
	fields map[string]*FieldPostings

	// numTerms total number of unique terms across all fields
	numTerms int64
}

// FieldPostings holds postings for a single field.
type FieldPostings struct {
	mu sync.RWMutex

	// terms maps term text to posting list
	terms map[string]*Posting

	// fieldInfo is the field info for this field
	fieldInfo *FieldInfo
}

// Posting holds the posting list for a single term.
type Posting struct {
	// docIDs is the list of document IDs containing this term
	docIDs []int

	// freqs is the frequency of this term in each document
	freqs []int

	// positions holds positions for each occurrence (if positions are indexed)
	positions [][]int

	// startOffsets holds start character offsets (if offsets are indexed)
	startOffsets [][]int

	// endOffsets holds end character offsets (if offsets are indexed)
	endOffsets [][]int
}

// StoredFieldsBuffer holds stored field data in memory.
type StoredFieldsBuffer struct {
	mu sync.RWMutex

	// documents holds stored field data per document
	documents []*StoredDocument

	// totalBytes estimates total bytes stored
	totalBytes int64
}

// StoredDocument holds stored fields for a single document.
type StoredDocument struct {
	// fields holds the stored fields for this document
	fields []*StoredField
}

// StoredField represents a single stored field value.
type StoredField struct {
	name         string
	stringValue  string
	binaryValue  []byte
	numericValue interface{}
}

// DocValuesBuffer holds doc values for a field.
type DocValuesBuffer struct {
	mu sync.RWMutex

	// values holds doc values per document
	values []interface{}

	// dvType is the doc values type
	dvType DocValuesType
}

// VectorValuesBuffer holds the buffered KNN vector values for a single
// field, accumulated in document order during indexing and replayed into
// the codec's KnnVectorsWriter at flush time.
//
// Exactly one of floatValues / byteValues is populated per buffer,
// according to encoding. docIDs[i] is the document that supplied
// floatValues[i] (or byteValues[i]); docIDs is strictly increasing because
// ProcessDocument assigns monotonically increasing docIDs and a field is
// recorded at most once per document.
type VectorValuesBuffer struct {
	dimension   int
	encoding    VectorEncoding
	similarity  VectorSimilarityFunction
	docIDs      []int
	floatValues [][]float32
	byteValues  [][]byte
}

// PointValuesBuffer holds the buffered multi-dimensional point (BKD) values
// for a single field, accumulated in document order during indexing and
// replayed into the codec's PointsWriter at flush time.
//
// docIDs[i] is the document that supplied packedValues[i]. A document may
// contribute more than one value for a multi-valued point field, so docIDs is
// non-decreasing (not strictly increasing). dimensionCount /
// indexDimensionCount / bytesPerDim mirror the field's point attributes and
// are used to size the BKD config at flush time.
type PointValuesBuffer struct {
	dimensionCount      int
	indexDimensionCount int
	bytesPerDim         int
	docIDs              []int
	packedValues        [][]byte
}

// TermVectorsBuffer holds term vectors for documents.
type TermVectorsBuffer struct {
	mu sync.RWMutex

	// vectors holds term vectors per document
	// maps docID -> field name -> term vector data
	vectors []map[string]*FieldTermVector
}

// FieldTermVector holds term vector data for a field.
type FieldTermVector struct {
	// terms is the list of terms in this field
	terms []string

	// freqs is the frequency of each term
	freqs []int

	// positions holds positions for each term (if positions are stored)
	positions [][]int

	// startOffsets holds start offsets for each term (if offsets are stored)
	startOffsets [][]int

	// endOffsets holds end offsets for each term (if offsets are stored)
	endOffsets [][]int
}

// NewDocumentsWriterPerThread creates a new DWPT.
func NewDocumentsWriterPerThread(parent *DocumentsWriter) *DocumentsWriterPerThread {
	return &DocumentsWriterPerThread{
		parent:            parent,
		fieldInfosBuilder: NewFieldInfosBuilder(),
		invertedIndex:     NewInvertedIndex(),
		storedFields:      NewStoredFieldsBuffer(),
		docValues:         make(map[string]*DocValuesBuffer),
		termVectors:       NewTermVectorsBuffer(),
		vectorValues:      make(map[string]*VectorValuesBuffer),
		pointValues:       make(map[string]*PointValuesBuffer),
		lastDocID:         -1,
	}
}

// NewInvertedIndex creates a new empty inverted index.
func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		fields: make(map[string]*FieldPostings),
	}
}

// NewStoredFieldsBuffer creates a new empty stored fields buffer.
func NewStoredFieldsBuffer() *StoredFieldsBuffer {
	return &StoredFieldsBuffer{
		documents: make([]*StoredDocument, 0),
	}
}

// NewTermVectorsBuffer creates a new empty term vectors buffer.
func NewTermVectorsBuffer() *TermVectorsBuffer {
	return &TermVectorsBuffer{
		vectors: make([]map[string]*FieldTermVector, 0),
	}
}

// dwptField is a flat duck-type interface for a field accepted by ProcessDocument.
// Rather than nesting a FieldType() call (whose return type varies between
// document.Field and index.IndexableField), all field-type properties are
// promoted to the top level.  This allows both document.Field (which returns
// *document.FieldType from FieldType()) and index.IndexableField (which returns
// index.FieldTypeInterface) to be adapted to a common surface without any
// circular import.
//
// Concrete types do NOT need to implement this interface directly; asDwptField
// constructs a concrete dwptFieldRecord from the field's accessor methods.
type dwptField struct {
	name        string
	stringValue string
	binaryValue []byte
	numericVal  interface{}
	// field-type properties
	isIndexed                bool
	isStored                 bool
	isTokenized              bool
	omitNorms                bool
	indexOptions             IndexOptions
	docValuesType            DocValuesType
	storeTermVectors         bool
	storeTermVectorPositions bool
	storeTermVectorOffsets   bool
	// customTermFreq, when > 0, overrides the default initial TF of 1.
	// Used by fields such as FeatureField that encode a value as TF.
	customTermFreq int
	// Vector (KNN) attributes. hasVector is true when the field carries a
	// KNN vector value (vectorDimension > 0). The per-document value is
	// stored in exactly one of vectorFloatValue / vectorByteValue
	// according to vectorEncoding, mirroring Lucene's
	// IndexingChain.indexVectorValue dispatch on VectorEncoding.
	hasVector        bool
	vectorDimension  int
	vectorEncoding   VectorEncoding
	vectorSimilarity VectorSimilarityFunction
	vectorFloatValue []float32
	vectorByteValue  []byte
	// Point (BKD) attributes. hasPoint is true when the field carries a
	// multi-dimensional point value (pointDimensionCount > 0). The packed
	// per-document value (pointDimensionCount * pointNumBytes bytes) is
	// stored in pointPackedValue. Mirrors Lucene's
	// IndexingChain.indexPoint dispatch on FieldInfo point dimensions.
	hasPoint                 bool
	pointDimensionCount      int
	pointIndexDimensionCount int
	pointNumBytes            int
	pointPackedValue         []byte
}

// omitNormsGetter is a narrow interface satisfied by field types that expose
// OmitNorms (e.g. document.FieldType via its IsOmitNorms() method, and
// document.Field via OmitNorms()). The FieldTypeInterface used by codec-facing
// fields does not include OmitNorms, so we use a separate assertion.
type omitNormsGetter interface {
	OmitNorms() bool
}

// indexableFieldPromoter is satisfied by document.Field via its
// AsIndexableField() method.  Using this intermediate interface lets the index
// package coerce a document.Field — which cannot directly satisfy
// index.IndexableField due to the FieldType() return-type mismatch — into a
// proper IndexableField without importing the document package.
type indexableFieldPromoter interface {
	AsIndexableField() IndexableField
}

// termFrequencyProvider is satisfied by fields (e.g. FeatureField) that need
// a custom initial term frequency instead of the default of 1.
type termFrequencyProvider interface {
	TermFrequency() int
}

// indexFieldTypeProvider is satisfied by fields that expose the codec-facing
// FieldTypeInterface (the legacy index-side IndexableField surface, retained
// for fields that need to advertise indexed/stored/term-vector flags to the
// indexing chain). The unified spi.IndexableField is narrower and does not
// carry FieldType(); this probe restores access without bloating the SPI.
type indexFieldTypeProvider interface {
	FieldType() FieldTypeInterface
}

// vectorFieldTypeProvider is satisfied by a field type that advertises KNN
// vector attributes. document.FieldType (via its
// fieldTypeAsIndexInterface bridge) implements it once a field has been
// configured with SetVectorAttributes; non-vector field types do not.
//
// Probing this optional interface — rather than widening
// FieldTypeInterface — keeps the existing stored-fields-only
// FieldTypeInterface implementers (simpleFieldType, the spatial shape
// types, the stored-fields consumers) unchanged, matching the
// established optional-probe pattern used for OmitNorms and
// TermFrequency above. It is the index-side projection of
// IndexableFieldType.VectorDimension/VectorEncoding/VectorSimilarityFunction.
type vectorFieldTypeProvider interface {
	VectorDimension() int
	VectorEncoding() VectorEncoding
	VectorSimilarityFunction() VectorSimilarityFunction
}

// floatVectorValueProvider is satisfied by document.KnnFloatVectorField via
// its VectorValue() []float32 accessor. It lets the indexing chain pull the
// per-document float vector without importing the document package.
type floatVectorValueProvider interface {
	VectorValue() []float32
}

// byteVectorValueProvider is satisfied by document.KnnByteVectorField via its
// VectorValue() []byte accessor, the byte analogue of
// floatVectorValueProvider.
type byteVectorValueProvider interface {
	VectorValue() []byte
}

// pointFieldTypeProvider is satisfied by a field type that advertises
// multi-dimensional point (BKD) attributes. document.FieldType (via its
// fieldTypeAsIndexInterface bridge) implements it once a field has been
// configured with SetDimensions; non-point field types report 0 dimensions.
//
// Probing this optional interface — rather than widening FieldTypeInterface —
// follows the same pattern as vectorFieldTypeProvider. It is the index-side
// projection of IndexableFieldType.PointDimensionCount/PointIndexDimensionCount
// /PointNumBytes.
type pointFieldTypeProvider interface {
	PointDimensionCount() int
	PointIndexDimensionCount() int
	PointNumBytes() int
}

// pointValueProvider is satisfied by document.Point via its PointValues()
// []byte accessor. It lets the indexing chain pull the per-document packed
// point value without importing the document package.
type pointValueProvider interface {
	PointValues() []byte
}

// asDwptField builds a flat dwptField record from fieldInterface using
// structural type assertions.  It supports two concrete field layouts:
//
//  1. index.IndexableField (codec-facing, FieldType() returns FieldTypeInterface).
//  2. document.Field — coerced via its AsIndexableField() bridge method.
//
// Returns (nil, false) when the field does not expose the minimal surface.
//
// After the SPI unification (rmp #4693) the IndexableField interface no
// longer carries FieldType(); fields that need to advertise codec-facing
// type properties must do so via a separate FieldType() FieldTypeInterface
// method. The original incoming value is preferred over the (possibly
// wrapped) IndexableField projection so that document.Field instances —
// which satisfy spi.IndexableField directly but only expose
// FieldTypeInterface through their explicit AsIndexableField() bridge —
// are routed through the bridge.
func asDwptField(fieldInterface interface{}) (*dwptField, bool) {
	if fieldInterface == nil {
		return nil, false
	}

	// If the value exposes the legacy AsIndexableField() bridge (i.e.
	// document.Field) always prefer it: that wrapper carries the
	// FieldType() FieldTypeInterface accessor that the indexing chain
	// needs to populate dwptField.isStored / .isIndexed / etc. The raw
	// document.Field type also satisfies spi.IndexableField but does
	// not expose FieldTypeInterface, which would leave those flags at
	// their zero value and break the on-disk persistence of stored /
	// indexed / docvalues fields.
	var idxF IndexableField
	if promoter, ok := fieldInterface.(indexableFieldPromoter); ok {
		idxF = promoter.AsIndexableField()
	}
	if idxF == nil {
		// Fall back to the codec-facing path (concrete IndexableField
		// implementations that already expose the narrow SPI surface).
		if v, ok := fieldInterface.(IndexableField); ok {
			idxF = v
		}
	}
	if idxF == nil {
		return nil, false
	}

	f := &dwptField{
		name:        idxF.Name(),
		stringValue: idxF.StringValue(),
		binaryValue: idxF.BinaryValue(),
		numericVal:  idxF.NumericValue(),
	}
	// FieldType() is no longer on the unified spi.IndexableField
	// surface. Probe the IndexableField value first (it is the
	// wrapping value when the field came through AsIndexableField),
	// then the original incoming field.
	var ft FieldTypeInterface
	if ftp, ok := any(idxF).(indexFieldTypeProvider); ok {
		ft = ftp.FieldType()
	} else if ftp, ok := fieldInterface.(indexFieldTypeProvider); ok {
		ft = ftp.FieldType()
	}
	if ft != nil {
		f.isIndexed = ft.IsIndexed()
		f.isStored = ft.IsStored()
		f.isTokenized = ft.IsTokenized()
		f.indexOptions = ft.GetIndexOptions()
		f.docValuesType = ft.GetDocValuesType()
		f.storeTermVectors = ft.StoreTermVectors()
		f.storeTermVectorPositions = ft.StoreTermVectorPositions()
		f.storeTermVectorOffsets = ft.StoreTermVectorOffsets()
	}
	// FieldTypeInterface does not expose OmitNorms, so probe the original
	// field object directly. document.Field satisfies omitNormsGetter.
	if ong, ok := fieldInterface.(omitNormsGetter); ok {
		f.omitNorms = ong.OmitNorms()
	}
	if tfp, ok := fieldInterface.(termFrequencyProvider); ok {
		f.customTermFreq = tfp.TermFrequency()
	}

	// Vector (KNN) attributes. The field type carries the dimension /
	// encoding / similarity (via the optional vectorFieldTypeProvider
	// probe); the per-document value lives on the concrete field
	// (KnnFloatVectorField / KnnByteVectorField). A field is treated as a
	// vector field only when the declared dimension is positive, matching
	// Lucene's FieldInfo.hasVectorValues() contract.
	//
	// The encoding selects which value accessor to consult:
	// floatVectorValueProvider for FLOAT32, byteVectorValueProvider for
	// BYTE. Both accessors are named VectorValue() but differ in return
	// type, so the encoding (not duck-typing alone) disambiguates which
	// concrete field type produced the value.
	if vtp, ok := ft.(vectorFieldTypeProvider); ok && vtp.VectorDimension() > 0 {
		f.hasVector = true
		f.vectorDimension = vtp.VectorDimension()
		f.vectorEncoding = vtp.VectorEncoding()
		f.vectorSimilarity = vtp.VectorSimilarityFunction()
		switch f.vectorEncoding {
		case VectorEncodingByte:
			if bp, ok := fieldInterface.(byteVectorValueProvider); ok {
				f.vectorByteValue = bp.VectorValue()
			}
		default: // VectorEncodingFloat32
			if fp, ok := fieldInterface.(floatVectorValueProvider); ok {
				f.vectorFloatValue = fp.VectorValue()
			}
		}
	}

	// Point (BKD) attributes. The field type carries the dimension count /
	// index-dimension count / bytes-per-dimension (via the optional
	// pointFieldTypeProvider probe); the per-document packed value lives on
	// the concrete field (document.Point, via pointValueProvider). A field is
	// treated as a point field only when the declared dimension count is
	// positive, matching Lucene's FieldInfo.getPointDimensionCount() contract.
	if ptp, ok := ft.(pointFieldTypeProvider); ok && ptp.PointDimensionCount() > 0 {
		f.hasPoint = true
		f.pointDimensionCount = ptp.PointDimensionCount()
		f.pointIndexDimensionCount = ptp.PointIndexDimensionCount()
		if f.pointIndexDimensionCount <= 0 {
			f.pointIndexDimensionCount = f.pointDimensionCount
		}
		f.pointNumBytes = ptp.PointNumBytes()
		// The packed value comes from the concrete document.Point via its
		// PointValues() accessor; fields that encode the packed bytes as the
		// field's binary value (e.g. document.Field produced by
		// Geo3DPoint.ToIndexableFields) expose it through BinaryValue()
		// instead. Prefer the explicit accessor, fall back to the binary
		// value.
		if pp, ok := fieldInterface.(pointValueProvider); ok {
			f.pointPackedValue = pp.PointValues()
		}
		if len(f.pointPackedValue) == 0 {
			f.pointPackedValue = f.binaryValue
		}
	}
	return f, true
}

// ProcessDocument processes a single document.
// This is the main entry point for indexing a document.
func (dwpt *DocumentsWriterPerThread) ProcessDocument(doc Document) error {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()

	// Get analyzer from parent
	analyzer := dwpt.parent.analyzer

	// Assign a document ID
	dwpt.lastDocID++
	docID := dwpt.lastDocID
	dwpt.numDocsInRAM++

	// Track field processing for this document
	storedDoc := &StoredDocument{
		fields: make([]*StoredField, 0),
	}

	// Process each field
	for _, fieldInterface := range doc.GetFields() {
		field, ok := asDwptField(fieldInterface)
		if !ok {
			continue
		}

		fieldName := field.name

		// Build FieldInfoOptions from the flat dwptField properties.
		// IndexOptions: only set for indexed fields; non-indexed fields use None.
		// OmitNorms: propagate from the field type (e.g. StringField sets true).
		// Stored: propagate so FieldInfos.IsStored() reflects the real schema.
		indexOpts := IndexOptionsNone
		if field.isIndexed {
			indexOpts = field.indexOptions
			if indexOpts == IndexOptionsNone {
				indexOpts = IndexOptionsDocsAndFreqsAndPositions
			}
		}
		opts := FieldInfoOptions{
			IndexOptions:             indexOpts,
			StoreTermVectors:         field.storeTermVectors,
			StoreTermVectorPositions: field.storeTermVectorPositions,
			StoreTermVectorOffsets:   field.storeTermVectorOffsets,
			OmitNorms:                field.omitNorms,
			Stored:                   field.isStored,
			DocValuesType:            field.docValuesType,
		}
		// Vector (KNN) attributes: record the dimension / encoding /
		// similarity on the FieldInfo so it is serialised to the .fnm and
		// FieldInfos.HasVectorValues() reports true on reopen, lighting up
		// the codec KnnVectorsReader. Without a positive dimension the
		// codec write/read paths are never engaged. Mirrors how Lucene's
		// IndexingChain.processField sets the vector attributes on the
		// per-field FieldInfo via schema.setVectorDimensions.
		if field.hasVector {
			opts.VectorDimension = field.vectorDimension
			opts.VectorEncoding = field.vectorEncoding
			opts.VectorSimilarityFunction = field.vectorSimilarity
		}
		// Point (BKD) attributes: record the dimension count / index-dimension
		// count / bytes-per-dimension on the FieldInfo so it is serialised to
		// the .fnm and FieldInfos.HasPointValues() reports true on reopen,
		// lighting up the codec PointsReader. Without a positive dimension
		// count the codec point write/read paths are never engaged. Mirrors
		// how Lucene's IndexingChain.processField sets the point attributes on
		// the per-field FieldInfo via schema.setPointDimensions.
		if field.hasPoint {
			opts.PointDimensionCount = field.pointDimensionCount
			opts.PointIndexDimensionCount = field.pointIndexDimensionCount
			opts.PointNumBytes = field.pointNumBytes
		}

		// Get or create FieldInfo
		fieldInfo := dwpt.getOrAddFieldInfo(fieldName, opts)

		// Process based on field type
		if field.isIndexed {
			// Index the field in the inverted index
			if err := dwpt.indexFieldWithValue(docID, fieldName, field.stringValue, field.isTokenized, field.customTermFreq, fieldInfo, analyzer); err != nil {
				return err
			}
		}

		if field.isStored {
			// Add to stored fields
			storedDoc.fields = append(storedDoc.fields, &StoredField{
				name:         fieldName,
				stringValue:  field.stringValue,
				binaryValue:  field.binaryValue,
				numericValue: field.numericVal,
			})
		}

		if field.docValuesType != DocValuesTypeNone {
			// Add to doc values — requires index.IndexableField; adapt if possible.
			if idxField, ok2 := fieldInterface.(IndexableField); ok2 {
				dwpt.addDocValue(fieldName, docID, idxField, field.docValuesType)
			}
		}

		if field.storeTermVectors {
			// Build term vectors — requires index.IndexableField; adapt if possible.
			if idxField, ok2 := fieldInterface.(IndexableField); ok2 {
				dwpt.buildTermVector(docID, fieldName, idxField)
			}
		}

		if field.hasVector {
			// Buffer the per-document vector value for this field. Replayed
			// into the codec KnnVectorsWriter at flush time. Mirrors
			// IndexingChain.indexVectorValue, which routes the value through
			// the per-field KnnFieldVectorsWriter in document order.
			dwpt.addVectorValue(docID, fieldName, field)
		}

		if field.hasPoint && len(field.pointPackedValue) > 0 {
			// Buffer the per-document packed point value for this field.
			// Replayed into the codec PointsWriter (BKD) at flush time.
			// Mirrors IndexingChain.indexPoint, which feeds the value into the
			// field's PointValuesWriter in document order.
			dwpt.addPointValue(docID, fieldName, field)
		}
	}

	// Add stored document to buffer
	dwpt.storedFields.mu.Lock()
	dwpt.storedFields.documents = append(dwpt.storedFields.documents, storedDoc)
	dwpt.storedFields.totalBytes += int64(len(storedDoc.fields) * 64) // Estimate
	dwpt.storedFields.mu.Unlock()

	// Update memory usage estimate
	dwpt.bytesUsed += dwpt.estimateMemoryUsage(doc)

	return nil
}

// getIndexOptions extracts IndexOptions from FieldTypeInterface.
// Kept for callers outside ProcessDocument that still use FieldTypeInterface.
func getIndexOptions(ft FieldTypeInterface) IndexOptions {
	if ft == nil {
		return IndexOptionsDocsAndFreqsAndPositions
	}
	return ft.GetIndexOptions()
}

// getOrAddFieldInfo gets or creates a FieldInfo for a field.
func (dwpt *DocumentsWriterPerThread) getOrAddFieldInfo(fieldName string, opts FieldInfoOptions) *FieldInfo {
	// Check if field already exists
	if fi := dwpt.fieldInfosBuilder.FieldInfos().GetByName(fieldName); fi != nil {
		return fi
	}

	// Create new FieldInfo
	fi := NewFieldInfo(fieldName, dwpt.fieldInfosBuilder.FieldInfos().Size(), opts)
	dwpt.fieldInfosBuilder.Add(fi)
	return fi
}

// indexFieldWithValue indexes a field value in the inverted index.
// customTermFreq, when > 0, overrides the default initial TF of 1 for the
// indexed term (used by FeatureField which encodes a value as term frequency).
func (dwpt *DocumentsWriterPerThread) indexFieldWithValue(
	docID int,
	fieldName string,
	value string,
	tokenized bool,
	customTermFreq int,
	fieldInfo *FieldInfo,
	analyzer analysis.Analyzer,
) error {
	// Get or create field postings
	dwpt.invertedIndex.mu.Lock()
	fieldPostings, exists := dwpt.invertedIndex.fields[fieldName]
	if !exists {
		fieldPostings = &FieldPostings{
			terms:     make(map[string]*Posting),
			fieldInfo: fieldInfo,
		}
		dwpt.invertedIndex.fields[fieldName] = fieldPostings
	}
	dwpt.invertedIndex.mu.Unlock()

	// Tokenize the field value
	var tokens []string
	if tokenized {
		// Use analyzer to tokenize
		if analyzer != nil {
			tokenStream, err := analyzer.TokenStream(fieldName, strings.NewReader(value))
			if err != nil {
				return err
			}
			if tokenStream != nil {
				defer tokenStream.Close()
				for {
					hasNext, err := tokenStream.IncrementToken()
					if err != nil {
						return err
					}
					if !hasNext {
						break
					}
					// Get the term attribute from the token stream
					if attrSrc, ok := tokenStream.(interface {
						GetAttributeSource() *util.AttributeSource
					}); ok {
						if attr := attrSrc.GetAttributeSource().GetAttribute(analysis.CharTermAttributeType); attr != nil {
							if termAttr, ok := attr.(analysis.CharTermAttribute); ok {
								tokens = append(tokens, termAttr.String())
							}
						}
					}
				}
			}
		}
	} else {
		// Use the value directly as a single term
		tokens = []string{value}
	}

	// Add each token to the inverted index
	position := 0
	for _, token := range tokens {
		dwpt.addTermWithFreq(docID, fieldName, token, position, customTermFreq, fieldPostings, fieldInfo)
		position++
	}

	return nil
}

// addTerm adds a term to the inverted index with the default initial TF of 1.
func (dwpt *DocumentsWriterPerThread) addTerm(docID int, fieldName, term string, position int, fieldPostings *FieldPostings, fieldInfo *FieldInfo) {
	dwpt.addTermWithFreq(docID, fieldName, term, position, 0, fieldPostings, fieldInfo)
}

// addTermWithFreq adds a term to the inverted index.
// initialFreq, when > 0, is used as the initial term frequency for a new
// document entry; otherwise 1 is used (the standard Lucene default).
func (dwpt *DocumentsWriterPerThread) addTermWithFreq(docID int, fieldName, term string, position int, initialFreq int, fieldPostings *FieldPostings, fieldInfo *FieldInfo) {
	fieldPostings.mu.Lock()
	defer fieldPostings.mu.Unlock()

	posting, exists := fieldPostings.terms[term]
	if !exists {
		posting = &Posting{
			docIDs:       make([]int, 0),
			freqs:        make([]int, 0),
			positions:    make([][]int, 0),
			startOffsets: make([][]int, 0),
			endOffsets:   make([][]int, 0),
		}
		fieldPostings.terms[term] = posting
		dwpt.invertedIndex.numTerms++
	}

	if initialFreq <= 0 {
		initialFreq = 1
	}

	// Find or add document in posting list
	if len(posting.docIDs) > 0 && posting.docIDs[len(posting.docIDs)-1] == docID {
		// Same document, increment frequency
		idx := len(posting.docIDs) - 1
		posting.freqs[idx]++
		if fieldInfo.IndexOptions().HasPositions() {
			posting.positions[idx] = append(posting.positions[idx], position)
		}
	} else {
		// New document
		posting.docIDs = append(posting.docIDs, docID)
		posting.freqs = append(posting.freqs, initialFreq)
		if fieldInfo.IndexOptions().HasPositions() {
			posting.positions = append(posting.positions, []int{position})
		}
	}
}

// addDocValue adds a doc value for a field.
// Must be called with dwpt.mu held (write lock).
func (dwpt *DocumentsWriterPerThread) addDocValue(fieldName string, docID int, field IndexableField, dvType DocValuesType) {
	buf, exists := dwpt.docValues[fieldName]
	if !exists {
		buf = &DocValuesBuffer{
			values: make([]interface{}, 0),
			dvType: dvType,
		}
		dwpt.docValues[fieldName] = buf
	}

	buf.values = append(buf.values, field.NumericValue())
}

// addVectorValue buffers one document's KNN vector value for fieldName.
// Must be called with dwpt.mu held (write lock). The value type is selected
// by the field's encoding: floatValue for FLOAT32, byteValue for BYTE. A
// field that declared a positive dimension but supplied no value (a
// document without this vector field) is simply not buffered for that doc,
// matching Lucene's sparse-friendly per-field writer which only records
// docs that call addValue.
func (dwpt *DocumentsWriterPerThread) addVectorValue(docID int, fieldName string, field *dwptField) {
	buf, exists := dwpt.vectorValues[fieldName]
	if !exists {
		buf = &VectorValuesBuffer{
			dimension:  field.vectorDimension,
			encoding:   field.vectorEncoding,
			similarity: field.vectorSimilarity,
		}
		dwpt.vectorValues[fieldName] = buf
	}
	switch field.vectorEncoding {
	case VectorEncodingByte:
		if field.vectorByteValue == nil {
			return
		}
		buf.docIDs = append(buf.docIDs, docID)
		buf.byteValues = append(buf.byteValues, field.vectorByteValue)
	default: // VectorEncodingFloat32
		if field.vectorFloatValue == nil {
			return
		}
		buf.docIDs = append(buf.docIDs, docID)
		buf.floatValues = append(buf.floatValues, field.vectorFloatValue)
	}
}

// addPointValue buffers a per-document packed point value for a field,
// allocating the per-field PointValuesBuffer on first use. The packed value is
// copied because the caller's slice (document.Point's binary value) may be
// reused after ProcessDocument returns.
func (dwpt *DocumentsWriterPerThread) addPointValue(docID int, fieldName string, field *dwptField) {
	buf, exists := dwpt.pointValues[fieldName]
	if !exists {
		buf = &PointValuesBuffer{
			dimensionCount:      field.pointDimensionCount,
			indexDimensionCount: field.pointIndexDimensionCount,
			bytesPerDim:         field.pointNumBytes,
		}
		dwpt.pointValues[fieldName] = buf
	}
	// A single point value occupies dimensionCount * bytesPerDim bytes. A
	// multi-valued point field (e.g. document.NewIntPoints) packs N such
	// values back-to-back into one binary value; each is a distinct BKD point
	// for the same document. Split the binary on the per-point stride so the
	// codec sees one packedValue (and one BKDWriter.Add) per value, matching
	// Lucene's IndexableField.tokenStream/PointValuesWriter contract.
	stride := field.pointDimensionCount * field.pointNumBytes
	src := field.pointPackedValue
	if stride <= 0 || len(src)%stride != 0 {
		// Defensive: treat a non-conforming binary as a single opaque value;
		// the codec's BKDWriter.Add will surface the length mismatch.
		packed := make([]byte, len(src))
		copy(packed, src)
		buf.docIDs = append(buf.docIDs, docID)
		buf.packedValues = append(buf.packedValues, packed)
		return
	}
	for off := 0; off < len(src); off += stride {
		packed := make([]byte, stride)
		copy(packed, src[off:off+stride])
		buf.docIDs = append(buf.docIDs, docID)
		buf.packedValues = append(buf.packedValues, packed)
	}
}

// buildTermVector builds term vector data for a field.
func (dwpt *DocumentsWriterPerThread) buildTermVector(docID int, fieldName string, field IndexableField) {
	// Term vectors will be populated from the inverted index during flush
	// This is a placeholder for now
}

// estimateMemoryUsage estimates memory usage for a document.
func (dwpt *DocumentsWriterPerThread) estimateMemoryUsage(doc Document) int64 {
	// Rough estimate: 1KB per field + overhead
	fields := doc.GetFields()
	return int64(len(fields)*1024 + 256)
}

// GetNumDocs returns the number of documents in RAM.
func (dwpt *DocumentsWriterPerThread) GetNumDocs() int {
	dwpt.mu.RLock()
	defer dwpt.mu.RUnlock()
	return dwpt.numDocsInRAM
}

// GetFieldInfos returns the current FieldInfos snapshot for this DWPT.
// Used by IndexWriter.Commit to pass field metadata to the codec flush methods.
func (dwpt *DocumentsWriterPerThread) GetFieldInfos() *FieldInfos {
	dwpt.mu.RLock()
	defer dwpt.mu.RUnlock()
	return dwpt.fieldInfosBuilder.Build()
}

// GetBytesUsed returns the estimated memory usage.
func (dwpt *DocumentsWriterPerThread) GetBytesUsed() int64 {
	dwpt.mu.RLock()
	defer dwpt.mu.RUnlock()
	return dwpt.bytesUsed
}

// Reset resets the DWPT for a new segment.
func (dwpt *DocumentsWriterPerThread) Reset() {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()

	dwpt.numDocsInRAM = 0
	dwpt.lastDocID = -1
	dwpt.bytesUsed = 0
	dwpt.flushPending = false
	dwpt.fieldInfosBuilder = NewFieldInfosBuilder()
	dwpt.invertedIndex = NewInvertedIndex()
	dwpt.storedFields = NewStoredFieldsBuffer()
	dwpt.docValues = make(map[string]*DocValuesBuffer)
	dwpt.termVectors = NewTermVectorsBuffer()
	dwpt.vectorValues = make(map[string]*VectorValuesBuffer)
	dwpt.pointValues = make(map[string]*PointValuesBuffer)
	dwpt.pendingDeletes = nil
}

// PrepareFlush prepares this DWPT for flushing.
// Returns a FlushTicket that can be used to complete the flush.
func (dwpt *DocumentsWriterPerThread) PrepareFlush() (*FlushTicket, error) {
	dwpt.mu.RLock()
	defer dwpt.mu.RUnlock()

	return &FlushTicket{
		numDocs:       dwpt.numDocsInRAM,
		fieldInfos:    dwpt.fieldInfosBuilder.Build(),
		invertedIndex: dwpt.invertedIndex,
		storedFields:  dwpt.storedFields,
		docValues:     dwpt.docValues,
		termVectors:   dwpt.termVectors,
		bytesUsed:     dwpt.bytesUsed,
	}, nil
}

// FlushTicket holds the data needed to flush a segment.
type FlushTicket struct {
	numDocs       int
	fieldInfos    *FieldInfos
	invertedIndex *InvertedIndex
	storedFields  *StoredFieldsBuffer
	docValues     map[string]*DocValuesBuffer
	termVectors   *TermVectorsBuffer
	bytesUsed     int64
}

// Flush flushes the DWPT data to disk.
// Returns the new segment info.
func (dwpt *DocumentsWriterPerThread) Flush(directory store.Directory, codec Codec, segmentName string) (*SegmentInfo, error) {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()

	if dwpt.numDocsInRAM == 0 {
		return nil, nil // Nothing to flush
	}

	// Create segment info
	segmentInfo := NewSegmentInfo(segmentName, dwpt.numDocsInRAM, directory)
	segmentInfo.SetID(generateSegmentID())

	// Build field infos
	fieldInfos := dwpt.fieldInfosBuilder.Build()

	// Create segment write state
	writeState := &SegmentWriteState{
		Directory:     directory,
		SegmentInfo:   segmentInfo,
		FieldInfos:    fieldInfos,
		SegmentSuffix: "",
	}

	// 1. Write stored fields
	if err := dwpt.flushStoredFields(codec, writeState); err != nil {
		return nil, fmt.Errorf("failed to flush stored fields: %w", err)
	}

	// 2. Write postings (inverted index)
	if err := dwpt.flushPostings(codec, writeState, fieldInfos); err != nil {
		return nil, fmt.Errorf("failed to flush postings: %w", err)
	}

	// 3. Write field infos
	if err := dwpt.flushFieldInfos(codec, writeState); err != nil {
		return nil, fmt.Errorf("failed to flush field infos: %w", err)
	}

	// Update segment files list
	segmentInfo.SetFiles(dwpt.getGeneratedFiles(segmentName))

	return segmentInfo, nil
}

// flushStoredFields writes stored fields to disk.
func (dwpt *DocumentsWriterPerThread) flushStoredFields(codec Codec, state *SegmentWriteState) error {
	writer, err := codec.StoredFieldsFormat().FieldsWriter(state.Directory, state.SegmentInfo, store.IOContextWrite)
	if err != nil {
		return err
	}
	defer writer.Close()

	for _, doc := range dwpt.storedFields.documents {
		if err := writer.StartDocument(); err != nil {
			return err
		}
		for _, field := range doc.fields {
			// Convert StoredField to IndexableField adapter
			sf := &storedFieldAdapter{field: field}
			if err := writer.WriteField(sf); err != nil {
				return err
			}
		}
		if err := writer.FinishDocument(); err != nil {
			return err
		}
	}

	return nil
}

// flushPostings writes the inverted index to disk.
func (dwpt *DocumentsWriterPerThread) flushPostings(codec Codec, state *SegmentWriteState, fieldInfos *FieldInfos) error {
	consumer, err := codec.PostingsFormat().FieldsConsumer(state)
	if err != nil {
		return err
	}
	defer consumer.Close()

	// Write each field's postings
	for fieldName, fieldPostings := range dwpt.invertedIndex.fields {
		// Convert to Terms format
		terms := &postingTermsAdapter{
			postings: fieldPostings,
		}
		if err := consumer.Write(fieldName, terms); err != nil {
			return err
		}
	}

	return nil
}

// flushFieldInfos writes field infos to disk via the codec's FieldInfosFormat.
func (dwpt *DocumentsWriterPerThread) flushFieldInfos(codec Codec, state *SegmentWriteState) error {
	fif := codec.FieldInfosFormat()
	if fif == nil {
		return nil
	}
	return fif.Write(state.Directory, state.SegmentInfo, state.SegmentSuffix, state.FieldInfos, store.IOContextWrite)
}

// docTermEntry holds one term's contribution to a document's term vector.
type docTermEntry struct {
	text      string
	positions []int
	startOffs []int
	endOffs   []int
}

// flushTermVectors writes the term vectors for all in-RAM documents to the
// codec's TermVectorsFormat. Inverts the per-field inverted index to produce
// per-document term vector data.
//
// Only invoked when at least one field in fieldInfos has StoreTermVectors=true.
// If the codec does not provide a TermVectorsFormat, the method is a no-op.
func (dwpt *DocumentsWriterPerThread) flushTermVectors(codec Codec, state *SegmentWriteState) error {
	tvFmt := codec.TermVectorsFormat()
	if tvFmt == nil {
		return nil
	}

	// Determine which fields carry term vectors and their per-field options.
	type tvFieldOpts struct {
		fieldInfo    *FieldInfo
		hasPositions bool
		hasOffsets   bool
		hasPayloads  bool
	}
	var tvFields []tvFieldOpts
	it := state.FieldInfos.Iterator()
	for {
		fi := it.Next()
		if fi == nil {
			break
		}
		if fi.StoreTermVectors() {
			tvFields = append(tvFields, tvFieldOpts{
				fieldInfo:    fi,
				hasPositions: fi.StoreTermVectorPositions(),
				hasOffsets:   fi.StoreTermVectorOffsets(),
				hasPayloads:  fi.StoreTermVectorPayloads(),
			})
		}
	}
	if len(tvFields) == 0 {
		return nil // Nothing to write.
	}

	numDocs := dwpt.numDocsInRAM

	// Build per-doc term vector data by inverting the field postings.
	// docVectors[docID] = map[fieldName] -> []docTermEntry
	type docTV map[string][]docTermEntry
	docVectors := make([]docTV, numDocs)
	for i := range docVectors {
		docVectors[i] = make(docTV)
	}

	dwpt.invertedIndex.mu.RLock()
	for _, opts := range tvFields {
		fp, ok := dwpt.invertedIndex.fields[opts.fieldInfo.Name()]
		if !ok {
			continue
		}
		fp.mu.RLock()
		for termText, posting := range fp.terms {
			for di, docID := range posting.docIDs {
				if docID < 0 || docID >= numDocs {
					continue
				}
				entry := docTermEntry{text: termText}
				if opts.hasPositions && di < len(posting.positions) {
					entry.positions = posting.positions[di]
				}
				if opts.hasOffsets && di < len(posting.startOffsets) {
					entry.startOffs = posting.startOffsets[di]
					entry.endOffs = posting.endOffsets[di]
				}
				docVectors[docID][opts.fieldInfo.Name()] = append(
					docVectors[docID][opts.fieldInfo.Name()], entry)
			}
		}
		fp.mu.RUnlock()
	}
	dwpt.invertedIndex.mu.RUnlock()

	// Open the writer and drive the StartDocument / StartField / StartTerm protocol.
	tvWriter, err := tvFmt.VectorsWriter(state)
	if err != nil {
		return fmt.Errorf("term vectors writer: %w", err)
	}
	defer tvWriter.Close()

	for docID := 0; docID < numDocs; docID++ {
		fieldMap := docVectors[docID]

		// Build the list of fields that have at least one term for this doc.
		var activeFields []tvFieldOpts
		for _, opts := range tvFields {
			if len(fieldMap[opts.fieldInfo.Name()]) > 0 {
				activeFields = append(activeFields, opts)
			}
		}

		if err := tvWriter.StartDocument(len(activeFields)); err != nil {
			return fmt.Errorf("term vectors StartDocument doc=%d: %w", docID, err)
		}

		for _, opts := range activeFields {
			entries := fieldMap[opts.fieldInfo.Name()]
			if err := tvWriter.StartField(opts.fieldInfo, len(entries),
				opts.hasPositions, opts.hasOffsets, opts.hasPayloads); err != nil {
				return fmt.Errorf("term vectors StartField doc=%d field=%s: %w",
					docID, opts.fieldInfo.Name(), err)
			}
			for _, entry := range entries {
				if err := tvWriter.StartTerm([]byte(entry.text)); err != nil {
					return fmt.Errorf("term vectors StartTerm doc=%d field=%s term=%s: %w",
						docID, opts.fieldInfo.Name(), entry.text, err)
				}
				// Determine occurrence count.
				occurrences := 1
				if opts.hasPositions && len(entry.positions) > 0 {
					occurrences = len(entry.positions)
				}
				for i := 0; i < occurrences; i++ {
					pos := -1
					so, eo := -1, -1
					if opts.hasPositions && i < len(entry.positions) {
						pos = entry.positions[i]
					}
					if opts.hasOffsets && i < len(entry.startOffs) {
						so = entry.startOffs[i]
						eo = entry.endOffs[i]
					}
					if err := tvWriter.AddPosition(pos, so, eo, nil); err != nil {
						return fmt.Errorf("term vectors AddPosition doc=%d: %w", docID, err)
					}
				}
				if err := tvWriter.FinishTerm(); err != nil {
					return err
				}
			}
			if err := tvWriter.FinishField(); err != nil {
				return err
			}
		}

		if err := tvWriter.FinishDocument(); err != nil {
			return fmt.Errorf("term vectors FinishDocument doc=%d: %w", docID, err)
		}
	}

	return nil
}

// flushKnnVectors writes the buffered KNN vector values for every vector
// field to the codec's KnnVectorsWriter and serialises the per-segment
// HNSW graph plus the flat vectors (.vec / .vex / .vem files).
//
// It mirrors the vectorValuesConsumer.flush step of Lucene's
// IndexingChain.flush: a per-field KnnFieldVectorsWriter is opened for each
// vector field, the buffered per-document values are replayed in increasing
// docID order, and the consumer serialises every field in one shot.
//
// The FieldInfo objects are taken from state.FieldInfos — the same
// instances that flushFieldInfos serialises to the .fnm — so that the
// PerFieldKnnVectorsWriter's PutCodecAttribute calls (format name + suffix)
// are recorded on the FieldInfo that reaches disk. This is why
// flushKnnVectors MUST run before flushFieldInfos.
//
// No-op when the codec has no KnnVectorsFormat or no vector fields were
// buffered.
func (dwpt *DocumentsWriterPerThread) flushKnnVectors(codec Codec, state *SegmentWriteState) error {
	if codec == nil || codec.KnnVectorsFormat() == nil {
		return nil
	}
	if len(dwpt.vectorValues) == 0 {
		return nil
	}

	// Collect the vector fields from the on-disk FieldInfos, preserving
	// field-number order so the AddField sequence (and thus the per-format
	// suffix assignment) is deterministic across runs.
	type vecField struct {
		fieldInfo *FieldInfo
		buf       *VectorValuesBuffer
	}
	var vecFields []vecField
	it := state.FieldInfos.Iterator()
	for {
		fi := it.Next()
		if fi == nil {
			break
		}
		if fi.VectorDimension() <= 0 {
			continue
		}
		buf, ok := dwpt.vectorValues[fi.Name()]
		if !ok {
			continue
		}
		vecFields = append(vecFields, vecField{fieldInfo: fi, buf: buf})
	}
	if len(vecFields) == 0 {
		return nil
	}

	consumer := newVectorValuesConsumer(codec, state.Directory, state.SegmentInfo, util.NoOpInfoStream)

	for _, vf := range vecFields {
		handle, err := consumer.AddField(vf.fieldInfo)
		if err != nil {
			consumer.Abort()
			return fmt.Errorf("knn vectors AddField %q: %w", vf.fieldInfo.Name(), err)
		}
		switch vf.buf.encoding {
		case VectorEncodingByte:
			for i, docID := range vf.buf.docIDs {
				if err := handle.AddValue(docID, vf.buf.byteValues[i]); err != nil {
					consumer.Abort()
					return fmt.Errorf("knn vectors AddValue (byte) field=%q doc=%d: %w",
						vf.fieldInfo.Name(), docID, err)
				}
			}
		default: // VectorEncodingFloat32
			for i, docID := range vf.buf.docIDs {
				if err := handle.AddValue(docID, vf.buf.floatValues[i]); err != nil {
					consumer.Abort()
					return fmt.Errorf("knn vectors AddValue (float) field=%q doc=%d: %w",
						vf.fieldInfo.Name(), docID, err)
				}
			}
		}
	}

	if err := consumer.Flush(state, nil); err != nil {
		return fmt.Errorf("knn vectors flush: %w", err)
	}
	return nil
}

// flushPoints writes the buffered multi-dimensional point (BKD) values for
// every point field to the codec's PointsWriter, serialising the per-segment
// .kdd / .kdi / .kdm files.
//
// It mirrors the writePoints step of Lucene's IndexingChain.flush: a single
// PointsWriter is opened for the segment, WriteField is invoked once per point
// field (pulling the buffered per-document packed values back through an
// in-memory PointsSource in document order), and Finish stamps the trailing
// metadata.
//
// The FieldInfo objects are taken from state.FieldInfos — the same instances
// flushFieldInfos serialises to the .fnm — so the FieldInfo point dimensions
// reach disk and FieldInfos.HasPointValues() reports true on reopen.
//
// No-op when the codec has no PointsFormat or no point fields were buffered.
func (dwpt *DocumentsWriterPerThread) flushPoints(codec Codec, state *SegmentWriteState) error {
	if codec == nil || codec.PointsFormat() == nil {
		return nil
	}
	if len(dwpt.pointValues) == 0 {
		return nil
	}

	// Collect the point fields from the on-disk FieldInfos, preserving
	// field-number order so the WriteField sequence (and thus the per-field
	// meta records) is deterministic across runs.
	type ptField struct {
		fieldInfo *FieldInfo
		buf       *PointValuesBuffer
	}
	var ptFields []ptField
	it := state.FieldInfos.Iterator()
	for {
		fi := it.Next()
		if fi == nil {
			break
		}
		if fi.PointDimensionCount() <= 0 {
			continue
		}
		buf, ok := dwpt.pointValues[fi.Name()]
		if !ok {
			continue
		}
		ptFields = append(ptFields, ptField{fieldInfo: fi, buf: buf})
	}
	if len(ptFields) == 0 {
		return nil
	}

	writer, err := codec.PointsFormat().FieldsWriter(state)
	if err != nil {
		return fmt.Errorf("points FieldsWriter: %w", err)
	}
	defer writer.Close()

	for _, pf := range ptFields {
		src := &dwptPointsSource{field: pf.fieldInfo.Name(), buf: pf.buf}
		if err := writer.WriteField(pf.fieldInfo, src); err != nil {
			return fmt.Errorf("points WriteField %q: %w", pf.fieldInfo.Name(), err)
		}
	}
	if err := writer.Finish(); err != nil {
		return fmt.Errorf("points finish: %w", err)
	}
	return nil
}

// dwptPointsSource adapts a single field's PointValuesBuffer to the in-memory
// point source the codec PointsWriter pulls values from. It satisfies the
// narrow codecs.PointsReader surface (CheckIntegrity/Close) that WriteField's
// reader parameter declares, plus — structurally — the codec's wider
// PointsSource contract (PointValueCount / VisitPoints). The codec writer
// type-asserts the reader to that wider surface.
type dwptPointsSource struct {
	field string
	buf   *PointValuesBuffer
}

// PointValueCount returns the number of buffered point values for field.
func (s *dwptPointsSource) PointValueCount(field string) int64 {
	if field != s.field {
		return 0
	}
	return int64(len(s.buf.packedValues))
}

// VisitPoints replays the buffered (docID, packedValue) pairs in document
// order, the order the codec writer feeds BKDWriter.Add.
func (s *dwptPointsSource) VisitPoints(field string, fn func(docID int, packedValue []byte) error) error {
	if field != s.field {
		return nil
	}
	for i, v := range s.buf.packedValues {
		if err := fn(s.buf.docIDs[i], v); err != nil {
			return err
		}
	}
	return nil
}

// CheckIntegrity is a no-op: the in-memory source has no on-disk checksum.
func (s *dwptPointsSource) CheckIntegrity() error { return nil }

// Close is a no-op: the in-memory source holds no resources.
func (s *dwptPointsSource) Close() error { return nil }

// getGeneratedFiles returns the list of files generated during flush.
func (dwpt *DocumentsWriterPerThread) getGeneratedFiles(segmentName string) []string {
	// Return the list of segment files
	// This would include: .fdt, .fdx, .tim, .tip, .doc, .pos, etc.
	files := []string{
		segmentName + ".fdt", // Stored fields data
		segmentName + ".fdx", // Stored fields index
		segmentName + ".tim", // Term dictionary
		segmentName + ".tip", // Term index
		segmentName + ".doc", // Doc values
		segmentName + ".pos", // Positions
	}
	return files
}

// storedFieldAdapter adapts StoredField to IndexableField interface.
type storedFieldAdapter struct {
	field *StoredField
}

func (s *storedFieldAdapter) Name() string              { return s.field.name }
func (s *storedFieldAdapter) StringValue() string       { return s.field.stringValue }
func (s *storedFieldAdapter) BinaryValue() []byte       { return s.field.binaryValue }
func (s *storedFieldAdapter) NumericValue() interface{} { return s.field.numericValue }
func (s *storedFieldAdapter) FieldType() FieldTypeInterface {
	return &simpleFieldType{}
}
func (s *storedFieldAdapter) ReaderValue() io.Reader { return nil }

// simpleFieldType provides a simple FieldTypeInterface implementation
type simpleFieldType struct{}

func (f *simpleFieldType) IsIndexed() bool               { return false }
func (f *simpleFieldType) IsStored() bool                { return true }
func (f *simpleFieldType) IsTokenized() bool             { return false }
func (f *simpleFieldType) GetIndexOptions() IndexOptions { return IndexOptionsNone }
func (f *simpleFieldType) GetDocValuesType() DocValuesType {
	return DocValuesTypeNone
}
func (f *simpleFieldType) StoreTermVectors() bool         { return false }
func (f *simpleFieldType) StoreTermVectorPositions() bool { return false }
func (f *simpleFieldType) StoreTermVectorOffsets() bool   { return false }

// postingTermsAdapter adapts FieldPostings to Terms interface.
type postingTermsAdapter struct {
	postings *FieldPostings
}

func (p *postingTermsAdapter) GetIterator() (TermsEnum, error) {
	return &postingTermsEnum{postings: p.postings, terms: getSortedTerms(p.postings)}, nil
}

func (p *postingTermsAdapter) GetIteratorWithSeek(seekTerm *Term) (TermsEnum, error) {
	terms := getSortedTerms(p.postings)
	// Find position at or after seek term
	for i, t := range terms {
		if t >= seekTerm.Text() {
			return &postingTermsEnum{postings: p.postings, terms: terms, index: i}, nil
		}
	}
	return &postingTermsEnum{postings: p.postings, terms: terms, index: len(terms)}, nil
}

func (p *postingTermsAdapter) Size() int64 {
	return int64(len(p.postings.terms))
}

func (p *postingTermsAdapter) GetDocCount() (int, error) {
	maxDoc := 0
	for _, posting := range p.postings.terms {
		for _, docID := range posting.docIDs {
			if docID > maxDoc {
				maxDoc = docID
			}
		}
	}
	return maxDoc + 1, nil
}

func (p *postingTermsAdapter) GetSumDocFreq() (int64, error) {
	var sum int64
	for _, posting := range p.postings.terms {
		sum += int64(len(posting.docIDs))
	}
	return sum, nil
}

func (p *postingTermsAdapter) GetSumTotalTermFreq() (int64, error) {
	var sum int64
	for _, posting := range p.postings.terms {
		for _, freq := range posting.freqs {
			sum += int64(freq)
		}
	}
	return sum, nil
}

func (p *postingTermsAdapter) HasFreqs() bool   { return true }
func (p *postingTermsAdapter) HasOffsets() bool { return false }
func (p *postingTermsAdapter) HasPositions() bool {
	return p.postings.fieldInfo != nil && p.postings.fieldInfo.IndexOptions().HasPositions()
}
func (p *postingTermsAdapter) HasPayloads() bool      { return false }
func (p *postingTermsAdapter) GetMin() (*Term, error) { return nil, nil }
func (p *postingTermsAdapter) GetMax() (*Term, error) { return nil, nil }

// GetPostingsReader returns the postings for a term.
func (p *postingTermsAdapter) GetPostingsReader(termText string, flags int) (PostingsEnum, error) {
	posting, ok := p.postings.terms[termText]
	if !ok {
		return nil, nil
	}
	return NewSingleDocPostingsEnum(posting.docIDs[0], posting.freqs[0]), nil
}

// getSortedTerms returns sorted term strings from postings
func getSortedTerms(postings *FieldPostings) []string {
	postings.mu.RLock()
	defer postings.mu.RUnlock()
	terms := make([]string, 0, len(postings.terms))
	for t := range postings.terms {
		terms = append(terms, t)
	}
	// Sort terms
	for i := 0; i < len(terms)-1; i++ {
		for j := i + 1; j < len(terms); j++ {
			if terms[i] > terms[j] {
				terms[i], terms[j] = terms[j], terms[i]
			}
		}
	}
	return terms
}

// postingTermsEnum iterates over terms in postings
type postingTermsEnum struct {
	postings *FieldPostings
	terms    []string
	index    int
}

func (e *postingTermsEnum) Next() (*Term, error) {
	if e.index >= len(e.terms) {
		return nil, nil
	}
	term := NewTerm(e.postings.fieldInfo.Name(), e.terms[e.index])
	e.index++
	return term, nil
}

func (e *postingTermsEnum) DocFreq() (int, error) {
	if e.index <= 0 || e.index > len(e.terms) {
		return 0, nil
	}
	term := e.terms[e.index-1]
	posting, ok := e.postings.terms[term]
	if !ok {
		return 0, nil
	}
	return len(posting.docIDs), nil
}

func (e *postingTermsEnum) TotalTermFreq() (int64, error) {
	if e.index <= 0 || e.index > len(e.terms) {
		return 0, nil
	}
	term := e.terms[e.index-1]
	posting, ok := e.postings.terms[term]
	if !ok {
		return 0, nil
	}
	var sum int64
	for _, freq := range posting.freqs {
		sum += int64(freq)
	}
	return sum, nil
}

// Postings returns a PostingsEnum for the current term. The current term is
// the one most recently returned by Next(), i.e. e.terms[e.index-1]. Returns
// nil when called before the first Next() or after exhaustion.
func (e *postingTermsEnum) Postings(flags int) (PostingsEnum, error) {
	if e.index <= 0 || e.index > len(e.terms) {
		return nil, nil
	}
	termText := e.terms[e.index-1]
	e.postings.mu.RLock()
	posting, ok := e.postings.terms[termText]
	e.postings.mu.RUnlock()
	if !ok || len(posting.docIDs) == 0 {
		return nil, nil
	}
	return &postingDataEnum{posting: posting, docIdx: -1, posIdx: -1}, nil
}

func (e *postingTermsEnum) SeekExact(term *Term) (bool, error) {
	for i, t := range e.terms {
		if t == term.Text() {
			e.index = i + 1
			return true, nil
		}
	}
	return false, nil
}

func (e *postingTermsEnum) SeekCeil(term *Term) (*Term, error) {
	for i, t := range e.terms {
		if t >= term.Text() {
			e.index = i + 1
			return NewTerm(e.postings.fieldInfo.Name(), t), nil
		}
	}
	e.index = len(e.terms)
	return nil, nil
}

func (e *postingTermsEnum) Term() *Term {
	if e.index <= 0 || e.index > len(e.terms) {
		return nil
	}
	return NewTerm(e.postings.fieldInfo.Name(), e.terms[e.index-1])
}

func (e *postingTermsEnum) Close() error { return nil }

func (e *postingTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (PostingsEnum, error) {
	return nil, nil
}

// postingDataEnum iterates over the doc/freq/position data of a single
// Posting list. It is returned by postingTermsEnum.Postings and drives
// the block-tree terms writer via WriteTerm.
type postingDataEnum struct {
	posting *Posting
	docIdx  int // index into posting.docIDs; -1 = before start
	posIdx  int // index into posting.positions[docIdx]; -1 = before start
}

func (p *postingDataEnum) NextDoc() (int, error) {
	p.docIdx++
	if p.docIdx >= len(p.posting.docIDs) {
		return NO_MORE_DOCS, nil
	}
	p.posIdx = -1
	return p.posting.docIDs[p.docIdx], nil
}

func (p *postingDataEnum) DocID() int {
	if p.docIdx < 0 || p.docIdx >= len(p.posting.docIDs) {
		return NO_MORE_DOCS
	}
	return p.posting.docIDs[p.docIdx]
}

func (p *postingDataEnum) Freq() (int, error) {
	if p.docIdx < 0 || p.docIdx >= len(p.posting.freqs) {
		return 0, nil
	}
	return p.posting.freqs[p.docIdx], nil
}

func (p *postingDataEnum) NextPosition() (int, error) {
	if p.docIdx < 0 || p.docIdx >= len(p.posting.positions) {
		return NO_MORE_POSITIONS, nil
	}
	positions := p.posting.positions[p.docIdx]
	p.posIdx++
	if p.posIdx >= len(positions) {
		return NO_MORE_POSITIONS, nil
	}
	return positions[p.posIdx], nil
}

func (p *postingDataEnum) StartOffset() (int, error) { return -1, nil }
func (p *postingDataEnum) EndOffset() (int, error)   { return -1, nil }
func (p *postingDataEnum) GetPayload() ([]byte, error) {
	return nil, nil
}

func (p *postingDataEnum) Advance(target int) (int, error) {
	for {
		docID, err := p.NextDoc()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if docID >= target {
			return docID, nil
		}
	}
}

func (p *postingDataEnum) Cost() int64 {
	return int64(len(p.posting.docIDs))
}

func (p *postingDataEnum) Attributes() interface{} { return nil }
func (p *postingDataEnum) SlowAdvance(target int) (int, error) {
	return p.Advance(target)
}

// generateSegmentID generates a unique segment ID.
func generateSegmentID() []byte {
	id := make([]byte, 16)
	// Use timestamp and random data for ID
	now := time.Now().UnixNano()
	for i := 0; i < 8; i++ {
		id[i] = byte(now >> (i * 8))
	}
	// Fill remaining with pseudo-random data
	for i := 8; i < 16; i++ {
		id[i] = byte(i*17 + int(now&0xFF))
	}
	return id
}
