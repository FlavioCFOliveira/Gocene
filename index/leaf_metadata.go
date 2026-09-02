// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// LeafMetaData provides read-only metadata about an index leaf. Mirrors
// org.apache.lucene.index.LeafMetaData from Apache Lucene 10.4.0 (a Java
// record). Fields are immutable after construction.
//
// CreatedVersionMajor is the major version of Lucene that created the index.
// A value of 6 means the version is unknown. Values >= 7 require MinVersion
// to be set.
//
// MinVersion is the lowest version that contributed documents to this leaf,
// or nil if the information is not available.
//
// IndexSort describes the per-leaf sort order (nil if the leaf is unsorted).
// Sort is a search-package type and is therefore kept as an opaque interface
// to avoid an import cycle. The canonical implementation will be returned by
// the search.Sort type once that sprint runs.
//
// HasBlocks reports whether the leaf contains document blocks created via
// IndexWriter.AddDocuments / UpdateDocuments. Pre-Lucene-9.9 leaves always
// report false.
type LeafMetaData struct {
	createdVersionMajor int
	minVersion          *util.Version
	indexSort           IndexSort
	hasBlocks           bool
}

// IndexSort is an opaque alias for org.apache.lucene.search.Sort. The full
// type lives in the search package; LeafMetaData only needs to forward an
// optional reference to it.
type IndexSort interface {
	// String returns a textual description for diagnostics.
	String() string
}

// NewLeafMetaData constructs a LeafMetaData and validates the version
// invariants enforced by Lucene's compact constructor:
//
//   - createdVersionMajor must not exceed util.LuceneVersionMajor.
//   - createdVersionMajor must be >= 6.
//   - if createdVersionMajor >= 7, minVersion must be non-nil.
//
// Returns an error rather than panicking, in line with Go conventions.
func NewLeafMetaData(createdVersionMajor int, minVersion *util.Version, sort IndexSort, hasBlocks bool) (*LeafMetaData, error) {
	if createdVersionMajor > util.LuceneVersionMajor {
		return nil, fmt.Errorf("createdVersionMajor is in the future: %d", createdVersionMajor)
	}
	if createdVersionMajor < 6 {
		return nil, fmt.Errorf("createdVersionMajor must be >= 6, got: %d", createdVersionMajor)
	}
	if createdVersionMajor >= 7 && minVersion == nil {
		return nil, fmt.Errorf("minVersion must be set when createdVersionMajor is >= 7")
	}
	return &LeafMetaData{
		createdVersionMajor: createdVersionMajor,
		minVersion:          minVersion,
		indexSort:           sort,
		hasBlocks:           hasBlocks,
	}, nil
}

// CreatedVersionMajor returns the major Lucene version that created this leaf.
func (m *LeafMetaData) CreatedVersionMajor() int { return m.createdVersionMajor }

// MinVersion returns the lowest version that contributed documents to this
// leaf, or nil if it is not recorded.
func (m *LeafMetaData) MinVersion() *util.Version { return m.minVersion }

// Sort returns the leaf's index sort, or nil if the leaf is unsorted.
func (m *LeafMetaData) Sort() IndexSort { return m.indexSort }

// HasBlocks reports whether this leaf contains parent/child document blocks.
func (m *LeafMetaData) HasBlocks() bool { return m.hasBlocks }
