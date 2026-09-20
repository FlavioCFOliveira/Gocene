// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package memory is the Go port of org.apache.lucene.index.memory from Apache
// Lucene 10.5.0. It holds MemoryIndex, a high-performance single-document
// main-memory fulltext search index.
package memory

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// debug renders the `private static final boolean DEBUG = false` switch of
// MemoryIndex.java:383. Lucene keeps the guarded System.err traces in the
// source; they are ported as guarded statements so the code shape matches.
const debug = false

// MemoryIndex is a high-performance single-document main memory Apache Lucene
// fulltext search index.
//
// This is the Go port of org.apache.lucene.index.memory.MemoryIndex
// (Apache Lucene 10.5.0, lucene/memory/src/java/org/apache/lucene/index/memory/MemoryIndex.java).
//
// Each instance can hold at most one Lucene "document", with a document
// containing zero or more "fields", each field having a name and a fulltext
// value. The fulltext value is tokenized into zero or more index terms on
// AddField, according to the policy implemented by an Analyzer.
//
// # Thread safety guarantees
//
// MemoryIndex is not normally thread-safe for adds or queries. However,
// queries are thread-safe after Freeze has been called.
type MemoryIndex struct {
	// fields is the info for each field: Map<String fieldName, Info field>.
	// Java declares it `SortedMap<String, Info> fields = new TreeMap<>()`.
	// Go has no sorted map, so the entries live in a plain map and every
	// traversal goes through sortedFieldNames, which reproduces the TreeMap
	// iteration order exactly.
	fields map[string]*info

	storeOffsets  bool
	storePayloads bool

	byteBlockPool      *util.ByteBlockPool
	slicedIntBlockPool *SlicedIntBlockPool
	postingsWriter     *SliceWriter
	// payloadsBytesRefs is non-nil only when storePayloads is set.
	payloadsBytesRefs *util.BytesRefArray

	bytesUsed *util.Counter

	frozen bool

	normSimilarity search.Similarity

	defaultFieldType *document.FieldType
}

// NewMemoryIndex constructs an empty instance that will not store offsets or
// payloads. Renders `public MemoryIndex()` (MemoryIndex.java:405).
func NewMemoryIndex() *MemoryIndex {
	return NewMemoryIndexWithOffsets(false)
}

// NewMemoryIndexWithOffsets constructs an empty instance that can optionally
// store the start and end character offset of each token term in the text.
// This can be useful for highlighting of hit locations with the Lucene
// highlighter package. But it will not store payloads; use another constructor
// for that.
//
// Renders `public MemoryIndex(boolean storeOffsets)` (MemoryIndex.java:417).
func NewMemoryIndexWithOffsets(storeOffsets bool) *MemoryIndex {
	return NewMemoryIndexWithOffsetsAndPayloads(storeOffsets, false)
}

// NewMemoryIndexWithOffsetsAndPayloads constructs an empty instance with the
// option of storing offsets and payloads.
//
// Renders `public MemoryIndex(boolean storeOffsets, boolean storePayloads)`
// (MemoryIndex.java:427).
func NewMemoryIndexWithOffsetsAndPayloads(storeOffsets, storePayloads bool) *MemoryIndex {
	return newMemoryIndexWithMaxReusedBytes(storeOffsets, storePayloads, 0)
}

// newMemoryIndexWithMaxReusedBytes is the expert constructor: it accepts an
// upper limit for the number of bytes that should be reused if this instance
// is Reset. The payload storage, if used, is unaffected by maxReusedBytes.
//
// Renders the package-private
// `MemoryIndex(boolean storeOffsets, boolean storePayloads, long maxReusedBytes)`
// (MemoryIndex.java:441); it is unexported here because Java declares it
// package-private.
func newMemoryIndexWithMaxReusedBytes(storeOffsets, storePayloads bool, maxReusedBytes int64) *MemoryIndex {
	mi := &MemoryIndex{
		fields:           make(map[string]*info),
		storeOffsets:     storeOffsets,
		storePayloads:    storePayloads,
		normSimilarity:   search.GetDefaultSimilarity(),
		defaultFieldType: document.NewFieldType(),
	}
	if storeOffsets {
		mi.defaultFieldType.SetIndexOptions(spi.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	} else {
		mi.defaultFieldType.SetIndexOptions(spi.IndexOptionsDocsAndFreqsAndPositions)
	}
	mi.defaultFieldType.SetStoreTermVectors(true)
	mi.bytesUsed = util.NewCounter()
	maxBufferedByteBlocks := int((maxReusedBytes / 2) / int64(util.ByteBlockSize))
	maxBufferedIntBlocks := int((maxReusedBytes - (int64(maxBufferedByteBlocks) * int64(util.ByteBlockSize))) /
		(int64(util.IntBlockSize) * 4))
	mi.byteBlockPool = util.NewByteBlockPool(
		util.NewRecyclingByteBlockAllocatorWithBuffered(maxBufferedByteBlocks, mi.bytesUsed))
	mi.slicedIntBlockPool = NewSlicedIntBlockPool(
		util.NewRecyclingIntBlockAllocator(util.IntBlockSize, maxBufferedIntBlocks, mi.bytesUsed))
	mi.postingsWriter = NewSliceWriter(mi.slicedIntBlockPool)
	// TODO refactor BytesRefArray to allow us to apply maxReusedBytes option.
	//
	// Gocene's util.BytesRefArray takes a block size where Lucene's takes the
	// Counter it should charge; it tracks its own byte usage instead. Zero
	// selects the default block size.
	if storePayloads {
		mi.payloadsBytesRefs = util.NewBytesRefArray(0)
	}
	return mi
}

// sortedFieldNames returns the field names in ascending order. It reproduces
// the iteration order of the java.util.TreeMap that backs MemoryIndex.fields.
func (mi *MemoryIndex) sortedFieldNames() []string {
	names := make([]string, 0, len(mi.fields))
	for name := range mi.fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// AddFieldText tokenizes the given field text and adds the resulting terms to
// the index; equivalent to adding an indexed non-keyword Lucene Field that is
// tokenized, not stored, termVectorStored with positions (or termVectorStored
// with positions and offsets).
//
// Renders `public void addField(String fieldName, String text, Analyzer analyzer)`
// (MemoryIndex.java:479). Java's IllegalArgumentException and IOException are
// both reported through the error result.
func (mi *MemoryIndex) AddFieldText(fieldName, text string, analyzer analysis.Analyzer) error {
	if fieldName == "" {
		return fmt.Errorf("fieldName must not be null")
	}
	if analyzer == nil {
		return fmt.Errorf("analyzer must not be null")
	}

	stream, err := analyzer.TokenStream(fieldName, strings.NewReader(text))
	if err != nil {
		return err
	}
	inf, err := mi.getInfo(fieldName, mi.defaultFieldType)
	if err != nil {
		return err
	}
	return mi.storeTerms(inf, stream,
		positionIncrementGapOf(analyzer, fieldName),
		offsetGapOf(analyzer, fieldName))
}

// FromDocument builds a MemoryIndex from a Lucene document using an analyzer.
//
// Renders `public static MemoryIndex fromDocument(Iterable<? extends IndexableField>, Analyzer)`
// (MemoryIndex.java:499).
func FromDocument(doc []index.IndexableField, analyzer analysis.Analyzer) (*MemoryIndex, error) {
	return FromDocumentWithMaxReusedBytes(doc, analyzer, false, false, 0)
}

// FromDocumentWithOptions builds a MemoryIndex from a Lucene document using an
// analyzer, with the option of storing offsets and payloads.
//
// Renders `public static MemoryIndex fromDocument(Iterable<? extends IndexableField>, Analyzer, boolean, boolean)`
// (MemoryIndex.java:513).
func FromDocumentWithOptions(doc []index.IndexableField, analyzer analysis.Analyzer, storeOffsets, storePayloads bool) (*MemoryIndex, error) {
	return FromDocumentWithMaxReusedBytes(doc, analyzer, storeOffsets, storePayloads, 0)
}

// FromDocumentWithMaxReusedBytes builds a MemoryIndex from a Lucene document
// using an analyzer, with the option of storing offsets and payloads and an
// upper limit for the number of bytes that should remain in the internal
// memory pools after Reset is called.
//
// Renders `public static MemoryIndex fromDocument(Iterable<? extends IndexableField>, Analyzer, boolean, boolean, long)`
// (MemoryIndex.java:532).
func FromDocumentWithMaxReusedBytes(doc []index.IndexableField, analyzer analysis.Analyzer, storeOffsets, storePayloads bool, maxReusedBytes int64) (*MemoryIndex, error) {
	mi := newMemoryIndexWithMaxReusedBytes(storeOffsets, storePayloads, maxReusedBytes)
	for _, field := range doc {
		if err := mi.AddFieldWithAnalyzer(field, analyzer); err != nil {
			return nil, err
		}
	}
	return mi, nil
}

// keywordTokenStream is the TokenStream returned by
// MemoryIndex.KeywordTokenStream. It renders the anonymous TokenStream
// subclass of MemoryIndex.java:558.
type keywordTokenStream struct {
	*analysis.BaseTokenStream
	keywords  []any
	iter      int
	start     int
	termAtt   analysis.CharTermAttribute
	offsetAtt analysis.OffsetAttribute
}

// IncrementToken renders `public boolean incrementToken()` (MemoryIndex.java:565).
func (ts *keywordTokenStream) IncrementToken() (bool, error) {
	if ts.iter >= len(ts.keywords) {
		return false, nil
	}

	obj := ts.keywords[ts.iter]
	ts.iter++
	if obj == nil {
		return false, fmt.Errorf("keyword must not be null")
	}

	term := fmt.Sprint(obj)
	ts.ClearAttributes()
	ts.termAtt.SetEmpty()
	ts.termAtt.AppendString(term)
	ts.offsetAtt.SetOffset(ts.start, ts.start+ts.termAtt.Length())
	ts.start += len(term) + 1 // separate words by 1 (blank) character
	return true, nil
}

// KeywordTokenStream creates and returns a token stream that generates a token
// for each keyword in the given collection, "as is", without any transforming
// text analysis. The resulting token stream can be fed into AddField, perhaps
// wrapped into another TokenFilter, as desired.
//
// Renders `public <T> TokenStream keywordTokenStream(final Collection<T> keywords)`
// (MemoryIndex.java:554). Go methods cannot carry type parameters, and Java
// erases T to call obj.toString(), so the element type is rendered as any and
// the conversion as fmt.Sprint.
func (mi *MemoryIndex) KeywordTokenStream(keywords []any) (analysis.TokenStream, error) {
	// TODO: deprecate & move this method into AnalyzerUtil?
	if keywords == nil {
		return nil, fmt.Errorf("keywords must not be null")
	}

	base := analysis.NewBaseTokenStream()
	ts := &keywordTokenStream{BaseTokenStream: base, keywords: keywords}
	ts.termAtt = base.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	ts.offsetAtt = base.AddAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	return ts, nil
}

// AddFieldWithAnalyzer adds a Lucene IndexableField to the MemoryIndex using
// the provided analyzer. Also stores doc values based on
// IndexableFieldType.DocValuesType if set.
//
// Renders `public void addField(IndexableField field, Analyzer analyzer)`
// (MemoryIndex.java:588).
func (mi *MemoryIndex) AddFieldWithAnalyzer(field index.IndexableField, analyzer analysis.Analyzer) error {
	inf, err := mi.getInfo(field.Name(), field.FieldType())
	if err != nil {
		return err
	}

	var offsetGap int
	var tokenStream analysis.TokenStream
	var positionIncrementGap int
	if analyzer != nil {
		offsetGap = offsetGapOf(analyzer, field.Name())
		tokenStream = field.TokenStream(analyzer, nil)
		positionIncrementGap = positionIncrementGapOf(analyzer, field.Name())
	} else {
		offsetGap = 1
		tokenStream = field.TokenStream(nil, nil)
		positionIncrementGap = 0
	}
	if tokenStream != nil {
		if err := mi.storeTerms(inf, tokenStream, positionIncrementGap, offsetGap); err != nil {
			return err
		}
	} else if field.FieldType().IndexOptions().Subsumes(spi.IndexOptionsDocs) {
		binaryValue := field.BinaryValue()
		if binaryValue == nil {
			return fmt.Errorf("Indexed field must provide a TokenStream or a binary value")
		}
		mi.storeTerm(inf, util.NewBytesRef(binaryValue))
	}

	docValuesType := field.FieldType().DocValuesType()
	var docValuesValue any
	switch docValuesType {
	case spi.DocValuesTypeNone:
		docValuesValue = nil
	case spi.DocValuesTypeBinary, spi.DocValuesTypeSorted, spi.DocValuesTypeSortedSet:
		if b := field.BinaryValue(); b != nil {
			docValuesValue = util.NewBytesRef(b)
		}
	case spi.DocValuesTypeNumeric, spi.DocValuesTypeSortedNumeric:
		docValuesValue = field.NumericValue()
	default:
		return fmt.Errorf("unknown doc values type [%v]", docValuesType)
	}
	if docValuesValue != nil {
		if err := mi.storeDocValues(inf, docValuesType, docValuesValue); err != nil {
			return err
		}
	}

	if field.FieldType().PointDimensionCount() > 0 {
		mi.storePointValues(inf, util.NewBytesRef(field.BinaryValue()))
	}

	if field.FieldType().Stored() {
		mi.storeValues(inf, field)
	}

	if field.FieldType().VectorDimension() > 0 {
		if err := mi.storeVectorValues(inf, field); err != nil {
			return err
		}
	}
	return nil
}

// AddField iterates over the given token stream and adds the resulting terms
// to the index; equivalent to adding a tokenized, indexed, termVectorStored,
// unstored Lucene Field. Finally closes the token stream. Note that
// untokenized keywords can be added with this method via KeywordTokenStream,
// the Lucene KeywordTokenizer or similar utilities.
//
// Renders `public void addField(String fieldName, TokenStream stream)`
// (MemoryIndex.java:660).
func (mi *MemoryIndex) AddField(fieldName string, stream analysis.TokenStream) error {
	return mi.AddFieldWithPositionIncrementGap(fieldName, stream, 0)
}

// AddFieldWithPositionIncrementGap iterates over the given token stream and
// adds the resulting terms to the index, applying positionIncrementGap when
// fields with the same name are added more than once.
//
// Renders `public void addField(String fieldName, TokenStream stream, int positionIncrementGap)`
// (MemoryIndex.java:676).
func (mi *MemoryIndex) AddFieldWithPositionIncrementGap(fieldName string, stream analysis.TokenStream, positionIncrementGap int) error {
	return mi.AddFieldWithGaps(fieldName, stream, positionIncrementGap, 1)
}

// AddFieldWithGaps iterates over the given token stream and adds the resulting
// terms to the index, applying positionIncrementGap and offsetGap when fields
// with the same name are added more than once. The token stream is guaranteed
// to be closed no matter what.
//
// Renders `public void addField(String fieldName, TokenStream tokenStream, int positionIncrementGap, int offsetGap)`
// (MemoryIndex.java:694).
func (mi *MemoryIndex) AddFieldWithGaps(fieldName string, tokenStream analysis.TokenStream, positionIncrementGap, offsetGap int) error {
	inf, err := mi.getInfo(fieldName, mi.defaultFieldType)
	if err != nil {
		return err
	}
	return mi.storeTerms(inf, tokenStream, positionIncrementGap, offsetGap)
}

// getInfo renders `private Info getInfo(String fieldName, IndexableFieldType fieldType)`
// (MemoryIndex.java:700).
//
// Lucene mutates the shared FieldInfo in place through the package-private
// setters FieldInfo.setPointDimensions and FieldInfo.setDocValuesType. Gocene's
// spi.FieldInfo is frozen on construction and exposes neither, so the two
// branches are rendered by rebuilding info.fieldInfo with the changed member —
// the same technique Lucene itself uses one method away, in storeDocValues.
func (mi *MemoryIndex) getInfo(fieldName string, fieldType spi.IndexableFieldType) (*info, error) {
	if mi.frozen {
		return nil, fmt.Errorf("Cannot call addField() when MemoryIndex is frozen")
	}
	if fieldName == "" {
		return nil, fmt.Errorf("fieldName must not be null")
	}
	inf := mi.fields[fieldName]
	if inf == nil {
		inf = newInfo(mi.createFieldInfo(fieldName, len(mi.fields), fieldType), mi.byteBlockPool)
		mi.fields[fieldName] = inf
	}
	if fieldType.PointDimensionCount() != inf.fieldInfo.PointDimensionCount() {
		if fieldType.PointDimensionCount() > 0 {
			updated, err := setPointDimensions(inf.fieldInfo,
				fieldType.PointDimensionCount(),
				fieldType.PointIndexDimensionCount(),
				fieldType.PointNumBytes())
			if err != nil {
				return nil, err
			}
			inf.fieldInfo = updated
		}
	}
	if fieldType.DocValuesType() != inf.fieldInfo.DocValuesType() {
		if fieldType.DocValuesType() != spi.DocValuesTypeNone {
			updated, err := setDocValuesType(inf.fieldInfo, fieldType.DocValuesType())
			if err != nil {
				return nil, err
			}
			inf.fieldInfo = updated
		}
	}
	return inf, nil
}

// setDocValuesType renders `public void setDocValuesType(DocValuesType type)`
// of org.apache.lucene.index.FieldInfo, guards included. Gocene's
// spi.FieldInfo is frozen after construction and declares no such mutator, so
// the member is rendered here as a rebuild of the FieldInfo — the same
// technique Lucene itself uses in MemoryIndex.storeDocValues — and it returns
// the updated FieldInfo instead of mutating in place.
func setDocValuesType(fi *spi.FieldInfo, dvType spi.DocValuesType) (*spi.FieldInfo, error) {
	if fi.DocValuesType() != spi.DocValuesTypeNone &&
		dvType != spi.DocValuesTypeNone &&
		fi.DocValuesType() != dvType {
		return nil, fmt.Errorf("cannot change DocValues type from %v to %v for field %q",
			fi.DocValuesType(), dvType, fi.Name())
	}
	updated := rebuildFieldInfo(fi, func(opts *spi.FieldInfoOptions) {
		opts.DocValuesType = dvType
	})
	if err := updated.CheckConsistency(); err != nil {
		return nil, err
	}
	return updated, nil
}

// setPointDimensions renders
// `public void setPointDimensions(int dimensionCount, int indexDimensionCount, int numBytes)`
// of org.apache.lucene.index.FieldInfo, guards included. See setDocValuesType
// for why it returns a rebuilt FieldInfo rather than mutating in place.
func setPointDimensions(fi *spi.FieldInfo, dimensionCount, indexDimensionCount, numBytes int) (*spi.FieldInfo, error) {
	name := fi.Name()
	if dimensionCount <= 0 {
		return nil, fmt.Errorf("point dimension count must be >= 0; got %d for field=%q", dimensionCount, name)
	}
	if indexDimensionCount > index.PointValuesMaxIndexDimensions {
		return nil, fmt.Errorf("point index dimension count must be < PointValues.MAX_INDEX_DIMENSIONS (= %d); got %d for field=%q",
			index.PointValuesMaxIndexDimensions, indexDimensionCount, name)
	}
	if indexDimensionCount > dimensionCount {
		return nil, fmt.Errorf("point index dimension count must be <= point dimension count (= %d); got %d for field=%q",
			dimensionCount, indexDimensionCount, name)
	}
	if numBytes <= 0 {
		return nil, fmt.Errorf("point numBytes must be >= 0; got %d for field=%q", numBytes, name)
	}
	if numBytes > index.PointValuesMaxNumBytes {
		return nil, fmt.Errorf("point numBytes must be <= PointValues.MAX_NUM_BYTES (= %d); got %d for field=%q",
			index.PointValuesMaxNumBytes, numBytes, name)
	}
	if fi.PointDimensionCount() != 0 && fi.PointDimensionCount() != dimensionCount {
		return nil, fmt.Errorf("cannot change point dimension count from %d to %d for field=%q",
			fi.PointDimensionCount(), dimensionCount, name)
	}
	if fi.PointIndexDimensionCount() != 0 && fi.PointIndexDimensionCount() != indexDimensionCount {
		return nil, fmt.Errorf("cannot change point index dimension count from %d to %d for field=%q",
			fi.PointIndexDimensionCount(), indexDimensionCount, name)
	}
	if fi.PointNumBytes() != 0 && fi.PointNumBytes() != numBytes {
		return nil, fmt.Errorf("cannot change point numBytes from %d to %d for field=%q",
			fi.PointNumBytes(), numBytes, name)
	}

	updated := rebuildFieldInfo(fi, func(opts *spi.FieldInfoOptions) {
		opts.PointDimensionCount = dimensionCount
		opts.PointIndexDimensionCount = indexDimensionCount
		opts.PointNumBytes = numBytes
	})
	if err := updated.CheckConsistency(); err != nil {
		return nil, err
	}
	return updated, nil
}

// optionsOf reads back every member of fi into a spi.FieldInfoOptions, so a
// FieldInfo can be rebuilt with one member changed. It exists only because
// spi.FieldInfo is immutable after construction; Lucene mutates in place.
func optionsOf(fi *spi.FieldInfo) spi.FieldInfoOptions {
	return spi.FieldInfoOptions{
		IndexOptions:             fi.GetIndexOptions(),
		DocValuesType:            fi.DocValuesType(),
		DocValuesSkipIndexType:   fi.DocValuesSkipIndexType(),
		DocValuesGen:             fi.DocValuesGen(),
		Stored:                   fi.IsStored(),
		Tokenized:                fi.IsTokenized(),
		OmitNorms:                fi.OmitNorms(),
		StoreTermVectors:         fi.HasTermVectors(),
		StoreTermVectorPositions: fi.StoreTermVectorPositions(),
		StoreTermVectorOffsets:   fi.StoreTermVectorOffsets(),
		StoreTermVectorPayloads:  fi.StoreTermVectorPayloads(),
		PointDimensionCount:      fi.PointDimensionCount(),
		PointIndexDimensionCount: fi.PointIndexDimensionCount(),
		PointNumBytes:            fi.PointNumBytes(),
		VectorDimension:          fi.VectorDimension(),
		VectorEncoding:           fi.VectorEncoding(),
		VectorSimilarityFunction: fi.VectorSimilarityFunction(),
		IsSoftDeletesField:       fi.IsSoftDeletesField(),
		IsParentField:            fi.IsParentField(),
	}
}

// rebuildFieldInfo returns a copy of fi with mutate applied to its options and
// with fi's attributes carried over. See optionsOf for why it exists.
func rebuildFieldInfo(fi *spi.FieldInfo, mutate func(*spi.FieldInfoOptions)) *spi.FieldInfo {
	opts := optionsOf(fi)
	mutate(&opts)
	rebuilt := spi.NewFieldInfo(fi.Name(), fi.Number(), opts)
	for k, v := range fi.GetAttributes() {
		rebuilt.PutAttribute(k, v)
	}
	if fi.HasPayloads() {
		rebuilt.SetStorePayloads()
	}
	return rebuilt
}

// createFieldInfo renders
// `private FieldInfo createFieldInfo(String fieldName, int ord, IndexableFieldType fieldType)`
// (MemoryIndex.java:727).
func (mi *MemoryIndex) createFieldInfo(fieldName string, ord int, fieldType spi.IndexableFieldType) *spi.FieldInfo {
	indexOptions := spi.IndexOptionsDocsAndFreqsAndPositions
	if mi.storeOffsets {
		indexOptions = spi.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	}
	fi := spi.NewFieldInfo(fieldName, ord, spi.FieldInfoOptions{
		StoreTermVectors:         fieldType.StoreTermVectors(),
		OmitNorms:                fieldType.OmitNorms(),
		IndexOptions:             indexOptions,
		DocValuesType:            fieldType.DocValuesType(),
		DocValuesSkipIndexType:   fieldType.DocValuesSkipIndexType(),
		DocValuesGen:             -1,
		PointDimensionCount:      fieldType.PointDimensionCount(),
		PointIndexDimensionCount: fieldType.PointIndexDimensionCount(),
		PointNumBytes:            fieldType.PointNumBytes(),
		VectorDimension:          fieldType.VectorDimension(),
		VectorEncoding:           fieldType.VectorEncoding(),
		VectorSimilarityFunction: fieldType.VectorSimilarityFunction(),
		IsSoftDeletesField:       false,
		IsParentField:            false,
	})
	// Java passes storePayloads as the FieldInfo constructor's storePayloads
	// argument. spi.FieldInfoOptions carries no such member, so the flag is
	// set through the dedicated mutator, which only ever sets it to true —
	// false is the value NewFieldInfo already leaves behind.
	if mi.storePayloads {
		fi.SetStorePayloads()
	}
	return fi
}

// storePointValues renders `private void storePointValues(Info info, BytesRef pointValue)`
// (MemoryIndex.java:753).
func (mi *MemoryIndex) storePointValues(inf *info, pointValue *util.BytesRef) {
	if inf.pointValues == nil {
		inf.pointValues = make([]*util.BytesRef, 4)
	}
	// ArrayUtil.grow(T[], int): grow only when the array is too small.
	if len(inf.pointValues) < inf.pointValuesCount+1 {
		inf.pointValues = util.GrowExact(inf.pointValues,
			util.Oversize(inf.pointValuesCount+1, util.NumBytesObjectRef))
	}
	inf.pointValues[inf.pointValuesCount] = util.BytesRefDeepCopyOf(pointValue)
	inf.pointValuesCount++
}

// storeVectorValues renders
// `private void storeVectorValues(Info info, IndexableField vectorField)`
// (MemoryIndex.java:761).
func (mi *MemoryIndex) storeVectorValues(inf *info, vectorField index.IndexableField) error {
	switch inf.fieldInfo.VectorEncoding() {
	case util.VectorEncodingByte:
		if byteVectorField, ok := vectorField.(*document.KnnByteVectorField); ok {
			if inf.byteVectorCount == 1 {
				return fmt.Errorf("Only one value per field allowed for byte vector field [%s]", vectorField.Name())
			}
			inf.byteVectorCount++
			if inf.byteVectorValues == nil {
				inf.byteVectorValues = make([][]byte, 1)
			}
			inf.byteVectorValues[0] = util.CopyOfSubArrayGeneric(
				byteVectorField.VectorValue(), 0, inf.fieldInfo.VectorDimension())
			return nil
		}
		return fmt.Errorf("Field [%s] is not a byte vector field, but the field info is configured for byte vectors",
			vectorField.Name())
	case util.VectorEncodingFloat32:
		if floatVectorField, ok := vectorField.(*document.KnnFloatVectorField); ok {
			if inf.floatVectorCount == 1 {
				return fmt.Errorf("Only one value per field allowed for float vector field [%s]", vectorField.Name())
			}
			inf.floatVectorCount++
			if inf.floatVectorValues == nil {
				inf.floatVectorValues = make([][]float32, 1)
			}
			inf.floatVectorValues[0] = util.CopyOfSubArrayGeneric(
				floatVectorField.VectorValue(), 0, inf.fieldInfo.VectorDimension())
			return nil
		}
		return fmt.Errorf("Field [%s] is not a float vector field, but the field info is configured for float vectors",
			vectorField.Name())
	}
	return nil
}

// storeValues renders `private void storeValues(Info info, IndexableField field)`
// (MemoryIndex.java:811).
func (mi *MemoryIndex) storeValues(inf *info, field index.IndexableField) {
	if inf.storedValues == nil {
		inf.storedValues = []any{}
	}
	binaryValue := field.BinaryValue()
	if binaryValue != nil {
		inf.storedValues = append(inf.storedValues, util.NewBytesRef(binaryValue))
		return
	}
	numberValue := field.NumericValue()
	if numberValue != nil {
		inf.storedValues = append(inf.storedValues, numberValue)
		return
	}
	stringValue := field.StringValue()
	if stringValue != "" {
		inf.storedValues = append(inf.storedValues, stringValue)
	}
}

// storeDocValues renders
// `private void storeDocValues(Info info, DocValuesType docValuesType, Object docValuesValue)`
// (MemoryIndex.java:831).
func (mi *MemoryIndex) storeDocValues(inf *info, docValuesType spi.DocValuesType, docValuesValue any) error {
	fieldName := inf.fieldInfo.Name()
	existingDocValuesType := inf.fieldInfo.DocValuesType()
	if existingDocValuesType == spi.DocValuesTypeNone {
		// first time we add doc values for this field:
		hadPayloads := inf.fieldInfo.HasPayloads()
		rebuilt := spi.NewFieldInfo(inf.fieldInfo.Name(), inf.fieldInfo.Number(), spi.FieldInfoOptions{
			StoreTermVectors:         inf.fieldInfo.HasTermVectors(),
			OmitNorms:                hadPayloads,
			IndexOptions:             inf.fieldInfo.GetIndexOptions(),
			DocValuesType:            docValuesType,
			DocValuesSkipIndexType:   spi.DocValuesSkipIndexTypeNone,
			DocValuesGen:             -1,
			PointDimensionCount:      inf.fieldInfo.PointDimensionCount(),
			PointIndexDimensionCount: inf.fieldInfo.PointIndexDimensionCount(),
			PointNumBytes:            inf.fieldInfo.PointNumBytes(),
			VectorDimension:          inf.fieldInfo.VectorDimension(),
			VectorEncoding:           inf.fieldInfo.VectorEncoding(),
			VectorSimilarityFunction: inf.fieldInfo.VectorSimilarityFunction(),
			IsSoftDeletesField:       inf.fieldInfo.IsSoftDeletesField(),
			IsParentField:            inf.fieldInfo.IsParentField(),
		})
		for k, v := range inf.fieldInfo.GetAttributes() {
			rebuilt.PutAttribute(k, v)
		}
		// Java's third constructor argument is storePayloads, which it also
		// feeds from hasPayloads(); see createFieldInfo for why it is set
		// through the mutator.
		if hadPayloads {
			rebuilt.SetStorePayloads()
		}
		inf.fieldInfo = rebuilt
	} else if existingDocValuesType != docValuesType {
		return fmt.Errorf("Can't add [%v] doc values field [%s], because [%v] doc values field already exists",
			docValuesType, fieldName, existingDocValuesType)
	}
	switch docValuesType {
	case spi.DocValuesTypeNumeric:
		if inf.numericProducer.dvLongValues != nil {
			return fmt.Errorf("Only one value per field allowed for [%v] doc values field [%s]",
				docValuesType, fieldName)
		}
		inf.numericProducer.dvLongValues = []int64{longValueOf(docValuesValue)}
		inf.numericProducer.count++
	case spi.DocValuesTypeSortedNumeric:
		if inf.numericProducer.dvLongValues == nil {
			inf.numericProducer.dvLongValues = make([]int64, 4)
		}
		if len(inf.numericProducer.dvLongValues) < inf.numericProducer.count+1 {
			inf.numericProducer.dvLongValues = util.GrowExactInt64(inf.numericProducer.dvLongValues,
				util.Oversize(inf.numericProducer.count+1, 8))
		}
		inf.numericProducer.dvLongValues[inf.numericProducer.count] = longValueOf(docValuesValue)
		inf.numericProducer.count++
	case spi.DocValuesTypeBinary, spi.DocValuesTypeSorted:
		if inf.binaryProducer.dvBytesValuesSet != nil {
			return fmt.Errorf("Only one value per field allowed for [%v] doc values field [%s]",
				docValuesType, fieldName)
		}
		inf.binaryProducer.dvBytesValuesSet = docValuesValue.(*util.BytesRef).Clone()
	case spi.DocValuesTypeSortedSet:
		if inf.bytesRefHashProducer.dvBytesRefHashValuesSet == nil {
			inf.bytesRefHashProducer.dvBytesRefHashValuesSet = util.NewBytesRefHashWithPool(mi.byteBlockPool)
		}
		if _, err := inf.bytesRefHashProducer.dvBytesRefHashValuesSet.Add(docValuesValue.(*util.BytesRef)); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown doc values type [%v]", docValuesType)
	}
	return nil
}

// longValueOf renders the `((Number) docValuesValue).longValue()` casts of
// storeDocValues. IndexableField.NumericValue returns any, whose dynamic type
// is one of Go's numeric kinds.
func longValueOf(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int8:
		return int64(n)
	case int16:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	case float32:
		return int64(n)
	case float64:
		return int64(n)
	}
	return 0
}

// storeTerm renders `private void storeTerm(Info info, BytesRef term)`
// (MemoryIndex.java:912).
func (mi *MemoryIndex) storeTerm(inf *info, term *util.BytesRef) {
	inf.numTokens++
	ord, _ := inf.terms.Add(term)
	if ord < 0 {
		ord = -ord - 1
		mi.postingsWriter.Reset(inf.sliceArray.end[ord])
	} else {
		inf.sliceArray.start[ord] = mi.postingsWriter.StartNewSlice()
	}
	inf.sliceArray.freq[ord]++
	if inf.sliceArray.freq[ord] > inf.maxTermFrequency {
		inf.maxTermFrequency = inf.sliceArray.freq[ord]
	}
	inf.sumTotalTermFreq++
	mi.postingsWriter.WriteInt(int32(inf.lastPosition)) // fake position
	inf.lastPosition++
	if mi.storeOffsets { // fake offsets
		mi.postingsWriter.WriteInt(0)
		mi.postingsWriter.WriteInt(0)
	}
	if mi.storePayloads {
		mi.postingsWriter.WriteInt(-1) // fake payload
	}
	inf.sliceArray.end[ord] = mi.postingsWriter.CurrentOffset()
}

// storeTerms renders
// `private void storeTerms(Info info, TokenStream tokenStream, int positionIncrementGap, int offsetGap)`
// (MemoryIndex.java:935).
func (mi *MemoryIndex) storeTerms(inf *info, tokenStream analysis.TokenStream, positionIncrementGap, offsetGap int) (err error) {
	pos := -1
	offset := 0
	if inf.numTokens > 0 {
		pos = inf.lastPosition + positionIncrementGap
		offset = inf.lastOffset + offsetGap
	}

	stream := tokenStream
	defer func() {
		if cerr := stream.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	source := stream.GetAttributeSource()
	if source == nil {
		return fmt.Errorf("memory: TokenStream %T exposes no AttributeSource", stream)
	}
	rawTermAtt := source.GetAttribute(analysis.TermToBytesRefAttributeType)
	if rawTermAtt == nil {
		return fmt.Errorf("memory: TokenStream %T carries no TermToBytesRefAttribute", stream)
	}
	termAtt := rawTermAtt.(analysis.TermToBytesRefAttribute)
	posIncrAttribute := source.AddAttribute(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
	offsetAtt := source.AddAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	var payloadAtt analysis.PayloadAttribute
	if mi.storePayloads {
		payloadAtt = source.AddAttribute(analysis.PayloadAttributeType).(analysis.PayloadAttribute)
	}
	if err := stream.Reset(); err != nil {
		return err
	}

	for {
		more, err := stream.IncrementToken()
		if err != nil {
			return err
		}
		if !more {
			break
		}
		inf.numTokens++
		posIncr := posIncrAttribute.GetPositionIncrement()
		if posIncr == 0 {
			inf.numOverlapTokens++
		}
		pos += posIncr
		ord, err := inf.terms.Add(termAtt.GetBytesRef())
		if err != nil {
			return err
		}
		if ord < 0 {
			ord = (-ord) - 1
			mi.postingsWriter.Reset(inf.sliceArray.end[ord])
		} else {
			inf.sliceArray.start[ord] = mi.postingsWriter.StartNewSlice()
		}
		inf.sliceArray.freq[ord]++
		if inf.sliceArray.freq[ord] > inf.maxTermFrequency {
			inf.maxTermFrequency = inf.sliceArray.freq[ord]
		}
		inf.sumTotalTermFreq++
		mi.postingsWriter.WriteInt(int32(pos))
		if mi.storeOffsets {
			mi.postingsWriter.WriteInt(int32(offsetAtt.StartOffset() + offset))
			mi.postingsWriter.WriteInt(int32(offsetAtt.EndOffset() + offset))
		}
		if mi.storePayloads {
			payload := payloadAtt.GetPayload()
			pIndex := -1
			if len(payload) != 0 {
				pIndex = mi.payloadsBytesRefs.Append(util.NewBytesRef(payload))
			}
			mi.postingsWriter.WriteInt(int32(pIndex))
		}
		inf.sliceArray.end[ord] = mi.postingsWriter.CurrentOffset()
	}
	if err := stream.End(); err != nil {
		return err
	}
	if inf.numTokens > 0 {
		inf.lastPosition = pos
		inf.lastOffset = offsetAtt.EndOffset() + offset
	}
	return nil
}

// SetSimilarity sets the Similarity to be used for calculating field norms.
//
// Renders `public void setSimilarity(Similarity similarity)` (MemoryIndex.java:1004).
func (mi *MemoryIndex) SetSimilarity(similarity search.Similarity) error {
	if mi.frozen {
		return fmt.Errorf("Cannot set Similarity when MemoryIndex is frozen")
	}
	if mi.normSimilarity == similarity {
		return nil
	}
	mi.normSimilarity = similarity
	// invalidate any cached norms that may exist
	for _, inf := range mi.fields {
		inf.norm = nil
	}
	return nil
}

// CreateSearcher creates and returns a searcher that can be used to execute
// arbitrary Lucene queries and to collect the resulting query results as hits.
//
// Renders `public IndexSearcher createSearcher()` (MemoryIndex.java:1023).
func (mi *MemoryIndex) CreateSearcher() *search.IndexSearcher {
	reader := newMemoryIndexReader(mi)
	searcher := search.NewIndexSearcher(reader) // ensures no auto-close !!
	searcher.SetSimilarity(mi.normSimilarity)
	searcher.SetQueryCache(nil)
	return searcher
}

// Freeze prepares the MemoryIndex for querying in a non-lazy way.
//
// After calling this you can query the MemoryIndex from multiple threads, but
// you cannot subsequently add new data.
//
// Renders `public void freeze()` (MemoryIndex.java:1037).
func (mi *MemoryIndex) Freeze() {
	mi.frozen = true
	for _, name := range mi.sortedFieldNames() {
		mi.fields[name].freeze(mi)
	}
}

// memoryIndexScoreCollector renders the anonymous SimpleCollector of
// MemoryIndex.java:1066: it keeps the score of the single collected document.
type memoryIndexScoreCollector struct {
	*search.BaseSimpleCollector
	// BaseLeafCollector carries the two LeafCollector defaults — finish() and
	// competitiveIterator() — that Java's SimpleCollector inherits from the
	// LeafCollector interface.
	*search.BaseLeafCollector
	scores *[1]float32
	scorer search.Scorable
}

// CollectRange renders the default body of LeafCollector.collectRange(int, int),
// which dispatches back to the abstract collect(int); the free function takes
// the concrete collector because an embedded Go struct cannot reach it.
func (c *memoryIndexScoreCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream renders the default body of
// LeafCollector.collect(DocIdStream). See CollectRange.
func (c *memoryIndexScoreCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// Collect renders `public void collect(int doc)` (MemoryIndex.java:1071).
func (c *memoryIndexScoreCollector) Collect(doc int) error {
	score, err := c.scorer.Score()
	if err != nil {
		return err
	}
	c.scores[0] = score
	return nil
}

// SetScorer renders `public void setScorer(Scorable scorer)` (MemoryIndex.java:1076).
func (c *memoryIndexScoreCollector) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	return nil
}

// ScoreMode renders `public ScoreMode scoreMode()` (MemoryIndex.java:1081).
func (c *memoryIndexScoreCollector) ScoreMode() search.ScoreMode {
	return search.ScoreModeComplete
}

// memoryIndexScoreCollectorManager renders the anonymous CollectorManager of
// MemoryIndex.java:1058.
type memoryIndexScoreCollectorManager struct {
	scores *[1]float32 // inits to 0.0f (no match)
}

// NewCollector renders `public Collector newCollector()` (MemoryIndex.java:1064).
func (m *memoryIndexScoreCollectorManager) NewCollector() (search.Collector, error) {
	c := &memoryIndexScoreCollector{
		BaseSimpleCollector: &search.BaseSimpleCollector{},
		BaseLeafCollector:   search.NewBaseLeafCollector(),
		scores:              m.scores,
	}
	// BaseSimpleCollector.Outer renders Java's `return this` in
	// SimpleCollector.getLeafCollector(LeafReaderContext).
	c.Outer = c
	return c, nil
}

// Reduce renders `public Float reduce(Collection<Collector> collectors)`
// (MemoryIndex.java:1086).
func (m *memoryIndexScoreCollectorManager) Reduce(_ []search.Collector) (float32, error) {
	return m.scores[0], nil
}

// Search is the convenience method that efficiently returns the relevance
// score by matching this index against the given Lucene query expression.
//
// The result is a number in the range [0.0 .. 1.0], with 0.0 indicating no
// match. The higher the number the better the match.
//
// Renders `public float search(Query query)` (MemoryIndex.java:1052).
func (mi *MemoryIndex) Search(query search.Query) (float32, error) {
	if query == nil {
		return 0, fmt.Errorf("query must not be null")
	}

	searcher := mi.CreateSearcher()
	manager := &memoryIndexScoreCollectorManager{scores: &[1]float32{}}
	return search.SearchWithCollectorManager[search.Collector, float32](searcher, query, manager)
}

// ToStringDebug returns a string representation of the index data for
// debugging purposes.
//
// Renders `public String toStringDebug()` (MemoryIndex.java:1102).
func (mi *MemoryIndex) ToStringDebug() string {
	var result strings.Builder
	sumPositions := 0
	sumTerms := 0
	spare := util.NewBytesRefEmpty()
	var payloadBuilder *util.BytesRef
	if mi.storePayloads {
		payloadBuilder = util.NewBytesRefEmpty()
	}
	for _, fieldName := range mi.sortedFieldNames() {
		inf := mi.fields[fieldName]
		inf.sortTerms()
		result.WriteString(fieldName)
		result.WriteString(":\n")
		sliceArray := inf.sliceArray
		numPositions := 0
		postingsReader := NewSliceReader(mi.slicedIntBlockPool)
		for j := 0; j < inf.terms.Size(); j++ {
			ord := inf.sortedTerms[j]
			inf.terms.Get(ord, spare)
			freq := sliceArray.freq[ord]
			result.WriteString(fmt.Sprintf("\t'%s':%d:", spare.String(), freq))
			postingsReader.Reset(sliceArray.start[ord], sliceArray.end[ord])
			result.WriteString(" [")
			iters := 1
			if mi.storeOffsets {
				iters = 3
			}
			for !postingsReader.EndOfSlice() {
				result.WriteString("(")

				for k := 0; k < iters; k++ {
					result.WriteString(fmt.Sprintf("%d", postingsReader.ReadInt()))
					if k < iters-1 {
						result.WriteString(", ")
					}
				}
				if mi.storePayloads {
					payloadIndex := int(postingsReader.ReadInt())
					if payloadIndex != -1 {
						mi.payloadsBytesRefs.Get(payloadIndex, payloadBuilder)
						result.WriteString(", ")
						result.WriteString(payloadBuilder.String())
					}
				}
				result.WriteString(")")

				if !postingsReader.EndOfSlice() {
					result.WriteString(", ")
				}
			}
			result.WriteString("]")
			result.WriteString("\n")
			numPositions += freq
		}

		result.WriteString(fmt.Sprintf("\tterms=%d", inf.terms.Size()))
		result.WriteString(fmt.Sprintf(", positions=%d", numPositions))
		result.WriteString("\n")
		sumPositions += numPositions
		sumTerms += inf.terms.Size()
	}

	result.WriteString(fmt.Sprintf("\nfields=%d", len(mi.fields)))
	result.WriteString(fmt.Sprintf(", terms=%d", sumTerms))
	result.WriteString(fmt.Sprintf(", positions=%d", sumPositions))
	return result.String()
}

// Reset resets the MemoryIndex to its initial state and recycles all internal
// buffers.
//
// Renders `public void reset()` (MemoryIndex.java:2234).
func (mi *MemoryIndex) Reset() {
	clear(mi.fields)
	mi.normSimilarity = search.GetDefaultSimilarity()
	mi.byteBlockPool.Reset(false, false)     // no need to 0-fill the buffers
	mi.slicedIntBlockPool.Reset(true, false) // here must must 0-fill since we use slices
	if mi.payloadsBytesRefs != nil {
		mi.payloadsBytesRefs.Clear()
	}
	mi.frozen = false
}

// positionIncrementGapOf renders `analyzer.getPositionIncrementGap(fieldName)`.
//
// Gocene's analysis.Analyzer interface declares only TokenStream, Normalize
// and Close, so the gap accessors are reached through the optional-interface
// probe the repository already uses in analysis.AnalyzerWrapper, with Lucene's
// Analyzer default (0) when the analyzer does not declare it.
func positionIncrementGapOf(analyzer analysis.Analyzer, fieldName string) int {
	if gap, ok := analyzer.(interface {
		GetPositionIncrementGap(string) int
	}); ok {
		return gap.GetPositionIncrementGap(fieldName)
	}
	return 0
}

// offsetGapOf renders `analyzer.getOffsetGap(fieldName)`. See
// positionIncrementGapOf; Lucene's Analyzer default is 1.
func offsetGapOf(analyzer analysis.Analyzer, fieldName string) int {
	if gap, ok := analyzer.(interface {
		GetOffsetGap(string) int
	}); ok {
		return gap.GetOffsetGap(fieldName)
	}
	return 1
}

// info is the index data structure for a field; it contains the tokenized term
// texts and their positions.
//
// Renders the private inner class MemoryIndex.Info (MemoryIndex.java:1165). Go
// has no nested types, so it is a package-level type; Java's `private` is
// rendered as Go's unexported spelling.
type info struct {
	fieldInfo *spi.FieldInfo

	// norm is Java's `Long norm`: nil until getNormDocValues computes it.
	norm *int64

	// terms holds the term strings and their positions for this field.
	// Note the unfortunate variable name clash with the Terms type.
	terms *util.BytesRefHash

	sliceArray *SliceByteStartArray

	// sortedTerms holds the terms sorted ascending by term text; computed on
	// demand. Java marks the field transient.
	sortedTerms []int

	// numTokens is the number of added tokens for this field.
	numTokens int

	// numOverlapTokens is the number of overlapping tokens for this field.
	numOverlapTokens int

	sumTotalTermFreq int64

	maxTermFrequency int

	// lastPosition is the last position encountered in this field for multi
	// field support.
	lastPosition int

	// lastOffset is the last offset encountered in this field for multi field
	// support.
	lastOffset int

	bytesRefHashProducer *bytesRefHashDocValuesProducer

	binaryProducer *binaryDocValuesProducer

	numericProducer *numericDocValuesProducer

	preparedDocValuesAndPointValues bool

	storedValues []any

	pointValues []*util.BytesRef

	// floatVectorCount is the number of float vectors added for this field.
	floatVectorCount int

	// floatVectorValues holds the float vectors added for this field.
	floatVectorValues [][]float32

	// byteVectorCount is the number of byte vectors added for this field.
	byteVectorCount int

	// byteVectorValues holds the byte vectors added for this field.
	byteVectorValues [][]byte

	minPackedValue []byte

	maxPackedValue []byte

	pointValuesCount int
}

// newInfo renders `private Info(FieldInfo fieldInfo, ByteBlockPool byteBlockPool)`
// (MemoryIndex.java:1228).
func newInfo(fieldInfo *spi.FieldInfo, byteBlockPool *util.ByteBlockPool) *info {
	inf := &info{fieldInfo: fieldInfo}
	inf.sliceArray = NewSliceByteStartArray(util.DefaultCapacity)
	inf.terms = util.NewBytesRefHashWithCapacity(byteBlockPool, util.DefaultCapacity, inf.sliceArray)

	inf.bytesRefHashProducer = &bytesRefHashDocValuesProducer{}
	inf.binaryProducer = &binaryDocValuesProducer{}
	inf.numericProducer = &numericDocValuesProducer{}
	return inf
}

// freeze renders `void freeze()` (MemoryIndex.java:1234).
func (inf *info) freeze(mi *MemoryIndex) {
	inf.sortTerms()
	inf.prepareDocValuesAndPointValues()
	inf.getNormDocValues(mi)
}

// sortTerms sorts hashed terms into ascending order, reusing memory along the
// way. Note that sorting is lazily delayed until required (often it's not
// required at all). If a sorted view is required then hashing + sort + binary
// search is still faster and smaller than TreeMap usage (which would be an
// alternative and somewhat more elegant approach, apart from more
// sophisticated Tries / prefix trees).
//
// Renders `void sortTerms()` (MemoryIndex.java:1247).
func (inf *info) sortTerms() {
	if inf.sortedTerms == nil {
		inf.sortedTerms = inf.terms.Sort()
	}
}

// prepareDocValuesAndPointValues renders `void prepareDocValuesAndPointValues()`
// (MemoryIndex.java:1253).
func (inf *info) prepareDocValuesAndPointValues() {
	if !inf.preparedDocValuesAndPointValues {
		dvType := inf.fieldInfo.DocValuesType()
		if dvType == spi.DocValuesTypeNumeric || dvType == spi.DocValuesTypeSortedNumeric {
			inf.numericProducer.prepareForUsage()
		}
		if dvType == spi.DocValuesTypeSortedSet {
			inf.bytesRefHashProducer.prepareForUsage()
		}
		if inf.pointValues != nil {
			numDimensions := inf.fieldInfo.PointDimensionCount()
			numBytesPerDimension := inf.fieldInfo.PointNumBytes()
			if numDimensions == 1 {
				// PointInSetQuery.MergePointVisitor expects values to be visited in increasing order,
				// this is a 1d optimization which has to be done here too. Otherwise we emit values
				// out of order which causes mismatches.
				sort.SliceStable(inf.pointValues[0:inf.pointValuesCount], func(a, b int) bool {
					return inf.pointValues[a].BytesRefCompareTo(inf.pointValues[b]) < 0
				})
				inf.minPackedValue = append([]byte(nil), inf.pointValues[0].Bytes...)
				inf.maxPackedValue = append([]byte(nil), inf.pointValues[inf.pointValuesCount-1].Bytes...)
			} else {
				inf.minPackedValue = append([]byte(nil), inf.pointValues[0].Bytes...)
				inf.maxPackedValue = append([]byte(nil), inf.pointValues[0].Bytes...)
				for i := 0; i < inf.pointValuesCount; i++ {
					pointValue := inf.pointValues[i]
					for dim := 0; dim < numDimensions; dim++ {
						offset := dim * numBytesPerDimension
						if bytes.Compare(
							pointValue.Bytes[offset:offset+numBytesPerDimension],
							inf.minPackedValue[offset:offset+numBytesPerDimension]) < 0 {
							copy(inf.minPackedValue[offset:offset+numBytesPerDimension],
								pointValue.Bytes[offset:offset+numBytesPerDimension])
						}
						if bytes.Compare(
							pointValue.Bytes[offset:offset+numBytesPerDimension],
							inf.maxPackedValue[offset:offset+numBytesPerDimension]) > 0 {
							copy(inf.maxPackedValue[offset:offset+numBytesPerDimension],
								pointValue.Bytes[offset:offset+numBytesPerDimension])
						}
					}
				}
			}
		}
		inf.preparedDocValuesAndPointValues = true
	}
}

// getNormDocValues renders `NumericDocValues getNormDocValues()`
// (MemoryIndex.java:1319). Java reaches MemoryIndex.normSimilarity through the
// enclosing instance; Go has no inner classes, so the owner is a parameter.
func (inf *info) getNormDocValues(mi *MemoryIndex) spi.NumericDocValues {
	if inf.norm == nil {
		invertState := index.NewFieldInvertStateFull(
			util.Latest.Major,
			inf.fieldInfo.Name(),
			inf.fieldInfo.GetIndexOptions(),
			inf.lastPosition,
			inf.numTokens,
			inf.numOverlapTokens,
			0,
			inf.maxTermFrequency,
			inf.terms.Size())
		value := mi.normSimilarity.ComputeNormFromInvertState(invertState)
		if debug {
			fmt.Printf("MemoryIndexReader.norms: %s:%d:%d\n", inf.fieldInfo.Name(), value, inf.numTokens)
		}

		inf.norm = &value
	}
	return newSingleValuedNumericDocValues(*inf.norm)
}

// memoryDocValuesIterator renders the private static class
// MemoryIndex.MemoryDocValuesIterator (MemoryIndex.java:1346).
type memoryDocValuesIterator struct {
	doc int
}

func newMemoryDocValuesIterator() *memoryDocValuesIterator {
	return &memoryDocValuesIterator{doc: -1}
}

// advance renders `int advance(int doc)` (MemoryIndex.java:1350).
func (it *memoryDocValuesIterator) advance(doc int) int {
	it.doc = doc
	return it.docID()
}

// nextDoc renders `int nextDoc()` (MemoryIndex.java:1355).
func (it *memoryDocValuesIterator) nextDoc() int {
	it.doc++
	return it.docID()
}

// docID renders `int docId()` (MemoryIndex.java:1360).
func (it *memoryDocValuesIterator) docID() int {
	if it.doc > 0 {
		return spi.NO_MORE_DOCS
	}
	return it.doc
}

// sortedNumericDocValuesOverArray renders the anonymous SortedNumericDocValues
// of `private static SortedNumericDocValues numericDocValues(long[] values, int count)`
// (MemoryIndex.java:1365).
type sortedNumericDocValuesOverArray struct {
	it     *memoryDocValuesIterator
	values []int64
	count  int
	ord    int
}

func newSortedNumericDocValuesOverArray(values []int64, count int) spi.SortedNumericDocValues {
	return &sortedNumericDocValuesOverArray{it: newMemoryDocValuesIterator(), values: values, count: count}
}

func (d *sortedNumericDocValuesOverArray) NextValue() (int64, error) {
	v := d.values[d.ord]
	d.ord++
	return v, nil
}

func (d *sortedNumericDocValuesOverArray) DocValueCount() (int, error) { return d.count, nil }

func (d *sortedNumericDocValuesOverArray) AdvanceExact(target int) (bool, error) {
	d.ord = 0
	return d.it.advance(target) == target, nil
}

func (d *sortedNumericDocValuesOverArray) DocID() int { return d.it.docID() }

func (d *sortedNumericDocValuesOverArray) NextDoc() (int, error) { return d.it.nextDoc(), nil }

func (d *sortedNumericDocValuesOverArray) Advance(target int) (int, error) {
	return d.it.advance(target), nil
}

func (d *sortedNumericDocValuesOverArray) Cost() int64 { return 1 }

// LongValue satisfies Gocene's spi.SortedNumericDocValues, which embeds
// spi.NumericDocValues. Java's SortedNumericDocValues carries no longValue();
// the value at the current ordinal is the faithful answer.
func (d *sortedNumericDocValuesOverArray) LongValue() (int64, error) {
	return d.values[d.ord], nil
}

func (d *sortedNumericDocValuesOverArray) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(d, upTo, bitSet, offset)
}

func (d *sortedNumericDocValuesOverArray) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(d)
}

// singleValuedNumericDocValues renders the anonymous NumericDocValues of
// `private static NumericDocValues numericDocValues(long value)`
// (MemoryIndex.java:1409).
type singleValuedNumericDocValues struct {
	it    *memoryDocValuesIterator
	value int64
}

func newSingleValuedNumericDocValues(value int64) spi.NumericDocValues {
	return &singleValuedNumericDocValues{it: newMemoryDocValuesIterator(), value: value}
}

func (d *singleValuedNumericDocValues) LongValue() (int64, error) { return d.value, nil }

func (d *singleValuedNumericDocValues) AdvanceExact(target int) (bool, error) {
	adv, err := d.Advance(target)
	if err != nil {
		return false, err
	}
	return adv == target, nil
}

func (d *singleValuedNumericDocValues) DocID() int { return d.it.docID() }

func (d *singleValuedNumericDocValues) NextDoc() (int, error) { return d.it.nextDoc(), nil }

func (d *singleValuedNumericDocValues) Advance(target int) (int, error) {
	return d.it.advance(target), nil
}

func (d *singleValuedNumericDocValues) Cost() int64 { return 1 }

func (d *singleValuedNumericDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(d, upTo, bitSet, offset)
}

func (d *singleValuedNumericDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(d)
}

// singleValuedSortedDocValues renders the anonymous SortedDocValues of
// `private static SortedDocValues sortedDocValues(BytesRef value)`
// (MemoryIndex.java:1444).
type singleValuedSortedDocValues struct {
	it    *memoryDocValuesIterator
	value *util.BytesRef
}

func newSingleValuedSortedDocValues(value *util.BytesRef) spi.SortedDocValues {
	return &singleValuedSortedDocValues{it: newMemoryDocValuesIterator(), value: value}
}

func (d *singleValuedSortedDocValues) OrdValue() (int, error) { return 0, nil }

func (d *singleValuedSortedDocValues) LookupOrd(ord int) ([]byte, error) {
	return d.value.ValidBytes(), nil
}

func (d *singleValuedSortedDocValues) GetValueCount() int { return 1 }

func (d *singleValuedSortedDocValues) AdvanceExact(target int) (bool, error) {
	return d.it.advance(target) == target, nil
}

func (d *singleValuedSortedDocValues) DocID() int { return d.it.docID() }

func (d *singleValuedSortedDocValues) NextDoc() (int, error) { return d.it.nextDoc(), nil }

func (d *singleValuedSortedDocValues) Advance(target int) (int, error) {
	return d.it.advance(target), nil
}

func (d *singleValuedSortedDocValues) Cost() int64 { return 1 }

// LongValue satisfies Gocene's spi.SortedDocValues, which embeds
// spi.NumericDocValues. Java's SortedDocValues extends BinaryDocValues and
// carries no longValue(); the current ordinal is the faithful answer.
func (d *singleValuedSortedDocValues) LongValue() (int64, error) { return 0, nil }

func (d *singleValuedSortedDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(d, upTo, bitSet, offset)
}

func (d *singleValuedSortedDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(d)
}

// bytesRefHashSortedSetDocValues renders the anonymous SortedSetDocValues of
// `private static SortedSetDocValues sortedSetDocValues(BytesRefHash values, int[] bytesIds)`
// (MemoryIndex.java:1489).
type bytesRefHashSortedSetDocValues struct {
	it       *memoryDocValuesIterator
	values   *util.BytesRefHash
	bytesIds []int
	scratch  *util.BytesRef
	ord      int
}

func newBytesRefHashSortedSetDocValues(values *util.BytesRefHash, bytesIds []int) spi.SortedSetDocValues {
	return &bytesRefHashSortedSetDocValues{
		it:       newMemoryDocValuesIterator(),
		values:   values,
		bytesIds: bytesIds,
		scratch:  util.NewBytesRefEmpty(),
	}
}

func (d *bytesRefHashSortedSetDocValues) NextOrd() (int, error) {
	o := d.ord
	d.ord++
	return o, nil
}

func (d *bytesRefHashSortedSetDocValues) DocValueCount() int { return d.values.Size() }

func (d *bytesRefHashSortedSetDocValues) LookupOrd(ord int) ([]byte, error) {
	d.values.Get(d.bytesIds[ord], d.scratch)
	return d.scratch.ValidBytes(), nil
}

func (d *bytesRefHashSortedSetDocValues) GetValueCount() int { return d.values.Size() }

func (d *bytesRefHashSortedSetDocValues) AdvanceExact(target int) (bool, error) {
	d.ord = 0
	return d.it.advance(target) == target, nil
}

func (d *bytesRefHashSortedSetDocValues) DocID() int { return d.it.docID() }

func (d *bytesRefHashSortedSetDocValues) NextDoc() (int, error) { return d.it.nextDoc(), nil }

func (d *bytesRefHashSortedSetDocValues) Advance(target int) (int, error) {
	return d.it.advance(target), nil
}

func (d *bytesRefHashSortedSetDocValues) Cost() int64 { return 1 }

func (d *bytesRefHashSortedSetDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(d, upTo, bitSet, offset)
}

func (d *bytesRefHashSortedSetDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(d)
}

// binaryDocValuesProducer renders the private static final class
// MemoryIndex.BinaryDocValuesProducer (MemoryIndex.java:1543).
type binaryDocValuesProducer struct {
	dvBytesValuesSet *util.BytesRef
}

// bytesRefHashDocValuesProducer renders the private static final class
// MemoryIndex.BytesRefHashDocValuesProducer (MemoryIndex.java:1547).
type bytesRefHashDocValuesProducer struct {
	dvBytesRefHashValuesSet *util.BytesRefHash
	bytesIds                []int
}

// prepareForUsage renders `private void prepareForUsage()` (MemoryIndex.java:1552).
func (p *bytesRefHashDocValuesProducer) prepareForUsage() {
	p.bytesIds = p.dvBytesRefHashValuesSet.Sort()
}

// numericDocValuesProducer renders the private static final class
// MemoryIndex.NumericDocValuesProducer (MemoryIndex.java:1557).
type numericDocValuesProducer struct {
	dvLongValues []int64
	count        int
}

// prepareForUsage renders `private void prepareForUsage()` (MemoryIndex.java:1562).
func (p *numericDocValuesProducer) prepareForUsage() {
	sort.Slice(p.dvLongValues[0:p.count], func(a, b int) bool {
		return p.dvLongValues[a] < p.dvLongValues[b]
	})
}

// SliceByteStartArray renders the private static final class
// MemoryIndex.SliceByteStartArray (MemoryIndex.java:2245), which extends
// BytesRefHash.DirectBytesStartArray. It is exported because it is the
// BytesStartArray handed to util.NewBytesRefHashWithCapacity.
type SliceByteStartArray struct {
	*util.DirectBytesStartArray
	// start is the start offset in the IntBlockPool per term.
	start []int
	// end is the end pointer in the IntBlockPool for the postings slice per term.
	end []int
	// freq is the term frequency.
	freq []int
}

// NewSliceByteStartArray renders `public SliceByteStartArray(int initSize)`
// (MemoryIndex.java:2250).
func NewSliceByteStartArray(initSize int) *SliceByteStartArray {
	return &SliceByteStartArray{DirectBytesStartArray: util.NewDirectBytesStartArray(initSize)}
}

// Init renders `public int[] init()` (MemoryIndex.java:2255).
func (a *SliceByteStartArray) Init() []int {
	ord := a.DirectBytesStartArray.Init()
	a.start = make([]int, util.Oversize(len(ord), 4))
	a.end = make([]int, util.Oversize(len(ord), 4))
	a.freq = make([]int, util.Oversize(len(ord), 4))
	return ord
}

// Grow renders `public int[] grow()` (MemoryIndex.java:2267).
func (a *SliceByteStartArray) Grow() []int {
	ord := a.DirectBytesStartArray.Grow()
	if len(a.start) < len(ord) {
		// ArrayUtil.grow(int[], int)
		a.start = util.GrowExact(a.start, util.Oversize(len(ord), 4))
		a.end = util.GrowExact(a.end, util.Oversize(len(ord), 4))
		a.freq = util.GrowExact(a.freq, util.Oversize(len(ord), 4))
	}
	return ord
}

// Clear renders `public int[] clear()` (MemoryIndex.java:2281).
func (a *SliceByteStartArray) Clear() []int {
	a.start = nil
	a.end = nil
	return a.DirectBytesStartArray.Clear()
}
