// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// buildComparableProviders returns one ComparableProvider per reader for the
// given SortField. Each provider reads the field's DocValues from its reader
// and encodes every document's sort value as a monotonic int64 comparable in
// the same total order as the configured sort; documents missing the field
// receive the comparable derived from SortField.missingValue.
//
// This is the cross-segment counterpart of the per-segment comparators in
// index_sorter.go (buildIntComparator / buildStringOrdComparator / ...). It
// mirrors the per-type IndexSorter.getComparableProviders subclasses of Apache
// Lucene 10.4.0 (IntSorter / LongSorter / FloatSorter / DoubleSorter /
// StringSorter, plus the multi-valued SortedNumeric/SortedSet selectors).
//
// A non-empty SortField.Selector marks a multi-valued field
// (SortedNumericSortField / SortedSetSortField); the selector ("min" / "max")
// chooses which of a document's values participates in the order.
func buildComparableProviders(sf SortField, readers []*CodecReader) ([]ComparableProvider, error) {
	multiValued := sf.Selector() != ""
	switch sf.SortType() {
	case SortTypeInt, SortTypeLong, SortTypeFloat, SortTypeDouble:
		if multiValued {
			return buildSortedNumericProviders(sf, readers)
		}
		return buildNumericProviders(sf, readers)
	case SortTypeString:
		if multiValued {
			return buildSortedSetProviders(sf, readers)
		}
		return buildSortedProviders(sf, readers)
	default:
		return nil, fmt.Errorf("index: index sort: unsupported sort type %v for field %q", sf.SortType(), sf.Field())
	}
}

// encodeNumericComparable turns the raw long stored for a numeric DocValues
// value into a comparable int64 that orders identically to the logical value.
// INT/LONG store the value directly; FLOAT/DOUBLE store the IEEE-754 bit
// pattern, which is mapped to a sortable integer via NumericUtils so negative
// values order correctly.
func encodeNumericComparable(st SortType, raw int64) int64 {
	switch st {
	case SortTypeFloat:
		return int64(util.FloatToSortableInt(math.Float32frombits(uint32(raw))))
	case SortTypeDouble:
		return util.DoubleToSortableLong(math.Float64frombits(uint64(raw)))
	default:
		return raw
	}
}

// numericMissingComparable encodes SortField.missingValue for a numeric sort.
// The missing value is treated exactly like a present value of the same type,
// so a caller that sets it to the type minimum/maximum gets missing-first /
// missing-last placement; an unset missing value defaults to zero.
func numericMissingComparable(sf SortField) int64 {
	mv := sf.MissingValue()
	switch sf.SortType() {
	case SortTypeFloat:
		var f float32
		switch v := mv.(type) {
		case float32:
			f = v
		case float64:
			f = float32(v)
		}
		return int64(util.FloatToSortableInt(f))
	case SortTypeDouble:
		var d float64
		switch v := mv.(type) {
		case float64:
			d = v
		case float32:
			d = float64(v)
		}
		return util.DoubleToSortableLong(d)
	default:
		switch v := mv.(type) {
		case int64:
			return v
		case int32:
			return int64(v)
		case int:
			return int64(v)
		}
		return 0
	}
}

// stringMissingComparable maps SortField.missingValue for a string sort to a
// comparable that sorts before every present value (STRING_FIRST) or after
// every present value (STRING_LAST). Present comparables are non-negative
// global ordinals, so the int64 extremes are always outside their range.
func stringMissingComparable(sf SortField) int64 {
	if stringMissingFirst(sf.MissingValue()) {
		return math.MinInt64
	}
	return math.MaxInt64
}

// stringMissingFirst reports whether a string sort's missing value requests
// missing-first placement. Gocene accepts the sentinel string "STRING_FIRST"
// and the empty byte slice (the representation used by the index-sorting test
// suite) as "first"; everything else, including a nil missing value, defaults
// to missing-last — matching Apache Lucene's SortField.STRING_FIRST /
// STRING_LAST semantics (the absolute first/last position, independent of the
// reverse flag, which MultiSorter applies on top via the reverse multiplier).
func stringMissingFirst(mv interface{}) bool {
	switch v := mv.(type) {
	case string:
		return v == "STRING_FIRST"
	case []byte:
		return len(v) == 0
	}
	return false
}

func buildNumericProviders(sf SortField, readers []*CodecReader) ([]ComparableProvider, error) {
	missing := numericMissingComparable(sf)
	providers := make([]ComparableProvider, len(readers))
	for idx, reader := range readers {
		vals, err := materializeNumeric(sf, reader, missing)
		if err != nil {
			return nil, err
		}
		v := vals
		providers[idx] = func(docID int) (int64, error) {
			if docID < 0 || docID >= len(v) {
				return missing, nil
			}
			return v[docID], nil
		}
	}
	return providers, nil
}

func materializeNumeric(sf SortField, reader *CodecReader, missing int64) ([]int64, error) {
	maxDoc := 0
	if reader != nil {
		maxDoc = reader.MaxDoc()
	}
	vals := make([]int64, maxDoc)
	for d := range vals {
		vals[d] = missing
	}
	if reader == nil {
		return vals, nil
	}
	prod := dvProducerOf(reader)
	fi := subFieldInfo(reader, sf.Field())
	if prod == nil || fi == nil {
		return vals, nil
	}
	ndv, err := prod.GetNumeric(fi)
	if err != nil {
		return nil, err
	}
	if ndv == nil {
		return vals, nil
	}
	for {
		d, err := ndv.NextDoc()
		if err != nil {
			return nil, err
		}
		if dvExhaustedDoc(d, maxDoc) {
			break
		}
		raw, err := ndv.LongValue()
		if err != nil {
			return nil, err
		}
		vals[d] = encodeNumericComparable(sf.SortType(), raw)
	}
	return vals, nil
}

func buildSortedNumericProviders(sf SortField, readers []*CodecReader) ([]ComparableProvider, error) {
	missing := numericMissingComparable(sf)
	useMax := sf.Selector() == "max"
	providers := make([]ComparableProvider, len(readers))
	for idx, reader := range readers {
		vals, err := materializeSortedNumeric(sf, reader, missing, useMax)
		if err != nil {
			return nil, err
		}
		v := vals
		providers[idx] = func(docID int) (int64, error) {
			if docID < 0 || docID >= len(v) {
				return missing, nil
			}
			return v[docID], nil
		}
	}
	return providers, nil
}

func materializeSortedNumeric(sf SortField, reader *CodecReader, missing int64, useMax bool) ([]int64, error) {
	maxDoc := 0
	if reader != nil {
		maxDoc = reader.MaxDoc()
	}
	vals := make([]int64, maxDoc)
	for d := range vals {
		vals[d] = missing
	}
	if reader == nil {
		return vals, nil
	}
	prod := dvProducerOf(reader)
	fi := subFieldInfo(reader, sf.Field())
	if prod == nil || fi == nil {
		return vals, nil
	}
	sdv, err := prod.GetSortedNumeric(fi)
	if err != nil {
		return nil, err
	}
	if sdv == nil {
		return vals, nil
	}
	for {
		d, err := sdv.NextDoc()
		if err != nil {
			return nil, err
		}
		if dvExhaustedDoc(d, maxDoc) {
			break
		}
		count, err := sdv.DocValueCount()
		if err != nil {
			return nil, err
		}
		// SortedNumeric values are emitted in ascending order, so the first
		// value is the minimum and the last is the maximum. The stored values
		// are already in sortable form (int/long verbatim; float/double encoded
		// at index time via NumericUtils.{float,double}ToSortable*), so the
		// selected value is itself the comparable — no further encoding.
		var sel int64
		for j := 0; j < count; j++ {
			val, err := sdv.NextValue()
			if err != nil {
				return nil, err
			}
			if j == 0 || useMax {
				sel = val
			}
		}
		if count > 0 {
			vals[d] = sel
		}
	}
	return vals, nil
}

func buildSortedProviders(sf SortField, readers []*CodecReader) ([]ComparableProvider, error) {
	missing := stringMissingComparable(sf)

	// Build a global OrdinalMap across the readers that carry the field, then
	// reopen each field iterator to materialise per-doc global ordinals.
	var omSubs []SortedDocValues
	subPos := make([]int, len(readers))
	for i := range subPos {
		subPos[i] = -1
	}
	for i, reader := range readers {
		if reader == nil {
			continue
		}
		prod := dvProducerOf(reader)
		fi := subFieldInfo(reader, sf.Field())
		if prod == nil || fi == nil {
			continue
		}
		sdv, err := prod.GetSorted(fi)
		if err != nil {
			return nil, err
		}
		if sdv == nil {
			continue
		}
		subPos[i] = len(omSubs)
		omSubs = append(omSubs, sdv)
	}

	providers := make([]ComparableProvider, len(readers))
	if len(omSubs) == 0 {
		for i := range providers {
			providers[i] = func(int) (int64, error) { return missing, nil }
		}
		return providers, nil
	}
	om, err := BuildOrdinalMapFromSortedValues(NewCacheKey(), omSubs, 0)
	if err != nil {
		return nil, fmt.Errorf("index: index sort: sorted %q ordinal map: %w", sf.Field(), err)
	}

	for i, reader := range readers {
		maxDoc := 0
		if reader != nil {
			maxDoc = reader.MaxDoc()
		}
		vals := make([]int64, maxDoc)
		for d := range vals {
			vals[d] = missing
		}
		if p := subPos[i]; p >= 0 {
			prod := dvProducerOf(reader)
			fi := subFieldInfo(reader, sf.Field())
			sdv, err := prod.GetSorted(fi) // fresh iterator (OM build consumed the first)
			if err != nil {
				return nil, err
			}
			globals := om.GetGlobalOrds(p)
			for {
				d, err := sdv.NextDoc()
				if err != nil {
					return nil, err
				}
				if dvExhaustedDoc(d, maxDoc) {
					break
				}
				so, err := sdv.OrdValue()
				if err != nil {
					return nil, err
				}
				vals[d] = globals[so]
			}
		}
		v := vals
		providers[i] = func(docID int) (int64, error) {
			if docID < 0 || docID >= len(v) {
				return missing, nil
			}
			return v[docID], nil
		}
	}
	return providers, nil
}

func buildSortedSetProviders(sf SortField, readers []*CodecReader) ([]ComparableProvider, error) {
	missing := stringMissingComparable(sf)
	useMax := sf.Selector() == "max"

	var omSubs []SortedSetDocValues
	subPos := make([]int, len(readers))
	for i := range subPos {
		subPos[i] = -1
	}
	for i, reader := range readers {
		if reader == nil {
			continue
		}
		prod := dvProducerOf(reader)
		fi := subFieldInfo(reader, sf.Field())
		if prod == nil || fi == nil {
			continue
		}
		ssdv, err := prod.GetSortedSet(fi)
		if err != nil {
			return nil, err
		}
		if ssdv == nil {
			continue
		}
		subPos[i] = len(omSubs)
		omSubs = append(omSubs, ssdv)
	}

	providers := make([]ComparableProvider, len(readers))
	if len(omSubs) == 0 {
		for i := range providers {
			providers[i] = func(int) (int64, error) { return missing, nil }
		}
		return providers, nil
	}
	om, err := BuildOrdinalMapFromSortedSetValues(NewCacheKey(), omSubs, 0)
	if err != nil {
		return nil, fmt.Errorf("index: index sort: sorted-set %q ordinal map: %w", sf.Field(), err)
	}

	for i, reader := range readers {
		maxDoc := 0
		if reader != nil {
			maxDoc = reader.MaxDoc()
		}
		vals := make([]int64, maxDoc)
		for d := range vals {
			vals[d] = missing
		}
		if p := subPos[i]; p >= 0 {
			prod := dvProducerOf(reader)
			fi := subFieldInfo(reader, sf.Field())
			ssdv, err := prod.GetSortedSet(fi) // fresh iterator
			if err != nil {
				return nil, err
			}
			globals := om.GetGlobalOrds(p)
			for {
				d, err := ssdv.NextDoc()
				if err != nil {
					return nil, err
				}
				if dvExhaustedDoc(d, maxDoc) {
					break
				}
				// Ordinals are emitted ascending, so the first global ord is
				// the minimum and the last is the maximum.
				var sel int64
				have := false
				for {
					so, err := ssdv.NextOrd()
					if err != nil {
						return nil, err
					}
					if so < 0 {
						break
					}
					g := globals[so]
					if !have || useMax {
						sel = g
						have = true
					}
				}
				if have {
					vals[d] = sel
				}
			}
		}
		v := vals
		providers[i] = func(docID int) (int64, error) {
			if docID < 0 || docID >= len(v) {
				return missing, nil
			}
			return v[docID], nil
		}
	}
	return providers, nil
}
