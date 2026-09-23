// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.5.0:
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
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// pointsWriter writes dimensional values. It is the Go port of
// org.apache.lucene.codecs.lucene90.Lucene90PointsWriter (Apache Lucene
// 10.5.0).
type pointsWriter struct {
	*codecs.BasePointsWriter

	// Outputs used to write the BKD tree data files.
	metaOut, indexOut, dataOut store.IndexOutput

	writeState          *codecs.SegmentWriteState
	maxPointsInLeafNode int
	maxMBSortInHeap     float64
	version             int32

	finished bool
}

// newPointsWriter renders the package-private constructor
// Lucene90PointsWriter(SegmentWriteState, int version) ("used for testing with
// older versions") as reached from Lucene90PointsFormat.fieldsWriter, with the
// defaults BKDConfig.DEFAULT_MAX_POINTS_IN_LEAF_NODE and
// BKDWriter.DEFAULT_MAX_MB_SORT_IN_HEAP. Installed as the codecs Lucene90
// points writer hook.
func newPointsWriter(writeState *codecs.SegmentWriteState, version int32) (codecs.PointsWriter, error) {
	return NewLucene90PointsWriterWithSortParamsAndVersion(
		writeState, bkd.DefaultMaxPointsInLeafNode, bkd.DefaultMaxMBSortInHeap, version)
}

// NewLucene90PointsWriter renders the public constructor
// Lucene90PointsWriter(SegmentWriteState): it uses the default values for
// maxPointsInLeafNode (512) and maxMBSortInHeap (16.0).
func NewLucene90PointsWriter(writeState *codecs.SegmentWriteState) (codecs.PointsWriter, error) {
	return NewLucene90PointsWriterWithSortParamsAndVersion(
		writeState, bkd.DefaultMaxPointsInLeafNode, bkd.DefaultMaxMBSortInHeap, codecs.Lucene90PointsVersionCurrent)
}

// NewLucene90PointsWriterWithSortParams renders the public constructor
// Lucene90PointsWriter(SegmentWriteState, int maxPointsInLeafNode, double
// maxMBSortInHeap), which writes Lucene90PointsFormat.VERSION_CURRENT.
func NewLucene90PointsWriterWithSortParams(writeState *codecs.SegmentWriteState, maxPointsInLeafNode int,
	maxMBSortInHeap float64) (codecs.PointsWriter, error) {
	return NewLucene90PointsWriterWithSortParamsAndVersion(
		writeState, maxPointsInLeafNode, maxMBSortInHeap, codecs.Lucene90PointsVersionCurrent)
}

// NewLucene90PointsWriterWithSortParamsAndVersion renders the public
// constructor Lucene90PointsWriter(SegmentWriteState, int maxPointsInLeafNode,
// double maxMBSortInHeap, int version).
func NewLucene90PointsWriterWithSortParamsAndVersion(writeState *codecs.SegmentWriteState,
	maxPointsInLeafNode int, maxMBSortInHeap float64, version int32) (codecs.PointsWriter, error) {
	if util.AssertsEnabled() && !writeState.FieldInfos.HasPointValues() {
		panic(util.NewAssertionError(nil))
	}
	w := &pointsWriter{
		writeState:          writeState,
		maxPointsInLeafNode: maxPointsInLeafNode,
		maxMBSortInHeap:     maxMBSortInHeap,
		version:             version,
	}
	w.BasePointsWriter = codecs.NewBasePointsWriter(w)

	files := codecs.Lucene90PointFileList(writeState.SegmentInfo.Name(), writeState.SegmentSuffix)
	dataEntry, indexEntry, metaEntry := files[0], files[1], files[2]
	create := func(entry codecs.Lucene90PointFileEntry) (store.IndexOutput, error) {
		raw, err := writeState.Directory.CreateOutput(entry.Name, store.IOContext{Context: store.ContextWrite})
		if err != nil {
			return nil, err
		}
		return store.NewChecksumIndexOutput(raw), nil
	}

	success := false
	defer func() {
		if !success {
			// IOUtils.closeWhileHandlingException(this)
			util.CloseAllWhileHandlingException(w.metaOut, w.indexOut, w.dataOut)
		}
	}()

	var err error
	if w.dataOut, err = create(dataEntry); err != nil {
		return nil, err
	}
	if err := codecs.WriteIndexHeader(w.dataOut, dataEntry.Codec, codecs.Lucene90PointsVersionCurrent,
		writeState.SegmentInfo.GetID(), writeState.SegmentSuffix); err != nil {
		return nil, err
	}

	if w.metaOut, err = create(metaEntry); err != nil {
		return nil, err
	}
	if err := codecs.WriteIndexHeader(w.metaOut, metaEntry.Codec, codecs.Lucene90PointsVersionCurrent,
		writeState.SegmentInfo.GetID(), writeState.SegmentSuffix); err != nil {
		return nil, err
	}

	if w.indexOut, err = create(indexEntry); err != nil {
		return nil, err
	}
	if err := codecs.WriteIndexHeader(w.indexOut, indexEntry.Codec, codecs.Lucene90PointsVersionCurrent,
		writeState.SegmentInfo.GetID(), writeState.SegmentSuffix); err != nil {
		return nil, err
	}
	success = true
	return w, nil
}

// WriteField renders Lucene90PointsWriter.writeField(FieldInfo, PointsReader).
func (w *pointsWriter) WriteField(fieldInfo *spi.FieldInfo, reader codecs.PointsReader) error {
	pointValues, err := reader.GetValues(fieldInfo.Name())
	if err != nil {
		return err
	}
	// Java: reader.getValues(fieldInfo.name).getPointTree(). spi.PointValues
	// does not declare getPointTree, so it is reached through the member every
	// PointValues handed to a points writer carries.
	treeSource, ok := pointValues.(interface {
		GetPointTree() (bkd.PointTree, error)
	})
	if !ok {
		return fmt.Errorf("lucene90 points: PointValues %T does not expose getPointTree", pointValues)
	}
	values, err := treeSource.GetPointTree()
	if err != nil {
		return err
	}

	config, err := bkd.NewBKDConfig(
		fieldInfo.PointDimensionCount(),
		fieldInfo.PointIndexDimensionCount(),
		fieldInfo.PointNumBytes(),
		w.maxPointsInLeafNode)
	if err != nil {
		return err
	}

	bkdVersion, err := codecs.Lucene90PointsBKDVersion(w.version)
	if err != nil {
		return err
	}
	writer, err := bkd.NewBKDWriterWithVersion(
		w.writeState.SegmentInfo.MaxDoc(),
		w.writeState.Directory,
		w.writeState.SegmentInfo.Name(),
		config,
		w.maxMBSortInHeap,
		values.Size(),
		int(bkdVersion))
	if err != nil {
		return err
	}
	// try (BKDWriter writer = ...) { ... }
	writeErr := w.writeFieldWith(writer, fieldInfo, values)
	closeErr := writer.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

// writeFieldWith is the body of the try-with-resources block of
// Lucene90PointsWriter.writeField.
func (w *pointsWriter) writeFieldWith(writer *bkd.BKDWriter, fieldInfo *spi.FieldInfo, values bkd.PointTree) error {
	if mutable, ok := values.(bkd.MutablePointTree); ok {
		finalizer, err := writer.WriteField(w.metaOut, w.indexOut, w.dataOut, fieldInfo.Name(), mutable, int(values.Size()))
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

	if err := values.VisitDocValues(&writeFieldIntersectVisitor{writer: writer}); err != nil {
		return err
	}

	// We could have 0 points on merge since all docs with dimensional fields may be deleted:
	finalizer, err := writer.Finish(w.metaOut, w.indexOut, w.dataOut)
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

// writeFieldIntersectVisitor is the anonymous IntersectVisitor that
// Lucene90PointsWriter.writeField passes to visitDocValues.
type writeFieldIntersectVisitor struct {
	writer *bkd.BKDWriter
}

// Visit throws IllegalStateException in Java.
func (v *writeFieldIntersectVisitor) Visit(docID int) error {
	return errors.New("lucene90 points: writeField: visit(int) called during visitDocValues")
}

func (v *writeFieldIntersectVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	return v.writer.Add(packedValue, docID)
}

func (v *writeFieldIntersectVisitor) Compare(minPackedValue, maxPackedValue []byte) index.Relation {
	return index.CellCrossesQuery
}

// Grow keeps the IntersectVisitor default body, which does nothing.
func (v *writeFieldIntersectVisitor) Grow(count int) {}

// Merge renders Lucene90PointsWriter.merge(MergeState).
func (w *pointsWriter) Merge(mergeState *index.MergeState) error {
	/*
	 * If indexSort is activated and some of the leaves are not sorted the next test will catch that
	 * and the non-optimized merge will run. If the readers are all sorted then it's safe to perform
	 * a bulk merge of the points.
	 */
	for _, reader := range mergeState.PointsReaders {
		// Java: reader instanceof Lucene90PointsReader == false, which also
		// holds for a null reader.
		if _, ok := reader.(*pointsReader); !ok {
			// We can only bulk merge when all to-be-merged segments use our format:
			return w.BasePointsWriter.Merge(mergeState)
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

	for _, fieldInfo := range mergeState.MergeFieldInfos.Infos() {
		if fieldInfo.PointDimensionCount() != 0 {
			if fieldInfo.PointDimensionCount() == 1 {
				if err := w.merge1DField(mergeState, fieldInfo); err != nil {
					return err
				}
			} else {
				if err := w.MergeOneField(mergeState, fieldInfo); err != nil {
					return err
				}
			}
		}
	}

	return w.Finish()
}

// merge1DField is the fieldInfo.getPointDimensionCount() == 1 branch of
// Lucene90PointsWriter.merge.
func (w *pointsWriter) merge1DField(mergeState *index.MergeState, fieldInfo *spi.FieldInfo) error {
	// Worst case total maximum size (if none of the points are deleted):
	var totMaxSize int64
	for i, reader := range mergeState.PointsReaders {
		if reader != nil {
			readerFieldInfos := mergeState.FieldInfos[i]
			readerFieldInfo := readerFieldInfos.FieldInfoByName(fieldInfo.Name())
			if readerFieldInfo != nil && readerFieldInfo.PointDimensionCount() > 0 {
				values, err := reader.GetValues(fieldInfo.Name())
				if err != nil {
					return err
				}
				if values != nil {
					totMaxSize += values.Size()
				}
			}
		}
	}

	config, err := bkd.NewBKDConfig(
		fieldInfo.PointDimensionCount(),
		fieldInfo.PointIndexDimensionCount(),
		fieldInfo.PointNumBytes(),
		w.maxPointsInLeafNode)
	if err != nil {
		return err
	}

	// Optimize the 1D case to use BKDWriter.merge, which does a single merge sort of the
	// already sorted incoming segments, instead of trying to sort all points again as if
	// we were simply reindexing them:
	bkdVersion, err := codecs.Lucene90PointsBKDVersion(w.version)
	if err != nil {
		return err
	}
	writer, err := bkd.NewBKDWriterWithVersion(
		w.writeState.SegmentInfo.MaxDoc(),
		w.writeState.Directory,
		w.writeState.SegmentInfo.Name(),
		config,
		w.maxMBSortInHeap,
		totMaxSize,
		int(bkdVersion))
	if err != nil {
		return err
	}
	// try (BKDWriter writer = ...) { ... }
	mergeErr := w.merge1DFieldWith(writer, mergeState, fieldInfo)
	closeErr := writer.Close()
	if mergeErr != nil {
		return mergeErr
	}
	return closeErr
}

// merge1DFieldWith is the body of the try-with-resources block of the 1D
// branch of Lucene90PointsWriter.merge.
func (w *pointsWriter) merge1DFieldWith(writer *bkd.BKDWriter, mergeState *index.MergeState, fieldInfo *spi.FieldInfo) error {
	var pointValues []spi.PointValues
	var docMaps []spi.DocMap
	for i, reader := range mergeState.PointsReaders {
		if reader != nil {
			// we confirmed this up above
			reader90 := reader.(*pointsReader)

			// NOTE: we cannot just use the merged fieldInfo.number (instead of resolving to
			// this
			// reader's FieldInfo as we do below) because field numbers can easily be different
			// when addIndexes(Directory...) copies over segments from another index:

			readerFieldInfos := mergeState.FieldInfos[i]
			readerFieldInfo := readerFieldInfos.FieldInfoByName(fieldInfo.Name())
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

// Finish renders Lucene90PointsWriter.finish(): the meta sentinel (-1), the
// index and data footers, the index and data lengths, and the meta footer.
func (w *pointsWriter) Finish() error {
	if w.finished {
		return errors.New("already finished")
	}
	w.finished = true
	if err := w.metaOut.WriteInt(-1); err != nil {
		return err
	}
	if err := store.WriteFooter(w.indexOut); err != nil {
		return err
	}
	if err := store.WriteFooter(w.dataOut); err != nil {
		return err
	}
	if err := w.metaOut.WriteLong(w.indexOut.GetFilePointer()); err != nil {
		return err
	}
	if err := w.metaOut.WriteLong(w.dataOut.GetFilePointer()); err != nil {
		return err
	}
	return store.WriteFooter(w.metaOut)
}

// Close renders Lucene90PointsWriter.close(): IOUtils.close(metaOut, indexOut,
// dataOut).
func (w *pointsWriter) Close() error {
	return util.CloseAll(w.metaOut, w.indexOut, w.dataOut)
}

var (
	_ codecs.PointsWriter  = (*pointsWriter)(nil)
	_ bkd.IntersectVisitor = (*writeFieldIntersectVisitor)(nil)
)

// VisitByDocIDSetIterator renders the default body of
// PointValues.IntersectVisitor.visit(DocIdSetIterator), which v does not
// override.
func (v *writeFieldIntersectVisitor) VisitByDocIDSetIterator(iterator spi.DocIdSetIterator) error {
	return spi.DefaultVisitByDocIDSetIterator(v, iterator)
}

// VisitByIntsRef renders the default body of
// PointValues.IntersectVisitor.visit(IntsRef), which v does not override.
func (v *writeFieldIntersectVisitor) VisitByIntsRef(ref *util.IntsRef) error {
	return spi.DefaultVisitByIntsRef(v, ref)
}

// VisitByDocIDSetIteratorAndPackedValue renders the default body of
// PointValues.IntersectVisitor.visit(DocIdSetIterator, byte[]), which v
// does not override.
func (v *writeFieldIntersectVisitor) VisitByDocIDSetIteratorAndPackedValue(iterator spi.DocIdSetIterator, packedValue []byte) error {
	return spi.DefaultVisitByDocIDSetIteratorAndPackedValue(v, iterator, packedValue)
}
