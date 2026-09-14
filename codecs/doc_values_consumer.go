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
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// errDocValuesConsumerUnsupportedOperation renders the
// UnsupportedOperationException thrown by the forward-only merged views and
// by MergedTermsEnum in org.apache.lucene.codecs.DocValuesConsumer.
var errDocValuesConsumerUnsupportedOperation = errors.New("DocValuesConsumer: unsupported operation")

// BaseDocValuesConsumer carries the concrete members of the abstract class
// org.apache.lucene.codecs.DocValuesConsumer of Apache Lucene 10.5.0: merge,
// mergeNumericField, mergeBinaryField, mergeSortedNumericField,
// mergeSortedField and mergeSortedSetField, together with the nested classes
// and static helpers they use. The abstract members (addNumericField,
// addBinaryField, addSortedField, addSortedNumericField, addSortedSetField and
// close) are the methods of [DocValuesConsumer].
//
// Every concrete consumer embeds a BaseDocValuesConsumer built with
// [NewBaseDocValuesConsumer], passing itself as impl: impl is the receiver on
// which the merge members invoke the abstract addXxxField methods, which Java
// dispatches through this.
type BaseDocValuesConsumer struct {
	impl DocValuesConsumer
}

// NewBaseDocValuesConsumer returns the base of the consumer impl. Mirrors the
// protected constructor DocValuesConsumer().
func NewBaseDocValuesConsumer(impl DocValuesConsumer) *BaseDocValuesConsumer {
	return &BaseDocValuesConsumer{impl: impl}
}

// Merge merges in the fields from the readers in mergeState. The default
// implementation calls MergeNumericField, MergeBinaryField, MergeSortedField,
// MergeSortedSetField, or MergeSortedNumericField for each field, depending on
// its type. Implementations can override this method for more sophisticated
// merging (bulk-byte copying, etc).
func (b *BaseDocValuesConsumer) Merge(mergeState *index.MergeState) error {
	for _, docValuesProducer := range mergeState.DocValuesProducers {
		if docValuesProducer != nil {
			if err := mergeState.CheckAborted(); err != nil {
				return err
			}
			if err := docValuesProducer.CheckIntegrity(); err != nil {
				return err
			}
		}
	}

	// Java: for (FieldInfo mergeFieldInfo : mergeState.mergeFieldInfos).
	// FieldInfos is Iterable<FieldInfo> in Java; Infos() exposes the very
	// collection FieldInfos.iterator() walks, in the same order.
	for _, mergeFieldInfo := range mergeState.MergeFieldInfos.Infos() {
		typ := mergeFieldInfo.DocValuesType()
		if typ != spi.DocValuesTypeNone {
			var err error
			switch typ {
			case spi.DocValuesTypeNumeric:
				err = b.MergeNumericField(mergeFieldInfo, mergeState)
			case spi.DocValuesTypeBinary:
				err = b.MergeBinaryField(mergeFieldInfo, mergeState)
			case spi.DocValuesTypeSorted:
				err = b.MergeSortedField(mergeFieldInfo, mergeState)
			case spi.DocValuesTypeSortedSet:
				err = b.MergeSortedSetField(mergeFieldInfo, mergeState)
			case spi.DocValuesTypeSortedNumeric:
				err = b.MergeSortedNumericField(mergeFieldInfo, mergeState)
			default:
				// Java throws AssertionError("type=" + type).
				err = fmt.Errorf("type=%v", typ)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// numericDocValuesSub tracks state of one numeric sub-reader that we are
// merging. Go rendering of the private static nested class
// DocValuesConsumer.NumericDocValuesSub (which extends DocIDMerger.Sub).
// NormsConsumer declares a nested class of the same simple name; see
// normsConsumerNumericDocValuesSub.
type numericDocValuesSub struct {
	docMap index.DocMap
	values spi.NumericDocValues
}

// MappedDocID returns the current docID mapped through the sub's DocMap.
func (s *numericDocValuesSub) MappedDocID() int {
	return s.docMap.Get(s.values.DocID())
}

// NextDoc mirrors NumericDocValuesSub.nextDoc().
func (s *numericDocValuesSub) NextDoc() (int, error) {
	return s.values.NextDoc()
}

// NextMappedDoc mirrors DocIDMerger.Sub.nextMappedDoc(): it skips documents
// the DocMap reports as deleted.
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

// MergeNumericField merges the numeric docvalues from mergeState.
//
// The default implementation calls AddNumericField, passing a
// DocValuesProducer that merges and filters deleted documents on the fly.
func (b *BaseDocValuesConsumer) MergeNumericField(mergeFieldInfo *spi.FieldInfo, mergeState *index.MergeState) error {
	return b.impl.AddNumericField(mergeFieldInfo, &mergeNumericFieldDocValuesProducer{
		mergeFieldInfo: mergeFieldInfo,
		mergeState:     mergeState,
	})
}

// mergeNumericFieldDocValuesProducer is the anonymous EmptyDocValuesProducer
// subclass built by mergeNumericField.
type mergeNumericFieldDocValuesProducer struct {
	index.EmptyDocValuesProducer
	mergeFieldInfo *spi.FieldInfo
	mergeState     *index.MergeState
}

// GetNumeric returns the merged numeric doc values.
func (p *mergeNumericFieldDocValuesProducer) GetNumeric(fieldInfo *spi.FieldInfo) (spi.NumericDocValues, error) {
	if fieldInfo != p.mergeFieldInfo {
		return nil, errors.New("wrong fieldInfo")
	}
	return GetMergedNumericDocValues(p.mergeState, p.mergeFieldInfo)
}

// GetMergeInstance returns the receiver (DocValuesProducer default).
func (p *mergeNumericFieldDocValuesProducer) GetMergeInstance() DocValuesProducer { return p }

// GetMergedNumericDocValues returns a merged numeric doc values instance from
// all producers in the provided merge state.
func GetMergedNumericDocValues(mergeState *index.MergeState, mergeFieldInfo *spi.FieldInfo) (spi.NumericDocValues, error) {
	var subs []*numericDocValuesSub
	for i := 0; i < len(mergeState.DocValuesProducers); i++ {
		var values spi.NumericDocValues
		docValuesProducer := mergeState.DocValuesProducers[i]
		if docValuesProducer != nil {
			readerFieldInfo := mergeState.FieldInfos[i].FieldInfo(mergeFieldInfo.Name())
			if readerFieldInfo != nil && readerFieldInfo.DocValuesType() == spi.DocValuesTypeNumeric {
				v, err := docValuesProducer.GetNumeric(readerFieldInfo)
				if err != nil {
					return nil, err
				}
				values = v
			}
		}
		if values != nil {
			subs = append(subs, &numericDocValuesSub{docMap: mergeState.DocMaps[i], values: values})
		}
	}
	return mergeNumericValues(subs, mergeState.NeedsIndexSort)
}

func mergeNumericValues(subs []*numericDocValuesSub, indexIsSorted bool) (spi.NumericDocValues, error) {
	var cost int64
	mergerSubs := make([]index.DocIDMergerSub, len(subs))
	for i, sub := range subs {
		cost += sub.values.Cost()
		mergerSubs[i] = sub
	}

	docIDMerger, err := index.NewDocIDMerger(mergerSubs, len(mergerSubs), indexIsSorted)
	if err != nil {
		return nil, err
	}

	return &mergedNumericDocValues{docID: -1, docIDMerger: docIDMerger, cost: cost}, nil
}

// mergedNumericDocValues is the anonymous NumericDocValues returned by
// mergeNumericValues.
type mergedNumericDocValues struct {
	docID       int
	current     *numericDocValuesSub
	docIDMerger index.DocIDMerger
	cost        int64
}

func (m *mergedNumericDocValues) DocID() int { return m.docID }

func (m *mergedNumericDocValues) NextDoc() (int, error) {
	sub, err := m.docIDMerger.Next()
	if err != nil {
		return 0, err
	}
	if sub == nil {
		m.current = nil
		m.docID = index.NO_MORE_DOCS
	} else {
		m.current = sub.(*numericDocValuesSub)
		m.docID = m.current.MappedDocID()
	}
	return m.docID, nil
}

func (m *mergedNumericDocValues) Advance(int) (int, error) {
	return 0, errDocValuesConsumerUnsupportedOperation
}

func (m *mergedNumericDocValues) AdvanceExact(int) (bool, error) {
	return false, errDocValuesConsumerUnsupportedOperation
}

func (m *mergedNumericDocValues) Cost() int64 { return m.cost }

func (m *mergedNumericDocValues) LongValue() (int64, error) {
	return m.current.values.LongValue()
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int), which the Java
// anonymous class does not override.
func (m *mergedNumericDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(m, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd().
func (m *mergedNumericDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(m)
}

// binaryDocValuesSub tracks state of one binary sub-reader that we are
// merging. Go rendering of DocValuesConsumer.BinaryDocValuesSub.
type binaryDocValuesSub struct {
	docMap index.DocMap
	values spi.BinaryDocValues
}

func (s *binaryDocValuesSub) MappedDocID() int {
	return s.docMap.Get(s.values.DocID())
}

func (s *binaryDocValuesSub) NextDoc() (int, error) {
	return s.values.NextDoc()
}

func (s *binaryDocValuesSub) NextMappedDoc() (int, error) {
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

// MergeBinaryField merges the binary docvalues from mergeState.
//
// The default implementation calls AddBinaryField, passing a
// DocValuesProducer that merges and filters deleted documents on the fly.
func (b *BaseDocValuesConsumer) MergeBinaryField(mergeFieldInfo *spi.FieldInfo, mergeState *index.MergeState) error {
	return b.impl.AddBinaryField(mergeFieldInfo, &mergeBinaryFieldDocValuesProducer{
		mergeFieldInfo: mergeFieldInfo,
		mergeState:     mergeState,
	})
}

// mergeBinaryFieldDocValuesProducer is the anonymous EmptyDocValuesProducer
// subclass built by mergeBinaryField.
type mergeBinaryFieldDocValuesProducer struct {
	index.EmptyDocValuesProducer
	mergeFieldInfo *spi.FieldInfo
	mergeState     *index.MergeState
}

// GetBinary returns the merged binary doc values.
func (p *mergeBinaryFieldDocValuesProducer) GetBinary(fieldInfo *spi.FieldInfo) (spi.BinaryDocValues, error) {
	if fieldInfo != p.mergeFieldInfo {
		return nil, errors.New("wrong fieldInfo")
	}
	return GetMergedBinaryDocValues(p.mergeFieldInfo, p.mergeState)
}

// GetMergeInstance returns the receiver (DocValuesProducer default).
func (p *mergeBinaryFieldDocValuesProducer) GetMergeInstance() DocValuesProducer { return p }

// GetMergedBinaryDocValues returns a merged binary doc values instance from
// all producers in the provided merge state.
func GetMergedBinaryDocValues(mergeFieldInfo *spi.FieldInfo, mergeState *index.MergeState) (spi.BinaryDocValues, error) {
	var subs []index.DocIDMergerSub

	var cost int64
	for i := 0; i < len(mergeState.DocValuesProducers); i++ {
		var values spi.BinaryDocValues
		docValuesProducer := mergeState.DocValuesProducers[i]
		if docValuesProducer != nil {
			readerFieldInfo := mergeState.FieldInfos[i].FieldInfo(mergeFieldInfo.Name())
			if readerFieldInfo != nil && readerFieldInfo.DocValuesType() == spi.DocValuesTypeBinary {
				v, err := docValuesProducer.GetBinary(readerFieldInfo)
				if err != nil {
					return nil, err
				}
				values = v
			}
		}
		if values != nil {
			cost += values.Cost()
			subs = append(subs, &binaryDocValuesSub{docMap: mergeState.DocMaps[i], values: values})
		}
	}

	docIDMerger, err := index.NewDocIDMerger(subs, len(subs), mergeState.NeedsIndexSort)
	if err != nil {
		return nil, err
	}

	return &mergedBinaryDocValues{docID: -1, docIDMerger: docIDMerger, cost: cost}, nil
}

// mergedBinaryDocValues is the anonymous BinaryDocValues returned by
// getMergedBinaryDocValues.
type mergedBinaryDocValues struct {
	current     *binaryDocValuesSub
	docID       int
	docIDMerger index.DocIDMerger
	cost        int64
}

func (m *mergedBinaryDocValues) DocID() int { return m.docID }

func (m *mergedBinaryDocValues) NextDoc() (int, error) {
	sub, err := m.docIDMerger.Next()
	if err != nil {
		return 0, err
	}
	if sub == nil {
		m.current = nil
		m.docID = index.NO_MORE_DOCS
	} else {
		m.current = sub.(*binaryDocValuesSub)
		m.docID = m.current.MappedDocID()
	}
	return m.docID, nil
}

func (m *mergedBinaryDocValues) Advance(int) (int, error) {
	return 0, errDocValuesConsumerUnsupportedOperation
}

func (m *mergedBinaryDocValues) AdvanceExact(int) (bool, error) {
	return false, errDocValuesConsumerUnsupportedOperation
}

func (m *mergedBinaryDocValues) Cost() int64 { return m.cost }

func (m *mergedBinaryDocValues) BinaryValue() ([]byte, error) {
	return m.current.values.BinaryValue()
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int), which the Java
// anonymous class does not override.
func (m *mergedBinaryDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(m, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd().
func (m *mergedBinaryDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(m)
}

// sortedNumericDocValuesSub tracks state of one sorted numeric sub-reader
// that we are merging. Go rendering of
// DocValuesConsumer.SortedNumericDocValuesSub.
type sortedNumericDocValuesSub struct {
	docMap index.DocMap
	values spi.SortedNumericDocValues
}

func (s *sortedNumericDocValuesSub) MappedDocID() int {
	return s.docMap.Get(s.values.DocID())
}

func (s *sortedNumericDocValuesSub) NextDoc() (int, error) {
	return s.values.NextDoc()
}

func (s *sortedNumericDocValuesSub) NextMappedDoc() (int, error) {
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

// MergeSortedNumericField merges the sorted numeric docvalues from
// mergeState.
//
// The default implementation calls AddSortedNumericField, passing iterables
// that filter deleted documents.
func (b *BaseDocValuesConsumer) MergeSortedNumericField(mergeFieldInfo *spi.FieldInfo, mergeState *index.MergeState) error {
	return b.impl.AddSortedNumericField(mergeFieldInfo, &mergeSortedNumericFieldDocValuesProducer{
		mergeFieldInfo: mergeFieldInfo,
		mergeState:     mergeState,
	})
}

// mergeSortedNumericFieldDocValuesProducer is the anonymous
// EmptyDocValuesProducer subclass built by mergeSortedNumericField.
type mergeSortedNumericFieldDocValuesProducer struct {
	index.EmptyDocValuesProducer
	mergeFieldInfo *spi.FieldInfo
	mergeState     *index.MergeState
}

// GetSortedNumeric returns the merged sorted numeric doc values.
func (p *mergeSortedNumericFieldDocValuesProducer) GetSortedNumeric(fieldInfo *spi.FieldInfo) (spi.SortedNumericDocValues, error) {
	if fieldInfo != p.mergeFieldInfo {
		return nil, errors.New("wrong FieldInfo")
	}
	return GetMergedSortedNumericDocValues(p.mergeFieldInfo, p.mergeState)
}

// GetMergeInstance returns the receiver (DocValuesProducer default).
func (p *mergeSortedNumericFieldDocValuesProducer) GetMergeInstance() DocValuesProducer { return p }

// GetMergedSortedNumericDocValues returns a merged sorted numeric doc values
// instance from all producers in the provided merge state.
func GetMergedSortedNumericDocValues(mergeFieldInfo *spi.FieldInfo, mergeState *index.MergeState) (spi.SortedNumericDocValues, error) {
	// We must make new iterators + DocIDMerger for each iterator:
	var subs []*sortedNumericDocValuesSub
	var cost int64
	allSingletons := true
	for i := 0; i < len(mergeState.DocValuesProducers); i++ {
		docValuesProducer := mergeState.DocValuesProducers[i]
		var values spi.SortedNumericDocValues
		if docValuesProducer != nil {
			readerFieldInfo := mergeState.FieldInfos[i].FieldInfo(mergeFieldInfo.Name())
			if readerFieldInfo != nil && readerFieldInfo.DocValuesType() == spi.DocValuesTypeSortedNumeric {
				v, err := docValuesProducer.GetSortedNumeric(readerFieldInfo)
				if err != nil {
					return nil, err
				}
				values = v
			}
		}
		if values == nil {
			values = index.EmptySortedNumeric()
		}
		cost += values.Cost()
		if allSingletons && index.UnwrapSingletonSortedNumeric(values) == nil {
			allSingletons = false
		}
		subs = append(subs, &sortedNumericDocValuesSub{docMap: mergeState.DocMaps[i], values: values})
	}

	if allSingletons {
		// All subs are single-valued.
		// We specialize for that case since it makes it easier for codecs to
		// optimize for single-valued fields.
		singleValuedSubs := make([]*numericDocValuesSub, 0, len(subs))
		for _, sub := range subs {
			singleValuedValues := index.UnwrapSingletonSortedNumeric(sub.values)
			singleValuedSubs = append(singleValuedSubs, &numericDocValuesSub{docMap: sub.docMap, values: singleValuedValues})
		}
		merged, err := mergeNumericValues(singleValuedSubs, mergeState.NeedsIndexSort)
		if err != nil {
			return nil, err
		}
		return index.Singleton(merged), nil
	}

	mergerSubs := make([]index.DocIDMergerSub, len(subs))
	for i, sub := range subs {
		mergerSubs[i] = sub
	}
	docIDMerger, err := index.NewDocIDMerger(mergerSubs, len(mergerSubs), mergeState.NeedsIndexSort)
	if err != nil {
		return nil, err
	}

	return &mergedSortedNumericDocValues{docID: -1, docIDMerger: docIDMerger, cost: cost}, nil
}

// mergedSortedNumericDocValues is the anonymous SortedNumericDocValues
// returned by getMergedSortedNumericDocValues.
type mergedSortedNumericDocValues struct {
	docID       int
	currentSub  *sortedNumericDocValuesSub
	docIDMerger index.DocIDMerger
	cost        int64
}

func (m *mergedSortedNumericDocValues) DocID() int { return m.docID }

func (m *mergedSortedNumericDocValues) NextDoc() (int, error) {
	sub, err := m.docIDMerger.Next()
	if err != nil {
		return 0, err
	}
	if sub == nil {
		m.currentSub = nil
		m.docID = index.NO_MORE_DOCS
	} else {
		m.currentSub = sub.(*sortedNumericDocValuesSub)
		m.docID = m.currentSub.MappedDocID()
	}
	return m.docID, nil
}

func (m *mergedSortedNumericDocValues) Advance(int) (int, error) {
	return 0, errDocValuesConsumerUnsupportedOperation
}

func (m *mergedSortedNumericDocValues) AdvanceExact(int) (bool, error) {
	return false, errDocValuesConsumerUnsupportedOperation
}

// DocValueCount mirrors docValueCount(). Java's returns int and does not
// throw; Gocene's returns (int, error), so the sub's error is propagated.
func (m *mergedSortedNumericDocValues) DocValueCount() (int, error) {
	return m.currentSub.values.DocValueCount()
}

func (m *mergedSortedNumericDocValues) Cost() int64 { return m.cost }

func (m *mergedSortedNumericDocValues) NextValue() (int64, error) {
	return m.currentSub.values.NextValue()
}

// LongValue satisfies the NumericDocValues surface spi.SortedNumericDocValues
// embeds (a Gocene divergence: Java's SortedNumericDocValues has no
// longValue()); it delegates to the current sub, like NextValue.
func (m *mergedSortedNumericDocValues) LongValue() (int64, error) {
	return m.currentSub.values.LongValue()
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int), which the Java
// anonymous class does not override.
func (m *mergedSortedNumericDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(m, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd().
func (m *mergedSortedNumericDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(m)
}

// mergedTermsEnum is a merged TermsEnum. This helps avoid relying on the
// default terms enum, which calls SortedDocValues.lookupOrd(int) or
// SortedSetDocValues.lookupOrd(long) on every call to TermsEnum.next(). Go
// rendering of the private static nested class
// DocValuesConsumer.MergedTermsEnum (which extends BaseTermsEnum).
type mergedTermsEnum struct {
	index.BaseTermsEnum

	subs       []spi.TermsEnum
	ordinalMap *index.OrdinalMap
	valueCount int64
	ord        int64
	term       *spi.Term
}

func newMergedTermsEnum(ordinalMap *index.OrdinalMap, subs []spi.TermsEnum) *mergedTermsEnum {
	return &mergedTermsEnum{
		ordinalMap: ordinalMap,
		subs:       subs,
		valueCount: ordinalMap.GetValueCount(),
		ord:        -1,
	}
}

// Term returns the current term.
func (e *mergedTermsEnum) Term() *spi.Term { return e.term }

// Ord returns the current global ordinal.
func (e *mergedTermsEnum) Ord() int64 { return e.ord }

// Next advances to the next global ordinal, positioning the first segment
// that contains it on the matching term.
func (e *mergedTermsEnum) Next() (*spi.Term, error) {
	e.ord++
	if e.ord >= e.valueCount {
		return nil, nil
	}
	subNum := e.ordinalMap.GetFirstSegmentNumber(e.ord)
	sub := e.subs[subNum]
	subOrd := e.ordinalMap.GetFirstSegmentOrd(e.ord)
	for {
		term, err := sub.Next()
		if err != nil {
			return nil, err
		}
		e.term = term
		if sub.Ord() >= subOrd {
			break
		}
	}
	return e.term, nil
}

// Attributes is unsupported. Java's attributes() throws
// UnsupportedOperationException; the Go signature carries no error, so the
// exception is rendered as a panic, as other unsupported error-less accessors
// in this module do.
func (e *mergedTermsEnum) Attributes() *util.AttributeSource {
	panic(errDocValuesConsumerUnsupportedOperation)
}

// SeekCeil is unsupported.
func (e *mergedTermsEnum) SeekCeil(*spi.Term) (*spi.Term, error) {
	return nil, errDocValuesConsumerUnsupportedOperation
}

// SeekExact carries the default body of BaseTermsEnum.seekExact(BytesRef),
// which delegates to seekCeil and therefore fails with the same unsupported
// error.
func (e *mergedTermsEnum) SeekExact(term *spi.Term) (bool, error) {
	return index.SeekExactDelegated(e, term)
}

// SeekExactOrd is unsupported (seekExact(long)).
func (e *mergedTermsEnum) SeekExactOrd(int64) error {
	return errDocValuesConsumerUnsupportedOperation
}

// DocFreq is unsupported.
func (e *mergedTermsEnum) DocFreq() (int, error) {
	return 0, errDocValuesConsumerUnsupportedOperation
}

// TotalTermFreq is unsupported.
func (e *mergedTermsEnum) TotalTermFreq() (int64, error) {
	return 0, errDocValuesConsumerUnsupportedOperation
}

// Postings is unsupported.
func (e *mergedTermsEnum) Postings(int) (spi.PostingsEnum, error) {
	return nil, errDocValuesConsumerUnsupportedOperation
}

// PostingsWithLiveDocs is unsupported, like Postings.
func (e *mergedTermsEnum) PostingsWithLiveDocs(util.Bits, int) (spi.PostingsEnum, error) {
	return nil, errDocValuesConsumerUnsupportedOperation
}

// Impacts is unsupported.
func (e *mergedTermsEnum) Impacts(int) (spi.ImpactsEnum, error) {
	return nil, errDocValuesConsumerUnsupportedOperation
}

// TermState is unsupported.
func (e *mergedTermsEnum) TermState() (index.TermState, error) {
	return nil, errDocValuesConsumerUnsupportedOperation
}

// sortedDocValuesSub tracks state of one sorted sub-reader that we are
// merging. Go rendering of DocValuesConsumer.SortedDocValuesSub; map is the
// segment-to-global ordinal mapping (LongValues in Java, a slice from
// OrdinalMap.GetGlobalOrds in Gocene).
type sortedDocValuesSub struct {
	docMap index.DocMap
	values spi.SortedDocValues
	m      []int64
}

func (s *sortedDocValuesSub) MappedDocID() int {
	return s.docMap.Get(s.values.DocID())
}

func (s *sortedDocValuesSub) NextDoc() (int, error) {
	return s.values.NextDoc()
}

func (s *sortedDocValuesSub) NextMappedDoc() (int, error) {
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

// MergeSortedField merges the sorted docvalues from mergeState.
//
// The default implementation calls AddSortedField, passing an Iterable that
// merges ordinals and values and filters deleted documents.
func (b *BaseDocValuesConsumer) MergeSortedField(fieldInfo *spi.FieldInfo, mergeState *index.MergeState) error {
	// step 1: iterate thru each sub and mark terms still in use
	// step 2: create ordinal map (this conceptually does the "merging")
	ordinalMap, err := CreateOrdinalMapForSortedDV(fieldInfo, mergeState)
	if err != nil {
		return err
	}

	// step 3: add field
	return b.impl.AddSortedField(fieldInfo, &mergeSortedFieldDocValuesProducer{
		fieldInfo:  fieldInfo,
		mergeState: mergeState,
		ordinalMap: ordinalMap,
	})
}

// mergeSortedFieldDocValuesProducer is the anonymous EmptyDocValuesProducer
// subclass built by mergeSortedField.
type mergeSortedFieldDocValuesProducer struct {
	index.EmptyDocValuesProducer
	fieldInfo  *spi.FieldInfo
	mergeState *index.MergeState
	ordinalMap *index.OrdinalMap
}

// GetSorted returns the merged sorted doc values.
func (p *mergeSortedFieldDocValuesProducer) GetSorted(fieldInfoIn *spi.FieldInfo) (spi.SortedDocValues, error) {
	if fieldInfoIn != p.fieldInfo {
		return nil, errors.New("wrong FieldInfo")
	}
	return GetMergedSortedSetDocValues(p.fieldInfo, p.mergeState, p.ordinalMap)
}

// GetMergeInstance returns the receiver (DocValuesProducer default).
func (p *mergeSortedFieldDocValuesProducer) GetMergeInstance() DocValuesProducer { return p }

// CreateOrdinalMapForSortedDV creates the ordinal map for a sorted field from
// all producers in the provided merge state, restricted to the terms still in
// use by live documents.
func CreateOrdinalMapForSortedDV(fieldInfo *spi.FieldInfo, mergeState *index.MergeState) (*index.OrdinalMap, error) {
	var toMerge []spi.SortedDocValues
	for i := 0; i < len(mergeState.DocValuesProducers); i++ {
		var values spi.SortedDocValues
		docValuesProducer := mergeState.DocValuesProducers[i]
		if docValuesProducer != nil {
			readerFieldInfo := mergeState.FieldInfos[i].FieldInfo(fieldInfo.Name())
			if readerFieldInfo != nil && readerFieldInfo.DocValuesType() == spi.DocValuesTypeSorted {
				v, err := docValuesProducer.GetSorted(readerFieldInfo)
				if err != nil {
					return nil, err
				}
				values = v
			}
		}
		if values == nil {
			values = index.EmptySorted()
		}
		toMerge = append(toMerge, values)
	}

	numReaders := len(toMerge)
	dvs := toMerge

	liveTerms := make([]spi.TermsEnum, len(dvs))
	weights := make([]int64, len(liveTerms))
	for sub := 0; sub < numReaders; sub++ {
		dv := dvs[sub]
		liveDocs := mergeState.LiveDocs[sub]
		if liveDocs == nil {
			termsEnum, err := sortedDocValuesTermsEnum(dv)
			if err != nil {
				return nil, err
			}
			liveTerms[sub] = termsEnum
			weights[sub] = int64(dv.GetValueCount())
		} else {
			bitset, err := util.NewLongBitSet(int64(dv.GetValueCount()))
			if err != nil {
				return nil, err
			}
			for {
				docID, err := dv.NextDoc()
				if err != nil {
					return nil, err
				}
				if docID == index.NO_MORE_DOCS {
					break
				}
				if liveDocs.Get(docID) {
					ord, err := dv.OrdValue()
					if err != nil {
						return nil, err
					}
					if ord >= 0 {
						bitset.Set(int64(ord))
					}
				}
			}
			termsEnum, err := sortedDocValuesTermsEnum(dv)
			if err != nil {
				return nil, err
			}
			liveTerms[sub] = newBitsFilteredTermsEnum(termsEnum, bitset)
			weights[sub] = bitset.Cardinality()
		}
	}

	return index.BuildOrdinalMap(nil, liveTerms, weights, packed.Compact)
}

// GetMergedSortedSetDocValues returns a merged sorted doc values instance from
// all producers in the provided merge state. Go rendering of the three-argument
// getMergedSortedSetDocValues(FieldInfo, MergeState, OrdinalMap), which returns
// SortedDocValues; the four-argument overload is
// [GetMergedSortedSetDocValuesToMerge].
func GetMergedSortedSetDocValues(fieldInfo *spi.FieldInfo, mergeState *index.MergeState, ordinalMap *index.OrdinalMap) (spi.SortedDocValues, error) {
	// We must make new iterators + DocIDMerger for each iterator:
	var subs []*sortedDocValuesSub
	for i := 0; i < len(mergeState.DocValuesProducers); i++ {
		var values spi.SortedDocValues
		docValuesProducer := mergeState.DocValuesProducers[i]
		if docValuesProducer != nil {
			readerFieldInfo := mergeState.FieldInfos[i].FieldInfo(fieldInfo.Name())
			if readerFieldInfo != nil && readerFieldInfo.DocValuesType() == spi.DocValuesTypeSorted {
				v, err := docValuesProducer.GetSorted(readerFieldInfo)
				if err != nil {
					return nil, err
				}
				values = v
			}
		}
		if values == nil {
			values = index.EmptySorted()
		}

		subs = append(subs, &sortedDocValuesSub{docMap: mergeState.DocMaps[i], values: values, m: ordinalMap.GetGlobalOrds(i)})
	}
	return mergeSortedValues(subs, mergeState, ordinalMap)
}

// mergeSortedValues returns a merged sorted doc values instance from the given
// subs. It is protected static in Java but takes the private nested
// SortedDocValuesSub, so no subclass outside DocValuesConsumer can call it; the
// Go rendering is unexported for the same reason.
func mergeSortedValues(subs []*sortedDocValuesSub, mergeState *index.MergeState, ordinalMap *index.OrdinalMap) (spi.SortedDocValues, error) {
	var cost int64
	mergerSubs := make([]index.DocIDMergerSub, len(subs))
	for i, sub := range subs {
		cost += sub.values.Cost()
		mergerSubs[i] = sub
	}

	docIDMerger, err := index.NewDocIDMerger(mergerSubs, len(mergerSubs), mergeState.NeedsIndexSort)
	if err != nil {
		return nil, err
	}

	return &mergedSortedDocValues{
		docID:       -1,
		docIDMerger: docIDMerger,
		cost:        cost,
		subs:        subs,
		ordinalMap:  ordinalMap,
	}, nil
}

// mergedSortedDocValues is the anonymous SortedDocValues returned by
// mergeSortedValues.
type mergedSortedDocValues struct {
	docID       int
	current     *sortedDocValuesSub
	docIDMerger index.DocIDMerger
	cost        int64
	subs        []*sortedDocValuesSub
	ordinalMap  *index.OrdinalMap
}

func (m *mergedSortedDocValues) DocID() int { return m.docID }

func (m *mergedSortedDocValues) NextDoc() (int, error) {
	sub, err := m.docIDMerger.Next()
	if err != nil {
		return 0, err
	}
	if sub == nil {
		m.current = nil
		m.docID = index.NO_MORE_DOCS
	} else {
		m.current = sub.(*sortedDocValuesSub)
		m.docID = m.current.MappedDocID()
	}
	return m.docID, nil
}

func (m *mergedSortedDocValues) OrdValue() (int, error) {
	subOrd, err := m.current.values.OrdValue()
	if err != nil {
		return 0, err
	}
	if subOrd < 0 || subOrd >= len(m.current.m) {
		// Java asserts subOrd != -1 and LongValues.get fails out of range.
		return 0, fmt.Errorf("merged sorted doc values: segment ord %d out of range [0, %d)", subOrd, len(m.current.m))
	}
	return int(m.current.m[subOrd]), nil
}

// LongValue satisfies the NumericDocValues surface spi.SortedDocValues embeds
// (a Gocene divergence: Java's SortedDocValues has no longValue()); it reports
// the ordinal, as the other SortedDocValues views of this module do.
func (m *mergedSortedDocValues) LongValue() (int64, error) {
	ord, err := m.OrdValue()
	if err != nil {
		return 0, err
	}
	return int64(ord), nil
}

func (m *mergedSortedDocValues) Advance(int) (int, error) {
	return 0, errDocValuesConsumerUnsupportedOperation
}

func (m *mergedSortedDocValues) AdvanceExact(int) (bool, error) {
	return false, errDocValuesConsumerUnsupportedOperation
}

func (m *mergedSortedDocValues) Cost() int64 { return m.cost }

func (m *mergedSortedDocValues) GetValueCount() int {
	return int(m.ordinalMap.GetValueCount())
}

func (m *mergedSortedDocValues) LookupOrd(ord int) ([]byte, error) {
	segmentNumber := m.ordinalMap.GetFirstSegmentNumber(int64(ord))
	segmentOrd := int(m.ordinalMap.GetFirstSegmentOrd(int64(ord)))
	return m.subs[segmentNumber].values.LookupOrd(segmentOrd)
}

// TermsEnum returns a MergedTermsEnum over the subs' terms enums.
func (m *mergedSortedDocValues) TermsEnum() (spi.TermsEnum, error) {
	termsEnurmSubs := make([]spi.TermsEnum, len(m.subs))
	for sub := 0; sub < len(termsEnurmSubs); sub++ {
		termsEnum, err := sortedDocValuesTermsEnum(m.subs[sub].values)
		if err != nil {
			return nil, err
		}
		termsEnurmSubs[sub] = termsEnum
	}
	return newMergedTermsEnum(m.ordinalMap, termsEnurmSubs), nil
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int), which the Java
// anonymous class does not override.
func (m *mergedSortedDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(m, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd().
func (m *mergedSortedDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(m)
}

// sortedSetDocValuesSub tracks state of one sorted set sub-reader that we are
// merging. Go rendering of DocValuesConsumer.SortedSetDocValuesSub.
type sortedSetDocValuesSub struct {
	docMap index.DocMap
	values spi.SortedSetDocValues
	m      []int64
}

func (s *sortedSetDocValuesSub) MappedDocID() int {
	return s.docMap.Get(s.values.DocID())
}

func (s *sortedSetDocValuesSub) NextDoc() (int, error) {
	return s.values.NextDoc()
}

func (s *sortedSetDocValuesSub) NextMappedDoc() (int, error) {
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

// String mirrors SortedSetDocValuesSub.toString().
func (s *sortedSetDocValuesSub) String() string {
	return fmt.Sprintf("SortedSetDocValuesSub(mappedDocID=%d values=%v)", s.MappedDocID(), s.values)
}

// MergeSortedSetField merges the sortedset docvalues from mergeState.
//
// The default implementation calls AddSortedSetField, passing an Iterable
// that merges ordinals and values and filters deleted documents.
func (b *BaseDocValuesConsumer) MergeSortedSetField(mergeFieldInfo *spi.FieldInfo, mergeState *index.MergeState) error {
	// step 1: iterate thru each sub and mark terms still in use
	// step 2: create ordinal map (this conceptually does the "merging")
	toMerge, err := SelectLeavesToMerge(mergeFieldInfo, mergeState)
	if err != nil {
		return err
	}
	ordinalMap, err := CreateOrdinalMapForSortedSetDV(toMerge, mergeState)
	if err != nil {
		return err
	}
	// step 3: add field
	return b.impl.AddSortedSetField(mergeFieldInfo, &mergeSortedSetFieldDocValuesProducer{
		mergeFieldInfo: mergeFieldInfo,
		mergeState:     mergeState,
		ordinalMap:     ordinalMap,
		toMerge:        toMerge,
	})
}

// mergeSortedSetFieldDocValuesProducer is the anonymous
// EmptyDocValuesProducer subclass built by mergeSortedSetField.
type mergeSortedSetFieldDocValuesProducer struct {
	index.EmptyDocValuesProducer
	mergeFieldInfo *spi.FieldInfo
	mergeState     *index.MergeState
	ordinalMap     *index.OrdinalMap
	toMerge        []spi.SortedSetDocValues
}

// GetSortedSet returns the merged sorted set doc values.
func (p *mergeSortedSetFieldDocValuesProducer) GetSortedSet(fieldInfo *spi.FieldInfo) (spi.SortedSetDocValues, error) {
	if fieldInfo != p.mergeFieldInfo {
		return nil, errors.New("wrong FieldInfo")
	}
	return GetMergedSortedSetDocValuesToMerge(p.mergeFieldInfo, p.mergeState, p.ordinalMap, p.toMerge)
}

// GetMergeInstance returns the receiver (DocValuesProducer default).
func (p *mergeSortedSetFieldDocValuesProducer) GetMergeInstance() DocValuesProducer { return p }

// CreateOrdinalMapForSortedSetDV creates an ordinal map based on the provided
// sorted set doc values to merge.
func CreateOrdinalMapForSortedSetDV(toMerge []spi.SortedSetDocValues, mergeState *index.MergeState) (*index.OrdinalMap, error) {
	liveTerms := make([]spi.TermsEnum, len(toMerge))
	weights := make([]int64, len(liveTerms))
	for sub := 0; sub < len(liveTerms); sub++ {
		dv := toMerge[sub]
		liveDocs := mergeState.LiveDocs[sub]
		if liveDocs == nil {
			termsEnum, err := sortedSetDocValuesTermsEnum(dv)
			if err != nil {
				return nil, err
			}
			liveTerms[sub] = termsEnum
			weights[sub] = int64(dv.GetValueCount())
		} else {
			bitset, err := util.NewLongBitSet(int64(dv.GetValueCount()))
			if err != nil {
				return nil, err
			}
			for {
				docID, err := dv.NextDoc()
				if err != nil {
					return nil, err
				}
				if docID == index.NO_MORE_DOCS {
					break
				}
				if liveDocs.Get(docID) {
					for i := 0; i < dv.DocValueCount(); i++ {
						ord, err := dv.NextOrd()
						if err != nil {
							return nil, err
						}
						bitset.Set(int64(ord))
					}
				}
			}
			termsEnum, err := sortedSetDocValuesTermsEnum(dv)
			if err != nil {
				return nil, err
			}
			liveTerms[sub] = newBitsFilteredTermsEnum(termsEnum, bitset)
			weights[sub] = bitset.Cardinality()
		}
	}

	return index.BuildOrdinalMap(nil, liveTerms, weights, packed.Compact)
}

// SelectLeavesToMerge selects the sorted set doc values to merge.
func SelectLeavesToMerge(mergeFieldInfo *spi.FieldInfo, mergeState *index.MergeState) ([]spi.SortedSetDocValues, error) {
	var toMerge []spi.SortedSetDocValues
	for i := 0; i < len(mergeState.DocValuesProducers); i++ {
		var values spi.SortedSetDocValues
		docValuesProducer := mergeState.DocValuesProducers[i]
		if docValuesProducer != nil {
			fieldInfo := mergeState.FieldInfos[i].FieldInfo(mergeFieldInfo.Name())
			if fieldInfo != nil && fieldInfo.DocValuesType() == spi.DocValuesTypeSortedSet {
				v, err := docValuesProducer.GetSortedSet(fieldInfo)
				if err != nil {
					return nil, err
				}
				values = v
			}
		}
		if values == nil {
			values = index.EmptySortedSet()
		}
		toMerge = append(toMerge, values)
	}
	return toMerge, nil
}

// GetMergedSortedSetDocValuesToMerge returns a sorted set doc values instance
// from all producers in the provided merge state. Go rendering of the
// four-argument getMergedSortedSetDocValues(FieldInfo, MergeState, OrdinalMap,
// List<SortedSetDocValues>); the three-argument overload is
// [GetMergedSortedSetDocValues].
func GetMergedSortedSetDocValuesToMerge(
	mergeFieldInfo *spi.FieldInfo,
	mergeState *index.MergeState,
	ordinalMap *index.OrdinalMap,
	toMerge []spi.SortedSetDocValues,
) (spi.SortedSetDocValues, error) {
	// We must make new iterators + DocIDMerger for each iterator:
	var subs []*sortedSetDocValuesSub

	var cost int64
	allSingletons := true

	for i := 0; i < len(mergeState.DocValuesProducers); i++ {
		var values spi.SortedSetDocValues
		docValuesProducer := mergeState.DocValuesProducers[i]
		if docValuesProducer != nil {
			readerFieldInfo := mergeState.FieldInfos[i].FieldInfo(mergeFieldInfo.Name())
			if readerFieldInfo != nil && readerFieldInfo.DocValuesType() == spi.DocValuesTypeSortedSet {
				v, err := docValuesProducer.GetSortedSet(readerFieldInfo)
				if err != nil {
					return nil, err
				}
				values = v
			}
		}
		if values == nil {
			values = index.EmptySortedSet()
		}
		cost += values.Cost()
		if allSingletons && index.UnwrapSingletonSortedSet(values) == nil {
			allSingletons = false
		}
		subs = append(subs, &sortedSetDocValuesSub{docMap: mergeState.DocMaps[i], values: values, m: ordinalMap.GetGlobalOrds(i)})
	}

	if allSingletons {
		// All subs are single-valued.
		// We specialize for that case since it makes it easier for codecs to
		// optimize for single-valued fields.
		singleValuedSubs := make([]*sortedDocValuesSub, 0, len(subs))
		for _, sub := range subs {
			singleValuedValues := index.UnwrapSingletonSortedSet(sub.values)
			singleValuedSubs = append(singleValuedSubs, &sortedDocValuesSub{docMap: sub.docMap, values: singleValuedValues, m: sub.m})
		}
		merged, err := mergeSortedValues(singleValuedSubs, mergeState, ordinalMap)
		if err != nil {
			return nil, err
		}
		return index.SingletonSortedSet(merged), nil
	}

	mergerSubs := make([]index.DocIDMergerSub, len(subs))
	for i, sub := range subs {
		mergerSubs[i] = sub
	}
	docIDMerger, err := index.NewDocIDMerger(mergerSubs, len(mergerSubs), mergeState.NeedsIndexSort)
	if err != nil {
		return nil, err
	}

	return &mergedSortedSetDocValues{
		docID:       -1,
		docIDMerger: docIDMerger,
		cost:        cost,
		ordinalMap:  ordinalMap,
		toMerge:     toMerge,
	}, nil
}

// mergedSortedSetDocValues is the anonymous SortedSetDocValues returned by
// the four-argument getMergedSortedSetDocValues.
type mergedSortedSetDocValues struct {
	docID       int
	currentSub  *sortedSetDocValuesSub
	docIDMerger index.DocIDMerger
	cost        int64
	ordinalMap  *index.OrdinalMap
	toMerge     []spi.SortedSetDocValues
}

func (m *mergedSortedSetDocValues) DocID() int { return m.docID }

func (m *mergedSortedSetDocValues) NextDoc() (int, error) {
	sub, err := m.docIDMerger.Next()
	if err != nil {
		return 0, err
	}
	if sub == nil {
		m.currentSub = nil
		m.docID = index.NO_MORE_DOCS
	} else {
		m.currentSub = sub.(*sortedSetDocValuesSub)
		m.docID = m.currentSub.MappedDocID()
	}
	return m.docID, nil
}

func (m *mergedSortedSetDocValues) Advance(int) (int, error) {
	return 0, errDocValuesConsumerUnsupportedOperation
}

func (m *mergedSortedSetDocValues) AdvanceExact(int) (bool, error) {
	return false, errDocValuesConsumerUnsupportedOperation
}

// NextOrd maps the current sub's next ordinal to the global ordinal space.
// A negative sub ordinal is the end-of-ordinals sentinel of the
// spi.SortedSetDocValues.NextOrd contract and is passed through unmapped.
func (m *mergedSortedSetDocValues) NextOrd() (int, error) {
	subOrd, err := m.currentSub.values.NextOrd()
	if err != nil {
		return 0, err
	}
	if subOrd < 0 {
		return subOrd, nil
	}
	if subOrd >= len(m.currentSub.m) {
		return 0, fmt.Errorf("merged sorted set doc values: segment ord %d out of range [0, %d)", subOrd, len(m.currentSub.m))
	}
	return int(m.currentSub.m[subOrd]), nil
}

func (m *mergedSortedSetDocValues) DocValueCount() int {
	return m.currentSub.values.DocValueCount()
}

func (m *mergedSortedSetDocValues) Cost() int64 { return m.cost }

func (m *mergedSortedSetDocValues) LookupOrd(ord int) ([]byte, error) {
	segmentNumber := m.ordinalMap.GetFirstSegmentNumber(int64(ord))
	segmentOrd := m.ordinalMap.GetFirstSegmentOrd(int64(ord))
	return m.toMerge[segmentNumber].LookupOrd(int(segmentOrd))
}

func (m *mergedSortedSetDocValues) GetValueCount() int {
	return int(m.ordinalMap.GetValueCount())
}

// TermsEnum returns a MergedTermsEnum over the toMerge terms enums.
func (m *mergedSortedSetDocValues) TermsEnum() (spi.TermsEnum, error) {
	subs := make([]spi.TermsEnum, len(m.toMerge))
	for sub := 0; sub < len(subs); sub++ {
		termsEnum, err := sortedSetDocValuesTermsEnum(m.toMerge[sub])
		if err != nil {
			return nil, err
		}
		subs[sub] = termsEnum
	}
	return newMergedTermsEnum(m.ordinalMap, subs), nil
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int), which the Java
// anonymous class does not override.
func (m *mergedSortedSetDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(m, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd().
func (m *mergedSortedSetDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(m)
}

// bitsFilteredTermsEnum is the Go rendering of the package-private static
// nested class DocValuesConsumer.BitsFilteredTermsEnum (which extends
// FilteredTermsEnum): it accepts only the terms whose ordinal is set in
// liveTerms.
type bitsFilteredTermsEnum struct {
	*index.FilteredTermsEnum
	liveTerms *util.LongBitSet
}

func newBitsFilteredTermsEnum(in spi.TermsEnum, liveTerms *util.LongBitSet) *bitsFilteredTermsEnum {
	e := &bitsFilteredTermsEnum{liveTerms: liveTerms}
	// super(in, false): not passing false here wasted about 3 hours of the
	// original author's time.
	e.FilteredTermsEnum = index.NewFilteredTermsEnumWithSeek(in, e, false)
	return e
}

// Accept mirrors BitsFilteredTermsEnum.accept(BytesRef).
func (e *bitsFilteredTermsEnum) Accept(*index.Term) (index.AcceptStatus, error) {
	if e.liveTerms.Get(e.Ord()) {
		return index.AcceptYes, nil
	}
	return index.AcceptNo, nil
}

// NextSeekTerm does not override FilteredTermsEnum.nextSeekTerm: returning
// nil hands control to the inherited default.
func (e *bitsFilteredTermsEnum) NextSeekTerm(*index.Term) (*index.Term, error) {
	return nil, nil
}

// sortedDocValuesTermsEnum renders the virtual call SortedDocValues.termsEnum():
// the implementation's own override when it declares one, otherwise the
// default body, new SortedDocValuesTermsEnum(this).
func sortedDocValuesTermsEnum(values spi.SortedDocValues) (spi.TermsEnum, error) {
	if override, ok := values.(interface {
		TermsEnum() (spi.TermsEnum, error)
	}); ok {
		return override.TermsEnum()
	}
	return index.OpenTermsEnum("", values)
}

// sortedSetDocValuesTermsEnum renders the virtual call
// SortedSetDocValues.termsEnum(): the implementation's own override when it
// declares one, otherwise the default body, new
// SortedSetDocValuesTermsEnum(this).
func sortedSetDocValuesTermsEnum(values spi.SortedSetDocValues) (spi.TermsEnum, error) {
	if override, ok := values.(interface {
		TermsEnum() (spi.TermsEnum, error)
	}); ok {
		return override.TermsEnum()
	}
	return index.OpenSetTermsEnum("", values)
}

// IsSingleValued returns true if the given docToValue count contains only at
// most one value. docToValueCount renders Iterable<Number>: every call returns
// a fresh iterator.
func IsSingleValued(docToValueCount func() util.Iterator[int64]) bool {
	for it := docToValueCount(); it.HasNext(); {
		if it.Next() > 1 {
			return false
		}
	}
	return true
}

// SingletonView returns a single-valued view, using missingValue when count is
// zero. docToValueCount and values render Iterable<Number>; so does the result.
func SingletonView(docToValueCount, values func() util.Iterator[int64], missingValue int64) func() util.Iterator[int64] {
	return func() util.Iterator[int64] {
		return &singletonViewIterator{
			countIterator:  docToValueCount(),
			valuesIterator: values(),
			missingValue:   missingValue,
		}
	}
}

// singletonViewIterator is the anonymous Iterator<Number> built by
// singletonView.
type singletonViewIterator struct {
	countIterator  util.Iterator[int64]
	valuesIterator util.Iterator[int64]
	missingValue   int64
}

func (it *singletonViewIterator) HasNext() bool {
	return it.countIterator.HasNext()
}

func (it *singletonViewIterator) Next() int64 {
	// Java: int count = countIterator.next().intValue();
	count := int32(it.countIterator.Next())
	if count == 0 {
		return it.missingValue
	}
	return it.valuesIterator.Next()
}
