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
// When the commit is obtained from a DirectoryReader, it also carries a
// reference to that reader. IndexWriter uses this reference for the NRT-reader
// reopen path and to detect stale or closed readers.
//
// IndexCommits are immutable once created, apart from the optional reader
// reference which is set by the reader implementation at construction time.
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

	// reader is the DirectoryReader this commit was obtained from, if any.
	// It is non-nil for commits produced by StandardDirectoryReader.GetIndexCommit
	// and nil for commits constructed from an on-disk segments_N file alone.
	reader *DirectoryReader

	// deleted is true once Delete() has been invoked.
	deleted bool

	// deletionHook, when non-nil, is invoked by Delete() instead of deleting
	// the segments file directly. The hook is responsible for marking the commit
	// for deletion so that IndexFileDeleter can DecRef its files in the proper
	// order. This mirrors Lucene's IndexCommit.delete() routing through the
	// IndexFileDeleter.
	deletionHook func() error
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

// GetReader returns the DirectoryReader this commit was obtained from, if any.
// Returns nil for commits that were not produced by a reader.
func (c *IndexCommit) GetReader() *DirectoryReader {
	return c.reader
}

// SetReader records the DirectoryReader this commit was obtained from.
func (c *IndexCommit) SetReader(r *DirectoryReader) {
	c.reader = r
}

// SetDeletionHook registers a callback that Delete() invokes instead of
// directly deleting the segments file. The callback should mark the commit as
// eligible for deletion in the owning deleter and return nil on success.
// It is used by IndexFileDeleter so that deletion policies participate in
// reference-counted file cleanup.
func (c *IndexCommit) SetDeletionHook(hook func() error) {
	c.deletionHook = hook
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

// Delete marks this commit for deletion. When the commit is wired to an
// IndexFileDeleter (the normal IndexWriter case), the hook is invoked: it
// enqueues the commit so the deleter can DecRef its referenced files and remove
// the segments file in the correct order. When no hook is present (e.g. a
// standalone IndexCommit obtained from ListCommits), the segments file is
// deleted directly and the deleted flag is set.
func (c *IndexCommit) Delete() error {
	if c.deleted {
		return nil
	}
	if c.deletionHook != nil {
		if err := c.deletionHook(); err != nil {
			return err
		}
		c.deleted = true
		return nil
	}
	if c.directory == nil {
		return fmt.Errorf("directory not set")
	}
	if err := c.directory.DeleteFile(c.segmentsFileName); err != nil {
		return fmt.Errorf("deleting segments file %s: %w", c.segmentsFileName, err)
	}
	c.deleted = true
	return nil
}

// IsDeleted returns true if this commit has been deleted. A commit is deleted
// when Delete() has been invoked on it, or — for standalone commits that are
// not managed by an IndexFileDeleter — when its segments file no longer exists
// in the directory. This matches the testable behavior expected by Lucene's
// IndexCommit contract and keeps manually-constructed commit points honest.
func (c *IndexCommit) IsDeleted() bool {
	if c.deleted {
		return true
	}
	if c.deletionHook != nil {
		// Real commit points are managed by IndexFileDeleter; the hook sets the
		// deleted flag once the deleter has queued the commit for cleanup.
		return false
	}
	if c.directory != nil && c.segmentsFileName != "" {
		if _, err := c.directory.FileLength(c.segmentsFileName); err != nil {
			return true
		}
	}
	return false
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
