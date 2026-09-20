// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BasePointsWriter carries the concrete members of the abstract class
// org.apache.lucene.codecs.PointsWriter (Apache Lucene 10.5.0): mergeOneField
// and merge. The abstract members (writeField, finish) and close are the
// methods of [PointsWriter].
//
// Every concrete writer embeds a BasePointsWriter built with
// [NewBasePointsWriter], passing itself as impl: impl is the receiver on which
// the merge members invoke writeField and finish, which Java dispatches
// through this. merge is not part of [PointsWriter] because MergeState lives
// in package index, which spi cannot import.
type BasePointsWriter struct {
	impl PointsWriter
}

// NewBasePointsWriter returns the base of the writer impl. Mirrors the
// protected constructor PointsWriter().
func NewBasePointsWriter(impl PointsWriter) *BasePointsWriter {
	return &BasePointsWriter{impl: impl}
}

// errMergedPointsUnsupported renders the UnsupportedOperationException thrown
// by the members of the anonymous PointsReader, PointValues and PointTree that
// PointsWriter.mergeOneField hands to writeField. Only size(), getPointTree()
// and the tree's moveTo*/size()/visitDocValues() carry a body in Java.
var errMergedPointsUnsupported = errors.New(
	"codecs: PointsWriter.mergeOneField's merged points reader supports only getValues, size and visitDocValues",
)

// MergeOneField is the default naive merge implementation for one field: it
// just re-indexes all the values from the incoming segment. The default codec
// overrides this for 1D fields and uses a faster but more complex
// implementation.
//
// Port of the protected PointsWriter.mergeOneField(MergeState, FieldInfo).
func (b *BasePointsWriter) MergeOneField(mergeState *index.MergeState, fieldInfo *spi.FieldInfo) error {
	var maxPointCount int64
	for i, pointsReader := range mergeState.PointsReaders {
		if pointsReader != nil {
			readerFieldInfo := mergeState.FieldInfos[i].FieldInfoByName(fieldInfo.Name())
			if readerFieldInfo != nil && readerFieldInfo.PointDimensionCount() > 0 {
				values, err := pointsReader.GetValues(fieldInfo.Name())
				if err != nil {
					return err
				}
				if values != nil {
					maxPointCount += values.Size()
				}
			}
		}
	}
	finalMaxPointCount := maxPointCount
	return b.impl.WriteField(fieldInfo, &mergedPointsReader{
		mergeState:         mergeState,
		fieldInfo:          fieldInfo,
		finalMaxPointCount: finalMaxPointCount,
	})
}

// Merge is the default merge implementation to merge incoming points readers
// by visiting all their points and adding to this writer.
//
// Port of PointsWriter.merge(MergeState).
func (b *BasePointsWriter) Merge(mergeState *index.MergeState) error {
	// check each incoming reader
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
	// merge field at a time
	// Java: for (FieldInfo fieldInfo : mergeState.mergeFieldInfos).
	for _, fieldInfo := range mergeState.MergeFieldInfos.Infos() {
		if fieldInfo.PointDimensionCount() != 0 {
			if err := b.MergeOneField(mergeState, fieldInfo); err != nil {
				return err
			}
		}
	}
	return b.impl.Finish()
}

// mergedPointsReader is the anonymous PointsReader that
// PointsWriter.mergeOneField hands to writeField.
type mergedPointsReader struct {
	mergeState         *index.MergeState
	fieldInfo          *spi.FieldInfo
	finalMaxPointCount int64
}

func (r *mergedPointsReader) Close() error { return nil }

// GetValues renders `public PointValues getValues(String fieldName)`.
func (r *mergedPointsReader) GetValues(fieldName string) (spi.PointValues, error) {
	if fieldName != r.fieldInfo.Name() {
		return nil, fmt.Errorf("field name must match the field being merged")
	}
	return newMergedPointValues(r, fieldName), nil
}

// CheckIntegrity throws UnsupportedOperationException in Java.
func (r *mergedPointsReader) CheckIntegrity() error {
	return errMergedPointsUnsupported
}

// GetMergeInstance returns this reader. The anonymous PointsReader does not
// override getMergeInstance(), so it keeps the PointsReader default body,
// `return this;`.
func (r *mergedPointsReader) GetMergeInstance() spi.PointsReader { return r }

// mergedPointValues is the anonymous PointValues returned by
// mergedPointsReader.GetValues.
type mergedPointValues struct {
	*spi.BasePointValues
	reader    *mergedPointsReader
	fieldName string
}

func newMergedPointValues(reader *mergedPointsReader, fieldName string) *mergedPointValues {
	v := &mergedPointValues{reader: reader, fieldName: fieldName}
	v.BasePointValues = spi.NewBasePointValues(v)
	return v
}

// GetPointTree renders `public PointTree getPointTree()`.
func (v *mergedPointValues) GetPointTree() (spi.PointTree, error) {
	return &mergedPointTree{values: v}, nil
}

// GetMinPackedValue throws UnsupportedOperationException in Java.
func (v *mergedPointValues) GetMinPackedValue() ([]byte, error) {
	return nil, errMergedPointsUnsupported
}

// GetMaxPackedValue throws UnsupportedOperationException in Java.
func (v *mergedPointValues) GetMaxPackedValue() ([]byte, error) {
	return nil, errMergedPointsUnsupported
}

// GetNumDimensions throws UnsupportedOperationException in Java.
func (v *mergedPointValues) GetNumDimensions() (int, error) {
	return 0, errMergedPointsUnsupported
}

// GetNumIndexDimensions throws UnsupportedOperationException in Java.
func (v *mergedPointValues) GetNumIndexDimensions() (int, error) {
	return 0, errMergedPointsUnsupported
}

// GetBytesPerDimension throws UnsupportedOperationException in Java.
func (v *mergedPointValues) GetBytesPerDimension() (int, error) {
	return 0, errMergedPointsUnsupported
}

// Size renders `public long size()`, whose body is
// `return finalMaxPointCount`.
func (v *mergedPointValues) Size() int64 { return v.reader.finalMaxPointCount }

// GetDocCount throws UnsupportedOperationException in Java.
func (v *mergedPointValues) GetDocCount() int { panic(errMergedPointsUnsupported) }

// mergedPointTree is the anonymous PointTree returned by
// mergedPointValues.GetPointTree.
type mergedPointTree struct {
	values *mergedPointValues
}

// Clone throws UnsupportedOperationException in Java.
func (t *mergedPointTree) Clone() spi.PointTree { panic(errMergedPointsUnsupported) }

func (t *mergedPointTree) MoveToChild() (bool, error) { return false, nil }

func (t *mergedPointTree) MoveToSibling() (bool, error) { return false, nil }

func (t *mergedPointTree) MoveToParent() (bool, error) { return false, nil }

// GetMinPackedValue throws UnsupportedOperationException in Java.
func (t *mergedPointTree) GetMinPackedValue() []byte { panic(errMergedPointsUnsupported) }

// GetMaxPackedValue throws UnsupportedOperationException in Java.
func (t *mergedPointTree) GetMaxPackedValue() []byte { panic(errMergedPointsUnsupported) }

func (t *mergedPointTree) Size() int64 { return t.values.reader.finalMaxPointCount }

// VisitDocIDs throws UnsupportedOperationException in Java.
func (t *mergedPointTree) VisitDocIDs(visitor spi.IntersectVisitor) error {
	return errMergedPointsUnsupported
}

// VisitDocValues renders `public void visitDocValues(IntersectVisitor
// mergedVisitor)`.
func (t *mergedPointTree) VisitDocValues(mergedVisitor spi.IntersectVisitor) error {
	mergeState := t.values.reader.mergeState
	fieldName := t.values.fieldName
	for i, pointsReader := range mergeState.PointsReaders {
		if pointsReader == nil {
			// This segment has no points
			continue
		}
		readerFieldInfo := mergeState.FieldInfos[i].FieldInfoByName(fieldName)
		if readerFieldInfo == nil {
			// This segment never saw this field
			continue
		}

		if readerFieldInfo.PointDimensionCount() == 0 {
			// This segment saw this field, but the field did not index points in it:
			continue
		}

		values, err := pointsReader.GetValues(fieldName)
		if err != nil {
			return err
		}
		if values == nil {
			continue
		}
		docMap := mergeState.DocMaps[i]
		tree, err := values.GetPointTree()
		if err != nil {
			return err
		}
		if err := tree.VisitDocValues(&mergedSegmentVisitor{
			mergedVisitor: mergedVisitor,
			docMap:        docMap,
		}); err != nil {
			return err
		}
	}
	return nil
}

// mergedSegmentVisitor is the anonymous IntersectVisitor that maps each source
// segment's docIDs to the merged segment's docIDs.
type mergedSegmentVisitor struct {
	mergedVisitor spi.IntersectVisitor
	docMap        index.DocMap
}

// Visit throws IllegalStateException in Java: it must never be called during
// visitDocValues.
func (v *mergedSegmentVisitor) Visit(docID int) error {
	return fmt.Errorf("codecs: merge points: visit(int) called during visitDocValues")
}

func (v *mergedSegmentVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	newDocID := v.docMap.Get(docID)
	if newDocID != -1 {
		// Not deleted:
		return v.mergedVisitor.VisitByPackedValue(newDocID, packedValue)
	}
	return nil
}

// Compare forces this segment's PointsReader to always visit all docs +
// values.
func (v *mergedSegmentVisitor) Compare(minPackedValue, maxPackedValue []byte) index.Relation {
	return index.CellCrossesQuery
}

// Grow keeps the IntersectVisitor default body, which does nothing.
func (v *mergedSegmentVisitor) Grow(count int) {}

var (
	_ PointsReader         = (*mergedPointsReader)(nil)
	_ spi.PointValues      = (*mergedPointValues)(nil)
	_ spi.PointTree        = (*mergedPointTree)(nil)
	_ spi.IntersectVisitor = (*mergedSegmentVisitor)(nil)
)

// VisitByDocIDSetIterator renders the default body of
// PointValues.IntersectVisitor.visit(DocIdSetIterator), which v does not
// override.
func (v *mergedSegmentVisitor) VisitByDocIDSetIterator(iterator spi.DocIdSetIterator) error {
	return spi.DefaultVisitByDocIDSetIterator(v, iterator)
}

// VisitByIntsRef renders the default body of
// PointValues.IntersectVisitor.visit(IntsRef), which v does not override.
func (v *mergedSegmentVisitor) VisitByIntsRef(ref *util.IntsRef) error {
	return spi.DefaultVisitByIntsRef(v, ref)
}

// VisitByDocIDSetIteratorAndPackedValue renders the default body of
// PointValues.IntersectVisitor.visit(DocIdSetIterator, byte[]), which v
// does not override.
func (v *mergedSegmentVisitor) VisitByDocIDSetIteratorAndPackedValue(iterator spi.DocIdSetIterator, packedValue []byte) error {
	return spi.DefaultVisitByDocIDSetIteratorAndPackedValue(v, iterator, packedValue)
}
