// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// IndexCommit represents a point-in-time commit of the index.
// This is the Go port of Lucene's org.apache.lucene.index.IndexCommit.
//
// An IndexCommit represents a specific commit (snapshot) of the index.
// It contains the segments file name and the number of segments at the
// time of the commit. IndexCommits are used by IndexDeletionPolicy to
// decide which commits should be kept or deleted.
//
// IndexCommits are immutable once created.
type IndexCommit struct {
	// segmentsFileName is the name of the segments file for this commit
	segmentsFileName string

	// directory is the directory containing this commit
	directory store.Directory

	// segmentCount is the number of segments in this commit
	segmentCount int

	// generation is the commit generation number
	generation int64

	// userData contains optional user-supplied commit data
	userData map[string]string

	// segmentInfos holds a reference to the SegmentInfos at commit time
	segmentInfos *SegmentInfos
}

// NewIndexCommit creates a new IndexCommit from the given SegmentInfos.
func NewIndexCommit(infos *SegmentInfos) *IndexCommit {
	return &IndexCommit{
		segmentsFileName: infos.GetFileName(),
		directory:        nil, // Will be set from segments
		segmentCount:     infos.Size(),
		generation:       infos.Generation(),
		userData:         infos.GetUserData(),
		segmentInfos:     infos.Clone(),
	}
}

// NewIndexCommitWithDirectory creates a new IndexCommit with the given directory.
func NewIndexCommitWithDirectory(segmentsFileName string, dir store.Directory, segmentCount int, generation int64) *IndexCommit {
	return &IndexCommit{
		segmentsFileName: segmentsFileName,
		directory:        dir,
		segmentCount:     segmentCount,
		generation:       generation,
		userData:         make(map[string]string),
		segmentInfos:     nil,
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

// SetDirectory sets the directory for this commit.
func (c *IndexCommit) SetDirectory(dir store.Directory) {
	c.directory = dir
}

// GetGeneration returns the commit generation number.
func (c *IndexCommit) GetGeneration() int64 {
	return c.generation
}

// GetUserData returns the user-supplied commit data.
func (c *IndexCommit) GetUserData() map[string]string {
	// Return a copy
	data := make(map[string]string, len(c.userData))
	for k, v := range c.userData {
		data[k] = v
	}
	return data
}

// GetUserDataValue returns a specific user data value.
func (c *IndexCommit) GetUserDataValue(key string) string {
	return c.userData[key]
}

// GetSegmentInfos returns the SegmentInfos for this commit.
func (c *IndexCommit) GetSegmentInfos() *SegmentInfos {
	return c.segmentInfos
}

// GetFileNames returns all file names associated with this commit.
func (c *IndexCommit) GetFileNames() ([]string, error) {
	if c.segmentInfos == nil {
		return nil, fmt.Errorf("segment infos not available")
	}

	files := make([]string, 0)

	// Add the segments file
	files = append(files, c.segmentsFileName)

	// Add all segment files
	for sci := range c.segmentInfos.Iterator() {
		segmentFiles := sci.GetFiles()
		files = append(files, segmentFiles...)
	}

	// Remove duplicates and sort
	uniqueFiles := make(map[string]bool)
	for _, f := range files {
		uniqueFiles[f] = true
	}

	result := make([]string, 0, len(uniqueFiles))
	for f := range uniqueFiles {
		result = append(result, f)
	}
	sort.Strings(result)

	return result, nil
}

// Delete deletes this commit by removing its segments file and associated files.
func (c *IndexCommit) Delete() error {
	if c.directory == nil {
		return fmt.Errorf("directory not set")
	}

	// Get all files associated with this commit
	files, err := c.GetFileNames()
	if err != nil {
		// If we can't get file names (e.g., segment infos not available),
		// at least try to delete the segments file
		if err := c.directory.DeleteFile(c.segmentsFileName); err != nil {
			return fmt.Errorf("deleting segments file: %w", err)
		}
		return nil
	}

	// Delete all files
	var lastErr error
	for _, file := range files {
		if err := c.directory.DeleteFile(file); err != nil {
			// Keep track of errors but try to delete all files
			lastErr = err
		}
	}

	return lastErr
}

// IsDeleted returns true if this commit has been deleted.
func (c *IndexCommit) IsDeleted() bool {
	if c.directory == nil {
		return false
	}
	return !c.directory.FileExists(c.segmentsFileName)
}

// String returns a string representation of this IndexCommit.
func (c *IndexCommit) String() string {
	return fmt.Sprintf("IndexCommit(segmentsFile=%s, segments=%d, generation=%d)",
		c.segmentsFileName, c.segmentCount, c.generation)
}

// Equals checks if two IndexCommits represent the same commit.
func (c *IndexCommit) Equals(other *IndexCommit) bool {
	if other == nil {
		return false
	}
	return c.segmentsFileName == other.segmentsFileName &&
		c.generation == other.generation
}

// IndexCommitList represents a list of IndexCommits.
type IndexCommitList []*IndexCommit

// Len returns the number of commits in the list.
func (l IndexCommitList) Len() int {
	return len(l)
}

// SortByGeneration sorts the commits by generation (oldest first).
func (l IndexCommitList) SortByGeneration() {
	sort.Slice(l, func(i, j int) bool {
		return l[i].generation < l[j].generation
	})
}

// SortByGenerationDesc sorts the commits by generation (newest first).
func (l IndexCommitList) SortByGenerationDesc() {
	sort.Slice(l, func(i, j int) bool {
		return l[i].generation > l[j].generation
	})
}

// GetLatest returns the commit with the highest generation.
func (l IndexCommitList) GetLatest() *IndexCommit {
	if len(l) == 0 {
		return nil
	}

	latest := l[0]
	for _, c := range l[1:] {
		if c.generation > latest.generation {
			latest = c
		}
	}
	return latest
}

// GetOldest returns the commit with the lowest generation.
func (l IndexCommitList) GetOldest() *IndexCommit {
	if len(l) == 0 {
		return nil
	}

	oldest := l[0]
	for _, c := range l[1:] {
		if c.generation < oldest.generation {
			oldest = c
		}
	}
	return oldest
}

// String returns a string representation of the IndexCommitList.
func (l IndexCommitList) String() string {
	return fmt.Sprintf("IndexCommitList(count=%d)", len(l))
}
