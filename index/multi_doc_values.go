// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// docValuesNoMoreDocs is the exhaustion sentinel for read-side doc-values
// iterators: DocIdSetIterator.NO_MORE_DOCS (Integer.MAX_VALUE) in Apache Lucene.
// It deliberately differs from the index package's NO_MORE_DOCS (-1), which is
// the PostingsEnum sentinel. The codec doc-values producers (and the search
// layer that consumes them) use this value, so the MultiDocValues iterators —
// being read-side doc-values iterators themselves — both compare against it and
// return it at exhaustion, matching Lucene's MultiDocValues exactly.
const docValuesNoMoreDocs = 2147483647

// dvExhausted reports whether a per-leaf doc-values NextDoc/Advance result marks
// the end of that leaf. It tolerates both read-side sentinels in play in Gocene:
// the codec producers' Integer.MAX_VALUE and the index package's empty-iterator
// -1 (used as the fallback for leaves that lack the field).
func dvExhausted(docID int) bool {
	return docID < 0 || docID >= docValuesNoMoreDocs
}

// MultiDocValues is a stateless container of static helpers that flatten
// composite-reader doc values into a single virtual iterator spanning all
// leaves. Mirrors org.apache.lucene.index.MultiDocValues from Apache Lucene
// 10.4.0.
//
// These helpers are an extremely slow way to access doc values; production
// code should access them per-segment via LeafReader.Get*DocValues. They exist
// so that code holding only a composite (multi-segment) reader can still read a
// field's doc values without first flattening the index, matching Lucene.
//
// Numeric/binary/sorted-numeric merging concatenates the per-leaf iterators and
// offsets each leaf's local docIDs by its docBase. Sorted/sorted-set merging
// additionally composes the per-segment ordinal spaces into one global ordinal
// space via an OrdinalMap.

// multiDocValuesLeaf is the read surface MultiDocValues needs from each leaf.
// Both *SegmentReader and *LeafReader satisfy it structurally, so a leaf's
// reader (typed as IndexReaderInterface) can be narrowed to it.
type multiDocValuesLeaf interface {
	GetFieldInfos() *FieldInfos
	GetNumericDocValues(field string) (NumericDocValues, error)
	GetBinaryDocValues(field string) (BinaryDocValues, error)
	GetSortedDocValues(field string) (SortedDocValues, error)
	GetSortedNumericDocValues(field string) (SortedNumericDocValues, error)
	GetSortedSetDocValues(field string) (SortedSetDocValues, error)
	GetNormValues(field string) (NumericDocValues, error)
}

// leafDocValuesReader narrows a leaf context's reader to multiDocValuesLeaf.
func leafDocValuesReader(ctx *LeafReaderContext) (multiDocValuesLeaf, error) {
	lr, ok := ctx.Reader().(multiDocValuesLeaf)
	if !ok {
		return nil, fmt.Errorf("MultiDocValues: leaf reader %T does not expose doc-values accessors", ctx.Reader())
	}
	return lr, nil
}

// buildDocStarts returns the docBase of every leaf followed by maxDoc as the
// trailing sentinel, matching the starts[] array Lucene feeds ReaderUtil.subIndex.
func buildDocStarts(leaves []*LeafReaderContext, maxDoc int) []int {
	starts := make([]int, len(leaves)+1)
	for i, leaf := range leaves {
		starts[i] = leaf.DocBase()
	}
	starts[len(leaves)] = maxDoc
	return starts
}

// MultiDocValuesGetNormValues returns the norm values for field, flattened
// across all leaves of r, or nil when no leaf has norms for field.
func MultiDocValuesGetNormValues(r IndexReaderInterface, field string) (NumericDocValues, error) {
	leaves, err := r.Leaves()
	if err != nil {
		return nil, err
	}
	switch len(leaves) {
	case 0:
		return nil, nil
	case 1:
		lr, err := leafDocValuesReader(leaves[0])
		if err != nil {
			return nil, err
		}
		return lr.GetNormValues(field)
	}

	normFound := false
	for _, leaf := range leaves {
		lr, err := leafDocValuesReader(leaf)
		if err != nil {
			return nil, err
		}
		info := lr.GetFieldInfos().GetByName(field)
		if info != nil && info.HasNorms() {
			normFound = true
			break
		}
	}
	if !normFound {
		return nil, nil
	}

	get := func(leafIdx int) (NumericDocValues, error) {
		lr, err := leafDocValuesReader(leaves[leafIdx])
		if err != nil {
			return nil, err
		}
		return lr.GetNormValues(field)
	}
	return newMultiNumericDocValues(buildDocStarts(leaves, r.MaxDoc()), len(leaves), get), nil
}

// MultiDocValuesGetNumericValues returns the numeric doc values for field,
// flattened across all leaves of r, or nil when no leaf carries NUMERIC doc
// values for field.
func MultiDocValuesGetNumericValues(r IndexReaderInterface, field string) (NumericDocValues, error) {
	leaves, err := r.Leaves()
	if err != nil {
		return nil, err
	}
	switch len(leaves) {
	case 0:
		return nil, nil
	case 1:
		lr, err := leafDocValuesReader(leaves[0])
		if err != nil {
			return nil, err
		}
		return lr.GetNumericDocValues(field)
	}

	anyReal, err := anyLeafHasDVType(leaves, field, DocValuesTypeNumeric)
	if err != nil || !anyReal {
		return nil, err
	}

	get := func(leafIdx int) (NumericDocValues, error) {
		lr, err := leafDocValuesReader(leaves[leafIdx])
		if err != nil {
			return nil, err
		}
		return lr.GetNumericDocValues(field)
	}
	return newMultiNumericDocValues(buildDocStarts(leaves, r.MaxDoc()), len(leaves), get), nil
}

// MultiDocValuesGetBinaryValues returns the binary doc values for field,
// flattened across all leaves of r, or nil when no leaf carries BINARY doc
// values for field.
func MultiDocValuesGetBinaryValues(r IndexReaderInterface, field string) (BinaryDocValues, error) {
	leaves, err := r.Leaves()
	if err != nil {
		return nil, err
	}
	switch len(leaves) {
	case 0:
		return nil, nil
	case 1:
		lr, err := leafDocValuesReader(leaves[0])
		if err != nil {
			return nil, err
		}
		return lr.GetBinaryDocValues(field)
	}

	anyReal, err := anyLeafHasDVType(leaves, field, DocValuesTypeBinary)
	if err != nil || !anyReal {
		return nil, err
	}

	get := func(leafIdx int) (BinaryDocValues, error) {
		lr, err := leafDocValuesReader(leaves[leafIdx])
		if err != nil {
			return nil, err
		}
		return lr.GetBinaryDocValues(field)
	}
	return newMultiBinaryDocValues(buildDocStarts(leaves, r.MaxDoc()), len(leaves), get), nil
}

// MultiDocValuesGetSortedNumericValues returns the sorted numeric doc values
// for field, flattened across all leaves of r, or nil when no leaf carries
// SORTED_NUMERIC doc values for field.
func MultiDocValuesGetSortedNumericValues(r IndexReaderInterface, field string) (SortedNumericDocValues, error) {
	leaves, err := r.Leaves()
	if err != nil {
		return nil, err
	}
	switch len(leaves) {
	case 0:
		return nil, nil
	case 1:
		lr, err := leafDocValuesReader(leaves[0])
		if err != nil {
			return nil, err
		}
		return lr.GetSortedNumericDocValues(field)
	}

	anyReal, err := anyLeafHasDVType(leaves, field, DocValuesTypeSortedNumeric)
	if err != nil || !anyReal {
		return nil, err
	}

	get := func(leafIdx int) (SortedNumericDocValues, error) {
		lr, err := leafDocValuesReader(leaves[leafIdx])
		if err != nil {
			return nil, err
		}
		return lr.GetSortedNumericDocValues(field)
	}
	return newMultiSortedNumericDocValues(buildDocStarts(leaves, r.MaxDoc()), len(leaves), get), nil
}

// MultiDocValuesGetSortedValues returns the sorted doc values for field,
// flattened across all leaves of r, or nil when no leaf carries SORTED doc
// values for field. Per-segment ordinals are mapped into one global ordinal
// space via an OrdinalMap.
func MultiDocValuesGetSortedValues(r IndexReaderInterface, field string) (SortedDocValues, error) {
	leaves, err := r.Leaves()
	if err != nil {
		return nil, err
	}
	switch len(leaves) {
	case 0:
		return nil, nil
	case 1:
		lr, err := leafDocValuesReader(leaves[0])
		if err != nil {
			return nil, err
		}
		return lr.GetSortedDocValues(field)
	}

	anyReal := false
	values := make([]SortedDocValues, len(leaves))
	var totalCost int64
	for i, leaf := range leaves {
		lr, err := leafDocValuesReader(leaf)
		if err != nil {
			return nil, err
		}
		v, err := lr.GetSortedDocValues(field)
		if err != nil {
			return nil, err
		}
		if v == nil {
			v = EmptySorted()
		} else {
			anyReal = true
			totalCost += v.Cost()
		}
		values[i] = v
	}
	if !anyReal {
		return nil, nil
	}

	mapping, err := BuildOrdinalMapFromSortedValues(readerOrdinalMapOwner(r), values, 0)
	if err != nil {
		return nil, fmt.Errorf("MultiDocValues: building ordinal map for %q: %w", field, err)
	}
	return &MultiSortedDocValues{
		values:    values,
		docStarts: buildDocStarts(leaves, r.MaxDoc()),
		mapping:   mapping,
		totalCost: totalCost,
		docID:     -1,
	}, nil
}

// MultiDocValuesGetSortedSetValues returns the sorted-set doc values for field,
// flattened across all leaves of r, or nil when no leaf carries SORTED_SET doc
// values for field. Per-segment ordinals are mapped into one global ordinal
// space via an OrdinalMap.
func MultiDocValuesGetSortedSetValues(r IndexReaderInterface, field string) (SortedSetDocValues, error) {
	leaves, err := r.Leaves()
	if err != nil {
		return nil, err
	}
	switch len(leaves) {
	case 0:
		return nil, nil
	case 1:
		lr, err := leafDocValuesReader(leaves[0])
		if err != nil {
			return nil, err
		}
		return lr.GetSortedSetDocValues(field)
	}

	anyReal := false
	values := make([]SortedSetDocValues, len(leaves))
	var totalCost int64
	for i, leaf := range leaves {
		lr, err := leafDocValuesReader(leaf)
		if err != nil {
			return nil, err
		}
		v, err := lr.GetSortedSetDocValues(field)
		if err != nil {
			return nil, err
		}
		if v == nil {
			v = EmptySortedSet()
		} else {
			anyReal = true
			totalCost += v.Cost()
		}
		values[i] = v
	}
	if !anyReal {
		return nil, nil
	}

	mapping, err := BuildOrdinalMapFromSortedSetValues(readerOrdinalMapOwner(r), values, 0)
	if err != nil {
		return nil, fmt.Errorf("MultiDocValues: building ordinal map for %q: %w", field, err)
	}
	return &MultiSortedSetDocValues{
		values:    values,
		docStarts: buildDocStarts(leaves, r.MaxDoc()),
		mapping:   mapping,
		totalCost: totalCost,
		docID:     -1,
	}, nil
}

// anyLeafHasDVType reports whether at least one leaf declares field with the
// given doc-values type, mirroring Lucene's pre-scan of FieldInfos before it
// builds a merging iterator.
func anyLeafHasDVType(leaves []*LeafReaderContext, field string, dvType DocValuesType) (bool, error) {
	for _, leaf := range leaves {
		lr, err := leafDocValuesReader(leaf)
		if err != nil {
			return false, err
		}
		info := lr.GetFieldInfos().GetByName(field)
		if info != nil && info.DocValuesType() == dvType {
			return true, nil
		}
	}
	return false, nil
}

// readerOrdinalMapOwner returns the cache key that owns the OrdinalMap so the
// map can be cached against the reader, or nil when the reader exposes no cache
// helper (matching Lucene, which passes a null owner in that case).
func readerOrdinalMapOwner(r IndexReaderInterface) *CacheKey {
	type cacheHelperProvider interface {
		GetReaderCacheHelper() CacheHelper
	}
	if p, ok := r.(cacheHelperProvider); ok {
		if h := p.GetReaderCacheHelper(); h != nil {
			return h.CacheKey()
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Numeric / norm merging iterator.
// ---------------------------------------------------------------------------

// multiNumericDocValues concatenates the NumericDocValues of every leaf,
// offsetting each leaf's local docIDs by its docBase. Backs both the numeric
// and the norm helpers (they differ only in the per-leaf getter). Mirrors the
// anonymous NumericDocValues in MultiDocValues.getNumericValues / getNormValues.
type multiNumericDocValues struct {
	docStarts       []int
	numLeaves       int
	get             func(leafIdx int) (NumericDocValues, error)
	nextLeaf        int
	current         NumericDocValues
	currentDocStart int
	docID           int
}

func newMultiNumericDocValues(docStarts []int, numLeaves int, get func(int) (NumericDocValues, error)) *multiNumericDocValues {
	return &multiNumericDocValues{docStarts: docStarts, numLeaves: numLeaves, get: get, docID: -1}
}

func (m *multiNumericDocValues) DocID() int { return m.docID }

func (m *multiNumericDocValues) NextDoc() (int, error) {
	for {
		for m.current == nil {
			if m.nextLeaf == m.numLeaves {
				m.docID = docValuesNoMoreDocs
				return m.docID, nil
			}
			m.currentDocStart = m.docStarts[m.nextLeaf]
			v, err := m.get(m.nextLeaf)
			if err != nil {
				return 0, err
			}
			m.current = v
			m.nextLeaf++
		}
		newDocID, err := m.current.NextDoc()
		if err != nil {
			return 0, err
		}
		if dvExhausted(newDocID) {
			m.current = nil
		} else {
			m.docID = m.currentDocStart + newDocID
			return m.docID, nil
		}
	}
}

func (m *multiNumericDocValues) Advance(target int) (int, error) {
	if target <= m.docID {
		return 0, fmt.Errorf("MultiDocValues numeric: can only advance beyond current doc %d (target %d)", m.docID, target)
	}
	readerIndex := ReaderUtilSubIndex(target, m.docStarts)
	if readerIndex >= m.nextLeaf {
		if readerIndex == m.numLeaves {
			m.current = nil
			m.docID = docValuesNoMoreDocs
			return m.docID, nil
		}
		m.currentDocStart = m.docStarts[readerIndex]
		v, err := m.get(readerIndex)
		if err != nil {
			return 0, err
		}
		m.current = v
		m.nextLeaf = readerIndex + 1
		if m.current == nil {
			return m.NextDoc()
		}
	}
	newDocID, err := m.current.Advance(target - m.currentDocStart)
	if err != nil {
		return 0, err
	}
	if dvExhausted(newDocID) {
		m.current = nil
		return m.NextDoc()
	}
	m.docID = m.currentDocStart + newDocID
	return m.docID, nil
}

func (m *multiNumericDocValues) AdvanceExact(target int) (bool, error) {
	if target < m.docID {
		return false, fmt.Errorf("MultiDocValues numeric: can only advance beyond current doc %d (target %d)", m.docID, target)
	}
	readerIndex := ReaderUtilSubIndex(target, m.docStarts)
	if readerIndex >= m.nextLeaf {
		if readerIndex == m.numLeaves {
			return false, fmt.Errorf("MultiDocValues numeric: out of range target %d", target)
		}
		m.currentDocStart = m.docStarts[readerIndex]
		v, err := m.get(readerIndex)
		if err != nil {
			return false, err
		}
		m.current = v
		m.nextLeaf = readerIndex + 1
	}
	m.docID = target
	if m.current == nil {
		return false, nil
	}
	return m.current.AdvanceExact(target - m.currentDocStart)
}

func (m *multiNumericDocValues) LongValue() (int64, error) {
	if m.current == nil {
		return 0, fmt.Errorf("MultiDocValues numeric: no current value")
	}
	return m.current.LongValue()
}

func (m *multiNumericDocValues) Cost() int64 { return 0 }

// ---------------------------------------------------------------------------
// Binary merging iterator.
// ---------------------------------------------------------------------------

// multiBinaryDocValues concatenates the BinaryDocValues of every leaf,
// offsetting each leaf's local docIDs by its docBase. Mirrors the anonymous
// BinaryDocValues in MultiDocValues.getBinaryValues.
type multiBinaryDocValues struct {
	docStarts       []int
	numLeaves       int
	get             func(leafIdx int) (BinaryDocValues, error)
	nextLeaf        int
	current         BinaryDocValues
	currentDocStart int
	docID           int
}

func newMultiBinaryDocValues(docStarts []int, numLeaves int, get func(int) (BinaryDocValues, error)) *multiBinaryDocValues {
	return &multiBinaryDocValues{docStarts: docStarts, numLeaves: numLeaves, get: get, docID: -1}
}

func (m *multiBinaryDocValues) DocID() int { return m.docID }

func (m *multiBinaryDocValues) NextDoc() (int, error) {
	for {
		for m.current == nil {
			if m.nextLeaf == m.numLeaves {
				m.docID = docValuesNoMoreDocs
				return m.docID, nil
			}
			m.currentDocStart = m.docStarts[m.nextLeaf]
			v, err := m.get(m.nextLeaf)
			if err != nil {
				return 0, err
			}
			m.current = v
			m.nextLeaf++
		}
		newDocID, err := m.current.NextDoc()
		if err != nil {
			return 0, err
		}
		if dvExhausted(newDocID) {
			m.current = nil
		} else {
			m.docID = m.currentDocStart + newDocID
			return m.docID, nil
		}
	}
}

func (m *multiBinaryDocValues) Advance(target int) (int, error) {
	if target <= m.docID {
		return 0, fmt.Errorf("MultiDocValues binary: can only advance beyond current doc %d (target %d)", m.docID, target)
	}
	readerIndex := ReaderUtilSubIndex(target, m.docStarts)
	if readerIndex >= m.nextLeaf {
		if readerIndex == m.numLeaves {
			m.current = nil
			m.docID = docValuesNoMoreDocs
			return m.docID, nil
		}
		m.currentDocStart = m.docStarts[readerIndex]
		v, err := m.get(readerIndex)
		if err != nil {
			return 0, err
		}
		m.current = v
		m.nextLeaf = readerIndex + 1
		if m.current == nil {
			return m.NextDoc()
		}
	}
	newDocID, err := m.current.Advance(target - m.currentDocStart)
	if err != nil {
		return 0, err
	}
	if dvExhausted(newDocID) {
		m.current = nil
		return m.NextDoc()
	}
	m.docID = m.currentDocStart + newDocID
	return m.docID, nil
}

func (m *multiBinaryDocValues) AdvanceExact(target int) (bool, error) {
	if target < m.docID {
		return false, fmt.Errorf("MultiDocValues binary: can only advance beyond current doc %d (target %d)", m.docID, target)
	}
	readerIndex := ReaderUtilSubIndex(target, m.docStarts)
	if readerIndex >= m.nextLeaf {
		if readerIndex == m.numLeaves {
			return false, fmt.Errorf("MultiDocValues binary: out of range target %d", target)
		}
		m.currentDocStart = m.docStarts[readerIndex]
		v, err := m.get(readerIndex)
		if err != nil {
			return false, err
		}
		m.current = v
		m.nextLeaf = readerIndex + 1
	}
	m.docID = target
	if m.current == nil {
		return false, nil
	}
	return m.current.AdvanceExact(target - m.currentDocStart)
}

func (m *multiBinaryDocValues) BinaryValue() ([]byte, error) {
	if m.current == nil {
		return nil, fmt.Errorf("MultiDocValues binary: no current value")
	}
	return m.current.BinaryValue()
}

func (m *multiBinaryDocValues) Cost() int64 { return 0 }

// ---------------------------------------------------------------------------
// Sorted-numeric merging iterator.
// ---------------------------------------------------------------------------

// multiSortedNumericDocValues concatenates the SortedNumericDocValues of every
// leaf, offsetting each leaf's local docIDs by its docBase. Per-document value
// access (NextValue / DocValueCount) delegates to the current leaf. Mirrors the
// anonymous SortedNumericDocValues in MultiDocValues.getSortedNumericValues.
type multiSortedNumericDocValues struct {
	docStarts       []int
	numLeaves       int
	get             func(leafIdx int) (SortedNumericDocValues, error)
	nextLeaf        int
	current         SortedNumericDocValues
	currentDocStart int
	docID           int
}

func newMultiSortedNumericDocValues(docStarts []int, numLeaves int, get func(int) (SortedNumericDocValues, error)) *multiSortedNumericDocValues {
	return &multiSortedNumericDocValues{docStarts: docStarts, numLeaves: numLeaves, get: get, docID: -1}
}

func (m *multiSortedNumericDocValues) DocID() int { return m.docID }

func (m *multiSortedNumericDocValues) NextDoc() (int, error) {
	for {
		for m.current == nil {
			if m.nextLeaf == m.numLeaves {
				m.docID = docValuesNoMoreDocs
				return m.docID, nil
			}
			m.currentDocStart = m.docStarts[m.nextLeaf]
			v, err := m.get(m.nextLeaf)
			if err != nil {
				return 0, err
			}
			m.current = v
			m.nextLeaf++
		}
		newDocID, err := m.current.NextDoc()
		if err != nil {
			return 0, err
		}
		if dvExhausted(newDocID) {
			m.current = nil
		} else {
			m.docID = m.currentDocStart + newDocID
			return m.docID, nil
		}
	}
}

func (m *multiSortedNumericDocValues) Advance(target int) (int, error) {
	if target <= m.docID {
		return 0, fmt.Errorf("MultiDocValues sorted-numeric: can only advance beyond current doc %d (target %d)", m.docID, target)
	}
	readerIndex := ReaderUtilSubIndex(target, m.docStarts)
	if readerIndex >= m.nextLeaf {
		if readerIndex == m.numLeaves {
			m.current = nil
			m.docID = docValuesNoMoreDocs
			return m.docID, nil
		}
		m.currentDocStart = m.docStarts[readerIndex]
		v, err := m.get(readerIndex)
		if err != nil {
			return 0, err
		}
		m.current = v
		m.nextLeaf = readerIndex + 1
		if m.current == nil {
			return m.NextDoc()
		}
	}
	newDocID, err := m.current.Advance(target - m.currentDocStart)
	if err != nil {
		return 0, err
	}
	if dvExhausted(newDocID) {
		m.current = nil
		return m.NextDoc()
	}
	m.docID = m.currentDocStart + newDocID
	return m.docID, nil
}

func (m *multiSortedNumericDocValues) AdvanceExact(target int) (bool, error) {
	if target < m.docID {
		return false, fmt.Errorf("MultiDocValues sorted-numeric: can only advance beyond current doc %d (target %d)", m.docID, target)
	}
	readerIndex := ReaderUtilSubIndex(target, m.docStarts)
	if readerIndex >= m.nextLeaf {
		if readerIndex == m.numLeaves {
			return false, fmt.Errorf("MultiDocValues sorted-numeric: out of range target %d", target)
		}
		m.currentDocStart = m.docStarts[readerIndex]
		v, err := m.get(readerIndex)
		if err != nil {
			return false, err
		}
		m.current = v
		m.nextLeaf = readerIndex + 1
	}
	m.docID = target
	if m.current == nil {
		return false, nil
	}
	return m.current.AdvanceExact(target - m.currentDocStart)
}

// LongValue returns the first value of the current document. SortedNumeric
// embeds NumericDocValues; Lucene's SortedNumericDocValues.longValue is not used
// by callers (they use NextValue/DocValueCount), but the embedded contract
// requires it, so delegate to the current leaf.
func (m *multiSortedNumericDocValues) LongValue() (int64, error) {
	if m.current == nil {
		return 0, fmt.Errorf("MultiDocValues sorted-numeric: no current value")
	}
	return m.current.LongValue()
}

func (m *multiSortedNumericDocValues) NextValue() (int64, error) {
	if m.current == nil {
		return 0, fmt.Errorf("MultiDocValues sorted-numeric: no current value")
	}
	return m.current.NextValue()
}

func (m *multiSortedNumericDocValues) DocValueCount() (int, error) {
	if m.current == nil {
		return 0, fmt.Errorf("MultiDocValues sorted-numeric: no current value")
	}
	return m.current.DocValueCount()
}

func (m *multiSortedNumericDocValues) Cost() int64 { return 0 }

// ---------------------------------------------------------------------------
// Sorted merging iterator (global ordinals).
// ---------------------------------------------------------------------------

// MultiSortedDocValues implements SortedDocValues over n sub-readers, mapping
// each segment's local ordinals into one global ordinal space via an
// OrdinalMap. Mirrors org.apache.lucene.index.MultiDocValues.MultiSortedDocValues.
type MultiSortedDocValues struct {
	// values holds the per-leaf SortedDocValues, parallel with docStarts.
	values []SortedDocValues
	// docStarts holds each leaf's docBase followed by maxDoc; len == len(values)+1.
	docStarts []int
	// mapping maps per-segment ordinals to the global ordinal space.
	mapping   *OrdinalMap
	totalCost int64

	nextLeaf        int
	current         SortedDocValues
	currentDocStart int
	docID           int
}

func (m *MultiSortedDocValues) DocID() int { return m.docID }

func (m *MultiSortedDocValues) NextDoc() (int, error) {
	for {
		for m.current == nil {
			if m.nextLeaf == len(m.values) {
				m.docID = docValuesNoMoreDocs
				return m.docID, nil
			}
			m.currentDocStart = m.docStarts[m.nextLeaf]
			m.current = m.values[m.nextLeaf]
			m.nextLeaf++
		}
		newDocID, err := m.current.NextDoc()
		if err != nil {
			return 0, err
		}
		if dvExhausted(newDocID) {
			m.current = nil
		} else {
			m.docID = m.currentDocStart + newDocID
			return m.docID, nil
		}
	}
}

func (m *MultiSortedDocValues) Advance(target int) (int, error) {
	if target <= m.docID {
		return 0, fmt.Errorf("MultiSortedDocValues: can only advance beyond current doc %d (target %d)", m.docID, target)
	}
	readerIndex := ReaderUtilSubIndex(target, m.docStarts)
	if readerIndex >= m.nextLeaf {
		if readerIndex == len(m.values) {
			m.current = nil
			m.docID = docValuesNoMoreDocs
			return m.docID, nil
		}
		m.currentDocStart = m.docStarts[readerIndex]
		m.current = m.values[readerIndex]
		m.nextLeaf = readerIndex + 1
	}
	newDocID, err := m.current.Advance(target - m.currentDocStart)
	if err != nil {
		return 0, err
	}
	if dvExhausted(newDocID) {
		m.current = nil
		return m.NextDoc()
	}
	m.docID = m.currentDocStart + newDocID
	return m.docID, nil
}

func (m *MultiSortedDocValues) AdvanceExact(target int) (bool, error) {
	if target < m.docID {
		return false, fmt.Errorf("MultiSortedDocValues: can only advance beyond current doc %d (target %d)", m.docID, target)
	}
	readerIndex := ReaderUtilSubIndex(target, m.docStarts)
	if readerIndex >= m.nextLeaf {
		if readerIndex == len(m.values) {
			return false, fmt.Errorf("MultiSortedDocValues: out of range target %d", target)
		}
		m.currentDocStart = m.docStarts[readerIndex]
		m.current = m.values[readerIndex]
		m.nextLeaf = readerIndex + 1
	}
	m.docID = target
	if m.current == nil {
		return false, nil
	}
	return m.current.AdvanceExact(target - m.currentDocStart)
}

func (m *MultiSortedDocValues) OrdValue() (int, error) {
	if m.current == nil {
		return 0, fmt.Errorf("MultiSortedDocValues: no current value")
	}
	segOrd, err := m.current.OrdValue()
	if err != nil {
		return 0, err
	}
	globalOrds := m.mapping.GetGlobalOrds(m.nextLeaf - 1)
	if globalOrds == nil || segOrd < 0 || segOrd >= len(globalOrds) {
		return 0, fmt.Errorf("MultiSortedDocValues: segment ord %d out of range for leaf %d", segOrd, m.nextLeaf-1)
	}
	return int(globalOrds[segOrd]), nil
}

func (m *MultiSortedDocValues) LookupOrd(ord int) ([]byte, error) {
	subIndex := m.mapping.GetFirstSegmentNumber(int64(ord))
	segmentOrd := m.mapping.GetFirstSegmentOrd(int64(ord))
	if subIndex < 0 || subIndex >= len(m.values) {
		return nil, fmt.Errorf("MultiSortedDocValues: global ord %d maps to invalid segment %d", ord, subIndex)
	}
	return m.values[subIndex].LookupOrd(int(segmentOrd))
}

func (m *MultiSortedDocValues) GetValueCount() int { return int(m.mapping.GetValueCount()) }

func (m *MultiSortedDocValues) Cost() int64 { return m.totalCost }

func (m *MultiSortedDocValues) LongValue() (int64, error) {
	// SortedDocValues embeds NumericDocValues; LongValue is not part of the
	// sorted contract callers use (they use OrdValue), but the embedded
	// interface requires it. Return the current ord as Lucene's
	// SortedDocValues does not, so surface an explicit error instead.
	return 0, fmt.Errorf("MultiSortedDocValues: LongValue is not supported; use OrdValue")
}

// ---------------------------------------------------------------------------
// Sorted-set merging iterator (global ordinals).
// ---------------------------------------------------------------------------

// MultiSortedSetDocValues implements SortedSetDocValues over n sub-readers,
// mapping each segment's local ordinals into one global ordinal space via an
// OrdinalMap. Mirrors
// org.apache.lucene.index.MultiDocValues.MultiSortedSetDocValues.
type MultiSortedSetDocValues struct {
	values    []SortedSetDocValues
	docStarts []int
	mapping   *OrdinalMap
	totalCost int64

	nextLeaf        int
	current         SortedSetDocValues
	currentDocStart int
	docID           int
}

func (m *MultiSortedSetDocValues) DocID() int { return m.docID }

func (m *MultiSortedSetDocValues) NextDoc() (int, error) {
	for {
		for m.current == nil {
			if m.nextLeaf == len(m.values) {
				m.docID = docValuesNoMoreDocs
				return m.docID, nil
			}
			m.currentDocStart = m.docStarts[m.nextLeaf]
			m.current = m.values[m.nextLeaf]
			m.nextLeaf++
		}
		newDocID, err := m.current.NextDoc()
		if err != nil {
			return 0, err
		}
		if dvExhausted(newDocID) {
			m.current = nil
		} else {
			m.docID = m.currentDocStart + newDocID
			return m.docID, nil
		}
	}
}

func (m *MultiSortedSetDocValues) Advance(target int) (int, error) {
	if target <= m.docID {
		return 0, fmt.Errorf("MultiSortedSetDocValues: can only advance beyond current doc %d (target %d)", m.docID, target)
	}
	readerIndex := ReaderUtilSubIndex(target, m.docStarts)
	if readerIndex >= m.nextLeaf {
		if readerIndex == len(m.values) {
			m.current = nil
			m.docID = docValuesNoMoreDocs
			return m.docID, nil
		}
		m.currentDocStart = m.docStarts[readerIndex]
		m.current = m.values[readerIndex]
		m.nextLeaf = readerIndex + 1
	}
	newDocID, err := m.current.Advance(target - m.currentDocStart)
	if err != nil {
		return 0, err
	}
	if dvExhausted(newDocID) {
		m.current = nil
		return m.NextDoc()
	}
	m.docID = m.currentDocStart + newDocID
	return m.docID, nil
}

func (m *MultiSortedSetDocValues) AdvanceExact(target int) (bool, error) {
	if target < m.docID {
		return false, fmt.Errorf("MultiSortedSetDocValues: can only advance beyond current doc %d (target %d)", m.docID, target)
	}
	readerIndex := ReaderUtilSubIndex(target, m.docStarts)
	if readerIndex >= m.nextLeaf {
		if readerIndex == len(m.values) {
			return false, fmt.Errorf("MultiSortedSetDocValues: out of range target %d", target)
		}
		m.currentDocStart = m.docStarts[readerIndex]
		m.current = m.values[readerIndex]
		m.nextLeaf = readerIndex + 1
	}
	m.docID = target
	if m.current == nil {
		return false, nil
	}
	return m.current.AdvanceExact(target - m.currentDocStart)
}

// NextOrd returns the next global ordinal of the current document, or -1 when
// the document has no more ordinals. The per-segment ordinal returned by the
// current leaf is mapped into the global space via the OrdinalMap.
func (m *MultiSortedSetDocValues) NextOrd() (int, error) {
	if m.current == nil {
		return 0, fmt.Errorf("MultiSortedSetDocValues: no current value")
	}
	segOrd, err := m.current.NextOrd()
	if err != nil {
		return 0, err
	}
	if segOrd == -1 {
		return -1, nil
	}
	globalOrds := m.mapping.GetGlobalOrds(m.nextLeaf - 1)
	if globalOrds == nil || segOrd < 0 || segOrd >= len(globalOrds) {
		return 0, fmt.Errorf("MultiSortedSetDocValues: segment ord %d out of range for leaf %d", segOrd, m.nextLeaf-1)
	}
	return int(globalOrds[segOrd]), nil
}

func (m *MultiSortedSetDocValues) LookupOrd(ord int) ([]byte, error) {
	subIndex := m.mapping.GetFirstSegmentNumber(int64(ord))
	segmentOrd := m.mapping.GetFirstSegmentOrd(int64(ord))
	if subIndex < 0 || subIndex >= len(m.values) {
		return nil, fmt.Errorf("MultiSortedSetDocValues: global ord %d maps to invalid segment %d", ord, subIndex)
	}
	return m.values[subIndex].LookupOrd(int(segmentOrd))
}

func (m *MultiSortedSetDocValues) GetValueCount() int { return int(m.mapping.GetValueCount()) }

func (m *MultiSortedSetDocValues) Cost() int64 { return m.totalCost }

// Compile-time assertions that the merging iterators satisfy their contracts.
var (
	_ NumericDocValues       = (*multiNumericDocValues)(nil)
	_ BinaryDocValues        = (*multiBinaryDocValues)(nil)
	_ SortedNumericDocValues = (*multiSortedNumericDocValues)(nil)
	_ SortedDocValues        = (*MultiSortedDocValues)(nil)
	_ SortedSetDocValues     = (*MultiSortedSetDocValues)(nil)
)
