// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// BaseNormsConsumer provides a base implementation of the NormsConsumer API.
// It implements the merge logic for normalization values.
//
// Mirrors org.apache.lucene.codecs.NormsConsumer in Apache Lucene 10.5.0.
type BaseNormsConsumer struct {
	// impl is the concrete implementation of the NormsConsumer interface
	// that performs the actual writing of norms.
	impl spi.NormsConsumer
}

// NewBaseNormsConsumer creates a new BaseNormsConsumer wrapper.
func NewBaseNormsConsumer(impl spi.NormsConsumer) *BaseNormsConsumer {
	return &BaseNormsConsumer{impl: impl}
}

// Merge merges in the fields from the readers in mergeState.
// Mirrors org.apache.lucene.codecs.NormsConsumer.merge in Apache Lucene 10.5.0.
func (b *BaseNormsConsumer) Merge(mergeState *index.MergeState) error {
	for _, normsProducer := range mergeState.NormsProducers {
		if normsProducer != nil {
			if err := mergeState.CheckAborted(); err != nil {
				return err
			}
			if err := normsProducer.CheckIntegrity(); err != nil {
				return err
			}
		}
	}
	for _, mergeFieldInfo := range mergeState.MergeFieldInfos.Fields() {
		if mergeFieldInfo.HasNorms() {
			if err := b.MergeNormsField(mergeFieldInfo, mergeState); err != nil {
				return err
			}
		}
	}
	return nil
}

// MergeNormsField merges the norms from the readers in mergeState.
// Mirrors org.apache.lucene.codecs.NormsConsumer.mergeNormsField in Apache Lucene 10.5.0.
func (b *BaseNormsConsumer) MergeNormsField(mergeFieldInfo *schema.FieldInfo, mergeState *index.MergeState) error {
	// The default implementation calls AddNormsField, passing an iterator
	// that merges and filters deleted documents on the fly.
	iterator := &mergeNormsIterator{
		mergeFieldInfo: mergeFieldInfo,
		mergeState:     mergeState,
	}

	return b.impl.AddNormsField(mergeFieldInfo, iterator)
}

// mergeNormsIterator implements spi.NormsIterator by merging multiple NormsProducers.
type mergeNormsIterator struct {
	mergeFieldInfo *schema.FieldInfo
	mergeState     *index.MergeState
	merger         index.DocIDMerger
	current        index.DocIDMergerSub
}

func (it *mergeNormsIterator) init() error {
	if it.merger != nil {
		return nil
	}

	var subs []index.DocIDMergerSub
	for i, normsProducer := range it.mergeState.NormsProducers {
		if normsProducer != nil {
			readerFieldInfo := it.mergeState.FieldInfos[i].FieldInfoByName(it.mergeFieldInfo.Name())
			if readerFieldInfo != nil && readerFieldInfo.HasNorms() {
				norms, err := normsProducer.GetNorms(readerFieldInfo)
				if err != nil {
					return err
				}
				subs = append(subs, &numericDocValuesSub{
					docMap: it.mergeState.DocMaps[i],
					values: norms,
				})
			}
		}
	}

	var err error
	it.merger, err = index.NewDocIDMerger(subs, 0, it.mergeState.NeedsIndexSort)
	return err
}

func (it *mergeNormsIterator) Next() bool {
	if err := it.init(); err != nil {
		return false
	}

	sub, err := it.merger.Next()
	if err != nil || sub == nil {
		it.current = nil
		return false
	}
	it.current = sub
	return true
}

func (it *mergeNormsIterator) DocID() int {
	if it.current == nil {
		return -1
	}
	return it.current.MappedDocID()
}

func (it *mergeNormsIterator) LongValue() int64 {
	if it.current == nil {
		return 0
	}
	sub, ok := it.current.(*numericDocValuesSub)
	if !ok {
		return 0
	}
	val, err := sub.values.LongValue()
	if err != nil {
		return 0
	}
	return val
}

type numericDocValuesSub struct {
	docMap index.DocMap
	values index.NumericDocValues
}

func (s *numericDocValuesSub) MappedDocID() int {
	return s.docMap.Get(s.values.DocID())
}

func (s *numericDocValuesSub) NextDoc() (int, error) {
	return s.values.NextDoc()
}

func (s *numericDocValuesSub) NextMappedDoc() (int, error) {
	for {
		doc, err := s.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == index.NO_MORE_DOCS {
			return index.NO_MORE_DOCS, nil
		}
		mapped := s.docMap.Get(doc)
		if mapped != -1 {
			return mapped, nil
		}
	}
}
