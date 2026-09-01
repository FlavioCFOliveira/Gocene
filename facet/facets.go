// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"errors"
)

// Facets is the common base for all facets implementations.
//
// This is the Go port of Lucene's org.apache.lucene.facet.Facets.
type Facets interface {
	// GetAllChildren returns all child labels with non-zero counts under the specified path.
	// Users should make no assumptions about ordering of the children.
	// Returns nil if the specified path doesn't exist or if this dimension was never seen.
	GetAllChildren(dim string, path ...string) (*FacetResult, error)

	// GetTopChildren returns the topN child labels under the specified path.
	// Returns nil if the specified path doesn't exist or if this dimension was never seen.
	GetTopChildren(topN int, dim string, path ...string) (*FacetResult, error)

	// GetSpecificValue returns the count or value for a specific path.
	// Returns -1 if this path doesn't exist, else the count.
	GetSpecificValue(dim string, path ...string) (interface{}, error)

	// GetAllDims returns topN labels for any dimension that had hits, sorted by the
	// number of hits that dimension matched.
	GetAllDims(topN int) ([]*FacetResult, error)

	// GetTopDims returns labels for topN dimensions and their topNChildren sorted by the
	// number of hits/aggregated values that dimension matched.
	GetTopDims(topNDims, topNChildren int) ([]*FacetResult, error)
}

// ValidateTopN checks if topN is valid for GetTopChildren and GetAllDims.
func ValidateTopN(topN int) error {
	if topN <= 0 {
		return errors.New("topN must be > 0")
	}
	return nil
}
