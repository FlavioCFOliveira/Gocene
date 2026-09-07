package facets

import (
	"fmt"
	"strings"
)

// FacetResult contains counts or aggregates for a single dimension.
type FacetResult struct {
	// Dim is the dimension that was requested.
	Dim string

	// Path is the path whose children were requested.
	Path []string

	// Value is the total number of documents containing a value for this path,
	// even those not included in the topN. If a document contains multiple values
	// for the same path, it will only be counted once in this value.
	Value float64

	// ChildCount is how many child labels were encountered.
	ChildCount int

	// LabelValues contains the child labels and their corresponding values.
	LabelValues []LabelAndValue
}

// NewFacetResult creates a new FacetResult.
func NewFacetResult(dim string, path []string, value float64, labelValues []LabelAndValue, childCount int) *FacetResult {
	return &FacetResult{
		Dim:         dim,
		Path:        path,
		Value:       value,
		LabelValues: labelValues,
		ChildCount:  childCount,
	}
}

func (fr *FacetResult) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "dim=%s path=%v value=%v childCount=%d\n", fr.Dim, fr.Path, fr.Value, fr.ChildCount)
	for _, lv := range fr.LabelValues {
		fmt.Fprintf(&sb, "  %s\n", lv.String())
	}
	return sb.String()
}
