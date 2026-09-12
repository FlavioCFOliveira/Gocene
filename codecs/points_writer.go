// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// BasePointsWriter provides the default merge implementation for point values.
// This is the Go port of the common logic in org.apache.lucene.codecs.PointsWriter.
type BasePointsWriter struct {
	writer spi.PointsWriter
}

// NewBasePointsWriter creates a new BasePointsWriter wrapping the given implementation.
func NewBasePointsWriter(writer spi.PointsWriter) *BasePointsWriter {
	return &BasePointsWriter{writer: writer}
}

// Merge coordinates the merging of incoming points readers by visiting all their points
// and adding them to the current writer.
func (b *BasePointsWriter) Merge(mergeState *index.MergeState) error {
	// Check each incoming reader
	for _, reader := range mergeState.Readers {
		if reader != nil {
			if err := mergeState.CheckAborted(); err != nil {
				return err
			}
			if err := reader.GetPointsReader().CheckIntegrity(); err != nil {
				return err
			}
		}
	}

	// Merge field at a time
	for _, fieldInfo := range mergeState.MergeFieldInfos {
		if fieldInfo.PointDimensionCount() != 0 {
			if err := b.mergeOneField(mergeState, fieldInfo); err != nil {
				return err
			}
		}
	}

	return b.writer.Finish()
}

// mergeOneField implements the default naive merge for one field: it just re-indexes
// all the values from the incoming segments.
func (b *BasePointsWriter) mergeOneField(mergeState *index.MergeState, fieldInfo *spi.FieldInfo) error {
	var maxPointCount int64
	for i := 0; i < len(mergeState.Readers); i++ {
		reader := mergeState.Readers[i]
		if reader != nil {
			readerFieldInfo := mergeState.FieldInfos[i].FieldInfoByName(fieldInfo.Name)
			if readerFieldInfo != nil && readerFieldInfo.PointDimensionCount() > 0 {
				values, err := reader.GetPointValues(fieldInfo.Name)
				if err == nil && values != nil {
					maxPointCount += values.GetValueCount()
				}
			}
		}
	}

	mergedReader := &mergedPointsReader{
		mergeState:    mergeState,
		fieldInfo:     fieldInfo,
		maxPointCount: maxPointCount,
	}

	return b.writer.WriteField(fieldInfo, mergedReader)
}

// mergedPointsReader is a temporary reader used during the naive merge process.
type mergedPointsReader struct {
	mergeState    *index.MergeState
	fieldInfo     *spi.FieldInfo
	maxPointCount int64
}

func (r *mergedPointsReader) CheckIntegrity() error { return nil }
func (r *mergedPointsReader) Close() error          { return nil }

// GetValues recovers the wide read surface for the merged points.
func (r *mergedPointsReader) GetValues(field string) (index.PointValues, error) {
	if field != r.fieldInfo.Name {
		return nil, fmt.Errorf("field name must match the field being merged")
	}
	return &mergedPointValues{
		reader: r,
	}, nil
}

// mergedPointValues implements a PointValues view over the incoming merge state.
type mergedPointValues struct {
	reader *mergedPointsReader
}

func (v *mergedPointValues) GetDocCount() int            { return 0 }
func (v *mergedPointValues) GetDocCountWithValue() int64 { return 0 }
func (v *mergedPointValues) GetValueCount() int64        { return v.reader.maxPointCount }
func (v *mergedPointValues) GetMinPackedValue() ([]byte, error) {
	return nil, fmt.Errorf("unsupported")
}
func (v *mergedPointValues) GetMaxPackedValue() ([]byte, error) {
	return nil, fmt.Errorf("unsupported")
}
func (v *mergedPointValues) GetNumDimensions() int     { return 0 }
func (v *mergedPointValues) GetBytesPerDimension() int { return 0 }

// GetPointTree returns a cursor that streams points from the source segments.
func (v *mergedPointValues) GetPointTree() bkd.PointTree {
	return &mergedPointTree{
		values: v,
	}
}

// mergedPointTree implements a PointTree that iterates over multiple source segments.
type mergedPointTree struct {
	values *mergedPointValues
}

func (t *mergedPointTree) Clone() bkd.PointTree {
	return nil // Not used in naive merge
}

func (t *mergedPointTree) MoveToChild() (bool, error) {
	return false, nil
}

func (t *mergedPointTree) MoveToSibling() (bool, error) {
	return false, nil
}

func (t *mergedPointTree) MoveToParent() (bool, error) {
	return false, nil
}

func (t *mergedPointTree) GetMinPackedValue() []byte {
	return nil
}

func (t *mergedPointTree) GetMaxPackedValue() []byte {
	return nil
}

func (t *mergedPointTree) Size() int64 {
	return t.values.reader.maxPointCount
}

func (t *mergedPointTree) VisitDocIDs(visitor bkd.IntersectVisitor) error {
	return nil // Not used in naive merge
}

func (t *mergedPointTree) VisitDocValues(visitor bkd.IntersectVisitor) error {
	ms := t.values.reader.mergeState
	fieldName := t.values.reader.fieldInfo.Name

	for i := 0; i < len(ms.Readers); i++ {
		reader := ms.Readers[i]
		if reader == nil {
			continue
		}

		readerFieldInfo := ms.FieldInfos[i].FieldInfoByName(fieldName)
		if readerFieldInfo == nil || readerFieldInfo.PointDimensionCount() == 0 {
			continue
		}

		values, err := reader.GetPointValues(fieldName)
		if err != nil || values == nil {
			continue
		}

		docMap := ms.DocMaps[i]

		// Recover the PointTree from the source values via assertion.
		if tree, ok := values.(interface{ GetPointTree() bkd.PointTree }); ok {
			if err := tree.VisitDocValues(&mergedVisitor{
				mergedVisitor: visitor,
				docMap:        docMap,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// mergedVisitor maps docIDs from source segments to the merged segment's docIDs.
type mergedVisitor struct {
	mergedVisitor bkd.IntersectVisitor
	docMap        index.DocMap
}

func (v *mergedVisitor) Visit(docID int) error {
	return fmt.Errorf("should never be called during VisitDocValues")
}

func (v *mergedVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	newDocID := v.docMap.Get(docID)
	if newDocID != -1 {
		return v.mergedVisitor.VisitByPackedValue(newDocID, packedValue)
	}
	return nil
}

func (v *mergedVisitor) Compare(minPackedValue, maxPackedValue []byte) geo.Relation {
	// Forces this segment's PointsReader to always visit all docs + values.
	return geo.CellCrossesQuery
}

func (v *mergedVisitor) Grow(count int) {
	v.mergedVisitor.Grow(count)
}
