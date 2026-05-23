// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// DiversifyingChildrenFloatKnnVectorQuery is the float32-vector variant.
// Mirrors org.apache.lucene.search.join.DiversifyingChildrenFloatKnnVectorQuery.
type DiversifyingChildrenFloatKnnVectorQuery struct {
	Field         string
	Target        []float32
	K             int
	ChildFilter   search.Query
	ParentsFilter BitSetProducer
}

// NewDiversifyingChildrenFloatKnnVectorQuery builds the descriptor.
func NewDiversifyingChildrenFloatKnnVectorQuery(field string, target []float32, k int, childFilter search.Query, parents BitSetProducer) *DiversifyingChildrenFloatKnnVectorQuery {
	clone := make([]float32, len(target))
	copy(clone, target)
	return &DiversifyingChildrenFloatKnnVectorQuery{
		Field:         field,
		Target:        clone,
		K:             k,
		ChildFilter:   childFilter,
		ParentsFilter: parents,
	}
}

// String returns a human-readable representation.
// Mirrors DiversifyingChildrenFloatKnnVectorQuery.toString.
func (q *DiversifyingChildrenFloatKnnVectorQuery) String() string {
	var sb strings.Builder
	sb.WriteString("DiversifyingChildrenFloatKnnVectorQuery:")
	sb.WriteString(q.Field)
	if len(q.Target) > 0 {
		sb.WriteString(fmt.Sprintf("[%g,...][%d]", q.Target[0], q.K))
	} else {
		sb.WriteString(fmt.Sprintf("[][%d]", q.K))
	}
	if q.ChildFilter != nil {
		sb.WriteString("[")
		sb.WriteString(fmt.Sprintf("%v", q.ChildFilter))
		sb.WriteString("]")
	}
	return sb.String()
}
