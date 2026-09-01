// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

type lucene99ScalarQuantizedVectorsReader struct {
	fields             map[int]*fieldEntry
	vectorScorer       FlatVectorsScorer
	quantizedVectorData IndexInput
	rawVectorsReader   FlatVectorsReader
	fieldInfos         FieldInfos
}

func NewLucene99ScalarQuantizedVectorsReader(
	state SegmentReadState,
	rawVectorsReader FlatVectorsReader,
	scorer FlatVectorsScorer) (FlatVectorsReader, error) {

	reader := &lucene99ScalarQuantizedVectorsReader{
		fields:           make(map[int]*fieldEntry),
		vectorScorer:     scorer,
		rawVectorsReader: rawVectorsReader,
		fieldInfos:       state.fieldInfos,
	}

	metaName := IndexFileNames.segmentFileName(
		state.segmentInfo.name,
		state.segmentSuffix,
		ScalarQuantizedMetaExtension)

	var versionMeta int
	var err error

	meta, err := state.directory.OpenChecksumInput(metaName, state.context)
	if err != nil {
		return nil, err
	}
	defer meta.Close()

	versionMeta, err = CodecUtil.CheckIndexHeader(
		meta,
		ScalarQuantizedMetaCodecName,
		ScalarQuantizedVersionStart,
		ScalarQuantizedVersionCurrent,
		state.segmentInfo.getId(),
		state.segmentSuffix)
	if err != nil {
		return nil, err
	}

	if err := reader.readFields(meta, versionMeta, state.fieldInfos); err != nil {
		return nil, err
	}

	dataName := IndexFileNames.segmentFileName(
		state.segmentInfo.name,
		state.segmentSuffix,
		ScalarQuantizedVectorDataExtension)

	data, err := state.directory.OpenInput(dataName, state.context)
	if err != nil {
		return nil, err
	}

	versionVectorData, err := CodecUtil.CheckIndexHeader(
		data,
		ScalarQuantizedVectorDataCodecName,
		ScalarQuantizedVersionStart,
		ScalarQuantizedVersionCurrent,
		state.segmentInfo.getId(),
		state.segmentSuffix)
	if err != nil {
		data.Close()
		return nil, err
	}

	if versionMeta != versionVectorData {
		data.Close()
		return nil, fmt.Errorf("format versions mismatch: meta=%d, data=%d", versionMeta, versionVectorData)
	}

	if err := CodecUtil.RetrieveChecksum(data); err != nil {
		data.Close()
		return nil, err
	}

	reader.quantizedVectorData = data

	return reader, nil
}

func (r *lucene99ScalarQuantizedVectorsReader) readFields(meta IndexInput, versionMeta int, infos FieldInfos) error {
	for {
		fieldNumber, err := meta.ReadInt()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if fieldNumber == -1 {
			break
		}

		info := infos.FieldInfo(fieldNumber)
		if info == nil {
			return fmt.Errorf("invalid field number: %d", fieldNumber)
		}

		entry, err := r.readField(meta, versionMeta, info)
		if err != nil {
			return err
		}
		if err := r.validateFieldEntry(info, entry); err != nil {
			return err
		}
		r.fields[info.number] = entry
	}
	return nil
}

func (r *lucene99ScalarQuantizedVectorsReader) readField(meta IndexInput, versionMeta int, info FieldInfo) (*fieldEntry, error) {
	vectorEncoding := r.readVectorEncoding(meta)
	similarityFunction := r.readSimilarityFunction(meta)
	if similarityFunction != info.GetVectorSimilarityFunction() {
		return nil, fmt.Errorf("inconsistent vector similarity function for field=\"%s\"; %v != %v",
			info.name, similarityFunction, info.GetVectorSimilarityFunction())
	}

	return createFieldEntry(meta, versionMeta, vectorEncoding, info.GetVectorSimilarityFunction())
}

func (r *lucene99ScalarQuantizedVectorsReader) validateFieldEntry(info FieldInfo, entry *fieldEntry) error {
	dimension := info.GetVectorDimension()
	if dimension != entry.dimension {
		return fmt.Errorf("inconsistent vector dimension for field=\"%s\"; %d != %d",
			info.name, dimension, entry.dimension)
	}

	var quantizedVectorBytes int64
	if entry.bits <= 4 && entry.compress {
		quantizedVectorBytes = int64((dimension+1)>>1) + 4
	} else {
		quantizedVectorBytes = int64(dimension) + 4
	}

	numQuantizedVectorBytes := quantizedVectorBytes * int64(entry.size)
	if numQuantizedVectorBytes != entry.vectorDataLength {
		return fmt.Errorf("quantized vector data length %d not matching size=%d * (dim=%d + 4) = %d",
			entry.vectorDataLength, entry.size, dimension, numQuantizedVectorBytes)
	}
	return nil
}

func (r *lucene99ScalarQuantizedVectorsReader) getFieldEntry(field string) (*fieldEntry, error) {
	info := r.fieldInfos.FieldInfo(field)
	if info == nil {
		return nil, fmt.Errorf("field=\"%s\" not found", field)
	}
	entry := r.fields[info.number]
	if entry == nil {
		return nil, fmt.Errorf("field=\"%s\" not found", field)
	}
	// Note: VectorEncoding is defined in the index package
	// This is a placeholder for the real check
	return entry, nil
}

func (r *lucene99ScalarQuantizedVectorsReader) GetFloatVectorValues(field string) (FloatVectorValues, error) {
	entry, err := r.getFieldEntry(field)
	if err != nil {
		return nil, err
	}
	rawVectorValues, err := r.rawVectorsReader.GetFloatVectorValues(field)
	if err != nil {
		return nil, err
	}

	if rawVectorValues.Size() == 0 {
		return loadOffHeapQuantizedFloatVectorValues(
			entry.ordToDoc,
			entry.dimension,
			entry.size,
			entry.scalarQuantizer,
			entry.similarityFunction,
			r.vectorScorer,
			entry.compress,
			entry.vectorDataOffset,
			entry.vectorDataLength,
			r.quantizedVectorData), nil
	}

	quantizedByteVectorValues := loadOffHeapQuantizedByteVectorValues(
		entry.ordToDoc,
		entry.dimension,
		entry.size,
		entry.scalarQuantizer,
		entry.similarityFunction,
		r.vectorScorer,
		entry.compress,
		entry.vectorDataOffset,
		entry.vectorDataLength,
		r.quantizedVectorData)

	return &quantizedVectorValues{
		rawVectorValues: rawVectorValues,
		quantizedByteVectorValues: quantizedByteVectorValues,
	}, nil
}

func (r *lucene99ScalarQuantizedVectorsReader) GetByteVectorValues(field string) (ByteVectorValues, error) {
	return r.rawVectorsReader.GetByteVectorValues(field)
}

func (r *lucene99ScalarQuantizedVectorsReader) GetFlatVectorScorer(field string) (FlatVectorsScorer, error) {
	return r.vectorScorer, nil
}

func (r *lucene99ScalarQuantizedVectorsReader) GetRandomVectorScorer(field string, target []float32) (RandomVectorScorer, error) {
	entry, err := r.getFieldEntry(field)
	if err != nil {
		return nil, err
	}
	if entry.scalarQuantizer == nil {
		return r.rawVectorsReader.GetRandomVectorScorer(field, target)
	}

	quantizedByteVectorValues := loadOffHeapQuantizedByteVectorValues(
		entry.ordToDoc,
		entry.dimension,
		entry.size,
		entry.scalarQuantizer,
		entry.similarityFunction,
		r.vectorScorer,
		entry.compress,
		entry.vectorDataOffset,
		entry.vectorDataLength,
		r.quantizedVectorData)

	return r.vectorScorer.GetRandomVectorScorer(entry.similarityFunction, quantizedByteVectorValues, target), nil
}

func (r *lucene99ScalarQuantizedVectorsReader) GetRandomVectorScorerByte(field string, target []byte) (RandomVectorScorer, error) {
	return r.rawVectorsReader.GetRandomVectorScorerByte(field, target)
}

func (r *lucene99ScalarQuantizedVectorsReader) Close() error {
	if err := r.quantizedVectorData.Close(); err != nil {
		return err
	}
	return r.rawVectorsReader.Close()
}

func (r *lucene99ScalarQuantizedVectorsReader) CheckIntegrity() error {
	if err := r.rawVectorsReader.CheckIntegrity(); err != nil {
		return err
	}
	return CodecUtil.ChecksumEntireFile(r.quantizedVectorData)
}

func (r *lucene99ScalarQuantizedVectorsReader) GetOffHeapByteSize(fieldInfo FieldInfo) map[string]int64 {
	raw := r.rawVectorsReader.GetOffHeapByteSize(fieldInfo)
	entry := r.fields[fieldInfo.number]
	if entry == nil {
		return raw
	}
	quant := map[string]int64{ScalarQuantizedVectorDataExtension: entry.vectorDataLength}
	return mergeOffHeapByteSizeMaps(raw, quant)
}

type fieldEntry struct {
	similarityFunction VectorSimilarityFunction
	vectorEncoding     VectorEncoding
	dimension          int
	vectorDataOffset   int64
	vectorDataLength   int64
	scalarQuantizer    quantization.ScalarQuantizer
	size               int
	bits               byte
	compress           bool
	ordToDoc           OrdToDocDISIReaderConfiguration
}

func createFieldEntry(input IndexInput, versionMeta int, vectorEncoding VectorEncoding, similarityFunction VectorSimilarityFunction) (*fieldEntry, error) {
	vectorDataOffset := input.ReadVLong()
	vectorDataLength := input.ReadVLong()
	dimension := input.ReadVInt()
	size := input.ReadInt()

	var scalarQuantizer quantization.ScalarQuantizer
	var bits byte
	var compress bool

	if size > 0 {
		if versionMeta < ScalarQuantizedVersionAddBits {
			_ = input.ReadInt() // confidenceInterval, unused
			bits = 7
			compress = false
			_ = input.ReadInt() // minQuantile
			_ = input.ReadInt() // maxQuantile
			scalarQuantizer, _ = quantization.NewScalarQuantizer(0, 0, bits)
		} else {
			input.ReadInt() // confidenceInterval, unused
			bits = input.ReadByte()
			compress = input.ReadByte() == 1
			_ = input.ReadInt() // minQuantile
			_ = input.ReadInt() // maxQuantile
			scalarQuantizer, _ = quantization.NewScalarQuantizer(0, 0, bits)
		}
	} else {
		scalarQuantizer = nil
		bits = 7
		compress = false
	}

	ordToDoc, err := lucene90.FromStoredMeta(input, size)
	if err != nil {
		return nil, err
	}

	return &fieldEntry{
		similarityFunction: similarityFunction,
		vectorEncoding:     vectorEncoding,
		dimension:          dimension,
		vectorDataOffset:   vectorDataOffset,
		vectorDataLength:   vectorDataLength,
		scalarQuantizer:    scalarQuantizer,
		size:               size,
		bits:               bits,
		compress:           compress,
		ordToDoc:           ordToDoc,
	}, nil
}

type quantizedVectorValues struct {
	rawVectorValues FloatVectorValues
	quantizedByteVectorValues ByteVectorValues
}

func (q *quantizedVectorValues) Dimension() int {
	return q.rawVectorValues.Dimension()
}

func (q *quantizedVectorValues) Size() int {
	return q.rawVectorValues.Size()
}

func (q *quantizedVectorValues) VectorValue(ord int) ([]float32, error) {
	return q.rawVectorValues.VectorValue(ord)
}

func (q *quantizedVectorValues) OrdToDoc(ord int) int {
	return q.rawVectorValues.OrdToDoc(ord)
}

func (q *quantizedVectorValues) Copy() (FloatVectorValues, error) {
	raw, err := q.rawVectorValues.Copy()
	if err != nil {
		return nil, err
	}
	return &quantizedVectorValues{
		rawVectorValues: raw,
		quantizedByteVectorValues: q.quantizedByteVectorValues,
	}, nil
}

func (q *quantizedVectorValues) Scorer(query []float32) (VectorScorer, error) {
	return q.quantizedByteVectorValues.Scorer(query)
}

func (q *quantizedVectorValues) Rescorer(query []float32) (VectorScorer, error) {
	return q.rawVectorValues.Rescorer(query)
}

func (q *quantizedVectorValues) Iterator() DocIndexIterator {
	return q.rawVectorValues.Iterator()
}

func mergeOffHeapByteSizeMaps(m1, m2 map[string]int64) map[string]int64 {
	res := make(map[string]int64)
	for k, v := range m1 {
		res[k] = v
	}
	for k, v := range m2 {
		res[k] += v
	}
	return res
}
