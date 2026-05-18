// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

// ToParentBlockJoinSortFieldType identifies the underlying scalar type the
// sort field operates over. Mirrors
// org.apache.lucene.search.join.ToParentBlockJoinSortField.Type.
type ToParentBlockJoinSortFieldType int

const (
	// SortInt sorts by reduced int32 values.
	SortInt ToParentBlockJoinSortFieldType = iota
	// SortLong sorts by reduced int64 values.
	SortLong
	// SortFloat sorts by reduced float32 values.
	SortFloat
	// SortDouble sorts by reduced float64 values.
	SortDouble
)

// ToParentBlockJoinSortField describes how a parent doc is sorted by an
// aggregate of its children's values. Mirrors
// org.apache.lucene.search.join.ToParentBlockJoinSortField.
type ToParentBlockJoinSortField struct {
	Field    string
	Type     ToParentBlockJoinSortFieldType
	Reverse  bool
	Selector BlockJoinSelectorType
}

// NewToParentBlockJoinSortField builds the sort-field descriptor.
func NewToParentBlockJoinSortField(field string, t ToParentBlockJoinSortFieldType, reverse bool, selector BlockJoinSelectorType) *ToParentBlockJoinSortField {
	return &ToParentBlockJoinSortField{
		Field:    field,
		Type:     t,
		Reverse:  reverse,
		Selector: selector,
	}
}

// IsAscending reports whether sort order is ascending (default).
func (s *ToParentBlockJoinSortField) IsAscending() bool { return !s.Reverse }
