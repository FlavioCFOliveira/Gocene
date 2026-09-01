// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SearchGroup represents a group of documents.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.SearchGroup.
type SearchGroup struct {
	GroupKey interface{}
	Docs     []int
}

// CollectedSearchGroup represents a group of documents collected during search.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.CollectedSearchGroup.
type CollectedSearchGroup struct {
	GroupKey interface{}
	Docs     []int
}

// GroupDocs provides access to documents in a group.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.GroupDocs.
type GroupDocs struct {
	Docs []int
}

func (gd *GroupDocs) GetDoc(idx int) int {
	return gd.Docs[idx]
}

func (gd *GroupDocs) Size() int {
	return len(gd.Docs)
}
