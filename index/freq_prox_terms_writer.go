// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// FreqProxTermsWriter is the segment-flush entry point for the freq/prox
// inversion pipeline. It is the Go port of
// org.apache.lucene.index.FreqProxTermsWriter from Apache Lucene 10.4.0.
//
// The Lucene type is package-private and extends TermsHash (a shared root
// that aggregates pool ownership across the inversion chain). Gocene has not
// yet ported the TermsHash root (tracked under the indexing-pipeline backlog),
// so this writer carries the four pool handles directly through a
// [FreqProxTermsHash] value and dispatches to the next-in-chain handler via
// the embedded NextTermsHash pointer. The construction order mirrors Lucene's:
// FreqProxTermsWriter is built first, then wraps the term-vectors writer.
//
// Divergences from Lucene 10.4.0:
//
//   - No TermsHash root: pool ownership and the BytesUsed counter are carried
//     on the writer itself. Flush therefore does not delegate to a
//     super.flush(); the next-in-chain writer flushes through NextTermsHash.
//
//   - applyDeletes is omitted: Gocene's [SegmentWriteState] does not yet
//     carry segUpdates / liveDocs / delCountOnFlush. The deletion-application
//     pass is deferred to a follow-up task that lands those fields on
//     SegmentWriteState. See the BUFFERED_UPDATES_FLUSH_TODO note in the
//     Flush method.
//
//   - FieldsConsumer dispatch: Gocene's [FieldsConsumer.Write] takes
//     (field, terms) per-field rather than Lucene's write(Fields, NormsProducer).
//     Flush iterates the prepared FreqProxFields (already sorted by name) and
//     invokes Write field-by-field. The norms argument is held for parity but
//     is not propagated until the FieldsConsumer surface accepts it.
//
//   - Sorter.DocMap is exposed locally as the [SorterDocMap] interface
//     (OldToNew + Size). Lucene's Sorter.DocMap subclass is not ported; the
//     identity-mapping case (sortMap == nil) goes through unchanged.
//
//   - The codec lookup goes through SegmentInfo.Codec (a string), so callers
//     must supply the PostingsFormat explicitly through Flush's
//     postingsFormat parameter. Lucene resolves this internally via
//     segmentInfo.getCodec().postingsFormat().
type FreqProxTermsWriter struct {
	// IntPool, BytePool, TermBytePool and BytesUsed are the shared pools
	// passed down to per-field handlers; see [FreqProxTermsHash].
	pools FreqProxTermsHash

	// NextTermsHash is the next-in-chain inversion handler (typically the
	// term-vectors writer). It may be nil when no further consumer is wired,
	// in which case AddField produces a per-field writer with no downstream
	// handler.
	//
	// The contract on NextTermsHash mirrors Lucene's TermsHash.nextTermsHash:
	// the AddField hook is invoked with the same FieldInvertState and
	// FieldInfo and the returned *TermsHashPerField is wired as the
	// downstream NextPerField on the FreqProx per-field writer.
	NextTermsHash FreqProxNextHandler

	// wrappers maps the embedded *TermsHashPerField of each per-field writer
	// back to its *FreqProxTermsWriterPerField wrapper. This registry is
	// necessary because Flush receives a map[string]*TermsHashPerField (keyed
	// by field name) but needs the FreqProx wrapper for each entry. In Java
	// this is resolved by a direct downcast; Go requires an explicit registry.
	// Populated by AddField.
	wrappers map[*TermsHashPerField]*FreqProxTermsWriterPerField
}

// FreqProxNextHandler is the contract satisfied by the next-in-chain
// inversion handler used by [FreqProxTermsWriter.AddField]. The single method
// mirrors Lucene's TermsHash.addField(invertState, fieldInfo) hook.
//
// The handler is invoked once per inverted field and returns the downstream
// per-field writer that the FreqProx writer wires as its NextPerField.
// Callers that do not need a secondary handler may leave [FreqProxTermsWriter.NextTermsHash]
// as nil.
type FreqProxNextHandler interface {
	// AddField returns the downstream per-field handler for the supplied
	// inversion state and field. Implementations must not retain references
	// to invertState beyond the lifetime of the returned per-field handler.
	AddField(invertState *FieldInvertState, fieldInfo *FieldInfo) (*TermsHashPerField, error)

	// Flush is invoked at segment flush time with the buffered per-field
	// handlers keyed by field name. The implementation flushes its own
	// stream (e.g. term vectors). sortMap is the active document re-mapping,
	// or nil for the identity mapping.
	Flush(fieldsToFlush map[string]*TermsHashPerField, state *SegmentWriteState, sortMap SorterDocMap) error
}

// The SorterDocMap consumed by Flush / SortingTerms is the package-level
// type declared in sorter.go; passing nil represents the identity mapping.

// NewFreqProxTermsWriter wires a writer over the supplied pool bundle and
// optional next-in-chain handler. The pool bundle is forwarded to each
// per-field writer; the bundle's IntPool, BytePool and TermBytePool must be
// non-nil. A nil BytesUsed counter is replaced by a fresh
// [util.NewCounter()] (this mirrors Lucene's Counter.newCounter() default).
func NewFreqProxTermsWriter(pools FreqProxTermsHash, nextTermsHash FreqProxNextHandler) (*FreqProxTermsWriter, error) {
	if pools.IntPool == nil || pools.BytePool == nil || pools.TermBytePool == nil {
		return nil, errors.New("FreqProxTermsWriter: pools must not be nil")
	}
	if pools.BytesUsed == nil {
		pools.BytesUsed = util.NewCounter()
	}
	return &FreqProxTermsWriter{
		pools:         pools,
		NextTermsHash: nextTermsHash,
	}, nil
}

// AddField returns a fresh [FreqProxTermsWriterPerField] wired into this
// writer's pool bundle and (optionally) into the next-in-chain handler.
// Mirrors Lucene's FreqProxTermsWriter.addField.
//
// invertState and fieldInfo must not be nil; the returned writer owns its
// embedded *TermsHashPerField, so callers should hold on to the returned
// FreqProx pointer rather than re-wrapping it.
func (w *FreqProxTermsWriter) AddField(invertState *FieldInvertState, fieldInfo *FieldInfo) (*FreqProxTermsWriterPerField, error) {
	if invertState == nil {
		return nil, errors.New("FreqProxTermsWriter.AddField: invertState must not be nil")
	}
	if fieldInfo == nil {
		return nil, errors.New("FreqProxTermsWriter.AddField: fieldInfo must not be nil")
	}

	var next *TermsHashPerField
	if w.NextTermsHash != nil {
		downstream, err := w.NextTermsHash.AddField(invertState, fieldInfo)
		if err != nil {
			return nil, fmt.Errorf("FreqProxTermsWriter.AddField: next-in-chain: %w", err)
		}
		next = downstream
	}

	// Attribute getters are owned by the indexing-pipeline glue; AddField
	// surfaces a zero-valued provider so callers that have not yet wired the
	// token-stream bridge can still construct a writer. Pipelines that index
	// real tokens must replace the provider via the constructor.
	pf, err := NewFreqProxTermsWriterPerField(invertState, w.pools, fieldInfo, next, FreqProxAttributeProvider{})
	if err != nil {
		return nil, err
	}
	// Register the wrapper so lookupFreqProxByBase can resolve it during Flush.
	if w.wrappers == nil {
		w.wrappers = make(map[*TermsHashPerField]*FreqProxTermsWriterPerField)
	}
	w.wrappers[pf.TermsHashPerField] = pf
	return pf, nil
}

// Flush is the segment-flush entry point. fieldsToFlush is keyed by field
// name and carries every per-field handler produced by [AddField] for the
// current segment. state holds the segment metadata; sortMap is the active
// document re-mapping (nil for the identity mapping). postingsFormat is the
// codec dispatch the caller resolves from state.SegmentInfo.Codec; norms is
// held for parity with Lucene's signature even though Gocene's
// [FieldsConsumer] does not yet accept a norms producer.
//
// The flow mirrors Lucene's FreqProxTermsWriter.flush:
//  1. Cast the buffered TermsHashPerField map to FreqProxTermsWriterPerField,
//     drop empty fields, and sort the survivors by field name (insertion
//     order in Lucene happens to coincide with field-name order because
//     CollectionUtil.introSort uses Comparable<FreqProxTermsWriterPerField>
//     and FreqProxTermsWriterPerField's compareTo defers to TermsHashPerField,
//     which is field-name-comparable).
//  2. If no field reports HasPostings, return early.
//  3. Build a FreqProxFields view, optionally wrap it in a sort-aware
//     FilterFields, and hand it to the active FieldsConsumer.
//
// Differences from Lucene:
//   - applyDeletes is skipped (BUFFERED_UPDATES_FLUSH_TODO).
//   - super.flush() is not invoked (no TermsHash root). NextTermsHash's
//     Flush is called instead with the same buffered map.
//   - The FieldsConsumer dispatch iterates field-by-field rather than
//     consumer.write(fields, norms).
func (w *FreqProxTermsWriter) Flush(
	fieldsToFlush map[string]*TermsHashPerField,
	state *SegmentWriteState,
	sortMap SorterDocMap,
	postingsFormat PostingsFormat,
	norms any, // forward-compat placeholder; the in-tree FieldsConsumer ignores it.
) error {
	if state == nil {
		return errors.New("FreqProxTermsWriter.Flush: state must not be nil")
	}
	if postingsFormat == nil {
		return errors.New("FreqProxTermsWriter.Flush: postingsFormat must not be nil")
	}

	// Step 1: gather active fields.
	allFields := make([]*FreqProxTermsWriterPerField, 0, len(fieldsToFlush))
	for _, base := range fieldsToFlush {
		if base == nil {
			continue
		}
		perField, ok := w.lookupFreqProxByBase(base)
		if !ok {
			// No FreqProx wrapper registered for this base handler (e.g. the
			// field was inserted without going through AddField). Skip it
			// rather than crash; pipelines that mix bare handlers with FreqProx
			// wrappers can still flush the FreqProx subset.
			continue
		}
		if perField.GetNumTerms() == 0 {
			continue
		}
		perField.SortTerms()
		if perField.fieldInfo.IndexOptions() == IndexOptionsNone {
			return fmt.Errorf("FreqProxTermsWriter.Flush: field %q has IndexOptionsNone but produced terms",
				perField.GetFieldName())
		}
		allFields = append(allFields, perField)
	}

	// Step 2: short-circuit when no postings exist for this segment.
	if !state.FieldInfos.HasPostings() {
		if len(allFields) != 0 {
			return fmt.Errorf("FreqProxTermsWriter.Flush: FieldInfos reports no postings but %d fields buffered terms",
				len(allFields))
		}
		// Fall through to next-in-chain so term vectors still flush even
		// when no field indexes postings.
	} else {
		sort.Slice(allFields, func(i, j int) bool {
			return allFields[i].CompareTo(allFields[j].TermsHashPerField) < 0
		})

		// Step 3: build the Fields view.
		var fields Fields = NewFreqProxFields(allFields)

		// Apply buffered term deletions before routing postings to the codec.
		// Mirrors FreqProxTermsWriter.applyDeletes in Lucene 10.4.0.
		if err := applyDeletes(state, fields); err != nil {
			return fmt.Errorf("FreqProxTermsWriter.Flush: applyDeletes: %w", err)
		}
		if sortMap != nil {
			fields = newSortingFilterFields(fields, state.FieldInfos, sortMap)
		}

		consumer, err := postingsFormat.FieldsConsumer(state)
		if err != nil {
			return fmt.Errorf("FreqProxTermsWriter.Flush: FieldsConsumer: %w", err)
		}
		writeErr := writeFreqProxFields(consumer, fields)
		closeErr := consumer.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return fmt.Errorf("FreqProxTermsWriter.Flush: consumer close: %w", closeErr)
		}
	}

	// Delegate to the next-in-chain handler. Lucene's super.flush() runs
	// before applyDeletes; the ordering does not matter functionally because
	// the next-in-chain writer (term vectors) does not consume the postings
	// stream. We flush it last so any errors propagate after the postings
	// have been persisted.
	if w.NextTermsHash != nil {
		if err := w.NextTermsHash.Flush(fieldsToFlush, state, sortMap); err != nil {
			return fmt.Errorf("FreqProxTermsWriter.Flush: next-in-chain flush: %w", err)
		}
	}

	return nil
}

// applyDeletes processes pending term deletions in state.SegUpdates against
// the freshly-built postings in fields. For every (term, docID-upper-bound)
// pair it clears live-doc bits for all docs whose docID is strictly less than
// the deletion's upper bound, incrementing state.DelCountOnFlush for each.
//
// state.LiveDocs is allocated lazily on first deletion. Callers must propagate
// the populated LiveDocs bitset to the segment infos writer.
//
// Mirrors org.apache.lucene.index.FreqProxTermsWriter.applyDeletes in
// Apache Lucene 10.4.0.
func applyDeletes(state *SegmentWriteState, fields Fields) error {
	// SegUpdates is typed as spi.BufferedUpdatesRef so the SPI surface
	// does not depend on the index package. Type-assert back to the
	// concrete *BufferedUpdates value before reaching into the buffered
	// term map.
	if state.SegUpdates == nil {
		return nil
	}
	bu, ok := state.SegUpdates.(*BufferedUpdates)
	if !ok || bu.deleteTerms.IsEmpty() {
		return nil
	}

	segDeletes := bu.deleteTerms
	iterator := NewTermDocsIteratorFromFields(fields, true /* sortedTerms */)
	maxDoc := 0
	if state.SegmentInfo != nil {
		maxDoc = state.SegmentInfo.DocCount()
	}

	for _, entry := range segDeletes.ForEachOrdered() {
		postings, err := iterator.NextTerm(entry.Field, entry.Bytes)
		if err != nil {
			return fmt.Errorf("applyDeletes: NextTerm(%q): %w", entry.Field, err)
		}
		if postings == nil {
			continue
		}
		docID := entry.Value // upper bound: delete docs with docID < entry.Value
		for {
			doc, err := postings.NextDoc()
			if err != nil {
				return fmt.Errorf("applyDeletes: NextDoc: %w", err)
			}
			if doc == NO_MORE_DOCS || doc >= docID {
				break
			}
			// Allocate liveDocs on first deletion, mirroring Lucene's lazy init.
			if state.LiveDocs == nil {
				var allocErr error
				state.LiveDocs, allocErr = util.NewFixedBitSet(maxDoc)
				if allocErr != nil {
					return fmt.Errorf("applyDeletes: alloc liveDocs: %w", allocErr)
				}
				state.LiveDocs.SetAll()
			}
			if state.LiveDocs.Get(doc) {
				state.LiveDocs.Clear(doc)
				state.DelCountOnFlush++
			}
		}
	}
	return nil
}

// lookupFreqProxByBase resolves the FreqProxTermsWriterPerField wrapper for
// base using the registry populated by AddField. Returns (nil, false) when
// base was never registered, which signals that the slot was inserted as a
// bare *TermsHashPerField rather than via AddField.
//
// In Lucene 10.4.0 the equivalent operation is a direct Java downcast
// `(FreqProxTermsWriterPerField) perField`; Gocene requires an explicit
// registry because Go does not support interface downcasts on embedded structs.
func (w *FreqProxTermsWriter) lookupFreqProxByBase(base *TermsHashPerField) (*FreqProxTermsWriterPerField, bool) {
	if w.wrappers == nil {
		return nil, false
	}
	pf, ok := w.wrappers[base]
	return pf, ok
}

// FlushFreqProx is a convenience overload for pipelines that already keep a
// wrapper-keyed map. It performs the same flush logic as [Flush] without the
// downcast indirection. The Lucene-aligned [Flush] method delegates to
// FlushFreqProx after the cast.
func (w *FreqProxTermsWriter) FlushFreqProx(
	fieldsToFlush map[string]*FreqProxTermsWriterPerField,
	state *SegmentWriteState,
	sortMap SorterDocMap,
	postingsFormat PostingsFormat,
	norms any,
) error {
	if state == nil {
		return errors.New("FreqProxTermsWriter.FlushFreqProx: state must not be nil")
	}
	if postingsFormat == nil {
		return errors.New("FreqProxTermsWriter.FlushFreqProx: postingsFormat must not be nil")
	}

	allFields := make([]*FreqProxTermsWriterPerField, 0, len(fieldsToFlush))
	for _, perField := range fieldsToFlush {
		if perField == nil || perField.GetNumTerms() == 0 {
			continue
		}
		perField.SortTerms()
		if perField.fieldInfo.IndexOptions() == IndexOptionsNone {
			return fmt.Errorf("FreqProxTermsWriter.FlushFreqProx: field %q has IndexOptionsNone but produced terms",
				perField.GetFieldName())
		}
		allFields = append(allFields, perField)
	}

	if !state.FieldInfos.HasPostings() {
		if len(allFields) != 0 {
			return fmt.Errorf("FreqProxTermsWriter.FlushFreqProx: FieldInfos reports no postings but %d fields buffered terms",
				len(allFields))
		}
		return nil
	}

	sort.Slice(allFields, func(i, j int) bool {
		return allFields[i].CompareTo(allFields[j].TermsHashPerField) < 0
	})

	var fields Fields = NewFreqProxFields(allFields)
	if sortMap != nil {
		fields = newSortingFilterFields(fields, state.FieldInfos, sortMap)
	}

	consumer, err := postingsFormat.FieldsConsumer(state)
	if err != nil {
		return fmt.Errorf("FreqProxTermsWriter.FlushFreqProx: FieldsConsumer: %w", err)
	}
	writeErr := writeFreqProxFields(consumer, fields)
	closeErr := consumer.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return fmt.Errorf("FreqProxTermsWriter.FlushFreqProx: consumer close: %w", closeErr)
	}
	return nil
}

// writeFreqProxFields iterates fields in their natural order (already
// field-name-sorted by FreqProxFields) and forwards each (field, terms) pair
// to consumer.Write. The function isolates the per-field dispatch loop from
// the surrounding flush code so the sort/cast pipeline stays linear.
func writeFreqProxFields(consumer FieldsConsumer, fields Fields) error {
	iter, err := fields.Iterator()
	if err != nil {
		return fmt.Errorf("FreqProxTermsWriter.write: field iterator: %w", err)
	}
	for {
		name, err := iter.Next()
		if err != nil {
			return fmt.Errorf("FreqProxTermsWriter.write: field iterator advance: %w", err)
		}
		if name == "" {
			return nil
		}
		terms, err := fields.Terms(name)
		if err != nil {
			return fmt.Errorf("FreqProxTermsWriter.write: terms(%q): %w", name, err)
		}
		if terms == nil {
			continue
		}
		if err := consumer.Write(name, terms); err != nil {
			return fmt.Errorf("FreqProxTermsWriter.write: consumer.Write(%q): %w", name, err)
		}
	}
}

// sortingFilterFields is the Gocene equivalent of the anonymous
// FilterLeafReader.FilterFields subclass Lucene's FreqProxTermsWriter.flush
// builds when sortMap != nil. It overrides Terms() to wrap the underlying
// Terms in a SortingTerms with the field's IndexOptions, mirroring Lucene.
type sortingFilterFields struct {
	delegate   Fields
	fieldInfos *FieldInfos
	docMap     SorterDocMap
}

func newSortingFilterFields(delegate Fields, fieldInfos *FieldInfos, docMap SorterDocMap) *sortingFilterFields {
	return &sortingFilterFields{delegate: delegate, fieldInfos: fieldInfos, docMap: docMap}
}

func (f *sortingFilterFields) Iterator() (FieldIterator, error) { return f.delegate.Iterator() }
func (f *sortingFilterFields) Size() int                        { return f.delegate.Size() }
func (f *sortingFilterFields) Terms(field string) (Terms, error) {
	terms, err := f.delegate.Terms(field)
	if err != nil || terms == nil {
		return terms, err
	}
	fi := f.fieldInfos.GetByName(field)
	if fi == nil {
		return terms, nil
	}
	return NewSortingTerms(terms, fi.IndexOptions(), f.docMap), nil
}

// SortingTerms wraps a Terms view and yields TermsEnums whose postings are
// re-sorted by the supplied [SorterDocMap]. It is the Go port of the nested
// SortingTerms class in Lucene 10.4.0's FreqProxTermsWriter.
//
// Divergences from Lucene:
//   - Lucene exposes a second factory (intersect) that wraps the underlying
//     Terms.intersect. Gocene's Terms interface does not expose intersect, so
//     SortingTerms only forwards GetIterator / GetIteratorWithSeek.
type SortingTerms struct {
	in           Terms
	docMap       SorterDocMap
	indexOptions IndexOptions
}

// NewSortingTerms wraps in with a sort-aware Terms view. The supplied
// indexOptions controls which posting flavour the produced TermsEnums emit
// (docs-only vs docs+positions), exactly matching Lucene's constructor.
func NewSortingTerms(in Terms, indexOptions IndexOptions, docMap SorterDocMap) *SortingTerms {
	return &SortingTerms{in: in, docMap: docMap, indexOptions: indexOptions}
}

// GetIterator returns a [SortingTermsEnum] over the wrapped Terms.
func (t *SortingTerms) GetIterator() (TermsEnum, error) {
	delegate, err := t.in.GetIterator()
	if err != nil {
		return nil, err
	}
	return NewSortingTermsEnum(delegate, t.docMap, t.indexOptions), nil
}

// GetIteratorWithSeek mirrors Lucene's SortingTerms.iterator, which only
// produces a base TermsEnum (Lucene's seekCeil + SortingTermsEnum chain is
// implicit in the wrap). The seekTerm is forwarded to the underlying iterator
// before wrapping so the returned enumerator starts at the correct position.
func (t *SortingTerms) GetIteratorWithSeek(seekTerm *Term) (TermsEnum, error) {
	delegate, err := t.in.GetIteratorWithSeek(seekTerm)
	if err != nil || delegate == nil {
		return delegate, err
	}
	return NewSortingTermsEnum(delegate, t.docMap, t.indexOptions), nil
}

// Forwarders for the Terms metadata. All but HasFreqs/HasOffsets/HasPositions
// are pass-throughs to the wrapped Terms.

func (t *SortingTerms) GetPostingsReader(termText string, flags int) (PostingsEnum, error) {
	enum, err := t.GetIterator()
	if err != nil {
		return nil, err
	}
	found, err := enum.SeekExact(NewTerm("", termText))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return enum.Postings(flags)
}

func (t *SortingTerms) Size() int64                         { return t.in.Size() }
func (t *SortingTerms) GetDocCount() (int, error)           { return t.in.GetDocCount() }
func (t *SortingTerms) GetSumDocFreq() (int64, error)       { return t.in.GetSumDocFreq() }
func (t *SortingTerms) GetSumTotalTermFreq() (int64, error) { return t.in.GetSumTotalTermFreq() }
func (t *SortingTerms) HasFreqs() bool                      { return t.in.HasFreqs() }
func (t *SortingTerms) HasOffsets() bool                    { return t.in.HasOffsets() }
func (t *SortingTerms) HasPositions() bool                  { return t.in.HasPositions() }
func (t *SortingTerms) HasPayloads() bool                   { return t.in.HasPayloads() }
func (t *SortingTerms) GetMin() (*Term, error)              { return t.in.GetMin() }
func (t *SortingTerms) GetMax() (*Term, error)              { return t.in.GetMax() }

// SortingTermsEnum forwards every TermsEnum method to the wrapped enumerator
// and re-sorts the postings stream via [SortingDocsEnum] / [SortingPostingsEnum].
// It is the Go port of the nested SortingTermsEnum class in Lucene 10.4.0.
type SortingTermsEnum struct {
	in           TermsEnum
	docMap       SorterDocMap
	indexOptions IndexOptions
}

// NewSortingTermsEnum wraps in with a sort-aware enumerator.
func NewSortingTermsEnum(in TermsEnum, docMap SorterDocMap, indexOptions IndexOptions) *SortingTermsEnum {
	return &SortingTermsEnum{in: in, docMap: docMap, indexOptions: indexOptions}
}

// Pass-through navigators.

func (e *SortingTermsEnum) Next() (*Term, error)               { return e.in.Next() }
func (e *SortingTermsEnum) SeekCeil(term *Term) (*Term, error) { return e.in.SeekCeil(term) }
func (e *SortingTermsEnum) SeekExact(term *Term) (bool, error) { return e.in.SeekExact(term) }
func (e *SortingTermsEnum) Term() *Term                        { return e.in.Term() }
func (e *SortingTermsEnum) DocFreq() (int, error)              { return e.in.DocFreq() }
func (e *SortingTermsEnum) TotalTermFreq() (int64, error)      { return e.in.TotalTermFreq() }

// Postings dispatches to a [SortingPostingsEnum] when the field indexes
// positions and the caller asks for FREQS or higher (mirroring Lucene's
// indexOptions vs FREQS check). Otherwise the postings are routed through a
// [SortingDocsEnum]. The underlying PostingsEnum is always re-read with the
// requested flags so the produced stream carries the right information.
func (e *SortingTermsEnum) Postings(flags int) (PostingsEnum, error) {
	if e.indexOptions >= IndexOptionsDocsAndFreqs && postingsFlagRequested(flags, postingsFlagFreqs) {
		inEnum, err := e.in.Postings(flags)
		if err != nil {
			return nil, err
		}
		wrap := NewSortingPostingsEnum()
		storePositions := e.indexOptions >= IndexOptionsDocsAndFreqsAndPositions
		storeOffsets := e.indexOptions >= IndexOptionsDocsAndFreqsAndPositionsAndOffsets
		if err := wrap.Reset(e.docMap, inEnum, storePositions, storeOffsets); err != nil {
			return nil, err
		}
		return wrap, nil
	}
	inEnum, err := e.in.Postings(flags)
	if err != nil {
		return nil, err
	}
	wrap := NewSortingDocsEnum()
	if err := wrap.Reset(e.docMap, inEnum); err != nil {
		return nil, err
	}
	return wrap, nil
}

func (e *SortingTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (PostingsEnum, error) {
	return e.Postings(flags)
}

// SortingDocsEnum is the Go port of the nested SortingDocsEnum class in
// Lucene 10.4.0. It collects the wrapped enumerator's docIDs, maps them
// through [SorterDocMap.OldToNew], and sorts them with [util.LSBRadixSorter]
// so iteration follows the re-sorted order.
//
// Divergence from Lucene: the sorted-array element type is int32 because
// LSBRadixSorter operates on []int32 (Lucene's LSBRadixSorter takes int[]).
// The cast is safe in practice because docIDs are bounded by MaxDoc, which
// fits comfortably in int32 throughout the index pipeline.
type SortingDocsEnum struct {
	PostingsEnumBase

	sorter *util.LSBRadixSorter
	in     PostingsEnum
	docs   []int32
	docIt  int
	upTo   int
}

// NewSortingDocsEnum constructs an empty enumerator. Call [Reset] before
// iterating; Reset accepts the docMap and source enumerator.
func NewSortingDocsEnum() *SortingDocsEnum {
	return &SortingDocsEnum{
		PostingsEnumBase: PostingsEnumBase{CurrentDoc: -1},
		sorter:           util.NewLSBRadixSorter(),
	}
}

// Reset drains in into the internal docs slice (re-mapped through docMap),
// appends the NO_MORE_DOCS sentinel and sorts the slice with LSBRadixSorter.
func (d *SortingDocsEnum) Reset(docMap SorterDocMap, in PostingsEnum) error {
	d.in = in
	i := 0
	for {
		doc, err := in.NextDoc()
		if err != nil {
			return fmt.Errorf("SortingDocsEnum.Reset: %w", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		if i >= len(d.docs) {
			grown := util.Oversize(i+1, 4)
			next := make([]int32, grown)
			copy(next, d.docs)
			d.docs = next
		}
		d.docs[i] = int32(docMap.OldToNew(doc))
		i++
	}
	d.upTo = i
	if d.upTo >= len(d.docs) {
		// Append the NO_MORE_DOCS sentinel slot used by nextDoc.
		grown := util.Oversize(d.upTo+1, 4)
		next := make([]int32, grown)
		copy(next, d.docs)
		d.docs = next
	}
	d.docs[d.upTo] = int32(NO_MORE_DOCS)
	maxDoc := docMap.Size()
	upper := int64(maxDoc - 1)
	if upper < 0 {
		upper = 0
	}
	numBits := packed.BitsRequired(upper)
	d.sorter.Sort(numBits, d.docs, d.upTo)
	d.docIt = -1
	d.CurrentDoc = -1
	return nil
}

// GetWrapped returns the source enumerator. Mirrors Lucene's package-private
// getWrapped() accessor used by FreqProxTermsWriter.SortingTermsEnum to
// detect and rewire a reused SortingDocsEnum.
func (d *SortingDocsEnum) GetWrapped() PostingsEnum { return d.in }

// Advance is implemented via the slow path because the in-RAM buffer does
// not maintain skip data. Mirrors Lucene's "slowAdvance" comment.
func (d *SortingDocsEnum) Advance(target int) (int, error) {
	for {
		doc, err := d.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == NO_MORE_DOCS || doc >= target {
			return doc, nil
		}
	}
}

// NextDoc returns the next docID in the re-sorted order.
func (d *SortingDocsEnum) NextDoc() (int, error) {
	d.docIt++
	if d.docIt > d.upTo {
		d.CurrentDoc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	d.CurrentDoc = int(d.docs[d.docIt])
	return d.CurrentDoc, nil
}

// DocID returns the current docID, or -1 before the first NextDoc.
func (d *SortingDocsEnum) DocID() int {
	if d.docIt < 0 {
		return -1
	}
	return int(d.docs[d.docIt])
}

// Cost reports the upper bound on the number of postings. Mirrors Lucene.
func (d *SortingDocsEnum) Cost() int64 { return int64(d.upTo) }

// Freq always reports 1; the docs-only path discards term frequencies.
func (d *SortingDocsEnum) Freq() (int, error) { return 1, nil }

// NextPosition / StartOffset / EndOffset / GetPayload always return the
// "no positions" sentinels. Mirrors Lucene.

func (d *SortingDocsEnum) NextPosition() (int, error)  { return -1, nil }
func (d *SortingDocsEnum) StartOffset() (int, error)   { return -1, nil }
func (d *SortingDocsEnum) EndOffset() (int, error)     { return -1, nil }
func (d *SortingDocsEnum) GetPayload() ([]byte, error) { return nil, nil }

// SortingPostingsEnum is the Go port of the nested SortingPostingsEnum class
// in Lucene 10.4.0. It buffers the wrapped enumerator's docs+positions
// stream into a [util.ByteBlockPool]-backed scratch, re-sorts the per-doc
// offset table via a [util.TimSorter], and replays the buffered stream in
// the re-sorted order.
//
// Divergences from Lucene:
//
//   - Lucene uses ByteBuffersDataOutput.newResettableInstance() backed by a
//     recyclable block buffer; Gocene's [store.ByteBuffersDataOutput] does
//     not expose the resettable variant yet. We use the plain
//     ByteBuffersDataOutput and rebuild it on each Reset, which costs one
//     extra allocation per term but otherwise mirrors the wire format.
//
//   - The internal DocOffsetSorter is built on top of [util.NewTimSorter]
//     because Gocene's TimSorter exposes the same Save/Restore/CompareSaved
//     contract Lucene's TimSorter does.
//
//   - The position-buffer cursor uses [store.ByteArrayDataInput.SetPosition]
//     for seeking (Gocene's ByteBuffersDataInput surface does not expose
//     Seek). The downstream protocol is the same.
type SortingPostingsEnum struct {
	PostingsEnumBase

	sorter       *util.TimSorter
	sorterImpl   *docOffsetSorter
	docs         []int32
	offsets      []int64
	upto         int
	postingInput *postingScratchReader
	in           PostingsEnum
	storePos     bool
	storeOff     bool

	docIt       int
	pos         int
	startOffset int
	endOffset   int
	payload     util.BytesRef
	hasPayload  bool
	currFreq    int
}

// NewSortingPostingsEnum constructs an empty enumerator. Call [Reset] before
// iterating; Reset accepts the docMap, source enumerator and the
// storePositions / storeOffsets flags derived from the field's
// [IndexOptions].
func NewSortingPostingsEnum() *SortingPostingsEnum {
	return &SortingPostingsEnum{
		PostingsEnumBase: PostingsEnumBase{CurrentDoc: -1},
	}
}

// Reset drains the wrapped enumerator's docs+positions stream into the
// internal scratch buffer, sorts the per-doc offset table by remapped docID,
// and prepares the iterator for replay.
func (p *SortingPostingsEnum) Reset(docMap SorterDocMap, in PostingsEnum, storePositions, storeOffsets bool) error {
	p.in = in
	p.storePos = storePositions
	p.storeOff = storeOffsets
	if p.sorter == nil {
		numTempSlots := docMap.Size() / 8
		if numTempSlots < 0 {
			numTempSlots = 0
		}
		p.sorterImpl = &docOffsetSorter{}
		p.sorter = util.NewTimSorter(p.sorterImpl, numTempSlots)
	}
	p.docIt = -1
	p.startOffset = -1
	p.endOffset = -1

	scratch := newPostingScratchWriter()
	i := 0
	for {
		doc, err := in.NextDoc()
		if err != nil {
			return fmt.Errorf("SortingPostingsEnum.Reset: %w", err)
		}
		if doc == NO_MORE_DOCS {
			break
		}
		if i == len(p.docs) {
			newLength := util.Oversize(i+1, 4)
			grown := make([]int32, newLength)
			copy(grown, p.docs)
			p.docs = grown
			growoff := make([]int64, newLength)
			copy(growoff, p.offsets)
			p.offsets = growoff
		}
		p.docs[i] = int32(docMap.OldToNew(doc))
		p.offsets[i] = scratch.size()
		if err := p.addPositions(in, scratch); err != nil {
			return err
		}
		i++
	}
	p.upto = i
	p.sorterImpl.docs = p.docs
	p.sorterImpl.offsets = p.offsets
	p.sorter.Sort(0, p.upto)

	p.postingInput = scratch.toReader()
	return nil
}

// addPositions mirrors Lucene's SortingPostingsEnum.addPositions: it writes
// the per-doc freq and (optionally) the prox / offset / payload triples to
// the scratch buffer in the same wire format the in-memory FreqProx pool
// uses.
func (p *SortingPostingsEnum) addPositions(in PostingsEnum, out *postingScratchWriter) error {
	freq, err := in.Freq()
	if err != nil {
		return fmt.Errorf("SortingPostingsEnum.addPositions: %w", err)
	}
	out.writeVInt(int32(freq))
	if !p.storePos {
		return nil
	}
	previousPosition := 0
	previousEndOffset := 0
	for i := 0; i < freq; i++ {
		pos, err := in.NextPosition()
		if err != nil {
			return fmt.Errorf("SortingPostingsEnum.addPositions pos: %w", err)
		}
		payload, err := in.GetPayload()
		if err != nil {
			return fmt.Errorf("SortingPostingsEnum.addPositions payload: %w", err)
		}
		// Low-order bit signals "payload present"; remaining bits carry the
		// delta-encoded position.
		token := (pos - previousPosition) << 1
		if payload != nil {
			token |= 1
		}
		out.writeVInt(int32(token))
		previousPosition = pos
		if p.storeOff {
			startOffset, err := in.StartOffset()
			if err != nil {
				return fmt.Errorf("SortingPostingsEnum.addPositions startOffset: %w", err)
			}
			endOffset, err := in.EndOffset()
			if err != nil {
				return fmt.Errorf("SortingPostingsEnum.addPositions endOffset: %w", err)
			}
			out.writeVInt(int32(startOffset - previousEndOffset))
			out.writeVInt(int32(endOffset - startOffset))
			previousEndOffset = endOffset
		}
		if payload != nil {
			out.writeVInt(int32(len(payload)))
			out.writeBytes(payload)
		}
	}
	return nil
}

// Advance is implemented via the slow path; the buffered postings stream
// has no skip data. Mirrors Lucene's "slowAdvance" comment.
func (p *SortingPostingsEnum) Advance(target int) (int, error) {
	for {
		doc, err := p.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == NO_MORE_DOCS || doc >= target {
			return doc, nil
		}
	}
}

// DocID returns the current docID, with -1 / NO_MORE_DOCS sentinels.
func (p *SortingPostingsEnum) DocID() int {
	if p.docIt < 0 {
		return -1
	}
	if p.docIt >= p.upto {
		return NO_MORE_DOCS
	}
	return int(p.docs[p.docIt])
}

// EndOffset returns the current end offset; valid only after NextPosition.
func (p *SortingPostingsEnum) EndOffset() (int, error) { return p.endOffset, nil }

// Freq returns the current docID's term frequency.
func (p *SortingPostingsEnum) Freq() (int, error) { return p.currFreq, nil }

// GetPayload returns the current position's payload, or nil if absent.
func (p *SortingPostingsEnum) GetPayload() ([]byte, error) {
	if p.payload.Length == 0 {
		return nil, nil
	}
	return p.payload.Bytes[p.payload.Offset : p.payload.Offset+p.payload.Length], nil
}

// NextDoc advances to the next remapped docID, seeks the scratch buffer to
// the doc's posting prefix and refreshes the per-doc state.
func (p *SortingPostingsEnum) NextDoc() (int, error) {
	p.docIt++
	if p.docIt >= p.upto {
		p.CurrentDoc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	if err := p.postingInput.seek(p.offsets[p.docIt]); err != nil {
		return 0, fmt.Errorf("SortingPostingsEnum.NextDoc seek: %w", err)
	}
	freq, err := p.postingInput.readVInt()
	if err != nil {
		return 0, fmt.Errorf("SortingPostingsEnum.NextDoc freq: %w", err)
	}
	p.currFreq = int(freq)
	p.pos = 0
	p.endOffset = 0
	p.CurrentDoc = int(p.docs[p.docIt])
	return p.CurrentDoc, nil
}

// NextPosition advances the position cursor; mirrors Lucene's protocol.
func (p *SortingPostingsEnum) NextPosition() (int, error) {
	if !p.storePos {
		return -1, nil
	}
	token, err := p.postingInput.readVInt()
	if err != nil {
		return 0, fmt.Errorf("SortingPostingsEnum.NextPosition: %w", err)
	}
	p.pos += int(uint32(token) >> 1)
	if p.storeOff {
		so, err := p.postingInput.readVInt()
		if err != nil {
			return 0, fmt.Errorf("SortingPostingsEnum.NextPosition startOffset: %w", err)
		}
		eo, err := p.postingInput.readVInt()
		if err != nil {
			return 0, fmt.Errorf("SortingPostingsEnum.NextPosition endOffset: %w", err)
		}
		p.startOffset = p.endOffset + int(so)
		p.endOffset = p.startOffset + int(eo)
	}
	if (token & 1) != 0 {
		p.payload.Offset = 0
		plen, err := p.postingInput.readVInt()
		if err != nil {
			return 0, fmt.Errorf("SortingPostingsEnum.NextPosition payload length: %w", err)
		}
		p.payload.Length = int(plen)
		if p.payload.Length > len(p.payload.Bytes) {
			grown := util.Oversize(p.payload.Length, 1)
			p.payload.Bytes = make([]byte, grown)
		}
		if p.payload.Length > 0 {
			if err := p.postingInput.readBytes(p.payload.Bytes[:p.payload.Length]); err != nil {
				return 0, fmt.Errorf("SortingPostingsEnum.NextPosition payload bytes: %w", err)
			}
		}
		p.hasPayload = true
	} else {
		p.payload.Length = 0
		p.hasPayload = false
	}
	return p.pos, nil
}

// StartOffset returns the current start offset; valid only after NextPosition.
func (p *SortingPostingsEnum) StartOffset() (int, error) { return p.startOffset, nil }

// GetWrapped returns the source enumerator. Mirrors Lucene's package-private
// getWrapped() accessor used by FreqProxTermsWriter.SortingTermsEnum.
func (p *SortingPostingsEnum) GetWrapped() PostingsEnum { return p.in }

// Cost forwards to the wrapped enumerator's cost.
func (p *SortingPostingsEnum) Cost() int64 { return p.in.Cost() }

// docOffsetSorter implements [util.TimSorterInterface] for the parallel
// (docs, offsets) arrays inside [SortingPostingsEnum]. It mirrors Lucene's
// inner DocOffsetSorter without allocating slices for the tmp buffer until
// Save first runs.
type docOffsetSorter struct {
	docs       []int32
	offsets    []int64
	tmpDocs    []int32
	tmpOffsets []int64
}

func (s *docOffsetSorter) Compare(i, j int) int {
	di, dj := s.docs[i], s.docs[j]
	switch {
	case di < dj:
		return -1
	case di > dj:
		return 1
	default:
		return 0
	}
}

func (s *docOffsetSorter) Swap(i, j int) {
	s.docs[i], s.docs[j] = s.docs[j], s.docs[i]
	s.offsets[i], s.offsets[j] = s.offsets[j], s.offsets[i]
}

// Sort is unused for TimSorter implementations; the parent TimSorter.Sort
// drives the algorithm. Mirrors arrayTimSorter.Sort.
func (s *docOffsetSorter) Sort(from, to int) {}

func (s *docOffsetSorter) Copy(src, dest int) {
	s.docs[dest] = s.docs[src]
	s.offsets[dest] = s.offsets[src]
}

func (s *docOffsetSorter) Save(i, length int) {
	if cap(s.tmpDocs) < length {
		s.tmpDocs = make([]int32, util.Oversize(length, 4))
		s.tmpOffsets = make([]int64, len(s.tmpDocs))
	}
	s.tmpDocs = s.tmpDocs[:length]
	s.tmpOffsets = s.tmpOffsets[:length]
	copy(s.tmpDocs, s.docs[i:i+length])
	copy(s.tmpOffsets, s.offsets[i:i+length])
}

func (s *docOffsetSorter) Restore(src, dest int) {
	s.docs[dest] = s.tmpDocs[src]
	s.offsets[dest] = s.tmpOffsets[src]
}

func (s *docOffsetSorter) CompareSaved(i, j int) int {
	di, dj := s.tmpDocs[i], s.docs[j]
	switch {
	case di < dj:
		return -1
	case di > dj:
		return 1
	default:
		return 0
	}
}

// postingScratchWriter is a thin wrapper around a growable []byte buffer
// with VInt encoding. It replaces Lucene's
// ByteBuffersDataOutput.newResettableInstance() for the SortingPostingsEnum
// scratch. The wire format matches Lucene's VInt encoding because both rely
// on the canonical 7-bits-per-byte continuation-bit scheme.
//
// The writer is local-only (single Reset usage per term) so it lives in this
// file rather than being added to the store package.
type postingScratchWriter struct {
	buf []byte
}

func newPostingScratchWriter() *postingScratchWriter {
	return &postingScratchWriter{buf: make([]byte, 0, 64)}
}

func (w *postingScratchWriter) size() int64 { return int64(len(w.buf)) }

func (w *postingScratchWriter) writeVInt(v int32) {
	uv := uint32(v)
	for uv&^0x7F != 0 {
		w.buf = append(w.buf, byte((uv&0x7F)|0x80))
		uv >>= 7
	}
	w.buf = append(w.buf, byte(uv))
}

func (w *postingScratchWriter) writeBytes(b []byte) {
	w.buf = append(w.buf, b...)
}

func (w *postingScratchWriter) toReader() *postingScratchReader {
	return &postingScratchReader{buf: w.buf}
}

// postingScratchReader is the read-side counterpart of postingScratchWriter.
// It exposes Seek (random access) and VInt/Bytes accessors that match the
// PostingsEnum protocol used by SortingPostingsEnum.NextDoc / NextPosition.
type postingScratchReader struct {
	buf []byte
	pos int
}

func (r *postingScratchReader) seek(offset int64) error {
	if offset < 0 || offset > int64(len(r.buf)) {
		return fmt.Errorf("postingScratchReader.seek: offset %d out of bounds [0,%d]", offset, len(r.buf))
	}
	r.pos = int(offset)
	return nil
}

func (r *postingScratchReader) readVInt() (int32, error) {
	var result int32
	shift := uint(0)
	for {
		if r.pos >= len(r.buf) {
			return 0, errors.New("postingScratchReader.readVInt: EOF")
		}
		b := r.buf[r.pos]
		r.pos++
		result |= int32(b&0x7F) << shift
		if (b & 0x80) == 0 {
			return result, nil
		}
		shift += 7
		if shift >= 32 {
			return 0, errors.New("postingScratchReader.readVInt: corrupted")
		}
	}
}

func (r *postingScratchReader) readBytes(dst []byte) error {
	if r.pos+len(dst) > len(r.buf) {
		return errors.New("postingScratchReader.readBytes: EOF")
	}
	copy(dst, r.buf[r.pos:r.pos+len(dst)])
	r.pos += len(dst)
	return nil
}
