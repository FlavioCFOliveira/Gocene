// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.5.0:
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with
//	this work for additional information regarding copyright ownership.
//	The ASF licenses this file to You under the Apache License, Version 2.0
//	(the "License"); you may not use this file except in compliance with
//	the License. You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0

package lucene102

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene95"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// Lucene102BinaryQuantizedVectorsReader is the Go port of
// org.apache.lucene.backward_codecs.lucene102.Lucene102BinaryQuantizedVectorsReader
// (Apache Lucene 10.5.0): the reader for binary quantized vectors in the
// Lucene 10.2 format.
type Lucene102BinaryQuantizedVectorsReader struct {
	hnsw.BaseFlatVectorsReader

	fields              map[string]*lucene102FieldEntry
	quantizedVectorData store.IndexInput
	rawVectorsReader    hnsw.FlatVectorsReader
	vectorScorer        *Lucene102BinaryFlatVectorsScorer
}

var (
	// lucene102ReaderShallowSize mirrors SHALLOW_SIZE.
	lucene102ReaderShallowSize = util.ShallowSizeOf(Lucene102BinaryQuantizedVectorsReader{})
	// lucene102FieldEntryShallowSize mirrors
	// RamUsageEstimator.shallowSizeOfInstance(FieldEntry.class).
	lucene102FieldEntryShallowSize = util.ShallowSizeOf(lucene102FieldEntry{})
)

// NewLucene102BinaryQuantizedVectorsReader creates a new reader for binary
// quantized vectors. Mirrors Lucene102BinaryQuantizedVectorsReader(
// SegmentReadState, FlatVectorsReader, Lucene102BinaryFlatVectorsScorer): the
// raw vectors reader is owned by the new reader and is closed when the
// constructor fails.
func NewLucene102BinaryQuantizedVectorsReader(
	state *codecs.SegmentReadState,
	rawVectorsReader hnsw.FlatVectorsReader,
	vectorsScorer *Lucene102BinaryFlatVectorsScorer,
) (*Lucene102BinaryQuantizedVectorsReader, error) {
	r := &Lucene102BinaryQuantizedVectorsReader{
		fields:           make(map[string]*lucene102FieldEntry),
		rawVectorsReader: rawVectorsReader,
		vectorScorer:     vectorsScorer,
	}
	if err := r.open(state); err != nil {
		// Java: IOUtils.closeWhileHandlingException(this).
		if r.quantizedVectorData != nil {
			util.CloseAllWhileHandlingException(r.quantizedVectorData)
		}
		if r.rawVectorsReader != nil {
			util.CloseAllWhileHandlingException(r.rawVectorsReader)
		}
		return nil, err
	}
	return r, nil
}

// open is the body of the Java constructor's try block.
func (r *Lucene102BinaryQuantizedVectorsReader) open(state *codecs.SegmentReadState) error {
	versionMeta := int32(-1)
	metaFileName := store.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene102MetaExtension)
	metaIn, err := state.Directory.OpenInput(metaFileName, store.IOContextRead)
	if err != nil {
		return err
	}
	meta := store.NewChecksumIndexInput(metaIn)

	var priorErr error
	func() {
		v, err := codecs.CheckIndexHeader(
			meta,
			lucene102MetaCodecName,
			lucene102VersionStart,
			lucene102VersionCurrent,
			state.SegmentInfo.GetID(),
			state.SegmentSuffix)
		if err != nil {
			priorErr = err
			return
		}
		versionMeta = v
		priorErr = r.readFields(meta, state.FieldInfos)
	}()
	_, footerErr := codecs.CheckFooter(meta)
	closeErr := metaIn.Close()
	if priorErr != nil {
		return priorErr
	}
	if footerErr != nil {
		return footerErr
	}
	if closeErr != nil {
		return closeErr
	}

	// Quantized vectors are accessed randomly from their node ID stored in the HNSW
	// graph.
	quantizedVectorData, err := openLucene102DataInput(
		state, versionMeta, lucene102VectorDataExtension, lucene102VectorDataCodecName)
	if err != nil {
		return err
	}
	r.quantizedVectorData = quantizedVectorData
	return nil
}

// readFields mirrors the private readFields(ChecksumIndexInput, FieldInfos).
func (r *Lucene102BinaryQuantizedVectorsReader) readFields(meta store.IndexInput, infos *index.FieldInfos) error {
	for {
		fieldNumber, err := meta.ReadInt()
		if err != nil {
			return err
		}
		if fieldNumber == -1 {
			return nil
		}
		info := infos.GetByNumber(int(fieldNumber))
		if info == nil {
			return fmt.Errorf("Invalid field number: %d", fieldNumber)
		}
		fieldEntry, err := readLucene102Field(meta, info)
		if err != nil {
			return err
		}
		if err := validateLucene102FieldEntry(info, fieldEntry); err != nil {
			return err
		}
		r.fields[info.Name()] = fieldEntry
	}
}

// validateLucene102FieldEntry mirrors the package-private static
// validateFieldEntry(FieldInfo, FieldEntry), whose IllegalStateException is
// returned as an error. The Math.multiplyExact of the Java body cannot
// overflow: both operands are bounded by the int range.
func validateLucene102FieldEntry(info *index.FieldInfo, fieldEntry *lucene102FieldEntry) error {
	dimension := info.VectorDimension()
	if dimension != fieldEntry.dimension {
		return fmt.Errorf("Inconsistent vector dimension for field=\"%s\"; %d != %d",
			info.Name(), dimension, fieldEntry.dimension)
	}

	binaryDims := quantization.Discretize(dimension, 64) / 8
	numQuantizedVectorBytes := int64(binaryDims+(floatBytes*3)+shortBytes) * int64(fieldEntry.size)
	if numQuantizedVectorBytes != fieldEntry.vectorDataLength {
		return fmt.Errorf("Binarized vector data length %d not matching size = %d * (binaryBytes=%d + 14) = %d",
			fieldEntry.vectorDataLength, fieldEntry.size, binaryDims, numQuantizedVectorBytes)
	}
	return nil
}

// GetFlatVectorScorer mirrors getFlatVectorScorer(String).
func (r *Lucene102BinaryQuantizedVectorsReader) GetFlatVectorScorer(_ string) (hnsw.FlatVectorsScorer, error) {
	return r.vectorScorer, nil
}

// GetRandomVectorScorerFloat mirrors getRandomVectorScorer(String, float[]):
// the quantized vectors of the field are scored against target; a field
// without an entry yields no scorer.
func (r *Lucene102BinaryQuantizedVectorsReader) GetRandomVectorScorerFloat(field string, target []float32) (utilhnsw.RandomVectorScorer, error) {
	fi := r.fields[field]
	if fi == nil {
		return nil, nil
	}
	values, err := loadOffHeapBinarizedVectorValues(
		fi.ordToDocDISIReaderConfiguration,
		fi.dimension,
		fi.size,
		quantization.NewDefaultOptimizedScalarQuantizer(fi.similarityFunction),
		fi.similarityFunction,
		r.vectorScorer,
		fi.centroid,
		fi.centroidDP,
		fi.vectorDataOffset,
		fi.vectorDataLength,
		r.quantizedVectorData)
	if err != nil {
		return nil, err
	}
	return r.vectorScorer.GetRandomVectorScorer(fi.similarityFunction, values, target)
}

// GetRandomVectorScorerByte mirrors getRandomVectorScorer(String, byte[]),
// which delegates to the raw vectors reader.
func (r *Lucene102BinaryQuantizedVectorsReader) GetRandomVectorScorerByte(field string, target []byte) (utilhnsw.RandomVectorScorer, error) {
	return r.rawVectorsReader.GetRandomVectorScorerByte(field, target)
}

// CheckIntegrity mirrors checkIntegrity().
func (r *Lucene102BinaryQuantizedVectorsReader) CheckIntegrity() error {
	if err := r.rawVectorsReader.CheckIntegrity(); err != nil {
		return err
	}
	_, err := codecs.ChecksumEntireFile(r.quantizedVectorData)
	return err
}

// binarizedVectorValues is the body of getFloatVectorValues(String), typed as
// the covariant BinarizedVectorValues.
func (r *Lucene102BinaryQuantizedVectorsReader) binarizedVectorValues(field string) (*BinarizedVectorValues, error) {
	fi := r.fields[field]
	if fi == nil {
		return nil, nil
	}
	if fi.vectorEncoding != index.VectorEncodingFloat32 {
		return nil, fmt.Errorf("field=\"%s\" is encoded as: %v expected: %v",
			field, fi.vectorEncoding, index.VectorEncodingFloat32)
	}
	bvv, err := loadOffHeapBinarizedVectorValues(
		fi.ordToDocDISIReaderConfiguration,
		fi.dimension,
		fi.size,
		quantization.NewDefaultOptimizedScalarQuantizer(fi.similarityFunction),
		fi.similarityFunction,
		r.vectorScorer,
		fi.centroid,
		fi.centroidDP,
		fi.vectorDataOffset,
		fi.vectorDataLength,
		r.quantizedVectorData)
	if err != nil {
		return nil, err
	}
	rawVectorValues, err := r.rawVectorsReader.GetFloatVectorValues(field)
	if err != nil {
		return nil, err
	}
	return newBinarizedVectorValues(rawVectorValues, bvv), nil
}

// GetFloatVectorValues mirrors getFloatVectorValues(String), which returns
// the raw vectors paired with their binarized counterparts, or null for a
// field without an entry.
func (r *Lucene102BinaryQuantizedVectorsReader) GetFloatVectorValues(field string) (index.FloatVectorValues, error) {
	values, err := r.binarizedVectorValues(field)
	if err != nil || values == nil {
		return nil, err
	}
	return values, nil
}

// GetByteVectorValues mirrors getByteVectorValues(String), which delegates to
// the raw vectors reader.
func (r *Lucene102BinaryQuantizedVectorsReader) GetByteVectorValues(field string) (index.ByteVectorValues, error) {
	return r.rawVectorsReader.GetByteVectorValues(field)
}

// SearchByte mirrors search(String, byte[], KnnCollector, AcceptDocs), which
// delegates to the raw vectors reader.
func (r *Lucene102BinaryQuantizedVectorsReader) SearchByte(
	field string, target []byte, knnCollector spi.KnnCollector, acceptDocs search.AcceptDocs,
) error {
	return r.rawVectorsReader.SearchByte(field, target, knnCollector, acceptDocs)
}

// SearchFloat mirrors search(String, float[], KnnCollector, AcceptDocs): every
// accepted ordinal is scored exhaustively against the quantized vectors.
func (r *Lucene102BinaryQuantizedVectorsReader) SearchFloat(
	field string, target []float32, knnCollector spi.KnnCollector, acceptDocs search.AcceptDocs,
) error {
	if knnCollector.K() == 0 {
		return nil
	}
	scorer, err := r.GetRandomVectorScorerFloat(field, target)
	if err != nil {
		return err
	}
	if scorer == nil {
		return nil
	}
	collector := utilhnsw.NewOrdinalTranslatedKnnCollector(knnCollector, utilhnsw.IntToIntFunc(scorer.OrdToDoc))
	acceptBits, err := acceptDocs.Bits()
	if err != nil {
		return err
	}
	acceptedOrds := scorer.GetAcceptOrds(acceptBits)
	for i := 0; i < scorer.MaxOrd(); i++ {
		if acceptedOrds == nil || acceptedOrds.Get(i) {
			score, err := scorer.Score(i)
			if err != nil {
				return err
			}
			collector.Collect(i, score)
			collector.IncVisitedCount(1)
		}
	}
	return nil
}

// GetMergeInstance carries the FlatVectorsReader.getMergeInstance() default,
// which returns this.
func (r *Lucene102BinaryQuantizedVectorsReader) GetMergeInstance() (codecs.KnnVectorsReader, error) {
	return r, nil
}

// FinishMerge carries the KnnVectorsReader.finishMerge() default, which does
// nothing.
func (r *Lucene102BinaryQuantizedVectorsReader) FinishMerge() error {
	return nil
}

// Close mirrors close(): IOUtils.close(quantizedVectorData, rawVectorsReader).
func (r *Lucene102BinaryQuantizedVectorsReader) Close() error {
	var firstErr error
	if r.quantizedVectorData != nil {
		if err := r.quantizedVectorData.Close(); err != nil {
			firstErr = err
		}
	}
	if r.rawVectorsReader != nil {
		if err := r.rawVectorsReader.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// RamBytesUsed mirrors ramBytesUsed().
func (r *Lucene102BinaryQuantizedVectorsReader) RamBytesUsed() int64 {
	size := lucene102ReaderShallowSize
	size += util.ShallowSizeOf(r.fields) + int64(len(r.fields))*lucene102FieldEntryShallowSize
	size += r.rawVectorsReader.RamBytesUsed()
	return size
}

// GetOffHeapByteSize mirrors getOffHeapByteSize(FieldInfo): the raw reader's
// accounting merged with the .veb bytes of the field.
func (r *Lucene102BinaryQuantizedVectorsReader) GetOffHeapByteSize(fieldInfo *index.FieldInfo) map[string]int64 {
	raw := r.rawVectorsReader.GetOffHeapByteSize(fieldInfo)
	fieldEntry := r.fields[fieldInfo.Name()]
	if fieldEntry == nil {
		return raw
	}
	quant := map[string]int64{lucene102VectorDataExtension: fieldEntry.vectorDataLength}
	return codecs.MergeOffHeapByteSizeMaps(raw, quant)
}

// getCentroid mirrors the package-private getCentroid(String).
func (r *Lucene102BinaryQuantizedVectorsReader) getCentroid(field string) []float32 {
	fieldEntry := r.fields[field]
	if fieldEntry != nil {
		return fieldEntry.centroid
	}
	return nil
}

// openLucene102DataInput mirrors the private static openDataInput.
func openLucene102DataInput(
	state *codecs.SegmentReadState, versionMeta int32, fileExtension, codecName string,
) (store.IndexInput, error) {
	fileName := store.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, fileExtension)
	in, err := state.Directory.OpenInput(fileName, store.IOContextRead)
	if err != nil {
		return nil, err
	}
	versionVectorData, err := codecs.CheckIndexHeader(
		in,
		codecName,
		lucene102VersionStart,
		lucene102VersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix)
	if err != nil {
		util.CloseAllWhileHandlingException(in)
		return nil, err
	}
	if versionMeta != versionVectorData {
		util.CloseAllWhileHandlingException(in)
		return nil, fmt.Errorf("Format versions mismatch: meta=%d, %s=%d", versionMeta, codecName, versionVectorData)
	}
	if _, err := codecs.RetrieveChecksum(in); err != nil {
		util.CloseAllWhileHandlingException(in)
		return nil, err
	}
	return in, nil
}

// readLucene102Field mirrors the private readField(IndexInput, FieldInfo).
func readLucene102Field(input store.IndexInput, info *index.FieldInfo) (*lucene102FieldEntry, error) {
	vectorEncoding, err := codecs.ReadVectorEncoding(input)
	if err != nil {
		return nil, err
	}
	similarityFunction, err := codecs.ReadSimilarityFunction(input)
	if err != nil {
		return nil, err
	}
	if similarityFunction != info.VectorSimilarityFunction() {
		return nil, fmt.Errorf("Inconsistent vector similarity function for field=\"%s\"; %v != %v",
			info.Name(), similarityFunction, info.VectorSimilarityFunction())
	}
	return createLucene102FieldEntry(input, vectorEncoding, info.VectorSimilarityFunction())
}

// GetQuantizedVectorValues mirrors getQuantizedVectorValues(String), which
// returns null.
func (r *Lucene102BinaryQuantizedVectorsReader) GetQuantizedVectorValues(_ string) (quantization.BaseQuantizedByteVectorValues, error) {
	return nil, nil
}

// GetQuantizationState mirrors getQuantizationState(String), which returns
// null.
func (r *Lucene102BinaryQuantizedVectorsReader) GetQuantizationState(_ string) *quantization.ScalarQuantizer {
	return nil
}

// GetRandomVectorScorerSupplierForMerge mirrors
// getRandomVectorScorerSupplierForMerge(FieldInfo, SegmentWriteState): the
// field's vectors are quantized to 4-bit queries in a temporary file, and the
// returned supplier scores them against the binarized vectors, closing the
// temporary input and deleting the file when closed.
//
// Directory.createTempOutput has no counterpart on spi.Directory; the
// directory is asked for the method through its concrete type, and a
// directory without it fails with UnsupportedOperationException.
func (r *Lucene102BinaryQuantizedVectorsReader) GetRandomVectorScorerSupplierForMerge(
	fieldInfo *spi.FieldInfo, segmentWriteState *spi.SegmentWriteState,
) (utilhnsw.CloseableRandomVectorScorerSupplier, error) {
	quantizer := quantization.NewDefaultOptimizedScalarQuantizer(fieldInfo.VectorSimilarityFunction())
	centroid := r.getCentroid(fieldInfo.Name())
	vectorValues, err := r.binarizedVectorValues(fieldInfo.Name())
	if err != nil {
		return nil, err
	}
	floatVectorValues := vectorValues.rawVectorValues
	if fieldInfo.VectorSimilarityFunction() == index.VectorSimilarityFunctionCosine {
		floatVectorValues = newLucene102NormalizedFloatVectorValues(floatVectorValues)
	}

	directory := segmentWriteState.Directory
	tempOutputs, ok := directory.(interface {
		CreateTempOutput(prefix, suffix string, ctx store.IOContext) (store.IndexOutput, error)
	})
	if !ok {
		return nil, fmt.Errorf("UnsupportedOperationException: %T cannot create temporary outputs", directory)
	}
	rawTempOutput, err := tempOutputs.CreateTempOutput(segmentWriteState.SegmentInfo.Name(), "queries", segmentWriteState.Context)
	if err != nil {
		return nil, err
	}
	tempScoreQuantizedVectorName := rawTempOutput.GetName()
	tempScoreQuantizedVector := store.NewChecksumIndexOutput(rawTempOutput)
	docsWithField, err := writeBinarizedQueryData(tempScoreQuantizedVector, floatVectorValues, centroid, quantizer)
	if err == nil {
		err = codecs.WriteFooter(tempScoreQuantizedVector)
	}
	if closeErr := tempScoreQuantizedVector.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		store.DeleteFilesIgnoringExceptions(directory, tempScoreQuantizedVectorName)
		return nil, err
	}

	quantizedScoreDataInput, err := directory.OpenInput(tempScoreQuantizedVectorName, segmentWriteState.Context)
	if err != nil {
		return nil, err
	}
	scorerSupplier := r.vectorScorer.getBinarizedRandomVectorScorerSupplier(
		fieldInfo.VectorSimilarityFunction(),
		newOffHeapBinarizedQueryVectorValues(
			quantizedScoreDataInput,
			fieldInfo.VectorDimension(),
			docsWithField.Cardinality()),
		vectorValues.quantizedVectorValues)
	finalTempScoreQuantizedVectorName := tempScoreQuantizedVectorName
	return utilhnsw.CreateCloseableRandomVectorScorerSupplier(
		scorerSupplier,
		vectorValues.Size(),
		func() error {
			if err := quantizedScoreDataInput.Close(); err != nil {
				return err
			}
			store.DeleteFilesIgnoringExceptions(directory, finalTempScoreQuantizedVectorName)
			return nil
		}), nil
}

// writeBinarizedQueryData mirrors the package-private static
// writeBinarizedQueryData(IndexOutput, FloatVectorValues, float[],
// OptimizedScalarQuantizer): every vector is quantized to QUERY_BITS against
// the centroid, transposed, and written with its corrective terms.
func writeBinarizedQueryData(
	binarizedQueryData store.IndexOutput,
	floatVectorValues index.FloatVectorValues,
	centroid []float32,
	binaryQuantizer *quantization.OptimizedScalarQuantizer,
) (*index.DocsWithFieldSet, error) {
	discretizedDimension := quantization.Discretize(floatVectorValues.Dimension(), 64)
	docsWithField := index.NewDocsWithFieldSet()
	quantizationScratch := make([]byte, floatVectorValues.Dimension())
	toQuery := make([]byte, (discretizedDimension/8)*int(lucene102QueryBits))
	iterator := floatVectorValues.Iterator()
	for {
		docV, err := iterator.NextDoc()
		if err != nil {
			return nil, err
		}
		if docV == util.NO_MORE_DOCS {
			break
		}
		// write index vector
		vector, err := floatVectorValues.VectorValue(iterator.Index())
		if err != nil {
			return nil, err
		}
		r := binaryQuantizer.ScalarQuantize(vector, quantizationScratch, lucene102QueryBits, centroid)
		if err := docsWithField.Add(docV); err != nil {
			return nil, err
		}

		// pack and store the 4bit query vector
		quantization.TransposeHalfByte(quantizationScratch, toQuery)
		if err := binarizedQueryData.WriteBytes(toQuery, 0, len(toQuery)); err != nil {
			return nil, err
		}
		if err := binarizedQueryData.WriteInt(int32(math.Float32bits(r.LowerInterval))); err != nil {
			return nil, err
		}
		if err := binarizedQueryData.WriteInt(int32(math.Float32bits(r.UpperInterval))); err != nil {
			return nil, err
		}
		if err := binarizedQueryData.WriteInt(int32(math.Float32bits(r.AdditionalCorrection))); err != nil {
			return nil, err
		}
		if err := binarizedQueryData.WriteShort(int16(r.QuantizedComponentSum)); err != nil {
			return nil, err
		}
	}
	return docsWithField, nil
}

// lucene102FieldEntry mirrors the private record FieldEntry.
type lucene102FieldEntry struct {
	similarityFunction              index.VectorSimilarityFunction
	vectorEncoding                  index.VectorEncoding
	dimension                       int
	descritizedDimension            int
	vectorDataOffset                int64
	vectorDataLength                int64
	size                            int
	centroid                        []float32
	centroidDP                      float32
	ordToDocDISIReaderConfiguration *lucene95.OrdToDocDISIReaderConfiguration
}

// createLucene102FieldEntry mirrors the static FieldEntry.create(IndexInput,
// VectorEncoding, VectorSimilarityFunction).
func createLucene102FieldEntry(
	input store.IndexInput, vectorEncoding index.VectorEncoding, similarityFunction index.VectorSimilarityFunction,
) (*lucene102FieldEntry, error) {
	dimension, err := store.ReadVInt(input)
	if err != nil {
		return nil, err
	}
	vectorDataOffset, err := input.ReadVLong()
	if err != nil {
		return nil, err
	}
	vectorDataLength, err := input.ReadVLong()
	if err != nil {
		return nil, err
	}
	size, err := store.ReadVInt(input)
	if err != nil {
		return nil, err
	}
	var centroid []float32
	var centroidDP float32
	if size > 0 {
		centroid = make([]float32, dimension)
		if err := input.ReadFloats(centroid, 0, int(dimension)); err != nil {
			return nil, err
		}
		centroidBits, err := input.ReadInt()
		if err != nil {
			return nil, err
		}
		centroidDP = math.Float32frombits(uint32(centroidBits))
	}
	conf, err := lucene95.FromStoredMeta(input, int(size))
	if err != nil {
		return nil, err
	}
	return &lucene102FieldEntry{
		similarityFunction:              similarityFunction,
		vectorEncoding:                  vectorEncoding,
		dimension:                       int(dimension),
		descritizedDimension:            quantization.Discretize(int(dimension), 64),
		vectorDataOffset:                vectorDataOffset,
		vectorDataLength:                vectorDataLength,
		size:                            int(size),
		centroid:                        centroid,
		centroidDP:                      centroidDP,
		ordToDocDISIReaderConfiguration: conf,
	}, nil
}

// BinarizedVectorValues is the Go port of the protected static final nested
// class Lucene102BinaryQuantizedVectorsReader.BinarizedVectorValues: binarized
// vector values holding raw and quantized vector values.
type BinarizedVectorValues struct {
	rawVectorValues       index.FloatVectorValues
	quantizedVectorValues BinarizedByteVectorValues
}

// newBinarizedVectorValues mirrors the BinarizedVectorValues constructor.
func newBinarizedVectorValues(rawVectorValues index.FloatVectorValues, quantizedVectorValues BinarizedByteVectorValues) *BinarizedVectorValues {
	return &BinarizedVectorValues{rawVectorValues: rawVectorValues, quantizedVectorValues: quantizedVectorValues}
}

// Dimension mirrors dimension().
func (v *BinarizedVectorValues) Dimension() int { return v.rawVectorValues.Dimension() }

// Size mirrors size().
func (v *BinarizedVectorValues) Size() int { return v.rawVectorValues.Size() }

// VectorValue mirrors vectorValue(int).
func (v *BinarizedVectorValues) VectorValue(ord int) ([]float32, error) {
	return v.rawVectorValues.VectorValue(ord)
}

// copyBinarized mirrors the covariant copy().
func (v *BinarizedVectorValues) copyBinarized() (*BinarizedVectorValues, error) {
	rawCopy, err := v.rawVectorValues.CopyFloatVectorValues()
	if err != nil {
		return nil, err
	}
	quantizedCopy, err := v.quantizedVectorValues.CopyBinarizedByteVectorValues()
	if err != nil {
		return nil, err
	}
	return newBinarizedVectorValues(rawCopy, quantizedCopy), nil
}

// Copy mirrors copy().
func (v *BinarizedVectorValues) Copy() (index.KnnVectorValues, error) {
	copied, err := v.copyBinarized()
	if err != nil {
		return nil, err
	}
	return copied, nil
}

// CopyFloatVectorValues mirrors copy() typed as FloatVectorValues.
func (v *BinarizedVectorValues) CopyFloatVectorValues() (index.FloatVectorValues, error) {
	copied, err := v.copyBinarized()
	if err != nil {
		return nil, err
	}
	return copied, nil
}

// GetAcceptOrds mirrors getAcceptOrds(Bits).
func (v *BinarizedVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return v.rawVectorValues.GetAcceptOrds(acceptDocs)
}

// OrdToDoc mirrors ordToDoc(int).
func (v *BinarizedVectorValues) OrdToDoc(ord int) int { return v.rawVectorValues.OrdToDoc(ord) }

// Iterator mirrors iterator().
func (v *BinarizedVectorValues) Iterator() index.DocIndexIterator {
	return v.rawVectorValues.Iterator()
}

// Scorer mirrors scorer(float[]), which scores against the quantized vectors.
func (v *BinarizedVectorValues) Scorer(query []float32) (util.VectorScorer, error) {
	return v.quantizedVectorValues.ScorerFloat(query)
}

// Rescorer mirrors rescorer(float[]), which scores against the raw vectors.
func (v *BinarizedVectorValues) Rescorer(query []float32) (util.VectorScorer, error) {
	return v.rawVectorValues.Rescorer(query)
}

// Prefetch carries the KnnVectorValues.prefetch default, which does nothing.
func (v *BinarizedVectorValues) Prefetch(_ []int, _ int) error { return nil }

// GetVectorByteLength carries the KnnVectorValues.getVectorByteLength default.
func (v *BinarizedVectorValues) GetVectorByteLength() int {
	return v.Dimension() * index.VectorEncodingByteSize(v.GetEncoding())
}

// GetEncoding carries the FloatVectorValues.getEncoding override.
func (v *BinarizedVectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingFloat32
}

// getQuantizedVectorValues mirrors the package-private getQuantizedVectorValues().
func (v *BinarizedVectorValues) getQuantizedVectorValues() BinarizedByteVectorValues {
	return v.quantizedVectorValues
}

// offHeapBinarizedQueryVectorValues is the Go port of the package-private
// static nested class OffHeapBinarizedQueryVectorValues. When accessing
// vectorValue, targetOrd is a row ordinal.
type offHeapBinarizedQueryVectorValues struct {
	slice                 store.IndexInput
	dimension             int
	size                  int
	binaryValue           []byte
	byteSize              int
	correctiveValues      []float32
	lastOrd               int
	quantizedComponentSum int
}

// newOffHeapBinarizedQueryVectorValues mirrors the
// OffHeapBinarizedQueryVectorValues constructor.
func newOffHeapBinarizedQueryVectorValues(data store.IndexInput, dimension, size int) *offHeapBinarizedQueryVectorValues {
	// 4x the quantized binary dimensions
	binaryDimensions := (quantization.Discretize(dimension, 64) / 8) * int(lucene102QueryBits)
	return &offHeapBinarizedQueryVectorValues{
		slice:            data,
		dimension:        dimension,
		size:             size,
		binaryValue:      make([]byte, binaryDimensions),
		correctiveValues: make([]float32, 3),
		byteSize:         binaryDimensions + floatBytes*3 + shortBytes,
		lastOrd:          -1,
	}
}

// getCorrectiveTerms mirrors getCorrectiveTerms(int).
func (v *offHeapBinarizedQueryVectorValues) getCorrectiveTerms(targetOrd int) (quantization.QuantizationResult, error) {
	if v.lastOrd != targetOrd {
		if _, err := v.vectorValue(targetOrd); err != nil {
			return quantization.QuantizationResult{}, err
		}
	}
	return quantization.QuantizationResult{
		LowerInterval:         v.correctiveValues[0],
		UpperInterval:         v.correctiveValues[1],
		AdditionalCorrection:  v.correctiveValues[2],
		QuantizedComponentSum: v.quantizedComponentSum,
	}, nil
}

// sizeOf mirrors size().
func (v *offHeapBinarizedQueryVectorValues) sizeOf() int { return v.size }

// quantizedLength mirrors quantizedLength().
func (v *offHeapBinarizedQueryVectorValues) quantizedLength() int { return len(v.binaryValue) }

// dimensionOf mirrors dimension().
func (v *offHeapBinarizedQueryVectorValues) dimensionOf() int { return v.dimension }

// copy mirrors copy(): a new instance over a clone of the slice.
func (v *offHeapBinarizedQueryVectorValues) copy() *offHeapBinarizedQueryVectorValues {
	return newOffHeapBinarizedQueryVectorValues(v.slice.Clone(), v.dimension, v.size)
}

// getSlice mirrors getSlice().
func (v *offHeapBinarizedQueryVectorValues) getSlice() store.IndexInput { return v.slice }

// vectorValue mirrors vectorValue(int).
func (v *offHeapBinarizedQueryVectorValues) vectorValue(targetOrd int) ([]byte, error) {
	if v.lastOrd == targetOrd {
		return v.binaryValue, nil
	}
	if err := v.slice.SetPosition(int64(targetOrd) * int64(v.byteSize)); err != nil {
		return nil, err
	}
	if err := v.slice.ReadBytes(v.binaryValue, 0, len(v.binaryValue)); err != nil {
		return nil, err
	}
	if err := v.slice.ReadFloats(v.correctiveValues, 0, 3); err != nil {
		return nil, err
	}
	sum, err := v.slice.ReadShort()
	if err != nil {
		return nil, err
	}
	v.quantizedComponentSum = int(uint16(sum))
	v.lastOrd = targetOrd
	return v.binaryValue, nil
}

// lucene102NormalizedFloatVectorValues is the Go port of the package-private
// static final nested class NormalizedFloatVectorValues: the vectors of the
// wrapped values, l2-normalized.
type lucene102NormalizedFloatVectorValues struct {
	values           index.FloatVectorValues
	normalizedVector []float32
}

// newLucene102NormalizedFloatVectorValues mirrors the
// NormalizedFloatVectorValues constructor.
func newLucene102NormalizedFloatVectorValues(values index.FloatVectorValues) *lucene102NormalizedFloatVectorValues {
	return &lucene102NormalizedFloatVectorValues{values: values, normalizedVector: make([]float32, values.Dimension())}
}

// Dimension mirrors dimension().
func (n *lucene102NormalizedFloatVectorValues) Dimension() int { return n.values.Dimension() }

// Size mirrors size().
func (n *lucene102NormalizedFloatVectorValues) Size() int { return n.values.Size() }

// OrdToDoc mirrors ordToDoc(int).
func (n *lucene102NormalizedFloatVectorValues) OrdToDoc(ord int) int { return n.values.OrdToDoc(ord) }

// VectorValue mirrors vectorValue(int): the vector is copied and
// l2-normalized in place.
func (n *lucene102NormalizedFloatVectorValues) VectorValue(ord int) ([]float32, error) {
	vector, err := n.values.VectorValue(ord)
	if err != nil {
		return nil, err
	}
	copy(n.normalizedVector, vector)
	util.L2Normalize(n.normalizedVector)
	return n.normalizedVector, nil
}

// Iterator mirrors iterator().
func (n *lucene102NormalizedFloatVectorValues) Iterator() index.DocIndexIterator {
	return n.values.Iterator()
}

// copyNormalized mirrors the covariant copy().
func (n *lucene102NormalizedFloatVectorValues) copyNormalized() (*lucene102NormalizedFloatVectorValues, error) {
	values, err := n.values.CopyFloatVectorValues()
	if err != nil {
		return nil, err
	}
	return newLucene102NormalizedFloatVectorValues(values), nil
}

// Copy mirrors copy().
func (n *lucene102NormalizedFloatVectorValues) Copy() (index.KnnVectorValues, error) {
	copied, err := n.copyNormalized()
	if err != nil {
		return nil, err
	}
	return copied, nil
}

// CopyFloatVectorValues mirrors copy() typed as FloatVectorValues.
func (n *lucene102NormalizedFloatVectorValues) CopyFloatVectorValues() (index.FloatVectorValues, error) {
	copied, err := n.copyNormalized()
	if err != nil {
		return nil, err
	}
	return copied, nil
}

// Prefetch carries the KnnVectorValues.prefetch default, which does nothing.
func (n *lucene102NormalizedFloatVectorValues) Prefetch(_ []int, _ int) error { return nil }

// GetVectorByteLength carries the KnnVectorValues.getVectorByteLength default.
func (n *lucene102NormalizedFloatVectorValues) GetVectorByteLength() int {
	return n.Dimension() * index.VectorEncodingByteSize(n.GetEncoding())
}

// GetEncoding carries the FloatVectorValues.getEncoding override.
func (n *lucene102NormalizedFloatVectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingFloat32
}

// GetAcceptOrds carries the KnnVectorValues.getAcceptOrds default.
func (n *lucene102NormalizedFloatVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return spi.DefaultGetAcceptOrds(n, acceptDocs)
}

// Scorer carries the FloatVectorValues.scorer default, which throws
// UnsupportedOperationException.
func (n *lucene102NormalizedFloatVectorValues) Scorer(_ []float32) (util.VectorScorer, error) {
	return nil, quantization.ErrUnsupportedOperation
}

// Rescorer carries the FloatVectorValues.rescorer default.
func (n *lucene102NormalizedFloatVectorValues) Rescorer(target []float32) (util.VectorScorer, error) {
	return n.Scorer(target)
}

// Compile-time guards.
var (
	_ hnsw.FlatVectorsReader              = (*Lucene102BinaryQuantizedVectorsReader)(nil)
	_ quantization.QuantizedVectorsReader = (*Lucene102BinaryQuantizedVectorsReader)(nil)
	_ index.FloatVectorValues             = (*BinarizedVectorValues)(nil)
	_ index.FloatVectorValues             = (*lucene102NormalizedFloatVectorValues)(nil)
)
