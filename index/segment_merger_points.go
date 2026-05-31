// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// mergePoints merges the point (BKD) values of every point field across the
// source segments into the new segment, remapping each point's docID through
// the merge DocMaps. It drives the codec PointsWriter with a merged PointsReader
// that re-emits every live source point — mirroring the net effect of Lucene's
// PointsWriter.merge(MergeState) (rmp #14/#114).
func (sm *SegmentMerger) mergePoints() error {
	if sm.codec == nil || sm.codec.PointsFormat() == nil {
		return nil
	}
	if sm.MergeState.DocMaps == nil {
		if err := sm.buildDocMaps(); err != nil {
			return err
		}
	}

	state := &SegmentWriteState{
		Directory:     sm.directory,
		SegmentInfo:   sm.MergeState.SegmentInfo,
		FieldInfos:    sm.MergeState.MergeFieldInfos,
		SegmentSuffix: "",
	}
	writer, err := sm.codec.PointsFormat().FieldsWriter(state)
	if err != nil {
		return fmt.Errorf("index: merge points: open writer: %w", err)
	}
	defer writer.Close()

	src := &mergePointsSource{sm: sm}
	wroteAny := false
	iter := sm.MergeState.MergeFieldInfos.Iterator()
	for iter.HasNext() {
		info := iter.Next()
		if info.PointDimensionCount() <= 0 {
			continue
		}
		if err := writer.WriteField(info, src); err != nil {
			return fmt.Errorf("index: merge points: write field %q: %w", info.Name(), err)
		}
		wroteAny = true
	}
	if !wroteAny {
		return nil
	}
	if err := writer.Finish(); err != nil {
		return fmt.Errorf("index: merge points: finish: %w", err)
	}
	return nil
}

// intersectablePointValues is the wider PointValues surface the codec's on-disk
// BKD-backed PointValues exposes (index.PointTreeIntersectVisitor walk), used to
// enumerate every point of a source segment during a merge.
type intersectablePointValues interface {
	Intersect(visitor PointTreeIntersectVisitor) error
}

// segPointValues returns the source reader's PointValues for field, or nil.
func segPointValues(reader *CodecReader, field string) PointValues {
	pr := reader.GetPointsReader()
	if pr == nil {
		return nil
	}
	getter, ok := pr.(interface {
		GetValues(field string) (PointValues, error)
	})
	if !ok {
		return nil
	}
	pv, err := getter.GetValues(field)
	if err != nil || pv == nil {
		return nil
	}
	return pv
}

// mergePointsSource is the merged PointsReader handed to the codec PointsWriter.
// It satisfies both the SPI PointsReader (CheckIntegrity/Close) and the codec's
// structural PointsSource (PointValueCount/VisitPoints) contracts.
type mergePointsSource struct {
	sm *SegmentMerger
}

func (s *mergePointsSource) CheckIntegrity() error { return nil }
func (s *mergePointsSource) Close() error          { return nil }

// PointValueCount returns the exact number of live points VisitPoints will emit
// for field across all segments — the count the BKD writer is sized against.
func (s *mergePointsSource) PointValueCount(field string) int64 {
	var total int64
	for i, reader := range s.sm.MergeState.Readers {
		if reader == nil {
			continue
		}
		pv := segPointValues(reader, field)
		if pv == nil {
			continue
		}
		// Fast path: with no deletions in this segment, every stored point is
		// live, so the codec's own value count is exact.
		if reader.GetLiveDocs() == nil {
			total += pv.GetValueCount()
			continue
		}
		// Otherwise count the live points by walking the tree.
		iv, ok := pv.(intersectablePointValues)
		if !ok {
			total += pv.GetValueCount()
			continue
		}
		var n int64
		_ = iv.Intersect(&countLivePointsVisitor{docMap: s.sm.MergeState.DocMaps[i], n: &n})
		total += n
	}
	return total
}

// VisitPoints re-emits every live point of field across all segments, remapping
// each docID into the merged doc space.
func (s *mergePointsSource) VisitPoints(field string, fn func(docID int, packedValue []byte) error) error {
	for i, reader := range s.sm.MergeState.Readers {
		if reader == nil {
			continue
		}
		pv := segPointValues(reader, field)
		if pv == nil {
			continue
		}
		iv, ok := pv.(intersectablePointValues)
		if !ok {
			return fmt.Errorf("index: merge points: field %q reader %d: PointValues %T is not intersectable", field, i, pv)
		}
		v := &mergePointVisitor{docMap: s.sm.MergeState.DocMaps[i], fn: fn}
		if err := iv.Intersect(v); err != nil {
			return err
		}
		if v.err != nil {
			return v.err
		}
	}
	return nil
}

// mergePointVisitor forwards every (docID, packedValue) of a source segment to
// the merge sink, remapping the docID and dropping deleted documents. Compare
// always returns CELL_CROSSES_QUERY (2) so the BKD walk delivers packed values
// for every point rather than docID-only inside-cell hits.
type mergePointVisitor struct {
	docMap DocMap
	fn     func(docID int, packedValue []byte) error
	err    error
}

func (v *mergePointVisitor) Visit(docID int) error {
	// Unreachable while Compare returns CROSSES (no inside-cell, value-less
	// hits); guarding makes a future Compare change fail loud instead of
	// silently dropping points.
	return fmt.Errorf("index: merge points: unexpected value-less Visit(doc=%d)", docID)
}

func (v *mergePointVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	mapped := v.docMap.Get(docID)
	if mapped < 0 {
		return nil // deleted in the source segment
	}
	cp := make([]byte, len(packedValue))
	copy(cp, packedValue)
	if err := v.fn(mapped, cp); err != nil {
		v.err = err
		return err
	}
	return nil
}

func (v *mergePointVisitor) Compare(minPackedValue, maxPackedValue []byte) int { return 2 }
func (v *mergePointVisitor) Grow(count int)                                    {}

// countLivePointsVisitor counts the live points of a segment (used to size the
// BKD writer when a segment carries deletions).
type countLivePointsVisitor struct {
	docMap DocMap
	n      *int64
}

func (c *countLivePointsVisitor) Visit(docID int) error {
	if c.docMap.Get(docID) >= 0 {
		*c.n++
	}
	return nil
}
func (c *countLivePointsVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	if c.docMap.Get(docID) >= 0 {
		*c.n++
	}
	return nil
}
func (c *countLivePointsVisitor) Compare(minPackedValue, maxPackedValue []byte) int { return 2 }
func (c *countLivePointsVisitor) Grow(count int)                                    {}
