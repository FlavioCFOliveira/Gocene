// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"fmt"
)

// LabelAndValue represents a single label and its value.
//
// This is the Go port of Lucene's org.apache.lucene.facet.LabelAndValue.
type LabelAndValue struct {
	Label string
	Value interface{} // Use interface{} to represent Lucene's Number
	Count int
}

// NewLabelAndValue creates a new LabelAndValue with unspecified count (assumed to be the value).
func NewLabelAndValue(label string, value interface{}) *LabelAndValue {
	count := 0
	switch v := value.(type) {
	case int:
		count = v
	case int64:
		count = int(v)
	case float32:
		count = int(v)
	case float64:
		count = int(v)
	}
	return &LabelAndValue{
		Label: label,
		Value: value,
		Count: count,
	}
}

// NewLabelAndValueWithCount creates a new LabelAndValue with a specified count.
func NewLabelAndValueWithCount(label string, value interface{}, count int) *LabelAndValue {
	return &LabelAndValue{
		Label: label,
		Value: value,
		Count: count,
	}
}

func (l *LabelAndValue) String() string {
	return fmt.Sprintf("%s (%v)", l.Label, l.Value)
}
