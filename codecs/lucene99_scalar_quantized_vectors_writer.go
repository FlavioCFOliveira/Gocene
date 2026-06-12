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
// Source: lucene/backward-codecs/src/test/org/apache/lucene/backward_codecs/lucene99/
//         Lucene99ScalarQuantizedVectorsWriter.java (Lucene 10.4.0)
//
// Test-only backward-compat writer that produces .veq / .vemq files in the
// Lucene 9.9 scalar-quantized vector format. The production
// Lucene99ScalarQuantizedVectorsFormat#fieldsWriter in Lucene 10.4.0 throws
// UnsupportedOperationException; this writer exists solely so Gocene can emit
// fixture segments that Java Lucene 10.4.0's backward-codecs reader can verify
// with CheckIndex.
//
// Wire-format parity:
//
//   - .veq carries, per vector: the quantized bytes (full dimension when
//     compress=false, dim/2 when compress=true), then one little-endian
//     float32 offset correction. Aligned to float boundary per field.
//     Opens with a CodecUtil index header and closes with a CodecUtil footer.
//   - .vemq carries one record per field: field number, vector encoding
//     ordinal, similarity ordinal, vlong .veq offset, vlong .veq length,
//     vint dimension, vint count, then (count>0, version>=1) confidenceInterval
//     as int32 float bits (-1 for null), bits byte, compress byte, then
//     lowerQuantile and upperQuantile as int32 float bits, then the
//     OrdToDocDISIReaderConfiguration stored-meta block. Terminated by an
//     int32 sentinel -1 and the CodecUtil footer.
//
// Deviation from the Java reference:
//
//  1. The merge paths (mergeOneField / mergeOneFieldToIndex / WriteField) and
//     the index-sort path (sortMap in Flush) are out of scope; they return
//     an explicit error rather than silently producing a non-faithful file.
//  2. Only the flush path (AddField / AddValue / Flush / Finish / Close) is
//     supported; this is sufficient for CheckIndex validation.

package codecs

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// Lucene99 scalar-quantized vector wire-level constants. Mirror the static
// definitions in org.apache.lucene.backward_codecs.lucene99.
// Lucene99ScalarQuantizedVectorsFormat (Lucene 10.4.0).
const (
	lucene99SQMetaCodecName     = "Lucene99ScalarQuantizedVectorsFormatMeta"
	lucene99SQDataCodecName     = "Lucene99ScalarQuantizedVectorsFormatData"
	lucene99SQMetaExtension     = "vemq"
	lucene99SQDataExtension     = "veq"
	lucene99SQVersionStart      int32 = 0
	lucene99SQVersionAddBits    int32 = 1
	lucene99SQVersionCurrent    int32 = lucene99SQVersionAddBits
	lucene99SQDirectMonotonicBlockShift = 16
	lucene99SQMinimumConfidenceInterval = 0.9
)

// Lucene99ScalarQuantizedVectorsWriter is the Go port of the Java test-only
// writer org.apache.lucene.backward_codecs.lucene99.Lucene99ScalarQuantizedVectorsWriter.
// It writes quantized vector values and metadata to .veq / .vemq segment files,
// byte-for-byte compatible with Lucene 9.9 format so that Lucene 10.4.0's
// backward-codecs reader can read them.
type Lucene99ScalarQuantizedVectorsWriter struct {
	state *SegmentWriteState

	meta              store.IndexOutput
	quantizedVectorData store.IndexOutput

	// rawVectorDelegate owns the raw FLOAT32 vectors (.vec / .vemf).
	rawVectorDelegate *Lucene99FlatVectorsWriter

	fields []*lucene99ScalarQuantizedFieldWriter

	confidenceInterval *float32
	bits               byte
	compress           bool
	version            int

	finished bool
	closed   bool
}

// NewLucene99ScalarQuantizedVectorsWriter constructs the writer bound to
// state with default bits=7 and compress=false. It creates the .veq / .vemq
// segment files, writes their CodecUtil index headers, and constructs the
// composed flat raw delegate.
func NewLucene99ScalarQuantizedVectorsWriter(state *SegmentWriteState, confidenceInterval *float32) (*Lucene99ScalarQuantizedVectorsWriter, error) {
	return NewLucene99ScalarQuantizedVectorsWriterWithBits(state, confidenceInterval, 7, false)
}

// NewLucene99ScalarQuantizedVectorsWriterWithBits constructs the writer with
// explicit bits and compress parameters. Mirrors the second Java constructor.
func NewLucene99ScalarQuantizedVectorsWriterWithBits(
	state *SegmentWriteState,
	confidenceInterval *float32,
	bits byte,
	compress bool,
) (*Lucene99ScalarQuantizedVectorsWriter, error) {
	if state == nil {
		return nil, errors.New("lucene99 sq: nil SegmentWriteState")
	}
	if state.SegmentInfo == nil {
		return nil, errors.New("lucene99 sq: nil SegmentInfo")
	}
	if state.Directory == nil {
		return nil, errors.New("lucene99 sq: nil Directory")
	}

	metaName := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, lucene99SQMetaExtension)
	dataName := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, lucene99SQDataExtension)

	rawMeta, err := state.Directory.CreateOutput(metaName, store.IOContextWrite)
	if err != nil {
		return nil, fmt.Errorf("lucene99 sq: create meta %q: %w", metaName, err)
	}
	meta := store.NewChecksumIndexOutput(rawMeta)

	w := &Lucene99ScalarQuantizedVectorsWriter{
		state:              state,
		meta:               meta,
		confidenceInterval: confidenceInterval,
		bits:               bits,
		compress:           compress,
		version:            int(lucene99SQVersionAddBits),
	}

	rawData, err := state.Directory.CreateOutput(dataName, store.IOContextWrite)
	if err != nil {
		_ = meta.Close()
		return nil, fmt.Errorf("lucene99 sq: create data %q: %w", dataName, err)
	}
	w.quantizedVectorData = store.NewChecksumIndexOutput(rawData)

	id := state.SegmentInfo.GetID()
	if err := WriteIndexHeader(
		w.meta, lucene99SQMetaCodecName, lucene99SQVersionCurrent, id, state.SegmentSuffix,
	); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("lucene99 sq: write meta header: %w", err)
	}
	if err := WriteIndexHeader(
		w.quantizedVectorData, lucene99SQDataCodecName, lucene99SQVersionCurrent, id, state.SegmentSuffix,
	); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("lucene99 sq: write data header: %w", err)
	}

	// Compose the raw FLOAT32 delegate (writes .vec / .vemf).
	rawDelegate, err := NewLucene99FlatVectorsWriter(state)
	if err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("lucene99 sq: create raw flat delegate: %w", err)
	}
	w.rawVectorDelegate = rawDelegate
	return w, nil
}

// AddField registers a FLOAT32 vector field, allocating a per-field writer
// backed by a composed flat field writer. Non-FLOAT32 fields fall through to
// the raw delegate. Mirrors Java's addField(FieldInfo).
func (w *Lucene99ScalarQuantizedVectorsWriter) AddField(fieldInfo *index.FieldInfo) (KnnFieldVectorsWriter, error) {
	if w.closed {
		return nil, errors.New("lucene99 sq: writer is closed")
	}
	if w.finished {
		return nil, errors.New("lucene99 sq: writer already finished")
	}
	if fieldInfo == nil {
		return nil, errors.New("lucene99 sq: AddField: nil FieldInfo")
	}

	delegate, err := w.rawVectorDelegate.AddField(fieldInfo)
	if err != nil {
		return nil, fmt.Errorf("lucene99 sq: raw delegate AddField: %w", err)
	}

	if fieldInfo.VectorEncoding() != index.VectorEncodingFloat32 {
		// BYTE fields are not scalar quantized; delegate to the flat writer.
		return &flatDelegateFieldWriter{delegate: delegate}, nil
	}

	if w.bits <= 4 && fieldInfo.VectorDimension()%2 != 0 {
		return nil, fmt.Errorf("lucene99 sq: bits=%d is not supported for odd vector dimensions; vector dimension=%d", w.bits, fieldInfo.VectorDimension())
	}

	fw := &lucene99ScalarQuantizedFieldWriter{
		fieldInfo:          fieldInfo,
		confidenceInterval: w.confidenceInterval,
		bits:               w.bits,
		compress:           w.compress,
		normalize:          fieldInfo.VectorSimilarityFunction() == index.VectorSimilarityFunctionCosine,
		delegate:           delegate,
	}
	w.fields = append(w.fields, fw)
	return fw, nil
}

// lucene99ScalarQuantizedFieldWriter accumulates per-document state for one
// FLOAT32 field. Mirrors the Java FieldWriter inner class.
type lucene99ScalarQuantizedFieldWriter struct {
	fieldInfo          *index.FieldInfo
	confidenceInterval *float32
	bits               byte
	compress           bool
	normalize          bool
	delegate           *lucene99FlatFieldWriter
	finished           bool
}

// AddValue records a FLOAT32 vector for docID, delegating storage of the raw
// vector to the flat writer. Mirrors Java's FieldWriter.addValue.
func (fw *lucene99ScalarQuantizedFieldWriter) AddValue(docID int, vectorValue any) error {
	if fw.finished {
		return errors.New("lucene99 sq: field writer already finished")
	}
	vec, ok := vectorValue.([]float32)
	if !ok {
		return fmt.Errorf("lucene99 sq: field %q expects []float32, got %T", fw.fieldInfo.Name(), vectorValue)
	}
	return fw.delegate.addValueFloat32(docID, vec)
}

// RamBytesUsed reports the per-field in-memory footprint. Mirrors Java's
// FieldWriter.ramBytesUsed.
func (fw *lucene99ScalarQuantizedFieldWriter) RamBytesUsed() int64 {
	return fw.delegate.ramBytesUsed()
}

// Finish marks the field complete. Mirrors Java's FieldWriter.finish.
func (fw *lucene99ScalarQuantizedFieldWriter) Finish() error {
	fw.finished = true
	return nil
}

// Flush serialises every accumulated field. It first flushes the raw delegate,
// then, per field, computes the ScalarQuantizer and writes the quantized
// vectors and per-field metadata. Mirrors Java's flush(int maxDoc, Sorter.DocMap
// sortMap); the index-sort path is out of scope.
func (w *Lucene99ScalarQuantizedVectorsWriter) Flush(maxDoc int, sortMap spi.SorterDocMap) error {
	if w.closed {
		return errors.New("lucene99 sq: writer is closed")
	}
	if w.finished {
		return errors.New("lucene99 sq: writer already finished")
	}
	if sortMap != nil {
		return errors.New("lucene99 sq: index-sort (sortMap) not supported yet")
	}

	// The raw delegate writes the un-normalized FLOAT32 vectors first.
	if err := w.rawVectorDelegate.Flush(maxDoc, nil); err != nil {
		return fmt.Errorf("lucene99 sq: raw delegate flush: %w", err)
	}

	for _, field := range w.fields {
		quantizer, err := w.buildScalarQuantizer(field)
		if err != nil {
			return fmt.Errorf("lucene99 sq: build scalar quantizer for field %q: %w", field.fieldInfo.Name(), err)
		}
		if err := w.writeField(field, maxDoc, quantizer); err != nil {
			return err
		}
		field.finished = true
	}
	return nil
}

// writeField writes one field's quantized vectors to .veq and its record to
// .vemq. Mirrors Java's private writeField.
func (w *Lucene99ScalarQuantizedVectorsWriter) writeField(
	field *lucene99ScalarQuantizedFieldWriter, maxDoc int, quantizer *quantization.ScalarQuantizer,
) error {
	// Align the .veq write position to a float boundary.
	vectorDataOffset, err := store.AlignFilePointer(w.quantizedVectorData, floatBytes)
	if err != nil {
		return fmt.Errorf("lucene99 sq: align veq: %w", err)
	}
	if err := w.writeVectors(field, quantizer); err != nil {
		return err
	}
	vectorDataLength := w.quantizedVectorData.GetFilePointer() - vectorDataOffset

	return w.writeMeta(field.fieldInfo, maxDoc, vectorDataOffset, vectorDataLength, quantizer, field.delegate.docIDs)
}

// writeVectors quantizes and writes every stored vector for the field to .veq.
// Mirrors Java's private writeQuantizedVectors.
func (w *Lucene99ScalarQuantizedVectorsWriter) writeVectors(
	field *lucene99ScalarQuantizedFieldWriter, quantizer *quantization.ScalarQuantizer,
) error {
	dim := field.fieldInfo.VectorDimension()
	quantizedScratch := make([]byte, dim)

	var compressedScratch []byte
	if field.compress {
		compressedScratch = make([]byte, dim/2)
	}

	copyBuf := make([]float32, dim)

	for _, v := range field.delegate.floats {
		vec := v
		if field.normalize {
			copy(copyBuf, vec)
			util.L2Normalize(copyBuf)
			vec = copyBuf
		}

		correction := quantizer.Quantize(vec, quantizedScratch, field.fieldInfo.VectorSimilarityFunction())

		if compressedScratch != nil {
			if err := packNibbles(quantizedScratch, compressedScratch); err != nil {
				return fmt.Errorf("lucene99 sq: pack nibbles: %w", err)
			}
			if err := w.quantizedVectorData.WriteBytes(compressedScratch); err != nil {
				return err
			}
		} else {
			if err := w.quantizedVectorData.WriteBytes(quantizedScratch); err != nil {
				return err
			}
		}

		if err := w.quantizedVectorData.WriteInt(int32(math.Float32bits(correction))); err != nil {
			return err
		}
	}
	return nil
}

// writeMeta writes one .vemq field record. Mirrors Java's private writeMeta.
func (w *Lucene99ScalarQuantizedVectorsWriter) writeMeta(
	fieldInfo *index.FieldInfo, maxDoc int,
	vectorDataOffset, vectorDataLength int64,
	quantizer *quantization.ScalarQuantizer,
	docIDs []int,
) error {
	simOrd, err := distFuncToOrd(fieldInfo.VectorSimilarityFunction())
	if err != nil {
		return err
	}
	if err := w.meta.WriteInt(int32(fieldInfo.Number())); err != nil {
		return err
	}
	if err := w.meta.WriteInt(vectorEncodingOrdinal(fieldInfo.VectorEncoding())); err != nil {
		return err
	}
	if err := w.meta.WriteInt(simOrd); err != nil {
		return err
	}
	if err := store.WriteVLong(w.meta, vectorDataOffset); err != nil {
		return err
	}
	if err := store.WriteVLong(w.meta, vectorDataLength); err != nil {
		return err
	}
	if err := store.WriteVInt(w.meta, int32(fieldInfo.VectorDimension())); err != nil {
		return err
	}
	count := len(docIDs)
	if err := store.WriteVInt(w.meta, int32(count)); err != nil {
		return err
	}
	if count > 0 {
		if w.version >= int(lucene99SQVersionAddBits) {
			if w.confidenceInterval == nil {
				if err := w.meta.WriteInt(-1); err != nil {
					return err
				}
			} else {
				if err := w.meta.WriteInt(int32(math.Float32bits(*w.confidenceInterval))); err != nil {
					return err
				}
			}
			if err := w.meta.WriteByte(w.bits); err != nil {
				return err
			}
			if err := w.meta.WriteByte(boolToByte(w.compress)); err != nil {
				return err
			}
		} else {
			ci := w.confidenceInterval
			if ci == nil {
				defaultCI := calculateDefaultConfidenceInterval(fieldInfo.VectorDimension())
				ci = &defaultCI
			}
			if err := w.meta.WriteInt(int32(math.Float32bits(*ci))); err != nil {
				return err
			}
		}
		if err := w.meta.WriteInt(int32(math.Float32bits(quantizer.GetLowerQuantile()))); err != nil {
			return err
		}
		if err := w.meta.WriteInt(int32(math.Float32bits(quantizer.GetUpperQuantile()))); err != nil {
			return err
		}
	}
	return writeFlatOrdToDocStoredMeta(
		lucene99SQDirectMonotonicBlockShift,
		w.meta, w.quantizedVectorData,
		count, maxDoc, docIDs)
}

// WriteField is the single-reader merge entrypoint. The scalar quantizer
// buffers vectors in memory and quantizes on Flush; the merge path is
// deferred, so this returns an explicit error.
func (w *Lucene99ScalarQuantizedVectorsWriter) WriteField(fieldInfo *index.FieldInfo, reader KnnVectorsReader) error {
	_ = fieldInfo
	_ = reader
	return errors.New("lucene99 sq: WriteField (merge path) not supported yet; use AddField/AddValue/Flush")
}

// Finish writes the end-of-fields sentinel (-1) and the CodecUtil footer on
// both .vemq and .veq, after finishing the raw delegate. Mirrors Java's finish().
func (w *Lucene99ScalarQuantizedVectorsWriter) Finish() error {
	if w.closed {
		return errors.New("lucene99 sq: writer is closed")
	}
	if w.finished {
		return errors.New("lucene99 sq: already finished")
	}
	w.finished = true

	if w.rawVectorDelegate != nil {
		if err := w.rawVectorDelegate.Finish(); err != nil {
			return fmt.Errorf("lucene99 sq: raw delegate finish: %w", err)
		}
	}
	if w.meta != nil {
		if err := w.meta.WriteInt(-1); err != nil {
			return fmt.Errorf("lucene99 sq: write meta sentinel: %w", err)
		}
		if err := WriteFooter(w.meta); err != nil {
			return fmt.Errorf("lucene99 sq: write meta footer: %w", err)
		}
	}
	if w.quantizedVectorData != nil {
		if err := WriteFooter(w.quantizedVectorData); err != nil {
			return fmt.Errorf("lucene99 sq: write data footer: %w", err)
		}
	}
	return nil
}

// RamBytesUsed sums the in-memory footprint of every per-field buffer plus the
// raw delegate. Mirrors Java's ramBytesUsed.
func (w *Lucene99ScalarQuantizedVectorsWriter) RamBytesUsed() int64 {
	var total int64
	if w.rawVectorDelegate != nil {
		total += w.rawVectorDelegate.RamBytesUsed()
	}
	for _, f := range w.fields {
		total += f.RamBytesUsed()
	}
	return total
}

// Close releases the .veq / .vemq outputs and the raw delegate. Idempotent.
func (w *Lucene99ScalarQuantizedVectorsWriter) Close() error {
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
	if w.quantizedVectorData != nil {
		if err := w.quantizedVectorData.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		w.quantizedVectorData = nil
	}
	if w.rawVectorDelegate != nil {
		if err := w.rawVectorDelegate.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		w.rawVectorDelegate = nil
	}
	return firstErr
}

// buildScalarQuantizer constructs a ScalarQuantizer from the accumulated raw
// vectors for the given field. Mirrors Java's static buildScalarQuantizer.
func (w *Lucene99ScalarQuantizedVectorsWriter) buildScalarQuantizer(
	field *lucene99ScalarQuantizedFieldWriter,
) (*quantization.ScalarQuantizer, error) {
	vectors := field.delegate.floats
	if len(vectors) == 0 {
		return quantization.NewScalarQuantizer(0, 0, field.bits)
	}

	fvv := quantization.FloatVectorValues(&floatVectorList{vectors: vectors, dim: field.fieldInfo.VectorDimension()})
	vecCount := len(vectors)
	simFunc := field.fieldInfo.VectorSimilarityFunction()

	if simFunc == index.VectorSimilarityFunctionCosine {
		fvv = &normalizedFloatVectorValues{values: fvv, copy: make([]float32, field.fieldInfo.VectorDimension())}
		simFunc = index.VectorSimilarityFunctionDotProduct
	}

	ci := w.confidenceInterval
	if ci != nil && *ci == 0 {
		// DYNAMIC_CONFIDENCE_INTERVAL
		return quantization.FromVectorsAutoInterval(fvv, simFunc, vecCount, field.bits)
	}

	if ci == nil {
		defaultCI := calculateDefaultConfidenceInterval(field.fieldInfo.VectorDimension())
		ci = &defaultCI
	}
	return quantization.FromVectors(fvv, *ci, vecCount, field.bits)
}

// calculateDefaultConfidenceInterval mirrors Java's
// Lucene99ScalarQuantizedVectorsFormat.calculateDefaultConfidenceInterval.
func calculateDefaultConfidenceInterval(vectorDimension int) float32 {
	ci := 1.0 - (1.0 / float64(vectorDimension+1))
	if ci < lucene99SQMinimumConfidenceInterval {
		ci = lucene99SQMinimumConfidenceInterval
	}
	return float32(ci)
}

// floatVectorList is a simple quantization.FloatVectorValues backed by an
// in-memory slice of float vectors. Mirrors Java's FloatVectorWrapper.
type floatVectorList struct {
	vectors [][]float32
	dim     int
}

func (f *floatVectorList) Dimension() int                          { return f.dim }
func (f *floatVectorList) VectorValue(ord int) ([]float32, error) { return f.vectors[ord], nil }
func (f *floatVectorList) Iterator() quantization.DocIndexIterator {
	return &floatVectorListIterator{vectors: f.vectors, docID: -1}
}

type floatVectorListIterator struct {
	vectors [][]float32
	docID   int
}

func (it *floatVectorListIterator) NextDoc() (int, error) {
	it.docID++
	if it.docID >= len(it.vectors) {
		it.docID = util.NO_MORE_DOCS
		return util.NO_MORE_DOCS, nil
	}
	return it.docID, nil
}

func (it *floatVectorListIterator) Index() int {
	if it.docID == util.NO_MORE_DOCS || it.docID < 0 {
		return -1
	}
	return it.docID
}

// normalizedFloatVectorValues wraps a FloatVectorValues and L2-normalizes each
// vector on the fly. Mirrors Java's NormalizedFloatVectorValues.
type normalizedFloatVectorValues struct {
	values quantization.FloatVectorValues
	copy   []float32
}

func (n *normalizedFloatVectorValues) Dimension() int                          { return n.values.Dimension() }
func (n *normalizedFloatVectorValues) VectorValue(ord int) ([]float32, error) {
	vec, err := n.values.VectorValue(ord)
	if err != nil {
		return nil, err
	}
	copy(n.copy, vec)
	util.L2Normalize(n.copy)
	return n.copy, nil
}
func (n *normalizedFloatVectorValues) Iterator() quantization.DocIndexIterator { return n.values.Iterator() }

// boolToByte returns 1 if b is true, 0 otherwise.
func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}
