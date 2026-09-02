//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"

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

func (w *storedFieldsWrapper) Prefetch(docID int) error {
	if docID < 0 || docID >= w.maxDoc {
		return fmt.Errorf("docID %d out of range [0, %d)", docID, w.maxDoc)
	}
	return w.reader.Prefetch(docID)
}

func (w *storedFieldsWrapper) Document(docID int, visitor StoredFieldVisitor) error {
	if docID < 0 || docID >= w.maxDoc {
		return fmt.Errorf("docID %d out of range [0, %d)", docID, w.maxDoc)
	}
	return w.reader.Document(docID, visitor)
}

func (b *baseCodecReader) TermVectors() (TermVectors, error) {
	reader := b.impl.GetTermVectorsReader()
	if reader == nil {
		return NewEmptyTermVectors(), nil
	}
	return reader, nil
}

func (b *baseCodecReader) Terms(field string) (Terms, error) {
	fi := b.impl.GetFieldInfos().FieldInfo(field)
	if fi == nil || fi.IndexOptions == IndexOptionsNone {
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
	return b.impl.GetDocValuesReader().GetNumeric(fi), nil
}

func (b *baseCodecReader) GetBinaryDocValues(field string) (BinaryDocValues, error) {
	fi := b.getDVField(field, DocValuesTypeBinary)
	if fi == nil {
		return nil, nil
	}
	return b.impl.GetDocValuesReader().GetBinary(fi), nil
}

func (b *baseCodecReader) GetSortedDocValues(field string) (SortedDocValues, error) {
	fi := b.getDVField(field, DocValuesTypeSorted)
	if fi == nil {
		return nil, nil
	}
	return b.impl.GetDocValuesReader().GetSorted(fi), nil
}

func (b *baseCodecReader) GetSortedNumericDocValues(field string) (SortedNumericDocValues, error) {
	fi := b.getDVField(field, DocValuesTypeSortedNumeric)
	if fi == nil {
		return nil, nil
	}
	return b.impl.GetDocValuesReader().GetSortedNumeric(fi), nil
}

func (b *baseCodecReader) GetSortedSetDocValues(field string) (SortedSetDocValues, error) {
	fi := b.getDVField(field, DocValuesTypeSortedSet)
	if fi == nil {
		return nil, nil
	}
	return b.impl.GetDocValuesReader().GetSortedSet(fi), nil
}

func (b *baseCodecReader) GetDocValuesSkipper(field string) (DocValuesSkipper, error) {
	fi := b.impl.GetFieldInfos().FieldInfo(field)
	if fi == nil || fi.DocValuesSkipIndexType == DocValuesSkipIndexTypeNone {
		return nil, nil
	}
	return b.impl.GetDocValuesReader().GetSkipper(fi), nil
}

func (b *baseCodecReader) GetNormValues(field string) (NumericDocValues, error) {
	fi := b.impl.GetFieldInfos().FieldInfo(field)
	if fi == nil || !fi.HasNorms() {
		// Field does not exist or does not index norms
		return nil, nil
	}
	return b.impl.GetNormsReader().GetNorms(fi), nil
}

func (b *baseCodecReader) GetPointValues(field string) (PointValues, error) {
	fi := b.impl.GetFieldInfos().FieldInfo(field)
	if fi == nil || fi.PointDimensionCount == 0 {
		// Field does not exist or does not index points
		return nil, nil
	}
	return b.impl.GetPointsReader().GetValues(field), nil
}

func (b *baseCodecReader) GetFloatVectorValues(field string) (FloatVectorValues, error) {
	fi := b.impl.GetFieldInfos().FieldInfo(field)
	if fi == nil || fi.VectorDimension == 0 || fi.VectorEncoding != VectorEncodingFloat32 {
		// Field does not exist or does not index vectors
		return nil, nil
	}
	return b.impl.GetVectorReader().GetFloatVectorValues(field), nil
}

func (b *baseCodecReader) GetByteVectorValues(field string) (ByteVectorValues, error) {
	fi := b.impl.GetFieldInfos().FieldInfo(field)
	if fi == nil || fi.VectorDimension == 0 || fi.VectorEncoding != VectorEncodingByte {
		// Field does not exist or does not index vectors
		return nil, nil
	}
	return b.impl.GetVectorReader().GetByteVectorValues(field), nil
}

func (b *baseCodecReader) SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits) (TopDocs, error) {
	fi := b.impl.GetFieldInfos().FieldInfo(field)
	if fi == nil || fi.VectorDimension == 0 || fi.VectorEncoding != VectorEncodingFloat32 {
		// Field does not exist or does not index vectors
		return TopDocs{}, nil
	}
	return b.impl.GetVectorReader().Search(field, target, k, acceptDocs)
}

func (b *baseCodecReader) SearchNearestVectorsByte(field string, target []byte, k int, acceptDocs util.Bits) (TopDocs, error) {
	fi := b.impl.GetFieldInfos().FieldInfo(field)
	if fi == nil || fi.VectorDimension == 0 || fi.VectorEncoding != VectorEncodingByte {
		// Field does not exist or does not index vectors
		return TopDocs{}, nil
	}
	return b.impl.GetVectorReader().SearchByte(field, target, k, acceptDocs)
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
	fi := b.impl.GetFieldInfos().FieldInfo(field)
	if fi == nil {
		return nil
	}
	if fi.DocValuesType == DocValuesTypeNone {
		return nil
	}
	if fi.DocValuesType != dvType {
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
