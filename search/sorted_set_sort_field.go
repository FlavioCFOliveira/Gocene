// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/SortedSetSortField.java

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// SortedSetSortField is a SortField for SortedSetDocValues. It "selects" a
// representative sort value from the document's multi-valued set.
//
// By default (SortedSetSelectorMin) the minimum value in the set is selected.
// Like sorting by string, this supports sorting missing values (the empty set)
// first or last via SetMissingValue.
//
// Mirrors org.apache.lucene.search.SortedSetSortField (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java's abstract SortFieldProvider / SortField.getIndexSorter is not yet
//     wired; Serialize / ReadSortedSetSortField are package-private helpers
//     that callers can invoke directly when the provider infrastructure lands.
//   - getComparator is a stub (TermOrdValComparator has not yet been deep-ported).
//   - Java uses CUSTOM sort type; Go keeps SortFieldTypeCustom for the same.
type SortedSetSortField struct {
	*SortField
	selector SortedSetSelectorType
}

// NewSortedSetSortField creates a sort, possibly in reverse, by the minimum
// value in the set for the document.
//
// Mirrors SortedSetSortField(String, boolean).
func NewSortedSetSortField(field string, reverse bool) *SortedSetSortField {
	return newSortedSetSortField(field, reverse, SortedSetSelectorMin, nil)
}

// NewSortedSetSortFieldWithSelector creates a sort with an explicit selector.
//
// Mirrors SortedSetSortField(String, boolean, SortedSetSelector.Type).
func NewSortedSetSortFieldWithSelector(field string, reverse bool, selector SortedSetSelectorType) *SortedSetSortField {
	return newSortedSetSortField(field, reverse, selector, nil)
}

// NewSortedSetSortFieldFull creates a sort with an explicit selector and
// initial missing-value sentinel.
//
// Mirrors SortedSetSortField(String, boolean, SortedSetSelector.Type, Object).
func NewSortedSetSortFieldFull(field string, reverse bool, selector SortedSetSelectorType, missingValue interface{}) *SortedSetSortField {
	return newSortedSetSortField(field, reverse, selector, missingValue)
}

func newSortedSetSortField(field string, reverse bool, selector SortedSetSelectorType, missingValue interface{}) *SortedSetSortField {
	sf := &SortField{
		Field:        field,
		Type:         SortFieldTypeCustom,
		Reverse:      reverse,
		MissingValue: missingValue,
	}
	return &SortedSetSortField{SortField: sf, selector: selector}
}

// GetSelector returns the selector used to pick the representative value from
// each document's set.
//
// Mirrors SortedSetSortField.getSelector().
func (s *SortedSetSortField) GetSelector() SortedSetSelectorType {
	return s.selector
}

// SetMissingValue sets how documents with an empty set are sorted.
// Only STRING_FIRST and STRING_LAST are valid sentinels.
//
// Mirrors SortedSetSortField.setMissingValue(Object).
func (s *SortedSetSortField) SetMissingValue(v interface{}) error {
	if v != STRING_FIRST && v != STRING_LAST {
		return fmt.Errorf("for SORTED_SET type, missing value must be either STRING_FIRST or STRING_LAST")
	}
	s.SortField.MissingValue = v
	return nil
}

// HashCode returns a hash that incorporates the parent SortField hash and the
// selector.
//
// Mirrors SortedSetSortField.hashCode().
func (s *SortedSetSortField) HashCode() int {
	return 31*sortFieldHashCode(s.SortField) + int(s.selector)
}

// sortFieldHashCode computes a stable hash for a SortField without using
// MissingValue identity (consistent with Java Objects.hashCode semantics).
func sortFieldHashCode(sf *SortField) int {
	h := 1
	h = 31*h + int(sf.Type)
	if sf.Reverse {
		h = 31*h + 1231
	} else {
		h = 31*h + 1237
	}
	if sf.Field != "" {
		for _, c := range sf.Field {
			h = 31*h + int(c)
		}
	}
	return h
}

// Equals reports whether other is a *SortedSetSortField with the same field,
// reverse flag, selector, and missing-value sentinel.
//
// Mirrors SortedSetSortField.equals(Object).
func (s *SortedSetSortField) Equals(other interface{}) bool {
	if s == other {
		return true
	}
	o, ok := other.(*SortedSetSortField)
	if !ok || o == nil {
		return false
	}
	if s.SortField.Field != o.SortField.Field {
		return false
	}
	if s.SortField.Reverse != o.SortField.Reverse {
		return false
	}
	if s.selector != o.selector {
		return false
	}
	return s.SortField.MissingValue == o.SortField.MissingValue
}

// String returns a human-readable description.
//
// Mirrors SortedSetSortField.toString().
func (s *SortedSetSortField) String() string {
	out := fmt.Sprintf(`<sortedset: "%s">`, s.SortField.Field)
	if s.SortField.Reverse {
		out += "!"
	}
	if s.SortField.MissingValue != nil {
		out += " missingValue=" + missingValueString(s.SortField.MissingValue)
	}
	out += " selector=" + s.selector.String()
	return out
}

// missingValueString returns the string representation of a missing-value
// sentinel, mirroring Java's sentinel .toString().
func missingValueString(v interface{}) string {
	if v == STRING_FIRST {
		return "STRING_FIRST"
	}
	if v == STRING_LAST {
		return "STRING_LAST"
	}
	return fmt.Sprintf("%v", v)
}

// Serialize writes this field to out in the format expected by
// SortedSetSortField.Provider.readSortField.
//
// Mirrors SortedSetSortField.serialize(DataOutput).
func (s *SortedSetSortField) Serialize(out store.DataOutput) error {
	if err := out.WriteString(s.SortField.Field); err != nil {
		return err
	}
	reverseInt := int32(0)
	if s.SortField.Reverse {
		reverseInt = 1
	}
	if err := out.WriteInt(reverseInt); err != nil {
		return err
	}
	if err := out.WriteInt(int32(s.selector)); err != nil {
		return err
	}
	mv := int32(0)
	if s.SortField.MissingValue == STRING_FIRST {
		mv = 1
	} else if s.SortField.MissingValue == STRING_LAST {
		mv = 2
	}
	return out.WriteInt(mv)
}

// ReadSortedSetSortField deserializes a SortedSetSortField from in.
//
// Mirrors SortedSetSortField.Provider.readSortField(DataInput).
func ReadSortedSetSortField(in store.DataInput) (*SortedSetSortField, error) {
	field, err := in.ReadString()
	if err != nil {
		return nil, err
	}
	reverseInt, err := in.ReadInt()
	if err != nil {
		return nil, err
	}
	selectorOrd, err := in.ReadInt()
	if err != nil {
		return nil, err
	}
	numSelectorTypes := int32(4) // MIN, MAX, MIDDLE_MIN, MIDDLE_MAX
	if selectorOrd < 0 || selectorOrd >= numSelectorTypes {
		return nil, fmt.Errorf("cannot deserialize SortedSetSortField: unknown selector type %d", selectorOrd)
	}
	missingOrd, err := in.ReadInt()
	if err != nil {
		return nil, err
	}
	var missingValue interface{}
	switch missingOrd {
	case 1:
		missingValue = STRING_FIRST
	case 2:
		missingValue = STRING_LAST
	}
	return newSortedSetSortField(field, reverseInt == 1, SortedSetSelectorType(selectorOrd), missingValue), nil
}
