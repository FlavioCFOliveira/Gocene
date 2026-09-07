// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SegmentReader is an IndexReader implementation over a single segment.
//
// This is the Go port of Lucene's org.apache.lucene.index.SegmentReader from Apache Lucene 10.5.0.
type SegmentReader struct {
	si             *SegmentCommitInfo
	originalSi     *SegmentCommitInfo
	metaData       *IndexReaderMetaData
	liveDocs       util.Bits
	hardLiveDocs   util.Bits
	numDocs        int
	core           *SegmentCoreReaders
	segDocValues   *SegmentDocValues
	isNRT          bool
	docValuesProducer DocValuesProducer
	fieldInfos     *FieldInfos
}

// NewSegmentReader constructs a new SegmentReader with a new core.
func NewSegmentReader(si *SegmentCommitInfo, createdVersionMajor int, context store.IOContext) (*SegmentReader, error) {
	siClone := si.Clone()
	originalSi := si

	metaData := &IndexReaderMetaData{
		HasDeletions: siClone.HasDeletions(),
		NumDocs:      siClone.SegmentInfo().MaxDoc() - siClone.GetDelCount(),
		MaxDoc:       siClone.SegmentInfo().MaxDoc(),
	}

	core, err := NewSegmentCoreReaders(siClone.SegmentInfo().Dir(), siClone, context)
	if err != nil {
		return nil, err
	}

	// Inject the DocValues producer factory into SegmentDocValues
	segDocValues, err := NewSegmentDocValues(func(si *SegmentCommitInfo, dir store.Directory, gen int64, infos *FieldInfos) (DocValuesProducer, error) {
		codec := LookupCodecByName(si.SegmentInfo().Codec())
		return codec.DocValuesFormat().FieldsProducer(si, dir, gen, infos)
	})
	if err != nil {
		core.DecRef()
		return nil, err
	}

	var liveDocs util.Bits
	var hardLiveDocs util.Bits
	isNRT := false

	codec := LookupCodecByName(siClone.SegmentInfo().Codec())
	if siClone.HasDeletions() {
		// NOTE: the bitvector is stored using the regular directory, not cfs
		ld, err := codec.LiveDocsFormat().ReadLiveDocs(siClone.SegmentInfo().Dir(), siClone, store.IOContextReadOnce)
		if err != nil {
			segDocValues.DecRef([]int64{-1})
			core.DecRef()
			return nil, err
		}
		liveDocs = ld
		hardLiveDocs = ld
	}

	numDocs := siClone.SegmentInfo().MaxDoc() - siClone.GetDelCount()

	sr := &SegmentReader{
		si:           siClone,
		originalSi:   originalSi,
		metaData:     metaData,
		liveDocs:     liveDocs,
		hardLiveDocs: hardLiveDocs,
		numDocs:      numDocs,
		core:         core,
		segDocValues: segDocValues,
		isNRT:        isNRT,
	}

	if err := sr.initFieldInfos(); err != nil {
		sr.doClose()
		return nil, err
	}

	if err := sr.initDocValuesProducer(); err != nil {
		sr.doClose()
		return nil, err
	}

	return sr, nil
}

// NewSegmentReaderFrom creates a new SegmentReader sharing core from a previous SegmentReader
// and using the provided liveDocs, and recording whether those liveDocs were carried in ram (isNRT=true).
func NewSegmentReaderFrom(si *SegmentCommitInfo, sr *SegmentReader, liveDocs util.Bits, hardLiveDocs util.Bits, numDocs int, isNRT bool) (*SegmentReader, error) {
	if numDocs > si.SegmentInfo().MaxDoc() {
		return nil, fmt.Errorf("numDocs=%d but maxDoc=%d", numDocs, si.SegmentInfo().MaxDoc())
	}
	if liveDocs != nil && liveDocs.Length() != si.SegmentInfo().MaxDoc() {
		return nil, fmt.Errorf("maxDoc=%d but liveDocs.size()=%d", si.SegmentInfo().MaxDoc(), liveDocs.Length())
	}

	siClone := si.Clone()
	originalSi := si

	if err := sr.core.IncRef(); err != nil {
		return nil, err
	}

	srNew := &SegmentReader{
		si:           siClone,
		originalSi:   originalSi,
		metaData:     sr.metaData,
		liveDocs:     liveDocs,
		hardLiveDocs: hardLiveDocs,
		isNRT:        isNRT,
		numDocs:      numDocs,
		core:         sr.core,
		segDocValues: sr.segDocValues,
	}

	if err := srNew.initFieldInfos(); err != nil {
		srNew.doClose()
		return nil, err
	}

	if err := srNew.initDocValuesProducer(); err != nil {
		srNew.doClose()
		return nil, err
	}

	return srNew, nil
}

func (sr *SegmentReader) initFieldInfos() error {
	if !sr.si.HasFieldUpdates() {
		sr.fieldInfos = sr.core.GetFieldInfos()
		return nil
	}

	codec := LookupCodecByName(sr.si.SegmentInfo().Codec())
	segmentSuffix := fmt.Sprintf("%d", sr.si.GetFieldInfosGen())

	fi, err := codec.FieldInfosFormat().Read(sr.si.SegmentInfo().Dir(), sr.si.SegmentInfo(), segmentSuffix, store.IOContextReadOnce)
	if err != nil {
		return err
	}
	sr.fieldInfos = fi
	return nil
}

func (sr *SegmentReader) initDocValuesProducer() error {
	if !sr.fieldInfos.HasDocValues() {
		sr.docValuesProducer = nil
		return nil
	}

	var dir store.Directory
	if sr.core.GetCompoundDirectory() != nil {
		dir = sr.core.GetCompoundDirectory()
	} else {
		dir = sr.si.SegmentInfo().Dir()
	}

	if sr.si.HasFieldUpdates() {
		producer, err := NewSegmentDocValuesProducer(sr.si, dir, sr.core.GetFieldInfos(), sr.fieldInfos, sr.segDocValues)
		if err != nil {
			return err
		}
		sr.docValuesProducer = producer
	} else {
		dv, err := sr.segDocValues.GetDocValuesProducer(-1, sr.si, dir, sr.fieldInfos)
		if err != nil {
			return err
		}
		sr.docValuesProducer = dv
	}
	return nil
}

func (sr *SegmentReader) doClose() error {
	var err error
	if decErr := sr.core.DecRef(); decErr != nil {
		err = decErr
	}

	if producer, ok := sr.docValuesProducer.(*SegmentDocValuesProducer); ok {
		if decErr := sr.segDocValues.DecRef(producer.Generations()); decErr != nil {
			err = decErr
		}
	} else if sr.docValuesProducer != nil {
		if decErr := sr.segDocValues.DecRef([]int64{-1}); decErr != nil {
			err = decErr
		}
	}
	return err
}

// --- CodecReader implementation ---

func (sr *SegmentReader) GetFieldsReader() StoredFieldsReader {
	return sr.core.GetStoredFieldsReader()
}

func (sr *SegmentReader) GetTermVectorsReader() TermVectorsReader {
	return sr.core.GetTermVectorsReader()
}

func (sr *SegmentReader) GetNormsReader() NormsProducer {
	return sr.core.GetNormsProducer()
}

func (sr *SegmentReader) GetDocValuesReader() DocValuesProducer {
	return sr.docValuesProducer
}

func (sr *SegmentReader) GetPostingsReader() FieldsProducer {
	return sr.core.GetFields()
}

func (sr *SegmentReader) GetPointsReader() PointsReader {
	return sr.core.GetPointsReader()
}

func (sr *SegmentReader) GetVectorReader() KnnVectorsReader {
	return sr.core.GetVectorReader()
}

func (sr *SegmentReader) GetSegmentInfo() *SegmentInfo {
	return sr.si.SegmentInfo()
}

// --- LeafReader implementation ---

func (sr *SegmentReader) DocID() int {
	return 0
}

func (sr *SegmentReader) MaxDoc() int {
	return sr.si.SegmentInfo().MaxDoc()
}

func (sr *SegmentReader) NumDocs() int {
	return sr.numDocs
}

func (sr *SegmentReader) GetLiveDocs() util.Bits {
	return sr.liveDocs
}

func (sr *SegmentReader) GetMetaData() *IndexReaderMetaData {
	return sr.metaData
}

func (sr *SegmentReader) GetFieldInfos() *FieldInfos {
	return sr.fieldInfos
}

func (sr *SegmentReader) Close() error {
	return sr.doClose()
}

func (sr *SegmentReader) IncRef() error {
	return sr.core.IncRef()
}

func (sr *SegmentReader) DecRef() error {
	return sr.doClose()
}

func (sr *SegmentReader) TryIncRef() bool {
	return true
}

func (sr *SegmentReader) GetRefCount() int32 {
	return sr.core.GetRefCount()
}

func (sr *SegmentReader) GetContext() (IndexReaderContext, error) {
	return nil, fmt.Errorf("GetContext not implemented")
}

func (sr *SegmentReader) DocFreq(term Term) (int, error) {
	t, err := sr.Terms(term.Field)
	if err != nil {
		return 0, err
	}
	if t == nil {
		return 0, nil
	}
	return t.DocFreq(term)
}

func (sr *SegmentReader) TotalTermFreq(term Term) (int64, error) {
	t, err := sr.Terms(term.Field)
	if err != nil {
		return 0, err
	}
	if t == nil {
		return 0, nil
	}
	return t.TotalTermFreq(term)
}

func (sr *SegmentReader) Terms(field string) (Terms, error) {
	fi := sr.fieldInfos.FieldInfoByName(field)
	if fi == nil || fi.IndexOptions() == IndexOptionsNone {
		return nil, nil
	}
	return sr.core.GetFields().Terms(field)
}

func (sr *SegmentReader) Postings(term Term, flags int) (PostingsEnum, error) {
	return sr.core.GetFields().Postings(term, flags)
}

func (sr *SegmentReader) GetNumericDocValues(field string) (NumericDocValues, error) {
	fi := sr.fieldInfos.FieldInfoByName(field)
	if fi == nil || fi.DocValuesType() == DocValuesTypeNone {
		return nil, nil
	}
	return sr.docValuesProducer.GetNumeric(fi)
}

func (sr *SegmentReader) GetBinaryDocValues(field string) (BinaryDocValues, error) {
	fi := sr.fieldInfos.FieldInfoByName(field)
	if fi == nil || fi.DocValuesType() == DocValuesTypeNone {
		return nil, nil
	}
	return sr.docValuesProducer.GetBinary(fi)
}

func (sr *SegmentReader) GetSortedDocValues(field string) (SortedDocValues, error) {
	fi := sr.fieldInfos.FieldInfoByName(field)
	if fi == nil || fi.DocValuesType() == DocValuesTypeNone {
		return nil, nil
	}
	return sr.docValuesProducer.GetSorted(fi)
}

func (sr *SegmentReader) GetSortedNumericDocValues(field string) (SortedNumericDocValues, error) {
	fi := sr.fieldInfos.FieldInfoByName(field)
	if fi == nil || fi.DocValuesType() == DocValuesTypeNone {
		return nil, nil
	}
	return sr.docValuesProducer.GetSortedNumeric(fi)
}

func (sr *SegmentReader) GetSortedSetDocValues(field string) (SortedSetDocValues, error) {
	fi := sr.fieldInfos.FieldInfoByName(field)
	if fi == nil || fi.DocValuesType() == DocValuesTypeNone {
		return nil, nil
	}
	return sr.docValuesProducer.GetSortedSet(fi)
}

func (sr *SegmentReader) GetDocValuesSkipper(field string) (DocValuesSkipper, error) {
	fi := sr.fieldInfos.FieldInfoByName(field)
	if fi == nil || fi.DocValuesSkipIndexType() == DocValuesSkipIndexTypeNone {
		return nil, nil
	}
	return sr.docValuesProducer.GetSkipper(fi)
}

func (sr *SegmentReader) GetNormValues(field string) (NumericDocValues, error) {
	fi := sr.fieldInfos.FieldInfoByName(field)
	if fi == nil || !fi.HasNorms() {
		return nil, nil
	}
	return sr.core.GetNormsProducer().GetNorms(fi)
}

func (sr *SegmentReader) GetPointValues(field string) (PointValues, error) {
	fi := sr.fieldInfos.FieldInfoByName(field)
	if fi == nil || fi.PointDimensionCount() == 0 {
		return nil, nil
	}
	// Note: core.GetPointsReader() returns spi.PointsReader,
	// and we need to assert it to one that provides GetValues.
	reader := sr.core.GetPointsReader()
	if vr, ok := reader.(interface{ GetValues(field string) (PointValues, error) }); ok {
		return vr.GetValues(field)
	}
	return nil, fmt.Errorf("points reader %T does not expose GetValues", reader)
}

func (sr *SegmentReader) GetFloatVectorValues(field string) (FloatVectorValues, error) {
	fi := sr.fieldInfos.FieldInfoByName(field)
	if fi == nil || fi.VectorDimension() == 0 || fi.VectorEncoding() != VectorEncodingFloat32 {
		return nil, nil
	}
	reader := sr.core.GetVectorReader()
	if vr, ok := reader.(interface{ GetFloatVectorValues(field string) (FloatVectorValues, error) }); ok {
		return vr.GetFloatVectorValues(field)
	}
	return nil, fmt.Errorf("vector reader %T does not expose GetFloatVectorValues", reader)
}

func (sr *SegmentReader) GetByteVectorValues(field string) (ByteVectorValues, error) {
	fi := sr.fieldInfos.FieldInfoByName(field)
	if fi == nil || fi.VectorDimension() == 0 || fi.VectorEncoding() != VectorEncodingByte {
		return nil, nil
	}
	reader := sr.core.GetVectorReader()
	if vr, ok := reader.(interface{ GetByteVectorValues(field string) (ByteVectorValues, error) }); ok {
		return vr.GetByteVectorValues(field)
	}
	return nil, fmt.Errorf("vector reader %T does not expose GetByteVectorValues", reader)
}

func (sr *SegmentReader) SearchNearestVectors(field string, target []float32, k int, acceptDocs util.Bits, visitedLimit int) (TopDocs, error) {
	fi := sr.fieldInfos.FieldInfoByName(field)
	if fi == nil || fi.VectorDimension() == 0 || fi.VectorEncoding() != VectorEncodingFloat32 {
		return TopDocs{}, nil
	}
	reader := sr.core.GetVectorReader()
	if vr, ok := reader.(interface{ Search(field string, target []float32, k int, acceptDocs util.Bits) (TopDocs, error) }); ok {
		return vr.Search(field, target, k, acceptDocs)
	}
	return TopDocs{}, fmt.Errorf("vector reader %T does not expose Search", reader)
}

func (sr *SegmentReader) CheckIntegrity() error {
	if sr.core.GetFields() != nil {
		if err := sr.core.GetFields().CheckIntegrity(); err != nil {
			return err
		}
	}
	if sr.core.GetNormsProducer() != nil {
		if err := sr.core.GetNormsProducer().CheckIntegrity(); err != nil {
			return err
		}
	}
	if sr.docValuesProducer != nil {
		if err := sr.docValuesProducer.CheckIntegrity(); err != nil {
			return err
		}
	}
	if sr.core.GetStoredFieldsReader() != nil {
		if err := sr.core.GetStoredFieldsReader().CheckIntegrity(); err != nil {
			return err
		}
	}
	if sr.core.GetTermVectorsReader() != nil {
		if err := sr.core.GetTermVectorsReader().CheckIntegrity(); err != nil {
			return err
		}
	}
	if sr.core.GetPointsReader() != nil {
		if err := sr.core.GetPointsReader().CheckIntegrity(); err != nil {
			return err
		}
	}
	if sr.core.GetVectorReader() != nil {
		if err := sr.core.GetVectorReader().CheckIntegrity(); err != nil {
			return err
		}
	}
	return nil
}

func (sr *SegmentReader) GetHardLiveDocs() util.Bits {
	return sr.hardLiveDocs
}

func (sr *SegmentReader) GetSegmentName() string {
	return sr.si.SegmentInfo().Name
}

func (sr *SegmentReader) Directory() store.Directory {
	return sr.si.SegmentInfo().Dir()
}

func (sr *SegmentReader) GetOriginalSegmentInfo() *SegmentCommitInfo {
	return sr.originalSi
}
