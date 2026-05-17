// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// RefreshCommitSupplier controls which IndexCommit a searcher is refreshed to.
// Returning nil indicates the latest commit, matching Lucene's default
// behavior.
//
// Mirrors org.apache.lucene.search.RefreshCommitSupplier.
type RefreshCommitSupplier interface {
	// GetSearcherRefreshCommit returns the commit to refresh to, or nil for
	// the latest commit.
	GetSearcherRefreshCommit(reader *index.DirectoryReader) (*index.IndexCommit, error)
}

// DefaultRefreshCommitSupplier returns nil (latest commit).
type DefaultRefreshCommitSupplier struct{}

// GetSearcherRefreshCommit always returns nil for the latest commit.
func (DefaultRefreshCommitSupplier) GetSearcherRefreshCommit(reader *index.DirectoryReader) (*index.IndexCommit, error) {
	return nil, nil
}
