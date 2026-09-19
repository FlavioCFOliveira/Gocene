// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

// Ported from Apache Lucene 10.5.0:
//
//	lucene/core/src/java/org/apache/lucene/codecs/lucene104/Lucene104ScalarQuantizedVectorsReader.java

package lucene104

import (
	"errors"
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

// lucene104ScalarQuantizedVectorsReaderShallowSize renders
// {@code RamUsageEstimator.shallowSizeOfInstance(
// Lucene104ScalarQuantizedVectorsReader.class)}.
const lucene104ScalarQuantizedVectorsReaderShallowSize = 64

// ExhaustiveBulkScoreOrds is the batch size the exhaustive float search uses
// when bulk-scoring ordinals.
//
// Mirrors {@code public static final int EXHAUSTIVE_BULK_SCORE_ORDS = 64}.
const ExhaustiveBulkScoreOrds = 64

// lucene104ScalarQuantizedFieldEntry renders the private record
//
//	private record FieldEntry(
//	    VectorSimilarityFunction similarityFunction, VectorEncoding vectorEncoding,
//	    int dimension, long vectorDataOffset, long vectorDataLength, int size,
//	    ScalarEncoding scalarEncoding, float[] centroid, float centroidDP,
//	    OrdToDocDISIReaderConfiguration ordToDocDISIReaderConfiguration)
type lucene104ScalarQuantizedFieldEntry struct {
	similarityFunction              index.VectorSimilarityFunction
	vectorEncoding                  index.VectorEncoding
	dimension                       int
	vectorDataOffset                int64
	vectorDataLength                int64
	size                            int
	scalarEncoding                  quantization.ScalarEncoding
	centroid                        []float32
	centroidDP                      float32
	ordToDocDISIReaderConfiguration *lucene95.OrdToDocDISIReaderConfiguration
}

// createLucene104ScalarQuantizedFieldEntry reproduces the static factory
// {@code FieldEntry.create(IndexInput, VectorEncoding, VectorSimilarityFunction)}.
func createLucene104ScalarQuantizedFieldEntry(
	input store.IndexInput,
	vectorEncoding index.VectorEncoding,
	similarityFunction index.VectorSimilarityFunction,
) (*lucene104ScalarQuantizedFieldEntry, error) {
	dimension, err := input.ReadVInt()
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
	size, err := input.ReadVInt()
	if err != nil {
		return nil, err
	}
	var centroid []float32
	var centroidDP float32
	scalarEncoding := quantization.ScalarEncodingUnsignedByte
	if size > 0 {
		wireNumber, err := input.ReadVInt()
		if err != nil {
			return nil, err
		}
		enc, ok := quantization.ScalarEncodingFromWireNumber(int(wireNumber))
		if !ok {
			return nil, fmt.Errorf("Could not get ScalarEncoding from wire number: %d", wireNumber)
		}
		scalarEncoding = enc
		centroid = make([]float32, dimension)
		if err := input.ReadFloats(centroid, 0, int(dimension)); err != nil {
			return nil, err
		}
		bits, err := input.ReadInt()
		if err != nil {
			return nil, err
		}
		centroidDP = math.Float32frombits(uint32(bits))
	}
	conf, err := lucene95.FromStoredMeta(input, int(size))
	if err != nil {
		return nil, err
	}
	return &lucene104ScalarQuantizedFieldEntry{
		similarityFunction:              similarityFunction,
		vectorEncoding:                  vectorEncoding,
		dimension:                       int(dimension),
		vectorDataOffset:                vectorDataOffset,
		vectorDataLength:                vectorDataLength,
		size:                            int(size),
		scalarEncoding:                  scalarEncoding,
		centroid:                        centroid,
		centroidDP:                      centroidDP,
		ordToDocDISIReaderConfiguration: conf,
	}, nil
}

// Lucene104ScalarQuantizedVectorsReader is the reader for scalar quantized
// vectors in the Lucene 10.4 format.
//
// Mirrors {@code public class Lucene104ScalarQuantizedVectorsReader extends
// FlatVectorsReader implements QuantizedVectorsReader} (Lucene 10.5.0,
// @lucene.experimental).
type Lucene104ScalarQuantizedVectorsReader struct {
	fields              map[string]*lucene104ScalarQuantizedFieldEntry
	quantizedVectorData store.IndexInput
	rawVectorsReader    hnsw.FlatVectorsReader
	vectorScorer        *Lucene104ScalarQuantizedVectorScorer
}

// NewLucene104ScalarQuantizedVectorsReader reproduces the three-argument
// constructor, whose body is
//
//	// Quantized vectors are accessed randomly from their node ID stored in
//	// the HNSW graph.
//	this(state, rawVectorsReader, vectorsScorer, DataAccessHint.RANDOM);
func NewLucene104ScalarQuantizedVectorsReader(
	state *codecs.SegmentReadState,
	rawVectorsReader hnsw.FlatVectorsReader,
	vectorsScorer *Lucene104ScalarQuantizedVectorScorer,
) (*Lucene104ScalarQuantizedVectorsReader, error) {
	return NewLucene104ScalarQuantizedVectorsReaderWithHint(
		state, rawVectorsReader, vectorsScorer, spi.DataAccessRandom)
}

// NewLucene104ScalarQuantizedVectorsReaderWithHint reproduces the
// four-argument constructor that carries the DataAccessHint.
func NewLucene104ScalarQuantizedVectorsReaderWithHint(
	state *codecs.SegmentReadState,
	rawVectorsReader hnsw.FlatVectorsReader,
	vectorsScorer *Lucene104ScalarQuantizedVectorScorer,
	accessHint spi.DataAccessHint,
) (*Lucene104ScalarQuantizedVectorsReader, error) {
	r := &Lucene104ScalarQuantizedVectorsReader{
		fields:           make(map[string]*lucene104ScalarQuantizedFieldEntry),
		rawVectorsReader: rawVectorsReader,
		vectorScorer:     vectorsScorer,
	}

	metaFileName := store.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, MetaExtension)
	metaRaw, err := state.Directory.OpenInput(metaFileName, spi.IOContextReadOnce)
	if err != nil {
		return nil, err
	}
	meta := store.NewChecksumIndexInput(metaRaw)

	versionMeta := int32(-1)
	var priorE error
	versionMeta, priorE = codecs.CheckIndexHeader(
		meta, MetaCodecName, VersionStart, VersionCurrent,
		state.SegmentInfo.GetID(), state.SegmentSuffix,
	)
	if priorE == nil {
		priorE = r.readFields(meta, state.FieldInfos)
	}
	if _, footerErr := store.CheckFooter(meta); priorE == nil && footerErr != nil {
		priorE = footerErr
	}
	_ = metaRaw.Close()
	if priorE != nil {
		return nil, priorE
	}

	// final IOContext.FileOpenHint[] hints =
	//     Stream.of(FileTypeHint.DATA, FileDataHint.KNN_VECTORS, accessHint)
	//         .filter(Objects::nonNull).toArray(IOContext.FileOpenHint[]::new);
	hints := []spi.FileOpenHint{spi.FileTypeData, spi.FileDataKNNVectors, accessHint}
	quantizedVectorData, err := openScalarQuantizedDataInput(
		state, versionMeta, VectorDataExtension, VectorDataCodecName,
		state.Context.WithHints(hints...),
	)
	if err != nil {
		return nil, err
	}
	r.quantizedVectorData = quantizedVectorData
	return r, nil
}

// readFields reproduces
//
//	for (int fieldNumber = meta.readInt(); fieldNumber != -1; fieldNumber = meta.readInt()) {
//	  FieldInfo info = infos.fieldInfo(fieldNumber);
//	  if (info == null) throw new CorruptIndexException("Invalid field number: " + fieldNumber, meta);
//	  FieldEntry fieldEntry = readField(meta, info);
//	  validateFieldEntry(info, fieldEntry);
//	  fields.put(info.name, fieldEntry);
//	}
func (r *Lucene104ScalarQuantizedVectorsReader) readFields(meta store.IndexInput, infos *index.FieldInfos) error {
	for {
		fieldNumber, err := meta.ReadInt()
		if err != nil {
			return err
		}
		if fieldNumber == -1 {
			return nil
		}
		var info *index.FieldInfo
		if infos != nil {
			info = infos.GetByNumber(int(fieldNumber))
		}
		if info == nil {
			return fmt.Errorf("Invalid field number: %d", fieldNumber)
		}
		fieldEntry, err := readScalarQuantizedField(meta, info)
		if err != nil {
			return err
		}
		if err := validateScalarQuantizedFieldEntry(info, fieldEntry); err != nil {
			return err
		}
		r.fields[info.Name()] = fieldEntry
	}
}

// validateScalarQuantizedFieldEntry reproduces the package-private static
// {@code validateFieldEntry(FieldInfo info, FieldEntry fieldEntry)}.
func validateScalarQuantizedFieldEntry(info *index.FieldInfo, fieldEntry *lucene104ScalarQuantizedFieldEntry) error {
	dimension := info.VectorDimension()
	if dimension != fieldEntry.dimension {
		return fmt.Errorf("Inconsistent vector dimension for field=%q; %d != %d",
			info.Name(), dimension, fieldEntry.dimension)
	}
	numQuantizedVectorBytes := int64(
		fieldEntry.scalarEncoding.GetDocPackedLength(dimension)+(4*3)+4) * int64(fieldEntry.size)
	if numQuantizedVectorBytes != fieldEntry.vectorDataLength {
		return fmt.Errorf("vector data length %d not matching size = %d * (dims=%d + 16) = %d",
			fieldEntry.vectorDataLength, fieldEntry.size, dimension, numQuantizedVectorBytes)
	}
	return nil
}

// GetFlatVectorScorer reproduces {@code return vectorScorer;}.
func (r *Lucene104ScalarQuantizedVectorsReader) GetFlatVectorScorer(_ string) (hnsw.FlatVectorsScorer, error) {
	return r.vectorScorer, nil
}

// GetRandomVectorScorerFloat reproduces the float[] overload of
// getRandomVectorScorer.
func (r *Lucene104ScalarQuantizedVectorsReader) GetRandomVectorScorerFloat(
	field string, target []float32,
) (utilhnsw.RandomVectorScorer, error) {
	fi, ok := r.fields[field]
	if !ok {
		return nil, nil
	}
	values, err := loadOffHeapScalarQuantizedVectorValues(
		fi.ordToDocDISIReaderConfiguration,
		fi.dimension,
		fi.size,
		quantization.NewDefaultOptimizedScalarQuantizer(fi.similarityFunction),
		fi.scalarEncoding,
		fi.similarityFunction,
		r.vectorScorer,
		fi.centroid,
		fi.centroidDP,
		fi.vectorDataOffset,
		fi.vectorDataLength,
		r.quantizedVectorData,
	)
	if err != nil {
		return nil, err
	}
	return r.vectorScorer.GetRandomVectorScorer(fi.similarityFunction, values, target)
}

// GetRandomVectorScorerByte reproduces the byte[] overload, whose body is
// {@code return rawVectorsReader.getRandomVectorScorer(field, target);}.
func (r *Lucene104ScalarQuantizedVectorsReader) GetRandomVectorScorerByte(
	field string, target []byte,
) (utilhnsw.RandomVectorScorer, error) {
	return r.rawVectorsReader.GetRandomVectorScorerByte(field, target)
}

// CheckIntegrity reproduces
//
//	rawVectorsReader.checkIntegrity();
//	CodecUtil.checksumEntireFile(quantizedVectorData);
func (r *Lucene104ScalarQuantizedVectorsReader) CheckIntegrity() error {
	if err := r.rawVectorsReader.CheckIntegrity(); err != nil {
		return err
	}
	_, err := codecs.ChecksumEntireFile(r.quantizedVectorData)
	return err
}

// GetFloatVectorValues reproduces getFloatVectorValues(String).
func (r *Lucene104ScalarQuantizedVectorsReader) GetFloatVectorValues(field string) (spi.FloatVectorValues, error) {
	fi, ok := r.fields[field]
	if !ok {
		return nil, nil
	}
	if fi.vectorEncoding != util.VectorEncodingFloat32 {
		return nil, fmt.Errorf("field=%q is encoded as: %v expected: %v",
			field, fi.vectorEncoding, util.VectorEncodingFloat32)
	}

	rawFloatVectorValues, err := r.rawVectorsReader.GetFloatVectorValues(field)
	if err != nil {
		return nil, err
	}

	if rawFloatVectorValues.Size() == 0 {
		return loadOffHeapScalarQuantizedFloatVectorValues(
			fi.ordToDocDISIReaderConfiguration,
			fi.dimension,
			fi.size,
			fi.scalarEncoding,
			fi.similarityFunction,
			r.vectorScorer,
			fi.centroid,
			fi.vectorDataOffset,
			fi.vectorDataLength,
			r.quantizedVectorData,
		)
	}

	sqvv, err := loadOffHeapScalarQuantizedVectorValues(
		fi.ordToDocDISIReaderConfiguration,
		fi.dimension,
		fi.size,
		quantization.NewDefaultOptimizedScalarQuantizer(fi.similarityFunction),
		fi.scalarEncoding,
		fi.similarityFunction,
		r.vectorScorer,
		fi.centroid,
		fi.centroidDP,
		fi.vectorDataOffset,
		fi.vectorDataLength,
		r.quantizedVectorData,
	)
	if err != nil {
		return nil, err
	}
	return newScalarQuantizedVectorValues(rawFloatVectorValues, sqvv), nil
}

// GetByteVectorValues reproduces
// {@code return rawVectorsReader.getByteVectorValues(field);}.
func (r *Lucene104ScalarQuantizedVectorsReader) GetByteVectorValues(field string) (spi.ByteVectorValues, error) {
	return r.rawVectorsReader.GetByteVectorValues(field)
}

// SearchByte reproduces
// {@code rawVectorsReader.search(field, target, knnCollector, acceptDocs);}.
func (r *Lucene104ScalarQuantizedVectorsReader) SearchByte(
	field string, target []byte, knnCollector spi.KnnCollector, acceptDocs search.AcceptDocs,
) error {
	return r.rawVectorsReader.SearchByte(field, target, knnCollector, acceptDocs)
}

// SearchFloat reproduces search(String, float[], KnnCollector, AcceptDocs):
// an exhaustive scan that bulk-scores accepted ordinals in batches of
// EXHAUSTIVE_BULK_SCORE_ORDS.
func (r *Lucene104ScalarQuantizedVectorsReader) SearchFloat(
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
	bits, err := acceptDocs.Bits()
	if err != nil {
		return err
	}
	acceptedOrds := scorer.GetAcceptOrds(bits)
	// if k is larger than the number of vectors we expect to visit in an HNSW
	// search, we can just iterate over all vectors and collect them.
	ords := make([]int, ExhaustiveBulkScoreOrds)
	scores := make([]float32, ExhaustiveBulkScoreOrds)
	numOrds := 0
	numVectors := scorer.MaxOrd()
	for i := 0; i < numVectors; i++ {
		if acceptedOrds == nil || acceptedOrds.Get(i) {
			if knnCollector.EarlyTerminated() {
				break
			}
			ords[numOrds] = i
			numOrds++
			if numOrds == len(ords) {
				knnCollector.IncVisitedCount(numOrds)
				maxScore, err := scorer.BulkScore(ords, scores, numOrds)
				if err != nil {
					return err
				}
				if maxScore > knnCollector.MinCompetitiveSimilarity() {
					for j := 0; j < numOrds; j++ {
						knnCollector.Collect(scorer.OrdToDoc(ords[j]), scores[j])
					}
				}
				numOrds = 0
			}
		}
	}

	if numOrds > 0 {
		knnCollector.IncVisitedCount(numOrds)
		maxScore, err := scorer.BulkScore(ords, scores, numOrds)
		if err != nil {
			return err
		}
		if maxScore > knnCollector.MinCompetitiveSimilarity() {
			for j := 0; j < numOrds; j++ {
				knnCollector.Collect(scorer.OrdToDoc(ords[j]), scores[j])
			}
		}
	}
	return nil
}

// Close reproduces {@code IOUtils.close(quantizedVectorData, rawVectorsReader);}.
func (r *Lucene104ScalarQuantizedVectorsReader) Close() error {
	var first error
	if r.quantizedVectorData != nil {
		if err := r.quantizedVectorData.Close(); err != nil {
			first = err
		}
	}
	if r.rawVectorsReader != nil {
		if err := r.rawVectorsReader.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// RamBytesUsed reproduces
//
//	long size = SHALLOW_SIZE;
//	size += RamUsageEstimator.sizeOfMap(fields, RamUsageEstimator.shallowSizeOfInstance(FieldEntry.class));
//	size += rawVectorsReader.ramBytesUsed();
//	return size;
func (r *Lucene104ScalarQuantizedVectorsReader) RamBytesUsed() int64 {
	size := int64(lucene104ScalarQuantizedVectorsReaderShallowSize)
	size += util.ShallowSizeOf(r.fields)
	if r.rawVectorsReader != nil {
		size += r.rawVectorsReader.RamBytesUsed()
	}
	return size
}

// GetOffHeapByteSize reproduces getOffHeapByteSize(FieldInfo).
func (r *Lucene104ScalarQuantizedVectorsReader) GetOffHeapByteSize(fieldInfo *spi.FieldInfo) map[string]int64 {
	if fieldInfo == nil {
		return nil
	}
	raw := r.rawVectorsReader.GetOffHeapByteSize(fieldInfo)
	fieldEntry, ok := r.fields[fieldInfo.Name()]
	if !ok {
		// assert fieldInfo.getVectorEncoding() == VectorEncoding.BYTE;
		return raw
	}
	quant := map[string]int64{VectorDataExtension: fieldEntry.vectorDataLength}
	return codecs.MergeOffHeapByteSizeMaps(raw, quant)
}

// GetMergeInstance carries the KnnVectorsReader default
// {@code return this;}: Lucene104ScalarQuantizedVectorsReader does not
// override it.
func (r *Lucene104ScalarQuantizedVectorsReader) GetMergeInstance() (spi.KnnVectorsReader, error) {
	return r, nil
}

// FinishMerge carries the KnnVectorsReader default, whose body is empty:
// Lucene104ScalarQuantizedVectorsReader does not override it.
func (r *Lucene104ScalarQuantizedVectorsReader) FinishMerge() error { return nil }

// GetCentroid reproduces
//
//	FieldEntry fieldEntry = fields.get(field);
//	if (fieldEntry != null) return fieldEntry.centroid;
//	return null;
func (r *Lucene104ScalarQuantizedVectorsReader) GetCentroid(field string) []float32 {
	if fieldEntry, ok := r.fields[field]; ok {
		return fieldEntry.centroid
	}
	return nil
}

// openScalarQuantizedDataInput reproduces the private static
// {@code openDataInput(SegmentReadState, int versionMeta, String
// fileExtension, String codecName, IOContext context)}.
func openScalarQuantizedDataInput(
	state *codecs.SegmentReadState,
	versionMeta int32,
	fileExtension string,
	codecName string,
	context spi.IOContext,
) (store.IndexInput, error) {
	fileName := store.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, fileExtension)
	in, err := state.Directory.OpenInput(fileName, context)
	if err != nil {
		return nil, err
	}
	versionVectorData, err := codecs.CheckIndexHeader(
		in, codecName, VersionStart, VersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix)
	if err != nil {
		_ = in.Close()
		return nil, err
	}
	if versionMeta != versionVectorData {
		_ = in.Close()
		return nil, fmt.Errorf("Format versions mismatch: meta=%d, %s=%d",
			versionMeta, codecName, versionVectorData)
	}
	if _, err := codecs.RetrieveChecksum(in); err != nil {
		_ = in.Close()
		return nil, err
	}
	return in, nil
}

// readScalarQuantizedField reproduces the private
// {@code readField(IndexInput input, FieldInfo info)}.
func readScalarQuantizedField(input store.IndexInput, info *index.FieldInfo) (*lucene104ScalarQuantizedFieldEntry, error) {
	vectorEncoding, err := codecs.ReadVectorEncoding(input)
	if err != nil {
		return nil, err
	}
	similarityFunction, err := codecs.ReadSimilarityFunction(input)
	if err != nil {
		return nil, err
	}
	if similarityFunction != info.VectorSimilarityFunction() {
		return nil, fmt.Errorf("Inconsistent vector similarity function for field=%q; %v != %v",
			info.Name(), similarityFunction, info.VectorSimilarityFunction())
	}
	return createLucene104ScalarQuantizedFieldEntry(input, vectorEncoding, info.VectorSimilarityFunction())
}

// GetQuantizedVectorValues reproduces getQuantizedVectorValues(String), the
// QuantizedVectorsReader member.
func (r *Lucene104ScalarQuantizedVectorsReader) GetQuantizedVectorValues(
	field string,
) (quantization.BaseQuantizedByteVectorValues, error) {
	fi, ok := r.fields[field]
	if !ok {
		return nil, nil
	}
	if fi.vectorEncoding != util.VectorEncodingFloat32 {
		return nil, fmt.Errorf("field=%q is encoded as: %v expected: %v",
			field, fi.vectorEncoding, util.VectorEncodingFloat32)
	}
	return loadOffHeapScalarQuantizedVectorValues(
		fi.ordToDocDISIReaderConfiguration,
		fi.dimension,
		fi.size,
		quantization.NewDefaultOptimizedScalarQuantizer(fi.similarityFunction),
		fi.scalarEncoding,
		fi.similarityFunction,
		r.vectorScorer,
		fi.centroid,
		fi.centroidDP,
		fi.vectorDataOffset,
		fi.vectorDataLength,
		r.quantizedVectorData,
	)
}

// GetQuantizationState reproduces {@code return null;}.
func (r *Lucene104ScalarQuantizedVectorsReader) GetQuantizationState(_ string) *quantization.ScalarQuantizer {
	return nil
}

// tempOutputCreator is the optional Directory capability that carries
// {@code Directory#createTempOutput(String, String, IOContext)}, which
// spi.Directory does not declare. Gocene already reaches the member this way
// in util/bkd and index/TrackingTmpOutputDirectoryWrapper.
type tempOutputCreator interface {
	CreateTempOutput(prefix, suffix string, ctx store.IOContext) (store.IndexOutput, error)
}

// fileDeleter is the optional Directory capability that carries
// {@code IOUtils.deleteFilesIgnoringExceptions(Directory, String...)}.
type fileDeleter interface {
	DeleteFile(name string) error
}

// GetRandomVectorScorerSupplierForMerge reproduces
// getRandomVectorScorerSupplierForMerge(FieldInfo, SegmentWriteState), the
// QuantizedVectorsReader member.
func (r *Lucene104ScalarQuantizedVectorsReader) GetRandomVectorScorerSupplierForMerge(
	fieldInfo *spi.FieldInfo, segmentWriteState *spi.SegmentWriteState,
) (utilhnsw.CloseableRandomVectorScorerSupplier, error) {
	fi, ok := r.fields[fieldInfo.Name()]
	if !ok {
		return nil, nil
	}
	vectorValuesBase, err := r.GetQuantizedVectorValues(fieldInfo.Name())
	if err != nil {
		return nil, err
	}
	vectorValues, ok := vectorValuesBase.(quantization.QuantizedByteVectorValues)
	if !ok {
		return nil, fmt.Errorf("lucene104 sq: %T is not a QuantizedByteVectorValues", vectorValuesBase)
	}
	if !fi.scalarEncoding.IsAsymmetric() {
		supplier, err := r.vectorScorer.GetRandomVectorScorerSupplier(
			fieldInfo.VectorSimilarityFunction(), vectorValues)
		if err != nil {
			return nil, err
		}
		return utilhnsw.CreateCloseableRandomVectorScorerSupplier(
			supplier, vectorValues.Size(), func() error { return nil }), nil
	}

	floatVectorValues, err := r.GetFloatVectorValues(fieldInfo.Name())
	if err != nil {
		return nil, err
	}
	quantizer := quantization.NewDefaultOptimizedScalarQuantizer(fieldInfo.VectorSimilarityFunction())

	creator, ok := segmentWriteState.Directory.(tempOutputCreator)
	if !ok {
		return nil, fmt.Errorf("lucene104 sq: directory %T does not support CreateTempOutput",
			segmentWriteState.Directory)
	}
	tempScoreQuantizedVector, err := creator.CreateTempOutput(
		segmentWriteState.SegmentInfo.Name(), "queries", segmentWriteState.Context)
	if err != nil {
		return nil, err
	}
	tempScoreQuantizedVectorName := tempScoreQuantizedVector.GetName()
	docsWithField, writeErr := writeBinarizedQueryData(
		vectorValues, fi.scalarEncoding, tempScoreQuantizedVector, floatVectorValues, quantizer)
	if writeErr == nil {
		writeErr = codecs.WriteFooter(tempScoreQuantizedVector)
	}
	closeErr := tempScoreQuantizedVector.Close()
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		deleteFileIgnoringExceptions(segmentWriteState.Directory, tempScoreQuantizedVectorName)
		return nil, writeErr
	}

	quantizedScoreDataInput, err := segmentWriteState.Directory.OpenInput(
		tempScoreQuantizedVectorName, segmentWriteState.Context)
	if err != nil {
		deleteFileIgnoringExceptions(segmentWriteState.Directory, tempScoreQuantizedVectorName)
		return nil, err
	}
	centroid, err := vectorValues.GetCentroid()
	if err != nil {
		_ = quantizedScoreDataInput.Close()
		deleteFileIgnoringExceptions(segmentWriteState.Directory, tempScoreQuantizedVectorName)
		return nil, err
	}
	centroidDP, err := vectorValues.GetCentroidDP()
	if err != nil {
		_ = quantizedScoreDataInput.Close()
		deleteFileIgnoringExceptions(segmentWriteState.Directory, tempScoreQuantizedVectorName)
		return nil, err
	}
	scoreVectorValues := newDenseOffHeapVectorValuesQuerySide(
		true,
		fieldInfo.VectorDimension(),
		docsWithField.Cardinality(),
		centroid,
		centroidDP,
		quantizer,
		fi.scalarEncoding,
		fieldInfo.VectorSimilarityFunction(),
		r.vectorScorer,
		quantizedScoreDataInput,
	)
	scorerSupplier := r.vectorScorer.GetAsymmetricQuantizedRandomVectorScorerSupplier(
		fieldInfo.VectorSimilarityFunction(), scoreVectorValues, vectorValues)
	return utilhnsw.CreateCloseableRandomVectorScorerSupplier(
		scorerSupplier,
		vectorValues.Size(),
		func() error {
			err := quantizedScoreDataInput.Close()
			deleteFileIgnoringExceptions(segmentWriteState.Directory, tempScoreQuantizedVectorName)
			return err
		},
	), nil
}

// deleteFileIgnoringExceptions renders
// {@code IOUtils.deleteFilesIgnoringExceptions(Directory, String...)}.
func deleteFileIgnoringExceptions(dir spi.Directory, name string) {
	if d, ok := dir.(fileDeleter); ok {
		// The error is discarded deliberately: Lucene's
		// deleteFilesIgnoringExceptions swallows it by contract.
		_ = d.DeleteFile(name)
	}
}

// writeBinarizedQueryData reproduces the package-private static
// {@code writeBinarizedQueryData(QuantizedByteVectorValues, ScalarEncoding,
// IndexOutput, FloatVectorValues, OptimizedScalarQuantizer)}.
func writeBinarizedQueryData(
	quantizedByteVectorValues quantization.QuantizedByteVectorValues,
	encoding quantization.ScalarEncoding,
	binarizedQueryData store.IndexOutput,
	floatVectorValues spi.FloatVectorValues,
	binaryQuantizer *quantization.OptimizedScalarQuantizer,
) (*index.DocsWithFieldSet, error) {
	if !encoding.IsAsymmetric() {
		return nil, errors.New("encoding and queryEncoding must be different")
	}
	docsWithField := index.NewDocsWithFieldSet()
	discretizedDims := encoding.GetDiscreteDimensions(floatVectorValues.Dimension())
	quantizationScratch := make([]byte, discretizedDims)
	toQuery := make([]byte, encoding.GetQueryPackedLength(discretizedDims))
	centroid, err := quantizedByteVectorValues.GetCentroid()
	if err != nil {
		return nil, err
	}
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
		res := binaryQuantizer.ScalarQuantize(
			vector, quantizationScratch, encoding.GetQueryBits(), centroid)
		if err := docsWithField.Add(docV); err != nil {
			return nil, err
		}
		// pack and store the 4bit query vector
		quantization.TransposeHalfByte(quantizationScratch, toQuery)
		if err := binarizedQueryData.WriteBytes(toQuery, 0, len(toQuery)); err != nil {
			return nil, err
		}
		if err := binarizedQueryData.WriteInt(int32(math.Float32bits(res.LowerInterval))); err != nil {
			return nil, err
		}
		if err := binarizedQueryData.WriteInt(int32(math.Float32bits(res.UpperInterval))); err != nil {
			return nil, err
		}
		if err := binarizedQueryData.WriteInt(int32(math.Float32bits(res.AdditionalCorrection))); err != nil {
			return nil, err
		}
		if err := binarizedQueryData.WriteInt(int32(res.QuantizedComponentSum)); err != nil {
			return nil, err
		}
	}
	return docsWithField, nil
}

// ─── ScalarQuantizedVectorValues ────────────────────────────────────────────

// scalarQuantizedVectorValues are vector values holding raw and quantized
// vector values.
//
// Mirrors the nested class
// {@code protected static final class
// Lucene104ScalarQuantizedVectorsReader.ScalarQuantizedVectorValues extends
// FloatVectorValues}.
type scalarQuantizedVectorValues struct {
	rawVectorValues       spi.FloatVectorValues
	quantizedVectorValues quantization.QuantizedByteVectorValues
}

// newScalarQuantizedVectorValues reproduces the package-private constructor.
func newScalarQuantizedVectorValues(
	rawVectorValues spi.FloatVectorValues,
	quantizedVectorValues quantization.QuantizedByteVectorValues,
) *scalarQuantizedVectorValues {
	return &scalarQuantizedVectorValues{
		rawVectorValues:       rawVectorValues,
		quantizedVectorValues: quantizedVectorValues,
	}
}

// Dimension reproduces {@code return rawVectorValues.dimension();}.
func (v *scalarQuantizedVectorValues) Dimension() int { return v.rawVectorValues.Dimension() }

// Size reproduces {@code return rawVectorValues.size();}.
func (v *scalarQuantizedVectorValues) Size() int { return v.rawVectorValues.Size() }

// VectorValue reproduces {@code return rawVectorValues.vectorValue(ord);}.
func (v *scalarQuantizedVectorValues) VectorValue(ord int) ([]float32, error) {
	return v.rawVectorValues.VectorValue(ord)
}

// copyScalarQuantized reproduces the covariant
// {@code public ScalarQuantizedVectorValues copy()}.
func (v *scalarQuantizedVectorValues) copyScalarQuantized() (*scalarQuantizedVectorValues, error) {
	raw, err := v.rawVectorValues.CopyFloatVectorValues()
	if err != nil {
		return nil, err
	}
	quantized, err := v.quantizedVectorValues.CopyQuantizedByteVectorValues()
	if err != nil {
		return nil, err
	}
	return newScalarQuantizedVectorValues(raw, quantized), nil
}

// Copy carries KnnVectorValues.copy().
func (v *scalarQuantizedVectorValues) Copy() (spi.KnnVectorValues, error) {
	return v.copyScalarQuantized()
}

// CopyFloatVectorValues carries the covariant FloatVectorValues.copy().
func (v *scalarQuantizedVectorValues) CopyFloatVectorValues() (spi.FloatVectorValues, error) {
	return v.copyScalarQuantized()
}

// GetAcceptOrds reproduces
// {@code return rawVectorValues.getAcceptOrds(acceptDocs);}.
func (v *scalarQuantizedVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return v.rawVectorValues.GetAcceptOrds(acceptDocs)
}

// OrdToDoc reproduces {@code return rawVectorValues.ordToDoc(ord);}.
func (v *scalarQuantizedVectorValues) OrdToDoc(ord int) int {
	return v.rawVectorValues.OrdToDoc(ord)
}

// Iterator reproduces {@code return rawVectorValues.iterator();}.
func (v *scalarQuantizedVectorValues) Iterator() spi.DocIndexIterator {
	return v.rawVectorValues.Iterator()
}

// Scorer reproduces {@code return quantizedVectorValues.scorer(query);}.
func (v *scalarQuantizedVectorValues) Scorer(query []float32) (util.VectorScorer, error) {
	return v.quantizedVectorValues.ScorerFloat(query)
}

// Rescorer reproduces {@code return rawVectorValues.rescorer(target);}.
func (v *scalarQuantizedVectorValues) Rescorer(target []float32) (util.VectorScorer, error) {
	return v.rawVectorValues.Rescorer(target)
}

// GetQuantizedVectorValues reproduces the package-private
// {@code QuantizedByteVectorValues getQuantizedVectorValues()}.
func (v *scalarQuantizedVectorValues) GetQuantizedVectorValues() (quantization.QuantizedByteVectorValues, error) {
	return v.quantizedVectorValues, nil
}

// Prefetch carries the KnnVectorValues default, whose body is empty.
func (v *scalarQuantizedVectorValues) Prefetch(ordsToPrefetch []int, numOrds int) error {
	return v.rawVectorValues.Prefetch(ordsToPrefetch, numOrds)
}

// GetVectorByteLength carries the KnnVectorValues default, which
// FloatVectorValues resolves as dimension * Float.BYTES.
func (v *scalarQuantizedVectorValues) GetVectorByteLength() int {
	return v.rawVectorValues.GetVectorByteLength()
}

// GetEncoding carries FloatVectorValues.getEncoding(), whose body is
// {@code return VectorEncoding.FLOAT32;}.
func (v *scalarQuantizedVectorValues) GetEncoding() spi.VectorEncoding {
	return util.VectorEncodingFloat32
}

var (
	_ hnsw.FlatVectorsReader              = (*Lucene104ScalarQuantizedVectorsReader)(nil)
	_ quantization.QuantizedVectorsReader = (*Lucene104ScalarQuantizedVectorsReader)(nil)
	_ spi.FloatVectorValues               = (*scalarQuantizedVectorValues)(nil)
)
