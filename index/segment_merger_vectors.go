// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// mergeVectorValues merges the KNN vector values of every vector field across
// the source segments into the new segment, remapping each vector's docID
// through the merge DocMaps. It reuses the same vectorValuesConsumer as the
// flush path: each merged vector is re-added (in merged-docID order) and the
// codec rebuilds the per-field HNSW graph — the net effect of Lucene's
// KnnVectorsWriter.mergeOneField (rmp #14/#114).
func (sm *SegmentMerger) mergeVectorValues() error {
	if sm.codec == nil || sm.codec.KnnVectorsFormat() == nil {
		return nil
	}
	if sm.MergeState.DocMaps == nil {
		if err := sm.buildDocMaps(); err != nil {
			return err
		}
	}

	// Skip the whole leg when no field carries vectors.
	hasVectors := false
	probe := sm.MergeState.MergeFieldInfos.Iterator()
	for probe.HasNext() {
		if probe.Next().VectorDimension() > 0 {
			hasVectors = true
			break
		}
	}
	if !hasVectors {
		return nil
	}

	consumer := newVectorValuesConsumer(sm.codec, sm.directory, sm.MergeState.SegmentInfo, util.NoOpInfoStream)

	iter := sm.MergeState.MergeFieldInfos.Iterator()
	for iter.HasNext() {
		info := iter.Next()
		if info.VectorDimension() <= 0 {
			continue
		}
		handle, err := consumer.AddField(info)
		if err != nil {
			consumer.Abort()
			return fmt.Errorf("index: merge vectors: add field %q: %w", info.Name(), err)
		}
		if err := sm.mergeOneVectorField(info, handle); err != nil {
			consumer.Abort()
			return err
		}
	}

	state := &SegmentWriteState{
		Directory:     sm.directory,
		SegmentInfo:   sm.MergeState.SegmentInfo,
		FieldInfos:    sm.MergeState.MergeFieldInfos,
		SegmentSuffix: "",
	}
	if err := consumer.Flush(state, nil); err != nil {
		return fmt.Errorf("index: merge vectors: flush: %w", err)
	}
	return nil
}

// mergeOneVectorField streams every live vector of info across the source
// segments into handle, remapping docIDs into the merged doc space.
func (sm *SegmentMerger) mergeOneVectorField(info *FieldInfo, handle KnnFieldVectorsWriterHandle) error {
	for i, reader := range sm.MergeState.Readers {
		if reader == nil {
			continue
		}
		vr := reader.GetVectorReader()
		if vr == nil {
			continue
		}
		delegate, ok := vr.(knnVectorsReaderDelegate)
		if !ok {
			continue
		}
		docMap := sm.MergeState.DocMaps[i]
		maxDoc := sm.MergeState.MaxDocs[i]

		switch info.VectorEncoding() {
		case VectorEncodingByte:
			bvv, err := delegate.ByteVectorValues(info.Name())
			if err != nil {
				return fmt.Errorf("index: merge vectors: byte values %q reader %d: %w", info.Name(), i, err)
			}
			if bvv == nil {
				continue
			}
			for {
				d, err := bvv.NextDoc()
				if err != nil {
					return err
				}
				if d < 0 || d >= maxDoc {
					break
				}
				mapped := docMap.Get(d)
				if mapped < 0 {
					continue
				}
				vec, err := bvv.Get(d)
				if err != nil {
					return err
				}
				cp := make([]byte, len(vec))
				copy(cp, vec)
				if err := handle.AddValue(mapped, cp); err != nil {
					return err
				}
			}
		default: // VectorEncodingFloat32
			fvv, err := delegate.FloatVectorValues(info.Name())
			if err != nil {
				return fmt.Errorf("index: merge vectors: float values %q reader %d: %w", info.Name(), i, err)
			}
			if fvv == nil {
				continue
			}
			for {
				d, err := fvv.NextDoc()
				if err != nil {
					return err
				}
				if d < 0 || d >= maxDoc {
					break
				}
				mapped := docMap.Get(d)
				if mapped < 0 {
					continue
				}
				vec, err := fvv.Get(d)
				if err != nil {
					return err
				}
				cp := make([]float32, len(vec))
				copy(cp, vec)
				if err := handle.AddValue(mapped, cp); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
