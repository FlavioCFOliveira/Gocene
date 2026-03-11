// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// IndexCommit represents a point-in-time commit of the index.
type IndexCommit struct {
	segmentsFileName string
	directory        store.Directory
	segmentCount     int
}

// NewIndexCommit creates a new IndexCommit.
func NewIndexCommit(segmentsFileName string, dir store.Directory, segmentCount int) *IndexCommit {
	return &IndexCommit{
		segmentsFileName: segmentsFileName,
		directory:        dir,
		segmentCount:     segmentCount,
	}
}

// GetSegmentsFileName returns the segments file name for this commit.
func (c *IndexCommit) GetSegmentsFileName() string {
	return c.segmentsFileName
}

// GetSegmentCount returns the number of segments in this commit.
func (c *IndexCommit) GetSegmentCount() int {
	return c.segmentCount
}

// GetDirectory returns the directory containing this commit.
func (c *IndexCommit) GetDirectory() store.Directory {
	return c.directory
}

// Delete deletes this commit.
func (c *IndexCommit) Delete() error {
	// TODO: Implement commit deletion
	return nil
}
