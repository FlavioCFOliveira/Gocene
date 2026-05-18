// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import "github.com/FlavioCFOliveira/Gocene/search"

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
