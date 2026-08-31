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
//   with the License.  You may obtain a copy of the License at
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
func (w *mutablePointTreeWrapper) GetByteAt(i, k int) byte         { return w.// a la Java Lucene90PointsWriter.java
	// This is a port of org.apache.lucene.codecs.lucene90.Lucene90PointsWriter.
}

// pointsWriter writes points in Lucene 9.0 format.
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
// reader (which must implement PointsSource).
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

// Merge merges multiple points readers.
func (w *pointsWriter) Merge(mergeState *codecs.MergeState) error {
	for _, reader := range mergeState.PointsReaders {
		if reader == nil {
			continue
		}
		if _, ok := reader.(*pointsReader); !ok {
			return codecs.DefaultPointsMerge(mergeState, w)
		}
	}

	for _, reader := range mergeState.PointsReaders {
		if reader != nil {
			if err := mergeState.CheckAborted(); err != nil {
				return err
			}
			if err := reader.CheckIntegrity(); err != nil {
				return err
			}
		}
	}

	for _, fieldInfo := range mergeState.MergeFieldInfos {
		if fieldInfo.PointDimensionCount() != 0 {
			if fieldInfo.PointDimensionCount() == 1 {
				if err := w.merge1D(mergeState, fieldInfo); err != nil {
					return err
				}
			} else {
				if err := w.mergeOneField(mergeState, fieldInfo); err != nil {
					return err
				}
			}
		}
	}

	return w.Finish()
}

func (w *pointsWriter) merge1D(mergeState *codecs.MergeState, fieldInfo *schema.FieldInfo) error {
	var totMaxSize int64
	for i, reader := range mergeState.PointsReaders {
		if reader == nil {
			continue
		}
		readerFieldInfos := mergeState.FieldInfos[i]
		readerFieldInfo := readerFieldInfos.FieldInfo(fieldInfo.Name())
		if readerFieldInfo != nil && readerFieldInfo.PointDimensionCount() > 0 {
			values, err := reader.GetValues(fieldInfo.Name())
			if err != nil {
				return err
			}
			if values != nil {
				totMaxSize += values.GetValueCount()
			}
		}
	}

	config, err := bkd.Of(
		fieldInfo.PointDimensionCount(),
		fieldInfo.PointIndexDimensionCount(),
		fieldInfo.PointNumBytes(),
		w.maxPointsInLeafNode,
	)
	if err != nil {
		return err
	}

	bkdVersion, err := codecs.Lucene90PointsBKDVersion(w.version)
	if err != nil {
		return err
	}

	writer, err := bkd.NewBKDWriterWithVersion(
		w.state.SegmentInfo.DocCount(),
		w.state.Directory,
		w.state.SegmentInfo.Name(),
		config,
		w.maxMBSortInHeap,
		totMaxSize,
		int(bkdVersion),
	)
	if err != nil {
		return err
	}
	defer func() { _ = writer.Close() }()

	var pointValues []index.PointValues
	var docMaps []*codecs.DocMap
	for i, reader := range mergeState.PointsReaders {
		if reader == nil {
			continue
		}
		reader90 := reader.(*pointsReader)
		readerFieldInfos := mergeState.FieldInfos[i]
		readerFieldInfo := readerFieldInfos.FieldInfo(fieldInfo.Name())
		if readerFieldInfo != nil && readerFieldInfo.PointDimensionCount() > 0 {
			aPointValues, err := reader90.GetValues(readerFieldInfo.Name())
			if err != nil {
				return err
			}
			if aPointValues != nil {
				pointValues = append(pointValues, aPointValues)
				docMaps = append(docMaps, mergeState.DocMaps[i])
			}
		}
	}

	finalizer, err := writer.Merge(w.metaOut, w.indexOut, w.dataOut, docMaps, pointValues)
	if err != nil {
		return err
	}
	if finalizer != nil {
		if err := w.metaOut.WriteInt(int32(fieldInfo.Number())); err != nil {
			return err
		}
		if err := finalizer(); err != nil {
			return err
		}
	}
	return nil
}

func (w *pointsWriter) mergeOneField(mergeState *codecs.MergeState, fieldInfo *schema.FieldInfo) error {
	var maxPointCount int64
	for i, reader := range mergeState.PointsReaders {
		if reader == nil {
			continue
		}
		readerFieldInfo := mergeState.FieldInfos[i].FieldInfo(fieldInfo.Name())
		if readerFieldInfo != nil && readerFieldInfo.PointDimensionCount() > 0 {
			values, err := reader.GetValues(fieldInfo.Name())
			if err != nil {
				return err
			}
			if values != nil {
				maxPointCount += values.GetValueCount()
			}
		}
	}

	mergedReader := &mergedPointsReader{
		mergeState: mergeState,
		fieldInfo:  fieldInfo,
		totalCount: maxPointCount,
	}

	return w.WriteField(fieldInfo, mergedReader)
}

type mergedPointsReader struct {
	mergeState *codecs.MergeState
	fieldInfo  *schema.FieldInfo
	totalCount int64
}

func (r *mergedPointsReader) Close() error { return nil }
func (r *mergedPointsReader) CheckIntegrity() error { return nil }
func (r *mergedPointsReader) GetValues(fieldName string) (index.PointValues, error) {
	if fieldName != r.fieldInfo.Name() {
		return nil, fmt.Errorf("field name must match the field being merged")
	}
	return &mergedPointValues{
		reader: r,
	}, nil
}

type mergedPointValues struct {
	reader *mergedPointsReader
}

func (pv *mergedPointValues) GetPointTree() (bkd.PointTree, error) {
	return &mergedPointTree{
		reader: pv.reader,
	}, nil
}

func (pv *mergedPointValues) GetMinPackedValue() ([]byte, error) { return nil, errors.New("not implemented") }
func (pv *mergedPointValues) GetMaxPackedValue() ([]byte, error) { return nil, errors.New("not implemented") }
func (pv *mergedPointValues) GetNumDimensions() int { return pv.reader.fieldInfo.PointDimensionCount() }
func (pv *mergedPointValues) GetBytesPerDimension() int { return pv.reader.fieldInfo.PointNumBytes() }
func (pv *mergedPointValues) GetDocCount() int { return 0 }
func (pv *mergedPointValues) GetDocCountWithValue() int64 { return 0 }
func (pv *mergedPointValues) GetValueCount() int64 { return pv.reader.totalCount }

type mergedPointTree struct {
	reader *mergedPointsReader
}

func (pt *mergedPointTree) Clone() bkd.PointTree { return nil }
func (pt *mergedPointTree) MoveToChild() (bool, error) { return false, nil }
func (pt *mergedPointTree) MoveToSibling() (bool, error) { return false, nil }
func (pt *mergedPointTree) MoveToParent() (bool, error) { return false, nil }
func (pt *mergedPointTree) GetMinPackedValue() []byte { return nil }
func (pt *mergedPointTree) GetMaxPackedValue() []byte { return nil }
func (pt *mergedPointTree) Size() int64 { return pt.reader.totalCount }
func (pt *mergedPointTree) VisitDocIDs(visitor bkd.IntersectVisitor) error { return nil }
func (pt *mergedPointTree) VisitDocValues(visitor bkd.IntersectVisitor) error {
	for i, reader := range pt.reader.mergeState.PointsReaders {
		if reader == nil {
			continue
		}
		readerFieldInfo := pt.reader.mergeState.FieldInfos[i].FieldInfo(pt.reader.fieldInfo.Name())
		if readerFieldInfo == nil || readerFieldInfo.PointDimensionCount() == 0 {
			continue
		}
		values, err := reader.GetValues(pt.reader.fieldInfo.Name())
		if err != nil {
			return err
		}
		if values == nil {
			continue
		}
		docMap := pt.reader.mergeState.DocMaps[i]
		tree, err := values.GetPointTree()
		if err != nil {
			return err
		}
		err = tree.VisitDocValues(func(docID int, packedValue []byte) error {
			newDocID := docMap.Get(docID)
			if newDocID != -1 {
				return visitor.VisitByPackedValue(newDocID, packedValue)
			}
			return nil
		})
		if err != nil {
			return err
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
