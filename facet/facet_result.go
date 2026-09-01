// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"fmt"
	"strings"
)

// FacetResult contains counts or aggregates for a single dimension.
//
// This is the Go port of Lucene's org.apache.lucene.facet.FacetResult.
type FacetResult struct {
	Dim         string
	Path        []string
	Value       interface{}
	ChildCount  int
	LabelValues []*LabelAndValue
}

// NewFacetResult creates a new FacetResult.
func NewFacetResult(dim string, path []string, value interface{}, labelValues []*LabelAndValue, childCount int) *FacetResult {
	return &FacetResult{
		Dim:         dim,
		Path:        path,
		Value:       value,
		LabelValues: labelValues,
		ChildCount:  childCount,
	}
}

func (f *FacetResult) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("dim=%s path=%v value=%v childCount=%d\n", f.Dim, f.Path, f.Value, f.ChildCount))
	for _, lv := range f.LabelValues {
		sb.WriteString(fmt.Sprintf("  %s\n", lv.String()))
	}
	return sb.String()
}
