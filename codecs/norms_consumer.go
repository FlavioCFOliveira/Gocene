// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
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
// Mirrors org.apache.lucene.codecs.NormsConsumer.mergeNormsField in Apache
// Lucene 10.5.0 (NormsConsumer.java:97-178).
func (b *BaseNormsConsumer) MergeNormsField(mergeFieldInfo *spi.FieldInfo, mergeState *index.MergeState) error {
	// The default implementation calls AddNormsField, passing a NormsProducer
	// that merges and filters deleted documents on the fly.
	return b.impl.AddNormsField(mergeFieldInfo, &mergingNormsProducer{
		mergeFieldInfo: mergeFieldInfo,
		mergeState:     mergeState,
	})
}

// errMergingNormsAdvance renders the UnsupportedOperationException that
// advance / advanceExact throw on the anonymous NumericDocValues built by
// NormsConsumer.mergeNormsField (NormsConsumer.java:152-160).
var errMergingNormsAdvance = errors.New("codecs: merging norms: Advance/AdvanceExact is not supported")

// mergingNormsProducer is the Go rendering of the anonymous NormsProducer
// NormsConsumer.mergeNormsField passes to addNormsField
// (NormsConsumer.java:105-177).
type mergingNormsProducer struct {
	mergeFieldInfo *spi.FieldInfo
	mergeState     *index.MergeState
}

// GetNorms builds the per-sub DocIDMerger and returns a fresh merged cursor.
// Mirrors getNorms(FieldInfo), which raises
// IllegalArgumentException("wrong fieldInfo") for any other field.
func (p *mergingNormsProducer) GetNorms(fieldInfo *spi.FieldInfo) (index.NumericDocValues, error) {
	if fieldInfo != p.mergeFieldInfo {
		return nil, errors.New("wrong fieldInfo")
	}

	var subs []index.DocIDMergerSub
	for i, normsProducer := range p.mergeState.NormsProducers {
		var norms index.NumericDocValues
		if normsProducer != nil {
			readerFieldInfo := p.mergeState.FieldInfos[i].FieldInfoByName(p.mergeFieldInfo.Name())
			if readerFieldInfo != nil && readerFieldInfo.HasNorms() {
				var err error
				norms, err = normsProducer.GetNorms(readerFieldInfo)
				if err != nil {
					return nil, err
				}
			}
		}
		if norms != nil {
			subs = append(subs, &normsConsumerNumericDocValuesSub{
				docMap: p.mergeState.DocMaps[i],
				values: norms,
			})
		}
	}

	merger, err := index.NewDocIDMerger(subs, 0, p.mergeState.NeedsIndexSort)
	if err != nil {
		return nil, err
	}
	return &mergingNormsValues{merger: merger, docID: -1}, nil
}

func (p *mergingNormsProducer) CheckIntegrity() error { return nil }

// GetMergeInstance carries the default body of
// NormsProducer.getMergeInstance(), which the anonymous subclass does not
// override: it returns the receiver.
func (p *mergingNormsProducer) GetMergeInstance() spi.NormsProducer { return p }

func (p *mergingNormsProducer) Close() error { return nil }

// mergingNormsValues is the Go rendering of the anonymous NumericDocValues
// returned by the merging NormsProducer (NormsConsumer.java:133-175).
type mergingNormsValues struct {
	merger  index.DocIDMerger
	docID   int
	current index.DocIDMergerSub
}

func (it *mergingNormsValues) DocID() int { return it.docID }

func (it *mergingNormsValues) NextDoc() (int, error) {
	sub, err := it.merger.Next()
	if err != nil {
		return 0, err
	}
	if sub == nil {
		it.current = nil
		it.docID = index.NO_MORE_DOCS
	} else {
		it.current = sub
		it.docID = sub.MappedDocID()
	}
	return it.docID, nil
}

func (it *mergingNormsValues) Advance(int) (int, error) {
	return 0, errMergingNormsAdvance
}

func (it *mergingNormsValues) AdvanceExact(int) (bool, error) {
	return false, errMergingNormsAdvance
}

// Cost mirrors the anonymous NumericDocValues, whose cost() returns 0.
func (it *mergingNormsValues) Cost() int64 { return 0 }

func (it *mergingNormsValues) LongValue() (int64, error) {
	if it.current == nil {
		return 0, errors.New("codecs: merging norms: LongValue called outside a document")
	}
	sub, ok := it.current.(*normsConsumerNumericDocValuesSub)
	if !ok {
		return 0, errors.New("codecs: merging norms: unexpected DocIDMerger sub type")
	}
	return sub.values.LongValue()
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int), which the anonymous
// Java subclass does not override.
func (it *mergingNormsValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd() —
// docID() + 1 — which the anonymous Java subclass does not override.
func (it *mergingNormsValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(it)
}

// normsConsumerNumericDocValuesSub is the Go rendering of the private static
// nested class NormsConsumer.NumericDocValuesSub of Apache Lucene 10.5.0.
// DocValuesConsumer declares a nested class of the same simple name; Java keeps
// the two apart by their enclosing class, so the Go names carry the owner.
type normsConsumerNumericDocValuesSub struct {
	docMap index.DocMap
	values index.NumericDocValues
}

func (s *normsConsumerNumericDocValuesSub) MappedDocID() int {
	return s.docMap.Get(s.values.DocID())
}

func (s *normsConsumerNumericDocValuesSub) NextDoc() (int, error) {
	return s.values.NextDoc()
}

func (s *normsConsumerNumericDocValuesSub) NextMappedDoc() (int, error) {
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
