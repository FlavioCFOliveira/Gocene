// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// GroupSelector is used to determine the group for a document.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.GroupSelector.
type GroupSelector interface {
	// GetGroup returns the group key for the given document.
	GetGroup(context *index.LeafReaderContext, doc int) (interface{}, bool)
}

// GroupReducer is used to reduce a group of documents to a smaller set.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.GroupReducer.
type GroupReducer interface {
	// Reduce reduces the group of documents.
	Reduce(group interface{}, docs []int) []int
}
