// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//   Licensed to the Apache Software Foundation (ASF) under one or more
//   contributor license agreements. See the NOTICE file distributed with
//   this work for additional information regarding copyright ownership.
//   The ASF licenses this file to You under the Apache License, Version
//   2.0 (the "License"); you may not use this file except in compliance
//   with the License. You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//   implied. See the License for the specific language governing
//   permissions and limitations under the License.

package lucene90

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// This file holds the BKD-backed Lucene 9.0 points writer/reader and their
// PointValues adapter. It lives in the codecs/lucene90 sub-package — not the
// top-level codecs package — because util/bkd imports codecs for its
// CodecUtil/Relation primitives; codecs therefore cannot import util/bkd
// without an import cycle. This package sits above both and installs the
// implementations into the top-level Lucene90PointsFormat facade via
// codecs.SetLucene90PointsImpl in init.
//
// On-disk framing is byte-faithful to
// org.apache.lucene.codecs.lucene90.Lucene90PointsWriter /
// Lucene90PointsReader (Lucene 10.4.0).

func init() {
	codecs.SetLucene90PointsImpl(newPointsWriter, newPointsReader)
}

// -----------------------------------------------------------------------------
// Write-side point source contract.
// -----------------------------------------------------------------------------

// PointsSource is the surface pointsWriter pulls point values from when
// WriteField is invoked. The indexing chain hands the writer a
// codecs.PointsReader that also implements this interface (the in-memory
// buffer flushed by DocumentsWriterPerThread).
//
// It mirrors the part of org.apache.lucene.index.PointValues that
// Lucene90PointsWriter.writeField consumes: the per-field point count (used to
// size the BKDWriter) and a visitor-style walk over every indexed (docID,
// packedValue) pair (PointValues.visitDocValues with a CELL_CROSSES_QUERY
// visitor).
type PointsSource interface {
	// PointValueCount returns the number of indexed point values for field
	// (PointValues.size()).
	PointValueCount(field string) int64

	// VisitPoints invokes fn for every indexed (docID, packedValue) pair for
	// field, in the order they were buffered. The packedValue slice is owned
	// by the source for the duration of the call; implementations that retain
	// it must copy.
	VisitPoints(field string, fn func(docID int, packedValue []byte) error) error
}

// pointTreeSource is an optional extension of PointsSource that exposes the
// in-memory MutablePointTree directly. When available, pointsWriter uses
// BKDWriter.WriteField (the heap path) instead of Add/Finish (the offline
// spill path), matching the code path Lucene 10.4.0 takes for buffered points
// and producing byte-identical output for small in-memory trees.
type pointTreeSource interface {
	PointsSource
	// MutablePointTree returns the in-memory tree and its point count.
	index.MutablePointTreeSource
}

// mutablePointTreeWrapper adapts an index.PointTreeBuffer to the
// bkd.MutablePointTree interface expected by BKDWriter.WriteField. The two
// interfaces have identical method sets; the wrapper avoids importing
// util/bkd into the index package (which would create an import cycle).
type mutablePointTreeWrapper struct {
	tree index.PointTreeBuffer
}

func (w *mutablePointTreeWrapper) Swap(i, j int)                   { w.tree.Swap(i, j) }
func (w *mutablePointTreeWrapper) GetValue(i int, dst *util.BytesRef) { w.tree.GetValue(i, dst) }
func (w *mutablePointTreeWrapper) GetByteAt(i, k int) byte         { return w.tree.GetByteAt(i, k) }
func (w *mutablePointTreeWrapper) GetDocID(i int) int              { return w.tree.GetDocID(i) }
func (w *mutablePointTreeWrapper) Save(i, j int)                    { w.tree.Save(i, j) }
func (w *mutablePointTreeWrapper) Restore(i, j int)                 { w.tree.Restore(i, j) }

// -----------------------------------------------------------------------------


// pointsWriter — byte-faithful BKD writer.
// -----------------------------------------------------------------------------

// pointsWriter writes points in Lucene 9.0 format. It is the Go port of
// org.apache.lucene.codecs.lucene90.Lucene90PointsWriter (Lucene 10.4.0).
type pointsWriter struct {
	state               *codecs.SegmentWriteState
	version             int32
	maxPointsInLeafNode int
	maxMBSortInHeap     float64

	metaOut  store.IndexOutput
	indexOut store.IndexOutput
	dataOut  store.IndexOutput

	finished bool
	closed   bool
}

// newPointsWriter opens and header-stamps the three output files (.kdd, .kdm,
// .kdi) and returns the writer. Installed as the codecs Lucene90 points writer
// hook.
func newPointsWriter(state *codecs.SegmentWriteState, version int32) (codecs.PointsWriter, error) {
	if _, err := codecs.Lucene90PointsBKDVersion(version); err != nil {
		return nil, err
	}
	w := &pointsWriter{
		state:               state,
		version:             version,
		maxPointsInLeafNode: bkd.DefaultMaxPointsInLeafNode,
		maxMBSortInHeap:     bkd.DefaultMaxMBSortInHeap,
	}

	files := codecs.Lucene90PointFileList(state.SegmentInfo.Name(), state.SegmentSuffix)
	open := func(entry codecs.Lucene90PointFileEntry) (store.IndexOutput, error) {
		raw, err := state.Directory.CreateOutput(entry.Name, store.IOContext{Context: store.ContextWrite})
		if err != nil {
			return nil, fmt.Errorf("lucene90 points: create %q: %w", entry.Name, err)
		}
		out := store.NewChecksumIndexOutput(raw)
		if err := codecs.WriteIndexHeader(out, entry.Codec, codecs.Lucene90PointsVersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
			_ = out.Close()
			return nil, fmt.Errorf("lucene90 points: header %q: %w", entry.Name, err)
		}
		return out, nil
	}

	var err error
	if w.dataOut, err = open(files[0]); err != nil {
		return nil, err
	}
	if w.metaOut, err = open(files[2]); err != nil {
		_ = w.dataOut.Close()
		return nil, err
	}
	if w.indexOut, err = open(files[1]); err != nil {
		_ = w.dataOut.Close()
		_ = w.metaOut.Close()
		return nil, err
	}
	return w, nil
}

// WriteField writes the BKD tree for fieldInfo, pulling its point values from
// reader (which must implement PointsSource). Mirrors
// Lucene90PointsWriter.writeField: build a BKDWriter sized to the field's
// point count, replay every (docID, packedValue) pair through BKDWriter.Add,
// then finish the tree and — when at least one point was written — prefix the
// field number on metaOut and run the index finalizer.
func (w *pointsWriter) WriteField(fieldInfo *schema.FieldInfo, reader codecs.PointsReader) error {
	if w.closed {
		return errors.New("lucene90 points: writer closed")
	}
	if w.finished {
		return errors.New("lucene90 points: writer already finished")
	}
	src, ok := reader.(PointsSource)
	if !ok {
		return fmt.Errorf("lucene90 points: reader %T does not implement PointsSource", reader)
	}

	config, err := bkd.Of(
		fieldInfo.PointDimensionCount(),
		fieldInfo.PointIndexDimensionCount(),
		fieldInfo.PointNumBytes(),
		w.maxPointsInLeafNode,
	)
	if err != nil {
		return fmt.Errorf("lucene90 points: field %q config: %w", fieldInfo.Name(), err)
	}

	bkdVersion, err := codecs.Lucene90PointsBKDVersion(w.version)
	if err != nil {
		return err
	}

	totalPointCount := src.PointValueCount(fieldInfo.Name())
	writer, err := bkd.NewBKDWriterWithVersion(
		w.state.SegmentInfo.DocCount(),
		w.state.Directory,
		w.state.SegmentInfo.Name(),
		config,
		w.maxMBSortInHeap,
		totalPointCount,
		int(bkdVersion),
	)
	if err != nil {
		return fmt.Errorf("lucene90 points: field %q new bkd writer: %w", fieldInfo.Name(), err)
	}
	defer func() { _ = writer.Close() }()

	fieldName := fieldInfo.Name()
	if pts, ok := reader.(pointTreeSource); ok {
		tree, size := pts.MutablePointTree()
		if tree != nil {
			finalizer, err := writer.WriteField(w.metaOut, w.indexOut, w.dataOut, fieldName, &mutablePointTreeWrapper{tree: tree}, size)
			if err != nil {
				return fmt.Errorf("lucene90 points: field %q writeField: %w", fieldName, err)
			}
			if finalizer != nil {
				if err := w.metaOut.WriteInt(int32(fieldInfo.Number())); err != nil {
					return fmt.Errorf("lucene90 points: field %q write field number: %w", fieldName, err)
				}
				if err := finalizer(); err != nil {
					return fmt.Errorf("lucene90 points: field %q finalizer: %w", fieldName, err)
				}
			}
			return nil
		}
	}

	if err := src.VisitPoints(fieldName, func(docID int, packedValue []byte) error {
		return writer.Add(packedValue, docID)
	}); err != nil {
		return fmt.Errorf("lucene90 points: field %q add: %w", fieldName, err)
	}

	finalizer, err := writer.Finish(w.metaOut, w.indexOut, w.dataOut)
	if err != nil {
		return fmt.Errorf("lucene90 points: field %q finish: %w", fieldName, err)
	}
	if finalizer != nil {
		if err := w.metaOut.WriteInt(int32(fieldInfo.Number())); err != nil {
			return fmt.Errorf("lucene90 points: field %q write field number: %w", fieldName, err)
		}
		if err := finalizer(); err != nil {
			return fmt.Errorf("lucene90 points: field %q finalizer: %w", fieldName, err)
		}
	}
	return nil
}

// Finish stamps the meta sentinel (-1), the index/data footers, the index and
// data lengths, and the meta footer. Mirrors Lucene90PointsWriter.finish.
func (w *pointsWriter) Finish() error {
	if w.closed {
		return errors.New("lucene90 points: writer closed")
	}
	if w.finished {
		return errors.New("lucene90 points: already finished")
	}
	w.finished = true

	if err := w.metaOut.WriteInt(-1); err != nil {
		return err
	}
	if err := codecs.WriteFooter(w.indexOut); err != nil {
		return err
	}
	if err := codecs.WriteFooter(w.dataOut); err != nil {
		return err
	}
	if err := w.metaOut.WriteLong(w.indexOut.GetFilePointer()); err != nil {
		return err
	}
	if err := w.metaOut.WriteLong(w.dataOut.GetFilePointer()); err != nil {
		return err
	}
	return codecs.WriteFooter(w.metaOut)
}

// Close releases the three outputs. Idempotent.
func (w *pointsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	var lastErr error
	for _, out := range []store.IndexOutput{w.metaOut, w.indexOut, w.dataOut} {
		if out == nil {
			continue
		}
		if err := out.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// -----------------------------------------------------------------------------
// pointsReader — byte-faithful BKD reader.
// -----------------------------------------------------------------------------

// pointsReader reads points in Lucene 9.0 format. It is the Go port of
// org.apache.lucene.codecs.lucene90.Lucene90PointsReader (Lucene 10.4.0).
type pointsReader struct {
	state   *codecs.SegmentReadState
	indexIn store.IndexInput
	dataIn  store.IndexInput
	readers map[int]*bkd.BKDReader
	closed  bool
}

// newPointsReader opens the points trio for the segment and builds a BKDReader
// per indexed field. Installed as the codecs Lucene90 points reader hook.
func newPointsReader(state *codecs.SegmentReadState) (codecs.PointsReader, error) {
	files := codecs.Lucene90PointFileList(state.SegmentInfo.Name(), state.SegmentSuffix)
	dataEntry, indexEntry, metaEntry := files[0], files[1], files[2]

	r := &pointsReader{state: state, readers: make(map[int]*bkd.BKDReader)}

	success := false
	defer func() {
		if !success {
			_ = r.Close()
		}
	}()

	var err error
	r.indexIn, err = state.Directory.OpenInput(indexEntry.Name, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene90 points: open %q: %w", indexEntry.Name, err)
	}
	if _, err = codecs.CheckIndexHeader(r.indexIn, indexEntry.Codec, codecs.Lucene90PointsVersionStart, codecs.Lucene90PointsVersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, fmt.Errorf("lucene90 points: header %q: %w", indexEntry.Name, err)
	}

	r.dataIn, err = state.Directory.OpenInput(dataEntry.Name, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene90 points: open %q: %w", dataEntry.Name, err)
	}
	if _, err = codecs.CheckIndexHeader(r.dataIn, dataEntry.Codec, codecs.Lucene90PointsVersionStart, codecs.Lucene90PointsVersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, fmt.Errorf("lucene90 points: header %q: %w", dataEntry.Name, err)
	}

	metaRaw, err := state.Directory.OpenInput(metaEntry.Name, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene90 points: open %q: %w", metaEntry.Name, err)
	}
	metaIn := store.NewChecksumIndexInput(metaRaw)
	if err := r.loadMeta(metaIn, metaEntry.Codec); err != nil {
		_ = metaRaw.Close()
		return nil, err
	}
	if err := metaRaw.Close(); err != nil {
		return nil, err
	}

	success = true
	return r, nil
}

// loadMeta validates the meta header, replays every per-field BKD metadata
// record into a BKDReader, reads the trailing index/data lengths, and checks
// the meta footer. Mirrors the metaIn loop in Lucene90PointsReader's
// constructor.
func (r *pointsReader) loadMeta(metaIn *store.ChecksumIndexInput, codec string) error {
	if _, err := codecs.CheckIndexHeader(metaIn, codec, codecs.Lucene90PointsVersionStart, codecs.Lucene90PointsVersionCurrent, r.state.SegmentInfo.GetID(), r.state.SegmentSuffix); err != nil {
		return fmt.Errorf("lucene90 points: meta header: %w", err)
	}
	for {
		fieldNumber, err := metaIn.ReadInt()
		if err != nil {
			return fmt.Errorf("lucene90 points: read field number: %w", err)
		}
		if fieldNumber == -1 {
			break
		}
		if fieldNumber < 0 {
			return fmt.Errorf("lucene90 points: illegal field number: %d", fieldNumber)
		}
		bkdReader, err := bkd.NewBKDReader(metaIn, r.indexIn, r.dataIn)
		if err != nil {
			return fmt.Errorf("lucene90 points: build bkd reader for field %d: %w", fieldNumber, err)
		}
		r.readers[int(fieldNumber)] = bkdReader
	}
	if _, err := metaIn.ReadLong(); err != nil { // indexLength
		return fmt.Errorf("lucene90 points: read index length: %w", err)
	}
	if _, err := metaIn.ReadLong(); err != nil { // dataLength
		return fmt.Errorf("lucene90 points: read data length: %w", err)
	}
	if _, err := codecs.CheckFooter(metaIn); err != nil {
		return fmt.Errorf("lucene90 points: meta footer: %w", err)
	}
	return nil
}

// GetValues returns the BKDReader-backed PointValues for fieldName, or nil
// when the field carries no indexed points. Mirrors
// Lucene90PointsReader.getValues.
func (r *pointsReader) GetValues(fieldName string) (index.PointValues, error) {
	if r.closed {
		return nil, errors.New("lucene90 points: reader closed")
	}
	fieldInfo := r.state.FieldInfos.GetByName(fieldName)
	if fieldInfo == nil {
		return nil, fmt.Errorf("lucene90 points: field %q is unrecognized", fieldName)
	}
	if fieldInfo.PointDimensionCount() == 0 {
		return nil, fmt.Errorf("lucene90 points: field %q did not index point values", fieldName)
	}
	bkdReader, ok := r.readers[fieldInfo.Number()]
	if !ok {
		return nil, nil
	}
	return newPointValues(bkdReader), nil
}

// CheckIntegrity verifies the index and data file checksums.
func (r *pointsReader) CheckIntegrity() error {
	if r.closed {
		return errors.New("lucene90 points: reader closed")
	}
	if _, err := codecs.ChecksumEntireFile(r.indexIn); err != nil {
		return err
	}
	if _, err := codecs.ChecksumEntireFile(r.dataIn); err != nil {
		return err
	}
	return nil
}

// Close releases the index and data inputs. Idempotent.
func (r *pointsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	var lastErr error
	if r.indexIn != nil {
		if err := r.indexIn.Close(); err != nil {
			lastErr = err
		}
	}
	if r.dataIn != nil {
		if err := r.dataIn.Close(); err != nil {
			lastErr = err
		}
	}
	r.readers = nil
	return lastErr
}

// pointsReaderGetValues is the wide read surface a Lucene90 points reader
// exposes beyond the narrow codecs.PointsReader (CheckIntegrity/Close). It
// mirrors org.apache.lucene.codecs.PointsReader.getValues. The index-side
// SegmentReader.GetPointValues type-asserts the codec reader to this
// interface.
type pointsReaderGetValues interface {
	GetValues(field string) (index.PointValues, error)
}

var (
	_ codecs.PointsReader   = (*pointsReader)(nil)
	_ pointsReaderGetValues = (*pointsReader)(nil)
)

// -----------------------------------------------------------------------------
// pointValues — index.PointValues view over a BKDReader.
// -----------------------------------------------------------------------------

// pointValues is the index.PointValues view of a single field's BKD tree,
// returned by pointsReader.GetValues. It projects the underlying BKDReader
// onto both the narrow canonical index.PointValues surface and the wider
// Intersect / EstimatePointCount surface the search-side point queries consume
// via the index.PointTreeIntersectVisitor contract.
//
// This is the Go counterpart of the anonymous PointValues returned by
// org.apache.lucene.codecs.lucene90.Lucene90PointsReader.getValues (the
// BKDReader exposed as a PointValues).
type pointValues struct {
	reader *bkd.BKDReader
}

func newPointValues(reader *bkd.BKDReader) *pointValues {
	return &pointValues{reader: reader}
}

// Intersect walks the BKD tree, driving visitor for every matching cell and
// point. It bridges the index.PointTreeIntersectVisitor (Compare returns an
// int) to the util/bkd.IntersectVisitor (Compare returns a codecs.Relation).
func (pv *pointValues) Intersect(visitor index.PointTreeIntersectVisitor) error {
	return pv.reader.Intersect(&bkdVisitorBridge{v: visitor})
}

// EstimatePointCount returns the BKDReader's estimate of how many points the
// visitor will match; a failing estimate is reported as 0.
func (pv *pointValues) EstimatePointCount(visitor index.PointTreeIntersectVisitor) int64 {
	count, err := pv.reader.EstimatePointCount(&bkdVisitorBridge{v: visitor})
	if err != nil || count < 0 {
		return 0
	}
	return count
}

// GetMinPackedValue returns the per-dimension minimum packed value across the
// tree. The error return matches index.PointValues; the BKDReader accessor
// never fails, so the error is always nil.
func (pv *pointValues) GetMinPackedValue() ([]byte, error) {
	return pv.reader.GetMinPackedValue(), nil
}

// GetMaxPackedValue returns the per-dimension maximum packed value.
func (pv *pointValues) GetMaxPackedValue() ([]byte, error) {
	return pv.reader.GetMaxPackedValue(), nil
}

// GetNumDimensions returns the number of indexed point dimensions.
func (pv *pointValues) GetNumDimensions() int { return pv.reader.GetNumDimensions() }

// GetBytesPerDimension returns the number of bytes per dimension.
func (pv *pointValues) GetBytesPerDimension() int { return pv.reader.GetBytesPerDimension() }

// GetDocCount returns the number of documents with at least one point value.
func (pv *pointValues) GetDocCount() int { return pv.reader.GetDocCount() }

// GetDocCountWithValue returns the document count (BKD tracks doc count, not
// per-document value multiplicity).
func (pv *pointValues) GetDocCountWithValue() int64 { return int64(pv.reader.GetDocCount()) }

// GetValueCount returns the total number of indexed point values
// (PointValues.size()).
func (pv *pointValues) GetValueCount() int64 { return pv.reader.Size() }

// GetPointTree returns a fresh BKD PointTree cursor positioned at the
// root of the field's tree. It exposes the cursor-shaped subset of
// org.apache.lucene.index.PointValues.PointTree (clone / moveToChild /
// moveToSibling / packed-value accessors / visitDocValues) that the
// nearest-neighbour KNN search walks, beyond the metadata-only and
// Intersect surfaces.
//
// The return type is bkd.PointTree; search-side consumers that drive the
// nearest-neighbour algorithm type-assert the index.PointValues to an
// interface exposing this method (mirroring the way PointRangeQuery
// type-asserts the Intersect surface), so they obtain the cursor without
// the codec leaking its private *bkd.BKDReader.
func (pv *pointValues) GetPointTree() (bkd.PointTree, error) {
	return pv.reader.GetPointTree()
}

var _ index.PointValues = (*pointValues)(nil)

// bkdVisitorBridge adapts an index.PointTreeIntersectVisitor (Compare returns
// an int in {0,1,2}) to a util/bkd.IntersectVisitor (Compare returns a
// codecs.Relation). The int convention matches the Relation enum order, so the
// conversion is a direct cast.
type bkdVisitorBridge struct {
	v index.PointTreeIntersectVisitor
}

func (b *bkdVisitorBridge) Visit(docID int) error { return b.v.Visit(docID) }

func (b *bkdVisitorBridge) VisitByPackedValue(docID int, packedValue []byte) error {
	return b.v.VisitByPackedValue(docID, packedValue)
}

func (b *bkdVisitorBridge) Compare(minPackedValue, maxPackedValue []byte) codecs.Relation {
	return codecs.Relation(b.v.Compare(minPackedValue, maxPackedValue))
}

func (b *bkdVisitorBridge) Grow(count int) { b.v.Grow(count) }

var _ bkd.IntersectVisitor = (*bkdVisitorBridge)(nil)
