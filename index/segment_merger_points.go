// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

// pointsWriterMerger is the merge member of PointsWriter. The codec writers
// carry it through codecs.BasePointsWriter (spi cannot declare it: MergeState
// lives in this package).
type pointsWriterMerger interface {
	Merge(mergeState *MergeState) error
}

// mergePoints merges the point values of the readers being merged into the
// new segment. Mirrors SegmentMerger.mergePoints: the codec's PointsWriter is
// opened for the merged segment and merges every point field of the
// MergeState.
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
		Directory:      sm.directory,
		SegmentInfo:    sm.MergeState.SegmentInfo,
		FieldInfos:     sm.MergeState.MergeFieldInfos,
		SegmentSuffix:  "",
		NeedsIndexSort: sm.MergeState.NeedsIndexSort,
		IsMerge:        true,
	}
	writer, err := sm.codec.PointsFormat().FieldsWriter(state)
	if err != nil {
		return fmt.Errorf("index: merge points: open writer: %w", err)
	}
	merger, ok := writer.(pointsWriterMerger)
	if !ok {
		closeErr := writer.Close()
		if closeErr != nil {
			return fmt.Errorf("index: merge points: writer %T does not carry PointsWriter.merge (close: %v)", writer, closeErr)
		}
		return fmt.Errorf("index: merge points: writer %T does not carry PointsWriter.merge", writer)
	}
	// try (PointsWriter writer = ...) { writer.merge(mergeState); }
	mergeErr := merger.Merge(sm.MergeState)
	closeErr := writer.Close()
	if mergeErr != nil {
		return mergeErr
	}
	return closeErr
}
