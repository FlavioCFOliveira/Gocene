// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// CodecReader is a LeafReader implemented by codec APIs.
// This is the Go port of Lucene's org.apache.lucene.index.CodecReader.
//
// In Lucene, this is an abstract class. In Gocene, we use an interface for the
// abstract methods and a base struct (baseCodecReader) to provide the common
// LeafReader implementation.
type CodecReader interface {
	LeafReader

	// GetFieldsReader retrieves the underlying StoredFieldsReader.
	GetFieldsReader() StoredFieldsReader

	// GetTermVectorsReader retrieves the underlying TermVectorsReader.
	GetTermVectorsReader() TermVectorsReader

	// GetNormsReader retrieves the underlying NormsProducer.
	GetNormsReader() NormsProducer

	// GetDocValuesReader retrieves the underlying DocValuesProducer.
	GetDocValuesReader() DocValuesProducer

	// GetPostingsReader retrieves the underlying FieldsProducer.
	GetPostingsReader() FieldsProducer

	// GetPointsReader retrieves the underlying PointsReader.
	GetPointsReader() PointsReader

	// GetVectorReader retrieves the underlying KnnVectorsReader.
	GetVectorReader() KnnVectorsReader

	// GetSegmentInfo retrieves the SegmentInfo backing this reader.
	//
	// PORT NOTE: Lucene declares getSegmentInfo() on SegmentReader rather than on
	// CodecReader. Gocene lifts it onto the codec-reader contract because the merge
	// and wrapper paths (SlowCodecReaderWrapper, FilterCodecReader, SegmentMerger)
	// consume it through this interface.
	GetSegmentInfo() *SegmentInfo
}

// pointsReaderWithValues is the wide read surface Lucene's
// org.apache.lucene.codecs.PointsReader exposes via getValues(String).
// spi.PointsReader deliberately carries only the integrity/close hooks because
// spi.PointValues lives in package index and cannot be lifted into the SPI without
// an import cycle (see the note on spi.PointsReader), so the wide surface is
// recovered here by assertion.
type pointsReaderWithValues interface {
	GetValues(field string) (spi.PointValues, error)
}

// knnVectorsReaderWithValues is the wide read surface Lucene's
// org.apache.lucene.codecs.KnnVectorsReader exposes. spi.KnnVectorsReader keeps
// only the integrity/close hooks for the same import-cycle reason as
// pointsReaderWithValues.
type knnVectorsReaderWithValues interface {
	GetFloatVectorValues(field string) (FloatVectorValues, error)
	GetByteVectorValues(field string) (ByteVectorValues, error)
}

// knnVectorsReaderWithSearch is the nearest-neighbour search half of the wide
// KnnVectorsReader surface.
type knnVectorsReaderWithSearch interface {
	Search(field string, target []float32, k int, acceptDocs util.Bits) (TopDocs, error)
	SearchByte(field string, target []byte, k int, acceptDocs util.Bits) (TopDocs, error)
}

// baseCodecReader provides the common implementation of LeafReader methods
// by delegating to the CodecReader interface.
type baseCodecReader struct {
	impl CodecReader
}

// NewBaseCodecReader creates a baseCodecReader wrapping the given CodecReader implementation.
func NewBaseCodecReader(impl CodecReader) *baseCodecReader {
	return &baseCodecReader{impl: impl}
}

func (b *baseCodecReader) StoredFields() (StoredFields, error) {
	reader := b.impl.GetFieldsReader()
	if reader == nil {
		return NewEmptyStoredFields(), nil
	}
	return &storedFieldsWrapper{
		reader: reader,
		maxDoc: b.impl.MaxDoc(),
	}, nil
}

type storedFieldsWrapper struct {
	reader StoredFieldsReader
	maxDoc int
}

// Prefetch hints that the stored fields of the given documents will be read
// soon. Lucene's StoredFields.prefetch takes a single docID and is a no-op
// unless the codec reader overrides it; spi.StoredFields batches the hint, so
// the wrapper forwards one call per document to the reader's override.
func (w *storedFieldsWrapper) Prefetch(docIDs []int) error {
	pf, ok := w.reader.(interface{ Prefetch(docID int) error })
	for _, docID := range docIDs {
		if docID < 0 || docID >= w.maxDoc {
			return fmt.Errorf("docID %d out of range [0, %d)", docID, w.maxDoc)
		}
		if !ok {
			continue
		}
		if err := pf.Prefetch(docID); err != nil {
			return err
		}
	}
	return nil
}

func (w *storedFieldsWrapper) Document(docID int, visitor StoredFieldVisitor) error {
	if docID < 0 || docID >= w.maxDoc {
		return fmt.Errorf("docID %d out of range [0, %d)", docID, w.maxDoc)
	}
	return w.reader.VisitDocument(docID, visitor)
}

func (b *baseCodecReader) TermVectors() (TermVectors, error) {
	reader := b.impl.GetTermVectorsReader()
	if reader == nil {
		return NewEmptyTermVectors(), nil
	}
	return &termVectorsWrapper{reader: reader}, nil
}

// termVectorsWrapper adapts a codec TermVectorsReader to the TermVectors surface.
// Lucene returns the reader directly because TermVectorsReader extends TermVectors;
// spi.TermVectorsReader omits the prefetch hook (a no-op by default in Lucene), so
// the wrapper supplies it and forwards to the reader when the codec overrides it.
type termVectorsWrapper struct {
	reader TermVectorsReader
}

// Prefetch hints that the term vectors of the given documents will be read
// soon. Lucene's TermVectors.prefetch takes a single docID and is a no-op
// unless the codec reader overrides it; spi.TermVectors batches the hint, so
// the wrapper forwards one call per document to the reader's override.
func (w *termVectorsWrapper) Prefetch(docIDs []int) error {
	pf, ok := w.reader.(interface{ Prefetch(docID int) error })
	if !ok {
		return nil
	}
	for _, docID := range docIDs {
		if err := pf.Prefetch(docID); err != nil {
			return err
		}
	}
	return nil
}

func (w *termVectorsWrapper) Get(docID int) (Fields, error) {
	return w.reader.Get(docID)
}

func (w *termVectorsWrapper) GetField(docID int, field string) (Terms, error) {
	return w.reader.GetField(docID, field)
}

func (b *baseCodecReader) Terms(field string) (Terms, error) {
	fi := b.impl.GetFieldInfos().FieldInfoByName(field)
	if fi == nil || fi.IndexOptions() == IndexOptionsNone {
		// Field does not exist or does not index postings
		return nil, nil
	}
	return b.impl.GetPostingsReader().Terms(field)
}

func (b *baseCodecReader) GetNumericDocValues(field string) (NumericDocValues, error) {
	fi := b.getDVField(field, DocValuesTypeNumeric)
	if fi == nil {
		return nil, nil
	}
	return b.impl.GetDocValuesReader().GetNumeric(fi)
}

func (b *baseCodecReader) GetBinaryDocValues(field string) (BinaryDocValues, error) {
	fi := b.getDVField(field, DocValuesTypeBinary)
	if fi == nil {
		return nil, nil
	}
	return b.impl.GetDocValuesReader().GetBinary(fi)
}

func (b *baseCodecReader) GetSortedDocValues(field string) (SortedDocValues, error) {
	fi := b.getDVField(field, DocValuesTypeSorted)
	if fi == nil {
		return nil, nil
	}
	return b.impl.GetDocValuesReader().GetSorted(fi)
}

func (b *baseCodecReader) GetSortedNumericDocValues(field string) (SortedNumericDocValues, error) {
	fi := b.getDVField(field, DocValuesTypeSortedNumeric)
	if fi == nil {
		return nil, nil
	}
	return b.impl.GetDocValuesReader().GetSortedNumeric(fi)
}

func (b *baseCodecReader) GetSortedSetDocValues(field string) (SortedSetDocValues, error) {
	fi := b.getDVField(field, DocValuesTypeSortedSet)
	if fi == nil {
		return nil, nil
	}
	return b.impl.GetDocValuesReader().GetSortedSet(fi)
}

func (b *baseCodecReader) GetDocValuesSkipper(field string) (spi.DocValuesSkipper, error) {
	fi := b.impl.GetFieldInfos().FieldInfoByName(field)
	if fi == nil || fi.DocValuesSkipIndexType() == spi.DocValuesSkipIndexTypeNone {
		return nil, nil
	}
	return b.impl.GetDocValuesReader().GetSkipper(fi)
}

func (b *baseCodecReader) GetNormValues(field string) (NumericDocValues, error) {
	fi := b.impl.GetFieldInfos().FieldInfoByName(field)
	if fi == nil || !fi.HasNorms() {
		// Field does not exist or does not index norms
		return nil, nil
	}
	return b.impl.GetNormsReader().GetNorms(fi)
}

func (b *baseCodecReader) GetPointValues(field string) (spi.PointValues, error) {
	fi := b.impl.GetFieldInfos().FieldInfoByName(field)
	if fi == nil || fi.PointDimensionCount() == 0 {
		// Field does not exist or does not index points
		return nil, nil
	}
	reader, ok := b.impl.GetPointsReader().(pointsReaderWithValues)
	if !ok {
		return nil, fmt.Errorf("points reader %T does not expose GetValues", b.impl.GetPointsReader())
	}
	return reader.GetValues(field)
}

func (b *baseCodecReader) GetFloatVectorValues(field string) (FloatVectorValues, error) {
	fi := b.impl.GetFieldInfos().FieldInfoByName(field)
	if fi == nil || fi.VectorDimension() == 0 || fi.VectorEncoding() != VectorEncodingFloat32 {
		// Field does not exist or does not index vectors
		return nil, nil
	}
	reader, ok := b.impl.GetVectorReader().(knnVectorsReaderWithValues)
	if !ok {
		return nil, fmt.Errorf("vector reader %T does not expose GetFloatVectorValues", b.impl.GetVectorReader())
	}
	return reader.GetFloatVectorValues(field)
}

func (b *baseCodecReader) GetByteVectorValues(field string) (ByteVectorValues, error) {
	fi := b.impl.GetFieldInfos().FieldInfoByName(field)
	if fi == nil || fi.VectorDimension() == 0 || fi.VectorEncoding() != VectorEncodingByte {
		// Field does not exist or does not index vectors
		return nil, nil
	}
	reader, ok := b.impl.GetVectorReader().(knnVectorsReaderWithValues)
	if !ok {
		return nil, fmt.Errorf("vector reader %T does not expose GetByteVectorValues", b.impl.GetVectorReader())
	}
	return reader.GetByteVectorValues(field)
}

func (b *baseCodecReader) SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits) (TopDocs, error) {
	fi := b.impl.GetFieldInfos().FieldInfoByName(field)
	if fi == nil || fi.VectorDimension() == 0 || fi.VectorEncoding() != VectorEncodingFloat32 {
		// Field does not exist or does not index vectors
		return TopDocs{}, nil
	}
	reader, ok := b.impl.GetVectorReader().(knnVectorsReaderWithSearch)
	if !ok {
		return TopDocs{}, fmt.Errorf("vector reader %T does not expose Search", b.impl.GetVectorReader())
	}
	return reader.Search(field, target, k, acceptDocs)
}

func (b *baseCodecReader) SearchNearestVectorsByte(field string, target []byte, k int, acceptDocs util.Bits) (TopDocs, error) {
	fi := b.impl.GetFieldInfos().FieldInfoByName(field)
	if fi == nil || fi.VectorDimension() == 0 || fi.VectorEncoding() != VectorEncodingByte {
		// Field does not exist or does not index vectors
		return TopDocs{}, nil
	}
	reader, ok := b.impl.GetVectorReader().(knnVectorsReaderWithSearch)
	if !ok {
		return TopDocs{}, fmt.Errorf("vector reader %T does not expose SearchByte", b.impl.GetVectorReader())
	}
	return reader.SearchByte(field, target, k, acceptDocs)
}

func (b *baseCodecReader) CheckIntegrity() error {
	if b.impl.GetPostingsReader() != nil {
		if err := b.impl.GetPostingsReader().CheckIntegrity(); err != nil {
			return err
		}
	}
	if b.impl.GetNormsReader() != nil {
		if err := b.impl.GetNormsReader().CheckIntegrity(); err != nil {
			return err
		}
	}
	if b.impl.GetDocValuesReader() != nil {
		if err := b.impl.GetDocValuesReader().CheckIntegrity(); err != nil {
			return err
		}
	}
	if b.impl.GetFieldsReader() != nil {
		if err := b.impl.GetFieldsReader().CheckIntegrity(); err != nil {
			return err
		}
	}
	if b.impl.GetTermVectorsReader() != nil {
		if err := b.impl.GetTermVectorsReader().CheckIntegrity(); err != nil {
			return err
		}
	}
	if b.impl.GetPointsReader() != nil {
		if err := b.impl.GetPointsReader().CheckIntegrity(); err != nil {
			return err
		}
	}
	if b.impl.GetVectorReader() != nil {
		if err := b.impl.GetVectorReader().CheckIntegrity(); err != nil {
			return err
		}
	}
	return nil
}

func (b *baseCodecReader) getDVField(field string, dvType DocValuesType) *FieldInfo {
	fi := b.impl.GetFieldInfos().FieldInfoByName(field)
	if fi == nil {
		return nil
	}
	if fi.DocValuesType() == DocValuesTypeNone {
		return nil
	}
	if fi.DocValuesType() != dvType {
		return nil
	}
	return fi
}

// Default methods for LeafReader that baseCodecReader must implement if it's to be used as one.
// These are usually provided by the implementation of LeafReader (like SegmentReader)
// but since baseCodecReader is the base for CodecReader, it should handle them.

func (b *baseCodecReader) DocID() int {
	return b.impl.DocID()
}

func (b *baseCodecReader) MaxDoc() int {
	return b.impl.MaxDoc()
}

func (b *baseCodecReader) DocFreq(term Term) (int, error) {
	return b.impl.DocFreq(term)
}

func (b *baseCodecReader) TotalTermFreq(term Term) (int64, error) {
	return b.impl.TotalTermFreq(term)
}

func (b *baseCodecReader) Postings(term Term, flags int) (PostingsEnum, error) {
	return b.impl.Postings(term, flags)
}

func (b *baseCodecReader) GetLiveDocs() util.Bits {
	return b.impl.GetLiveDocs()
}

func (b *baseCodecReader) GetMetaData() *IndexReaderMetaData {
	return b.impl.GetMetaData()
}

func (b *baseCodecReader) GetSegmentInfo() *SegmentInfo {
	return b.impl.GetSegmentInfo()
}

func (b *baseCodecReader) Close() error {
	return b.impl.Close()
}

func (b *baseCodecReader) IncRef() error {
	return b.impl.IncRef()
}

func (b *baseCodecReader) DecRef() error {
	return b.impl.DecRef()
}

func (b *baseCodecReader) TryIncRef() bool {
	return b.impl.TryIncRef()
}

func (b *baseCodecReader) GetRefCount() int32 {
	return b.impl.GetRefCount()
}

func (b *baseCodecReader) GetContext() (IndexReaderContext, error) {
	return b.impl.GetContext()
}
