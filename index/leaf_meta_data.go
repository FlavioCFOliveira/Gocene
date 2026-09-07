// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// LeafMetaData provides read-only metadata about a leaf.
type LeafMetaData struct {
	// CreatedVersionMajor is the Lucene version that created this index.
	// This can be used to implement backward compatibility on top of the codec API.
	// A value of 6 indicates that the created version is unknown.
	CreatedVersionMajor int
	// MinVersion is the minimum Lucene version that contributed documents to this index,
	// or nil if this information is not available.
	MinVersion *util.Version
	// Sort is the order in which documents from this index are sorted,
	// or nil if documents are in no particular order.
	Sort *schema.Sort
	// HasBlocks returns true iff this index contains blocks created with
	// IndexWriter.AddDocument(Iterable) or its corresponding update methods
	// with at least 2 or more documents per call.
	// Note: This property was not recorded before LUCENE_9_9_0;
	// this method will return false for all leaves written before LUCENE_9_9_0.
	HasBlocks bool
}

// NewLeafMetaData is the sole constructor for LeafMetaData.
// It validates the input parameters to ensure they are consistent with Lucene's requirements.
func NewLeafMetaData(createdVersionMajor int, minVersion *util.Version, sort *schema.Sort, hasBlocks bool) (*LeafMetaData, error) {
	if createdVersionMajor > util.Latest.Major {
		return nil, fmt.Errorf("createdVersionMajor is in the future: %d", createdVersionMajor)
	}
	if createdVersionMajor < 6 {
		return nil, fmt.Errorf("createdVersionMajor must be >= 6, got: %d", createdVersionMajor)
	}
	if createdVersionMajor >= 7 && minVersion == nil {
		return nil, fmt.Errorf("minVersion must be set when createdVersionMajor is >= 7")
	}

	return &LeafMetaData{
		CreatedVersionMajor: createdVersionMajor,
		MinVersion:          minVersion,
		Sort:                sort,
		HasBlocks:           hasBlocks,
	}, nil
}
