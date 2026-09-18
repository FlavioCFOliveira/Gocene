// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with
//	this work for additional information regarding copyright ownership.
//	The ASF licenses this file to You under the Apache License, Version 2.0
//	(the "License"); you may not use this file except in compliance with
//	the License. You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0
//
// Source: lucene/core/src/java/org/apache/lucene/codecs/lucene104/
//
//	Lucene104ScalarQuantizedVectorsWriter.java (Lucene 10.4.0)
//
// Byte-faithful port of the per-vector optimized scalar-quantization writer.
// It composes a Lucene99FlatVectorsWriter as the raw-vector delegate (which
// owns the .vec / .vemf raw FLOAT32 files) and writes the quantized vectors
// plus their corrective factors to the .veq data file, with per-field metadata
// (CodecUtil index header + field records + footer) to the .vemq file.
//
// Deviation from the Java reference (documented and bounded):
//
//  1. The merge paths (mergeOneField / mergeOneFieldToIndex) and the
//     index-sort path (writeSortingField) are out of scope for this task: a
//     non-nil sortMap to Flush, or a WriteField call, returns an explicit
//     error rather than silently producing a non-faithful file. The dense,
//     empty and sparse flush paths are fully supported.
//  2. Asymmetric encodings (SINGLE_BIT_QUERY_NIBBLE / DIBIT_QUERY_NIBBLE) are
//     written through their symmetric doc-side packing only on the flush path;
//     the asymmetric query-side file produced by Lucene during merge is part
//     of the deferred merge path.
//
// Concurrency: not safe for concurrent use. Mirrors the Java reference.
package lucene104

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene95"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// floatBytes renders java.lang.Float.BYTES, the wire width of one FLOAT32
// sample. The Java reference spells it Float.BYTES at every use site.
const floatBytes = 4

// Lucene104ScalarQuantizedVectorsWriter is the Go port of
// org.apache.lucene.codecs.lucene104.Lucene104ScalarQuantizedVectorsWriter
// (Lucene 10.4.0). It writes optimized-scalar-quantized vector values and
// per-field metadata to the .veq / .vemq segment files, byte-for-byte
// compatible with Lucene 10.4.0.
//
// Wire-format parity (see the Lucene104ScalarQuantizedVectorsFormat Javadoc
// for the full layout):
//
//   - .veq carries, per vector: the packed quantized bytes (at the encoding's
//     configured bit-width), then three little-endian float32 corrective
//     factors (lowerInterval, upperInterval, additionalCorrection), then one
//     little-endian int32 quantizedComponentSum. For sparse fields the
//     IndexedDISI doc-id set and the DirectMonotonicWriter ord->doc data are
//     appended after the vectors. The file opens with a CodecUtil index header
//     and closes with a CodecUtil footer.
//   - .vemq carries one record per field: field number, vector encoding
//     ordinal, similarity ordinal, vint dimension, vlong .veq offset, vlong
//     .veq length, vint count, then (when count > 0) the encoding wire number,
//     the centroid as dimension little-endian float32 values, and the centroid
//     square magnitude (centroidDP) as a little-endian int32; then the
//     OrdToDocDISIReaderConfiguration stored-meta block. Terminated by an int32
//     sentinel -1 and the CodecUtil footer.
type Lucene104ScalarQuantizedVectorsWriter struct {
	*hnsw.BaseFlatVectorsWriter
	state    *codecs.SegmentWriteState
	encoding quantization.ScalarEncoding

	meta       store.IndexOutput
	vectorData store.IndexOutput

	// rawVectorDelegate owns the raw FLOAT32 vectors (.vec / .vemf). The
	// scalar writer reads the delegate's accumulated vectors back during
	// flush to compute the centroid and quantize.
	rawVectorDelegate hnsw.FlatVectorsWriter
	fields            []*scalarQuantizedFieldWriter
	finished          bool
	closed            bool
}

// NewLucene104ScalarQuantizedVectorsWriter constructs the writer bound to
// state with the chosen encoding. It creates the .veq / .vemq segment files,
// writes their CodecUtil index headers, and constructs the composed flat raw
// delegate (which creates the .vec / .vemf files and writes their headers).
// Mirrors the Java constructor
// Lucene104ScalarQuantizedVectorsWriter(SegmentWriteState, ScalarEncoding,
// FlatVectorsWriter, Lucene104ScalarQuantizedVectorScorer).
func NewLucene104ScalarQuantizedVectorsWriter(
	state *codecs.SegmentWriteState,
	encoding quantization.ScalarEncoding,
	rawVectorDelegate hnsw.FlatVectorsWriter,
	vectorsScorer *Lucene104ScalarQuantizedVectorScorer,
) (*Lucene104ScalarQuantizedVectorsWriter, error) {
	if state == nil {
		return nil, errors.New("lucene104 sq: nil SegmentWriteState")
	}
	if state.SegmentInfo == nil {
		return nil, errors.New("lucene104 sq: nil SegmentInfo")
	}
	if state.Directory == nil {
		return nil, errors.New("lucene104 sq: nil Directory")
	}

	metaName := store.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, MetaExtension)
	dataName := store.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, VectorDataExtension)

	rawMeta, err := state.Directory.CreateOutput(metaName, store.IOContextWrite)
	if err != nil {
		return nil, fmt.Errorf("lucene104 sq: create meta %q: %w", metaName, err)
	}
	meta := store.NewChecksumIndexOutput(rawMeta)

	w := &Lucene104ScalarQuantizedVectorsWriter{
		BaseFlatVectorsWriter: hnsw.NewBaseFlatVectorsWriter(vectorsScorer),
		state:                 state,
		encoding:              encoding,
		meta:                  meta,
		rawVectorDelegate:     rawVectorDelegate,
	}

	rawData, err := state.Directory.CreateOutput(dataName, store.IOContextWrite)
	if err != nil {
		_ = meta.Close()
		return nil, fmt.Errorf("lucene104 sq: create data %q: %w", dataName, err)
	}
	w.vectorData = store.NewChecksumIndexOutput(rawData)

	id := state.SegmentInfo.GetID()
	if err := codecs.WriteIndexHeader(
		w.meta, MetaCodecName, VersionCurrent, id, state.SegmentSuffix,
	); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("lucene104 sq: write meta header: %w", err)
	}
	if err := codecs.WriteIndexHeader(
		w.vectorData, VectorDataCodecName, VersionCurrent, id, state.SegmentSuffix,
	); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("lucene104 sq: write data header: %w", err)
	}

	return w, nil
}

// MergeOneFlatVectorField mirrors the Java override
// mergeOneFlatVectorField(FieldInfo, MergeState). The merge path is not yet
// ported (see the type doc): it returns an explicit error rather than
// producing a non-faithful file.
func (w *Lucene104ScalarQuantizedVectorsWriter) MergeOneFlatVectorField(
	fieldInfo *index.FieldInfo,
	mergeState *index.MergeState,
) error {
	return errors.New("lucene104 sq: MergeOneFlatVectorField not supported yet (merge path deferred)")
}

// scalarQuantizedFieldWriter accumulates per-document state for one FLOAT32
// field and tracks the running dimension sums used to compute the centroid.
// Mirrors the Java FieldWriter inner class.
type scalarQuantizedFieldWriter struct {
	fieldInfo *index.FieldInfo
	encoding  quantization.ScalarEncoding

	// delegate is the composed flat field writer that holds the raw vectors.
	// The scalar writer normalizes (for COSINE) and quantizes these vectors
	// during flush. Mirrors the Java field
	// FlatFieldVectorsWriter<float[]> flatFieldVectorsWriter.
	delegate hnsw.FlatFieldVectorsWriter[float32]

	// dimensionSums accumulates the (optionally normalized) per-dimension
	// sums across all added vectors. centroid = dimensionSums / count.
	dimensionSums []float32
	// magnitudes records the L2 magnitude of each added vector under COSINE,
	// used by normalizeVectors to scale the stored raw vectors to unit length
	// before quantization. Mirrors Java's FloatArrayList magnitudes.
	magnitudes []float32

	finished bool
}

// AddField registers a FLOAT32 vector field, allocating a per-field writer
// backed by a composed flat field writer. Mirrors Java's addField(FieldInfo).
// Non-FLOAT32 fields fall through to the raw delegate (the scalar quantizer
// only handles float vectors); such a field returns the delegate's field
// writer so byte vectors are still persisted by the raw flat format.
func (w *Lucene104ScalarQuantizedVectorsWriter) AddField(fieldInfo *index.FieldInfo) (spi.KnnFieldVectorsWriter, error) {
	if w.closed {
		return nil, errors.New("lucene104 sq: writer is closed")
	}
	if w.finished {
		return nil, errors.New("lucene104 sq: writer already finished")
	}
	if fieldInfo == nil {
		return nil, errors.New("lucene104 sq: AddField: nil FieldInfo")
	}

	delegate, err := w.rawVectorDelegate.AddField(fieldInfo)
	if err != nil {
		return nil, fmt.Errorf("lucene104 sq: raw delegate AddField: %w", err)
	}

	if fieldInfo.VectorEncoding() != index.VectorEncodingFloat32 {
		// BYTE fields are not scalar quantized; the raw flat field writer
		// owns them. Mirrors the Java branch that returns rawVectorDelegate
		// for non-FLOAT32 encodings.
		return delegate, nil
	}

	floatDelegate, ok := delegate.(hnsw.FlatFieldVectorsWriter[float32])
	if !ok {
		// Mirrors the unchecked cast to FlatFieldVectorsWriter<float[]> in
		// the Java addField, which fails with ClassCastException.
		return nil, fmt.Errorf("ClassCastException: %T cannot be cast to FlatFieldVectorsWriter<float[]>", delegate)
	}

	fw := &scalarQuantizedFieldWriter{
		fieldInfo:     fieldInfo,
		encoding:      w.encoding,
		delegate:      floatDelegate,
		dimensionSums: make([]float32, fieldInfo.VectorDimension()),
	}
	w.fields = append(w.fields, fw)
	return fw, nil
}

// AddValue records a FLOAT32 vector for docID, delegating storage of the raw
// vector to the flat writer and accumulating the dimension sums (and, for
// COSINE, the per-vector magnitude). Mirrors Java's FieldWriter.addValue.
func (fw *scalarQuantizedFieldWriter) AddValue(docID int, vectorValue any) error {
	if fw.finished {
		return errors.New("lucene104 sq: field writer already finished")
	}
	vec, ok := vectorValue.([]float32)
	if !ok {
		return fmt.Errorf("lucene104 sq: field %q expects []float32, got %T", fw.fieldInfo.Name(), vectorValue)
	}
	if err := fw.delegate.AddValue(docID, vec); err != nil {
		return err
	}
	if fw.fieldInfo.VectorSimilarityFunction() == index.VectorSimilarityFunctionCosine {
		dp := util.DotProduct(vec, vec)
		divisor := float32(math.Sqrt(float64(dp)))
		fw.magnitudes = append(fw.magnitudes, divisor)
		for i := range vec {
			fw.dimensionSums[i] += vec[i] / divisor
		}
	} else {
		for i := range vec {
			fw.dimensionSums[i] += vec[i]
		}
	}
	return nil
}

// RamBytesUsed reports the per-field in-memory footprint: the delegate's raw
// vectors plus the magnitudes slice. Mirrors Java's FieldWriter.ramBytesUsed.
func (fw *scalarQuantizedFieldWriter) RamBytesUsed() int64 {
	return fw.quantizationOverheadBytesUsed() + fw.delegate.RamBytesUsed()
}

// quantizationOverheadBytesUsed reports the RAM usage of the
// quantization-specific state only (magnitudes and dimensionSums). The
// underlying flat vector data is tracked by the rawVectorDelegate at the
// writer level to avoid double-counting. Mirrors Java's
// FieldWriter.quantizationOverheadBytesUsed.
func (fw *scalarQuantizedFieldWriter) quantizationOverheadBytesUsed() int64 {
	return int64(len(fw.magnitudes))*floatBytes + int64(len(fw.dimensionSums))*floatBytes
}

// GetVectors mirrors FieldWriter.getVectors(): the raw vectors held by the
// composed flat field writer.
func (fw *scalarQuantizedFieldWriter) GetVectors() [][]float32 {
	return fw.delegate.GetVectors()
}

// GetDocsWithFieldSet mirrors FieldWriter.getDocsWithFieldSet().
func (fw *scalarQuantizedFieldWriter) GetDocsWithFieldSet() *index.DocsWithFieldSet {
	return fw.delegate.GetDocsWithFieldSet()
}

// IsFinished mirrors FieldWriter.isFinished(): finished &&
// flatFieldVectorsWriter.isFinished().
func (fw *scalarQuantizedFieldWriter) IsFinished() bool {
	return fw.finished && fw.delegate.IsFinished()
}

// AsKnnVectorValues carries the concrete
// FlatFieldVectorsWriter.asKnnVectorValues(VectorEncoding, int).
func (fw *scalarQuantizedFieldWriter) AsKnnVectorValues(encoding index.VectorEncoding, dim int) (index.KnnVectorValues, error) {
	return hnsw.DefaultAsKnnVectorValues[float32](fw, encoding, dim)
}

// Finish marks the field complete. Mirrors Java's FieldWriter.finish, which
// returns early when already finished and otherwise asserts that the composed
// flat field writer is itself finished.
func (fw *scalarQuantizedFieldWriter) Finish() error {
	if fw.finished {
		return nil
	}
	fw.finished = true
	return nil
}

// normalizeVectors scales every stored raw vector to unit length in place,
// dividing by the magnitude recorded at add time. Mirrors Java's
// FieldWriter.normalizeVectors. Called only for COSINE, after the raw delegate
// has flushed the un-normalized vectors to .vec.
func (fw *scalarQuantizedFieldWriter) normalizeVectors() {
	for i, vec := range fw.delegate.GetVectors() {
		mag := fw.magnitudes[i]
		for j := range vec {
			vec[j] /= mag
		}
	}
}

// Flush serialises every accumulated field. It first flushes the raw delegate
// (writing the un-normalized vectors to .vec), then, per field, normalizes the
// stored vectors for COSINE, computes the centroid, trains an
// OptimizedScalarQuantizer, and writes the quantized vectors and per-field
// metadata. Mirrors Java's flush(int maxDoc, Sorter.DocMap sortMap); the
// index-sort (sortMap) path is out of scope and returns an error.
func (w *Lucene104ScalarQuantizedVectorsWriter) Flush(maxDoc int, sortMap spi.SorterDocMap) error {
	if w.closed {
		return errors.New("lucene104 sq: writer is closed")
	}
	if w.finished {
		return errors.New("lucene104 sq: writer already finished")
	}
	if sortMap != nil {
		return errors.New("lucene104 sq: index-sort (sortMap) not supported yet (merge/sort path deferred)")
	}

	// The raw delegate writes the un-normalized FLOAT32 vectors first.
	if err := w.rawVectorDelegate.Flush(maxDoc, nil); err != nil {
		return fmt.Errorf("lucene104 sq: raw delegate flush: %w", err)
	}

	for _, field := range w.fields {
		if field.fieldInfo.VectorSimilarityFunction() == index.VectorSimilarityFunctionCosine {
			field.normalizeVectors()
		}

		vectorCount := len(field.delegate.GetVectors())
		clusterCenter := make([]float32, len(field.dimensionSums))
		if vectorCount > 0 {
			for i := range field.dimensionSums {
				clusterCenter[i] = field.dimensionSums[i] / float32(vectorCount)
			}
			if field.fieldInfo.VectorSimilarityFunction() == index.VectorSimilarityFunctionCosine {
				util.L2Normalize(clusterCenter)
			}
		}

		quantizer := quantization.NewDefaultOptimizedScalarQuantizer(field.fieldInfo.VectorSimilarityFunction())
		if err := w.writeField(field, clusterCenter, maxDoc, quantizer); err != nil {
			return err
		}
		if err := field.Finish(); err != nil {
			return err
		}
	}
	return nil
}

// writeField writes one field's quantized vectors to .veq and its record to
// .vemq. Mirrors Java's private writeField.
func (w *Lucene104ScalarQuantizedVectorsWriter) writeField(
	field *scalarQuantizedFieldWriter, clusterCenter []float32, maxDoc int, quantizer *quantization.OptimizedScalarQuantizer,
) error {
	// Align the .veq write position to a float boundary, mirroring
	// vectorData.alignFilePointer(Float.BYTES).
	vectorDataOffset, err := store.AlignFilePointer(w.vectorData, floatBytes)
	if err != nil {
		return fmt.Errorf("lucene104 sq: align veq: %w", err)
	}
	if err := w.writeVectors(field, clusterCenter, quantizer); err != nil {
		return err
	}
	vectorDataLength := w.vectorData.GetFilePointer() - vectorDataOffset

	var centroidDP float32
	if len(field.GetVectors()) > 0 {
		centroidDP = util.DotProduct(clusterCenter, clusterCenter)
	}

	return w.writeMeta(field.fieldInfo, maxDoc, vectorDataOffset, vectorDataLength, clusterCenter, centroidDP, field.GetDocsWithFieldSet())
}

// writeVectors quantizes and writes every stored vector for the field to .veq.
// Mirrors Java's private writeVectors.
func (w *Lucene104ScalarQuantizedVectorsWriter) writeVectors(
	field *scalarQuantizedFieldWriter, clusterCenter []float32, quantizer *quantization.OptimizedScalarQuantizer,
) error {
	dim := field.fieldInfo.VectorDimension()
	scratch := make([]byte, w.encoding.GetDiscreteDimensions(dim))

	// The packed output buffer: for UNSIGNED_BYTE / SEVEN_BIT it aliases the
	// quantizer scratch; otherwise it is a separate packed buffer sized by the
	// doc-packed length. Mirrors the encoding switch in Java's writeVectors.
	var packed []byte
	switch w.encoding {
	case quantization.ScalarEncodingUnsignedByte, quantization.ScalarEncodingSevenBit:
		packed = scratch
	default:
		packed = make([]byte, w.encoding.GetDocPackedLength(dim))
	}

	bits := w.encoding.GetBits()
	for _, raw := range field.GetVectors() {
		// scalarQuantize mutates its input in place (centres against the
		// centroid). The flat delegate stored copies, but those copies are no
		// longer needed once written to .vec, so quantizing them in place
		// matches the Java reference, which quantizes the same in-memory
		// vectors. We still defensively copy to avoid surprising a future
		// reader of field.delegate.floats.
		vec := make([]float32, len(raw))
		copy(vec, raw)
		corrections := quantizer.ScalarQuantize(vec, scratch, bits, clusterCenter)
		if err := packQuantized(w.encoding, scratch, packed); err != nil {
			return fmt.Errorf("lucene104 sq: pack quantized: %w", err)
		}
		if err := w.vectorData.WriteBytes(packed, 0, len(packed)); err != nil {
			return err
		}
		if err := w.writeCorrections(corrections); err != nil {
			return err
		}
	}
	return nil
}

// writeCorrections writes the three corrective float32 values (as little-endian
// int32 bit patterns) followed by the int32 quantizedComponentSum. Mirrors the
// trailing four writeInt calls in Java's writeVectors.
func (w *Lucene104ScalarQuantizedVectorsWriter) writeCorrections(c quantization.QuantizationResult) error {
	if err := w.vectorData.WriteInt(int32(math.Float32bits(c.LowerInterval))); err != nil {
		return err
	}
	if err := w.vectorData.WriteInt(int32(math.Float32bits(c.UpperInterval))); err != nil {
		return err
	}
	if err := w.vectorData.WriteInt(int32(math.Float32bits(c.AdditionalCorrection))); err != nil {
		return err
	}
	return w.vectorData.WriteInt(int32(c.QuantizedComponentSum))
}

// writeMeta writes one .vemq field record. Mirrors Java's private writeMeta.
func (w *Lucene104ScalarQuantizedVectorsWriter) writeMeta(
	fieldInfo *index.FieldInfo, maxDoc int, vectorDataOffset, vectorDataLength int64,
	clusterCenter []float32, centroidDP float32, docsWithField *index.DocsWithFieldSet,
) error {
	if err := w.meta.WriteInt(int32(fieldInfo.Number())); err != nil {
		return err
	}
	// field.getVectorEncoding().ordinal(): BYTE = 0, FLOAT32 = 1, which is
	// the declaration order of index.VectorEncoding.
	if err := w.meta.WriteInt(int32(fieldInfo.VectorEncoding())); err != nil {
		return err
	}
	// field.getVectorSimilarityFunction().ordinal(): EUCLIDEAN = 0,
	// DOT_PRODUCT = 1, COSINE = 2, MAXIMUM_INNER_PRODUCT = 3, which is the
	// declaration order of util.VectorSimilarityID.
	if err := w.meta.WriteInt(int32(fieldInfo.VectorSimilarityFunction().ID())); err != nil {
		return err
	}
	if err := w.meta.WriteVInt(int32(fieldInfo.VectorDimension())); err != nil {
		return err
	}
	if err := w.meta.WriteVLong(vectorDataOffset); err != nil {
		return err
	}
	if err := w.meta.WriteVLong(vectorDataLength); err != nil {
		return err
	}
	count := docsWithField.Cardinality()
	if err := w.meta.WriteVInt(int32(count)); err != nil {
		return err
	}
	if count > 0 {
		if err := w.meta.WriteVInt(int32(w.encoding.GetWireNumber())); err != nil {
			return err
		}
		if err := writeFloatsLE(w.meta, clusterCenter); err != nil {
			return err
		}
		if err := w.meta.WriteInt(int32(math.Float32bits(centroidDP))); err != nil {
			return err
		}
	}
	return lucene95.WriteStoredMeta(
		DirectMonotonicBlockShift,
		w.meta, w.vectorData, count, maxDoc, docsWithField)
}

// WriteField is the single-reader merge entrypoint. The scalar quantizer
// buffers vectors in memory and quantizes on Flush; the merge path is
// deferred (see the type doc), so this returns an explicit error rather than
// producing a non-faithful file. Mirrors the way buffering codecs reject the
// streaming merge entrypoint.
func (w *Lucene104ScalarQuantizedVectorsWriter) WriteField(fieldInfo *index.FieldInfo, reader spi.KnnVectorsReader) error {
	_ = fieldInfo
	_ = reader
	return errors.New("lucene104 sq: WriteField (merge path) not supported yet; use AddField/AddValue/Flush")
}

// Finish writes the end-of-fields sentinel (-1) and the CodecUtil footer on
// both .vemq and .veq, after finishing the raw delegate. Mirrors Java's
// finish().
func (w *Lucene104ScalarQuantizedVectorsWriter) Finish() error {
	if w.closed {
		return errors.New("lucene104 sq: writer is closed")
	}
	if w.finished {
		return errors.New("lucene104 sq: already finished")
	}
	w.finished = true

	if w.rawVectorDelegate != nil {
		if err := w.rawVectorDelegate.Finish(); err != nil {
			return fmt.Errorf("lucene104 sq: raw delegate finish: %w", err)
		}
	}
	if w.meta != nil {
		if err := w.meta.WriteInt(-1); err != nil {
			return fmt.Errorf("lucene104 sq: write meta sentinel: %w", err)
		}
		if err := store.WriteFooter(w.meta); err != nil {
			return fmt.Errorf("lucene104 sq: write meta footer: %w", err)
		}
	}
	if w.vectorData != nil {
		if err := store.WriteFooter(w.vectorData); err != nil {
			return fmt.Errorf("lucene104 sq: write data footer: %w", err)
		}
	}
	return nil
}

// RamBytesUsed sums the in-memory footprint of every per-field buffer plus the
// raw delegate. Mirrors Java's ramBytesUsed.
func (w *Lucene104ScalarQuantizedVectorsWriter) RamBytesUsed() int64 {
	var total int64
	if w.rawVectorDelegate != nil {
		total += w.rawVectorDelegate.RamBytesUsed()
	}
	for _, f := range w.fields {
		total += f.quantizationOverheadBytesUsed()
	}
	return total
}

// Close releases the .veq / .vemq outputs and the raw delegate. Idempotent.
func (w *Lucene104ScalarQuantizedVectorsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	var firstErr error
	if w.meta != nil {
		if err := w.meta.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		w.meta = nil
	}
	if w.vectorData != nil {
		if err := w.vectorData.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		w.vectorData = nil
	}
	if w.rawVectorDelegate != nil {
		if err := w.rawVectorDelegate.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		w.rawVectorDelegate = nil
	}
	return firstErr
}

// packQuantized rearranges the quantizer's per-dimension byte output into the
// on-disk packed layout for the encoding. For UNSIGNED_BYTE / SEVEN_BIT the
// scratch already aliases packed (no-op). Mirrors the encoding switch in Java's
// writeVectors.
func packQuantized(encoding quantization.ScalarEncoding, scratch, packed []byte) error {
	switch encoding {
	case quantization.ScalarEncodingUnsignedByte, quantization.ScalarEncodingSevenBit:
		return nil // packed aliases scratch
	case quantization.ScalarEncodingPackedNibble:
		return packNibbles(scratch, packed)
	case quantization.ScalarEncodingSingleBitQueryNibble:
		quantization.PackAsBinary(scratch, packed)
		return nil
	case quantization.ScalarEncodingDibitQueryNibble:
		quantization.TransposeDibit(scratch, packed)
		return nil
	default:
		return fmt.Errorf("lucene104 sq: unsupported encoding %s", encoding)
	}
}

// packNibbles packs the per-dimension 4-bit values in unpacked into packed,
// striped so packed[i] = (unpacked[i] << 4) | unpacked[len(packed)+i]. Mirrors
// org.apache.lucene.codecs.lucene104.OffHeapScalarQuantizedVectorValues.packNibbles
// (Lucene 10.4.0) and is the exact inverse of the read-side unpackNibblesPacked.
func packNibbles(unpacked, packed []byte) error {
	if len(unpacked) != len(packed)*2 {
		return fmt.Errorf("lucene104 sq: packNibbles: unpacked len %d != 2*packed len %d", len(unpacked), len(packed))
	}
	n := len(packed)
	for i := 0; i < n; i++ {
		packed[i] = byte(int(unpacked[i])<<4 | int(unpacked[n+i]))
	}
	return nil
}

// writeFloatsLE writes each float32 in vals as a little-endian int32 bit
// pattern. Mirrors Java's ByteBuffer(LITTLE_ENDIAN).asFloatBuffer().put(...)
// followed by writeBytes, which is the centroid serialisation in writeMeta.
func writeFloatsLE(out store.IndexOutput, vals []float32) error {
	for _, v := range vals {
		if err := out.WriteInt(int32(math.Float32bits(v))); err != nil {
			return err
		}
	}
	return nil
}
