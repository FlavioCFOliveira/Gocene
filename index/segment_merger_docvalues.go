// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
)

// docValuesConsumerMerger is the merge member of DocValuesConsumer. The codec
// consumers carry it through codecs.BaseDocValuesConsumer (spi cannot declare
// it: MergeState lives in this package).
type docValuesConsumerMerger interface {
	Merge(mergeState *MergeState) error
}

// mergeDocValues merges the doc values of the readers being merged into the
// new segment. Mirrors SegmentMerger.mergeDocValues: the codec's
// DocValuesConsumer is opened for the merged segment and merges every
// doc-values field of the MergeState.
func (sm *SegmentMerger) mergeDocValues() error {
	if sm.codec == nil || sm.codec.DocValuesFormat() == nil {
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
	consumer, err := sm.codec.DocValuesFormat().FieldsConsumer(state)
	if err != nil {
		return fmt.Errorf("index: merge doc values: open consumer: %w", err)
	}
	merger, ok := consumer.(docValuesConsumerMerger)
	if !ok {
		closeErr := consumer.Close()
		if closeErr != nil {
			return fmt.Errorf("index: merge doc values: consumer %T does not carry DocValuesConsumer.merge (close: %v)", consumer, closeErr)
		}
		return fmt.Errorf("index: merge doc values: consumer %T does not carry DocValuesConsumer.merge", consumer)
	}
	// try (DocValuesConsumer consumer = ...) { consumer.merge(mergeState); }
	mergeErr := merger.Merge(sm.MergeState)
	closeErr := consumer.Close()
	if mergeErr != nil {
		return mergeErr
	}
	return closeErr
}

// dvProducerOf returns the segment reader's DocValuesProducer, or nil.
func dvProducerOf(reader CodecReader) spi.DocValuesProducer {
	p := reader.GetDocValuesReader()
	if p == nil {
		return nil
	}
	dp, ok := p.(spi.DocValuesProducer)
	if !ok {
		return nil
	}
	return dp
}

// subFieldInfo returns the source reader's FieldInfo for field, or nil.
func subFieldInfo(reader CodecReader, field string) *FieldInfo {
	fis := reader.GetFieldInfos()
	if fis == nil {
		return nil
	}
	return fis.FieldInfoByName(field)
}

// dvExhaustedDoc reports whether a doc-values iterator docID marks the end. The
// codec producers use DocIdSetIterator.NO_MORE_DOCS (math.MaxInt32) while some
// in-memory iterators use index.NO_MORE_DOCS (-1); treating any out-of-range
// docID as the end is robust against both (matches rmp #6's docValuesNoMoreDocs).
func dvExhaustedDoc(docID, maxDoc int) bool {
	return docID < 0 || docID >= maxDoc
}

// numericByDoc sorts collected (docID, value) pairs into ascending docID
// order. When a merge honours an index sort the per-reader DocMaps renumber
// documents out of (reader, docID) order, so the pairs must be re-sorted
// before being written.
type numericByDoc struct {
	docIDs []int
	values []int64
}

func (s numericByDoc) Len() int           { return len(s.docIDs) }
func (s numericByDoc) Less(i, j int) bool { return s.docIDs[i] < s.docIDs[j] }
func (s numericByDoc) Swap(i, j int) {
	s.docIDs[i], s.docIDs[j] = s.docIDs[j], s.docIDs[i]
	s.values[i], s.values[j] = s.values[j], s.values[i]
}
