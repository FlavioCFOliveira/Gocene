// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"math"
	"sort"
)

// IndexSorter sorts documents during flush and merge operations.
//
// This is the Go port of Lucene's org.apache.lucene.index.IndexSorter.
//
// IndexSorter is responsible for sorting documents based on a configured
// Sort when flushing or merging segments. This enables creating sorted
// indexes where documents are stored in a specific order.
type IndexSorter struct {
	sort *Sort
}

// NewIndexSorter creates a new IndexSorter with the given sort.
func NewIndexSorter(sort *Sort) *IndexSorter {
	return &IndexSorter{
		sort: sort,
	}
}

// GetSort returns the sort specification.
func (s *IndexSorter) GetSort() *Sort {
	return s.sort
}

// SetSort sets the sort specification.
func (s *IndexSorter) SetSort(sort *Sort) {
	s.sort = sort
}

// SortSegment sorts the documents in a segment according to the sort
// specification and returns a mapping from old doc IDs to new doc IDs.
//
// When no sort is configured, or the sort has no fields, the identity mapping
// is returned (no reordering). Otherwise, each configured sort field is
// consulted in priority order: NumericDocValues are loaded for numeric fields,
// SortedDocValues ordinals are loaded for string/sorted fields, and
// documents missing a value are placed at the position indicated by
// SortField.missingValue.
//
// This mirrors the per-segment sorting step of Lucene's
// org.apache.lucene.index.IndexSorter.IntSorter / LongSorter.getDocComparator.
func (s *IndexSorter) SortSegment(reader *LeafReader) ([]int, error) {
	numDocs := reader.MaxDoc()

	if s.sort == nil || len(s.sort.fields) == 0 {
		return identityMapping(numDocs), nil
	}

	// Build a doc-ID slice [0, numDocs) and sort it using a multi-level
	// comparator that walks the sort fields in declaration order.
	docIDs := identityMapping(numDocs)

	// Preload per-field value arrays for all sort fields so the sort
	// comparator itself allocates nothing.
	cmpFuncs, err := buildFieldComparators(reader, s.sort.fields, numDocs)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(docIDs, func(i, j int) bool {
		for _, cmp := range cmpFuncs {
			c := cmp(docIDs[i], docIDs[j])
			if c != 0 {
				return c < 0
			}
		}
		return false
	})

	// Build inverse mapping: old doc ID → new position.
	// Lucene's convention for SortSegment is that mapping[oldDocID] = newDocID.
	mapping := make([]int, numDocs)
	for newPos, oldDocID := range docIDs {
		mapping[oldDocID] = newPos
	}
	return mapping, nil
}

// NeedsSorting returns true if the segment needs to be sorted.
func (s *IndexSorter) NeedsSorting(reader *LeafReader) bool {
	if s.sort == nil || len(s.sort.fields) == 0 {
		return false
	}
	return true
}

// SortType returns the name of this sorter.
func (s *IndexSorter) SortType() string {
	if s.sort == nil {
		return "none"
	}
	return "custom"
}

// identityMapping returns a slice [0, n).
func identityMapping(n int) []int {
	m := make([]int, n)
	for i := range m {
		m[i] = i
	}
	return m
}

// docCompareFn compares two doc IDs for a single sort dimension.
// Returns negative / zero / positive like a standard comparator.
type docCompareFn func(a, b int) int

// buildFieldComparators preloads the per-field value arrays and returns one
// comparator per SortField.  Fields for which DocValues are unavailable
// (GetNumericDocValues / GetSortedDocValues returns nil) fall back to
// identity ordering for that dimension, which is correct: all docs sort equal
// on that field and the next field breaks the tie.
func buildFieldComparators(reader *LeafReader, fields []SortField, numDocs int) ([]docCompareFn, error) {
	cmps := make([]docCompareFn, 0, len(fields))
	for _, sf := range fields {
		cmp, err := buildFieldComparator(reader, sf, numDocs)
		if err != nil {
			return nil, err
		}
		cmps = append(cmps, cmp)
	}
	return cmps, nil
}

// buildFieldComparator builds a single doc comparator for one SortField.
func buildFieldComparator(reader *LeafReader, sf SortField, numDocs int) (docCompareFn, error) {
	reverseMul := 1
	if sf.descending {
		reverseMul = -1
	}

	switch sf.sortType {
	case SortTypeInt:
		return buildIntComparator(reader, sf, numDocs, reverseMul)
	case SortTypeLong:
		return buildLongComparator(reader, sf, numDocs, reverseMul)
	case SortTypeFloat:
		return buildFloatComparator(reader, sf, numDocs, reverseMul)
	case SortTypeDouble:
		return buildDoubleComparator(reader, sf, numDocs, reverseMul)
	case SortTypeString:
		return buildStringOrdComparator(reader, sf, numDocs, reverseMul)
	default:
		// Unknown sort type: identity comparator (all equal on this dimension).
		return func(a, b int) int { return 0 }, nil
	}
}

func buildIntComparator(reader *LeafReader, sf SortField, numDocs, reverseMul int) (docCompareFn, error) {
	values := make([]int32, numDocs)
	var missingVal int32
	if mv, ok := sf.missingValue.(int32); ok {
		missingVal = mv
	} else if mv, ok := sf.missingValue.(int64); ok {
		missingVal = int32(mv)
	}
	for i := range values {
		values[i] = missingVal
	}

	dvs, err := reader.GetNumericDocValues(sf.field)
	if err != nil {
		return nil, err
	}
	if dvs != nil {
		for {
			docID, err := dvs.NextDoc()
			if err != nil {
				return nil, err
			}
			if docID < 0 || docID >= numDocs {
				break
			}
			v, err := dvs.Get(docID)
			if err != nil {
				return nil, err
			}
			values[docID] = int32(v)
		}
	}

	return func(a, b int) int {
		va, vb := values[a], values[b]
		if va < vb {
			return -1 * reverseMul
		}
		if va > vb {
			return 1 * reverseMul
		}
		return 0
	}, nil
}

func buildLongComparator(reader *LeafReader, sf SortField, numDocs, reverseMul int) (docCompareFn, error) {
	values := make([]int64, numDocs)
	var missingVal int64
	if mv, ok := sf.missingValue.(int64); ok {
		missingVal = mv
	}
	for i := range values {
		values[i] = missingVal
	}

	dvs, err := reader.GetNumericDocValues(sf.field)
	if err != nil {
		return nil, err
	}
	if dvs != nil {
		for {
			docID, err := dvs.NextDoc()
			if err != nil {
				return nil, err
			}
			if docID < 0 || docID >= numDocs {
				break
			}
			v, err := dvs.Get(docID)
			if err != nil {
				return nil, err
			}
			values[docID] = v
		}
	}

	return func(a, b int) int {
		va, vb := values[a], values[b]
		if va < vb {
			return -1 * reverseMul
		}
		if va > vb {
			return 1 * reverseMul
		}
		return 0
	}, nil
}

func buildFloatComparator(reader *LeafReader, sf SortField, numDocs, reverseMul int) (docCompareFn, error) {
	values := make([]float32, numDocs)
	var missingVal float32
	if mv, ok := sf.missingValue.(float32); ok {
		missingVal = mv
	} else if mv, ok := sf.missingValue.(float64); ok {
		missingVal = float32(mv)
	}
	for i := range values {
		values[i] = missingVal
	}

	dvs, err := reader.GetNumericDocValues(sf.field)
	if err != nil {
		return nil, err
	}
	if dvs != nil {
		for {
			docID, err := dvs.NextDoc()
			if err != nil {
				return nil, err
			}
			if docID < 0 || docID >= numDocs {
				break
			}
			// Float values are stored as int32 bit patterns (Float.floatToRawIntBits).
			raw, err := dvs.Get(docID)
			if err != nil {
				return nil, err
			}
			values[docID] = math.Float32frombits(uint32(raw))
		}
	}

	return func(a, b int) int {
		va, vb := values[a], values[b]
		if va < vb {
			return -1 * reverseMul
		}
		if va > vb {
			return 1 * reverseMul
		}
		return 0
	}, nil
}

func buildDoubleComparator(reader *LeafReader, sf SortField, numDocs, reverseMul int) (docCompareFn, error) {
	values := make([]float64, numDocs)
	var missingVal float64
	if mv, ok := sf.missingValue.(float64); ok {
		missingVal = mv
	}
	for i := range values {
		values[i] = missingVal
	}

	dvs, err := reader.GetNumericDocValues(sf.field)
	if err != nil {
		return nil, err
	}
	if dvs != nil {
		for {
			docID, err := dvs.NextDoc()
			if err != nil {
				return nil, err
			}
			if docID < 0 || docID >= numDocs {
				break
			}
			// Double values are stored as int64 bit patterns (Double.doubleToRawLongBits).
			raw, err := dvs.Get(docID)
			if err != nil {
				return nil, err
			}
			values[docID] = math.Float64frombits(uint64(raw))
		}
	}

	return func(a, b int) int {
		va, vb := values[a], values[b]
		if va < vb {
			return -1 * reverseMul
		}
		if va > vb {
			return 1 * reverseMul
		}
		return 0
	}, nil
}

func buildStringOrdComparator(reader *LeafReader, sf SortField, numDocs, reverseMul int) (docCompareFn, error) {
	// String fields are sorted by their SortedDocValues ordinal.
	// Ordinal -1 means "no value"; the placement relative to present values is
	// controlled by SortField.missingValue (SortField.STRING_FIRST or STRING_LAST).
	// Gocene uses the missingValue field with sentinel strings "STRING_FIRST" / "STRING_LAST".
	// Missing docs sort last (highest ordinal equivalent) unless STRING_FIRST.
	missingFirst := false
	if mv, ok := sf.missingValue.(string); ok && mv == "STRING_FIRST" {
		missingFirst = true
	}

	ords := make([]int, numDocs)
	for i := range ords {
		if missingFirst {
			ords[i] = -1 // will sort first (smallest)
		} else {
			ords[i] = int(^uint(0) >> 1) // MaxInt, will sort last
		}
	}

	dvs, err := reader.GetSortedDocValues(sf.field)
	if err != nil {
		return nil, err
	}
	if dvs != nil {
		for {
			docID, err := dvs.NextDoc()
			if err != nil {
				return nil, err
			}
			if docID < 0 || docID >= numDocs {
				break
			}
			ord, err := dvs.GetOrd(docID)
			if err != nil {
				return nil, err
			}
			ords[docID] = ord
		}
	}

	return func(a, b int) int {
		va, vb := ords[a], ords[b]
		if va < vb {
			return -1 * reverseMul
		}
		if va > vb {
			return 1 * reverseMul
		}
		return 0
	}, nil
}
