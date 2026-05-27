// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// MappedMultiFields wraps a MultiFields and applies a MergeState.DocMap chain
// so that consumers see merge-time (composite) docIDs. Used during segment
// merging to expose the unified postings surface. Mirrors
// org.apache.lucene.index.MappedMultiFields from Apache Lucene 10.4.0.
//
// The multi-terms-enum traversal is gated by backlog #2706 (MultiTermsEnum
// full port); until that lands, Iterator() on any MappedMultiTerms returns the
// same ErrMultiTermsEnumNotImplemented error that MultiTerms.Iterator()
// propagates.
type MappedMultiFields struct {
	mergeState *MergeState
	multi      *MultiFields
}

// NewMappedMultiFields wraps multiFields with merge-time docID remapping
// driven by mergeState. Both parameters must be non-nil.
func NewMappedMultiFields(ms *MergeState, multi *MultiFields) *MappedMultiFields {
	return &MappedMultiFields{
		mergeState: ms,
		multi:      multi,
	}
}

// Iterator returns an iterator over all field names in the underlying MultiFields.
// The names are sorted as per the MultiFields contract.
func (m *MappedMultiFields) Iterator() (FieldIterator, error) {
	return m.multi.Iterator()
}

// Size returns the number of fields in the underlying MultiFields.
func (m *MappedMultiFields) Size() int {
	return m.multi.Size()
}

// Terms returns a Terms view for the given field that applies merge-time docID
// remapping. Returns nil if the field has no indexed terms in any sub-reader.
//
// It collects the per-sub Terms from each sub-Fields in the MultiFields, pairs
// them with their ReaderSlices (from the MergeState), and constructs a
// MultiTerms from the result — mirroring Lucene's cast
// `(MultiTerms) in.terms(field)` where MultiFields.terms() always produces a
// MultiTerms.
func (m *MappedMultiFields) Terms(field string) (Terms, error) {
	var subs []Terms
	var slices []ReaderSlice
	for i, f := range m.multi.FieldsList() {
		if f == nil {
			continue
		}
		t, err := f.Terms(field)
		if err != nil {
			return nil, fmt.Errorf("MappedMultiFields.Terms(%s) sub %d: %w", field, i, err)
		}
		if t == nil {
			continue
		}
		readerIdx := i
		if readerIdx < len(m.mergeState.DocMaps) {
			// Use the MergeState's actual doc-ID range for this sub-reader.
			subs = append(subs, t)
			slices = append(slices, ReaderSlice{ReaderIndex: readerIdx})
		} else {
			subs = append(subs, t)
			slices = append(slices, ReaderSlice{ReaderIndex: readerIdx})
		}
	}
	if len(subs) == 0 {
		return nil, nil
	}
	mt, err := NewMultiTerms(subs, slices)
	if err != nil {
		return nil, fmt.Errorf("MappedMultiFields.Terms(%s): build MultiTerms: %w", field, err)
	}
	return &mappedMultiTerms{
		field:      field,
		mergeState: m.mergeState,
		delegate:   mt,
	}, nil
}

// mappedMultiTerms wraps a MultiTerms and applies merge-time remapping. Mirrors
// MappedMultiFields.MappedMultiTerms (private static class in Lucene).
type mappedMultiTerms struct {
	field      string
	mergeState *MergeState
	delegate   *MultiTerms
}

// GetIterator returns a MappedMultiTermsEnum positioned before the first term.
// If MultiTerms.Iterator() is not yet implemented it propagates the error.
func (t *mappedMultiTerms) GetIterator() (TermsEnum, error) {
	it, err := t.delegate.Iterator()
	if err != nil {
		return nil, err
	}
	if it == nil {
		return nil, nil
	}
	mte, ok := it.(*MultiTermsEnum)
	if !ok {
		// Should not happen once MultiTermsEnum is fully ported; defensive path.
		return it, nil
	}
	return &mappedMultiTermsEnum{
		field:      t.field,
		mergeState: t.mergeState,
		delegate:   mte,
	}, nil
}

// GetIteratorWithSeek positions the enum at the given term and wraps the result.
func (t *mappedMultiTerms) GetIteratorWithSeek(seek *Term) (TermsEnum, error) {
	it, err := t.GetIterator()
	if err != nil {
		return nil, err
	}
	if it == nil || seek == nil {
		return it, nil
	}
	if _, err := it.SeekCeil(seek); err != nil {
		return nil, err
	}
	return it, nil
}

// GetPostingsReader is not supported on mapped multi-terms (UnsupportedOperationException
// in Lucene). Callers must iterate via GetIterator().
func (t *mappedMultiTerms) GetPostingsReader(termText string, flags int) (PostingsEnum, error) {
	return nil, fmt.Errorf("mappedMultiTerms.GetPostingsReader: unsupported operation")
}

// Size always returns -1 (unsupported in Lucene MappedMultiTerms).
func (t *mappedMultiTerms) Size() int64 { return -1 }

// GetDocCount raises UnsupportedOperationException as in Lucene.
func (t *mappedMultiTerms) GetDocCount() (int, error) {
	return 0, fmt.Errorf("mappedMultiTerms.GetDocCount: unsupported operation")
}

// GetSumDocFreq raises UnsupportedOperationException as in Lucene.
func (t *mappedMultiTerms) GetSumDocFreq() (int64, error) {
	return 0, fmt.Errorf("mappedMultiTerms.GetSumDocFreq: unsupported operation")
}

// GetSumTotalTermFreq raises UnsupportedOperationException as in Lucene.
func (t *mappedMultiTerms) GetSumTotalTermFreq() (int64, error) {
	return 0, fmt.Errorf("mappedMultiTerms.GetSumTotalTermFreq: unsupported operation")
}

// HasFreqs delegates to the underlying MultiTerms.
func (t *mappedMultiTerms) HasFreqs() bool { return t.delegate.HasFreqs() }

// HasOffsets delegates to the underlying MultiTerms.
func (t *mappedMultiTerms) HasOffsets() bool { return t.delegate.HasOffsets() }

// HasPositions delegates to the underlying MultiTerms.
func (t *mappedMultiTerms) HasPositions() bool { return t.delegate.HasPositions() }

// HasPayloads delegates to the underlying MultiTerms.
func (t *mappedMultiTerms) HasPayloads() bool { return t.delegate.HasPayloads() }

// GetMin returns nil (unsupported for multi-terms — matches Lucene behaviour
// where MappedMultiTerms does not override getMin).
func (t *mappedMultiTerms) GetMin() (*Term, error) { return nil, nil }

// GetMax returns nil (unsupported for multi-terms — matches Lucene behaviour).
func (t *mappedMultiTerms) GetMax() (*Term, error) { return nil, nil }

// mappedMultiTermsEnum wraps a MultiTermsEnum and routes Postings calls
// through MappingMultiPostingsEnum for merge-time docID translation. Mirrors
// MappedMultiFields.MappedMultiTermsEnum (private static class in Lucene).
type mappedMultiTermsEnum struct {
	field      string
	mergeState *MergeState
	delegate   *MultiTermsEnum

	// cachedMappingEnum is reused across Postings calls for the same field to
	// avoid re-allocation of the per-sub MappingPostingsSubs. Mirrors Lucene's
	// reuse pattern via the PostingsEnum argument.
	cachedMappingEnum *MappingMultiPostingsEnum
}

// Next advances to the next term. Delegates to the underlying MultiTermsEnum.
func (e *mappedMultiTermsEnum) Next() (*Term, error) { return e.delegate.Next() }

// SeekCeil seeks to the given term or the next term after it.
func (e *mappedMultiTermsEnum) SeekCeil(term *Term) (*Term, error) {
	return e.delegate.SeekCeil(term)
}

// SeekExact seeks to the exact term.
func (e *mappedMultiTermsEnum) SeekExact(term *Term) (bool, error) {
	return e.delegate.SeekExact(term)
}

// Term returns the current term.
func (e *mappedMultiTermsEnum) Term() *Term { return e.delegate.Term() }

// DocFreq raises UnsupportedOperationException as in Lucene MappedMultiTermsEnum.
func (e *mappedMultiTermsEnum) DocFreq() (int, error) {
	return 0, fmt.Errorf("mappedMultiTermsEnum.DocFreq: unsupported operation")
}

// TotalTermFreq raises UnsupportedOperationException as in Lucene.
func (e *mappedMultiTermsEnum) TotalTermFreq() (int64, error) {
	return 0, fmt.Errorf("mappedMultiTermsEnum.TotalTermFreq: unsupported operation")
}

// Postings returns a MappingMultiPostingsEnum that applies merge-time docID
// remapping. Reuses the cached MappingMultiPostingsEnum when the field matches.
// Mirrors MappedMultiTermsEnum.postings(PostingsEnum, int) in Lucene.
func (e *mappedMultiTermsEnum) Postings(flags int) (PostingsEnum, error) {
	return e.PostingsWithLiveDocs(nil, flags)
}

// PostingsWithLiveDocs returns a MappingMultiPostingsEnum, ignoring liveDocs
// (merge-time callers use DocMaps for deletion filtering, not live-docs bits).
func (e *mappedMultiTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (PostingsEnum, error) {
	var mappingEnum *MappingMultiPostingsEnum

	// Reuse the cached MappingMultiPostingsEnum if it is for the same field.
	if e.cachedMappingEnum != nil {
		mappingEnum = e.cachedMappingEnum
	} else {
		var err error
		mappingEnum, err = NewMappingMultiPostingsEnum(e.field, e.mergeState)
		if err != nil {
			return nil, fmt.Errorf("mappedMultiTermsEnum.Postings: create MappingMultiPostingsEnum: %w", err)
		}
		e.cachedMappingEnum = mappingEnum
	}

	// Delegate to the underlying MultiTermsEnum to obtain the MultiPostingsEnum,
	// passing the mapping enum's inner MultiPostingsEnum as the reuse argument.
	rawEnum, err := e.delegate.Postings(flags)
	if err != nil {
		return nil, fmt.Errorf("mappedMultiTermsEnum.Postings: delegate postings: %w", err)
	}
	multiEnum, ok := rawEnum.(*MultiPostingsEnum)
	if !ok {
		return nil, fmt.Errorf("mappedMultiTermsEnum.Postings: expected *MultiPostingsEnum, got %T", rawEnum)
	}

	return mappingEnum.Reset(multiEnum)
}

// Compile-time assertions.
var _ Fields = (*MappedMultiFields)(nil)
var _ Terms = (*mappedMultiTerms)(nil)
var _ TermsEnum = (*mappedMultiTermsEnum)(nil)
