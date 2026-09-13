package facets

import (
	"fmt"
)

// LabelAndValue holds a single label and its value.
//
// This is the Go port of org.apache.lucene.facet.LabelAndValue.
type LabelAndValue struct {
	// Label is the facet's label.
	Label string

	// Value is the value associated with this label. It renders Lucene's
	// java.lang.Number.
	Value int64

	// Count is the number of occurrences for this label.
	Count int
}

// NewLabelAndValue creates a LabelAndValue with unspecified count; the value is
// assumed to be a count.
//
// Mirrors LabelAndValue(String label, Number value).
func NewLabelAndValue(label string, value int64) *LabelAndValue {
	return &LabelAndValue{
		Label: label,
		Value: value,
		Count: int(value),
	}
}

// NewLabelAndValueWithCount creates a LabelAndValue with value and count.
//
// Mirrors LabelAndValue(String label, Number value, int count).
func NewLabelAndValueWithCount(label string, value int64, count int) *LabelAndValue {
	return &LabelAndValue{
		Label: label,
		Value: value,
		Count: count,
	}
}

// String mirrors LabelAndValue.toString(): label + " (" + value + ")".
func (lv *LabelAndValue) String() string {
	return fmt.Sprintf("%s (%d)", lv.Label, lv.Value)
}

// Equals mirrors LabelAndValue.equals(): labels and values must match.
func (lv *LabelAndValue) Equals(other *LabelAndValue) bool {
	if other == nil {
		return false
	}
	return lv.Label == other.Label && lv.Value == other.Value
}
