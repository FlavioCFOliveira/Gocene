// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import "github.com/FlavioCFOliveira/Gocene/search"

// DiversifyingChildrenByteKnnVectorQuery is the byte-vector variant of the
// diversifying child-KNN query: at most one child per parent contributes to
// the result so the top-K cannot collapse onto a single parent. Mirrors
// org.apache.lucene.search.join.DiversifyingChildrenByteKnnVectorQuery.
type DiversifyingChildrenByteKnnVectorQuery struct {
	Field         string
	Target        []byte
	K             int
	ChildFilter   search.Query
	ParentsFilter BitSetProducer
}

// NewDiversifyingChildrenByteKnnVectorQuery builds the query descriptor.
func NewDiversifyingChildrenByteKnnVectorQuery(field string, target []byte, k int, childFilter search.Query, parents BitSetProducer) *DiversifyingChildrenByteKnnVectorQuery {
	clone := make([]byte, len(target))
	copy(clone, target)
	return &DiversifyingChildrenByteKnnVectorQuery{
		Field:         field,
		Target:        clone,
		K:             k,
		ChildFilter:   childFilter,
		ParentsFilter: parents,
	}
}
