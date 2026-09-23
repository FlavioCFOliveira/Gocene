// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// RefreshCommitSupplier is the Go port of
// org.apache.lucene.search.RefreshCommitSupplier (Apache Lucene 10.5.0).
//
// Expert: Interface to supply commit for searcher refresh.
//
// @lucene.experimental
type RefreshCommitSupplier interface {
	// GetSearcherRefreshCommit returns the index commit that searcher should
	// refresh on. A nil return value (default) indicates reader should
	// refresh on the latest commit.
	GetSearcherRefreshCommit(reader *index.DirectoryReader) (*index.IndexCommit, error)
}

// DefaultRefreshCommitSupplier carries the default method body of
// RefreshCommitSupplier.getSearcherRefreshCommit; it renders the anonymous
// `new RefreshCommitSupplier() {}`, and implementations embed it to inherit
// the default.
type DefaultRefreshCommitSupplier struct{}

// GetSearcherRefreshCommit renders the default body: it returns nil, the
// latest commit.
func (DefaultRefreshCommitSupplier) GetSearcherRefreshCommit(reader *index.DirectoryReader) (*index.IndexCommit, error) {
	return nil, nil
}
