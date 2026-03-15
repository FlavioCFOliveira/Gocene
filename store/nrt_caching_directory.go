// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

// NRTCachingDirectory wraps a RAM-resident directory around any provided delegate directory,
// to be used during NRT (Near Real-Time) search.
//
// This class is useful in a near-real-time context, where indexing rate is lowish
// but reopen rate is highish, resulting in many tiny files being written. This directory
// keeps such segments (as well as the segments produced by merging them, as long as they
// are small enough), in RAM.
//
// This is safe to use: when your app calls IndexWriter.Commit, all cached files will be
// flushed from the cache and sync'd.
//
// Example usage:
//
//	fsDir := store.NewFSDirectory("/path/to/index")
//	cachedFSDir := store.NewNRTCachingDirectory(fsDir, 5.0, 60.0)
//	writer, _ := index.NewIndexWriter(cachedFSDir, conf)
//
// This will cache all newly flushed segments, all merges whose expected segment size is
// <= 5 MB, unless the net cached bytes exceed 60 MB at which point all writes will not be cached
// (until the net bytes fall below 60 MB).
//
// This is the Go port of Lucene's org.apache.lucene.store.NRTCachingDirectory.
type NRTCachingDirectory struct {
	*FilterDirectory

	closed            atomic.Bool
	cacheSize         atomic.Int64
	cacheDir          *ByteBuffersDirectory
	maxMergeSizeBytes int64
	maxCachedBytes    int64
	mu                sync.Mutex
}

// NewNRTCachingDirectory creates a new NRTCachingDirectory wrapping the given delegate directory.
//
// Parameters:
//   - delegate: the directory to wrap
//   - maxMergeSizeMB: maximum merge size in MB to cache
//   - maxCachedMB: maximum total cached MB
func NewNRTCachingDirectory(delegate Directory, maxMergeSizeMB, maxCachedMB float64) *NRTCachingDirectory {
	return &NRTCachingDirectory{
		FilterDirectory:   NewFilterDirectory(delegate),
		cacheDir:          NewByteBuffersDirectory(),
		maxMergeSizeBytes: int64(maxMergeSizeMB * 1024 * 1024),
		maxCachedBytes:    int64(maxCachedMB * 1024 * 1024),
	}
}

// String returns a string representation of this NRTCachingDirectory.
func (d *NRTCachingDirectory) String() string {
	return fmt.Sprintf("NRTCachingDirectory(delegate=%v; maxCacheMB=%.2f maxMergeSizeMB=%.2f)",
		d.GetDelegate(),
		float64(d.maxCachedBytes)/(1024.0*1024.0),
		float64(d.maxMergeSizeBytes)/(1024.0*1024.0))
}

// ListAll returns the names of all files in this directory (both cached and delegate).
func (d *NRTCachingDirectory) ListAll() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	// Get files from both directories
	cacheFiles, err := d.cacheDir.ListAll()
	if err != nil {
		return nil, err
	}

	delegateFiles, err := d.GetDelegate().ListAll()
	if err != nil {
		return nil, err
	}

	// Merge and sort
	fileSet := make(map[string]struct{})
	for _, f := range cacheFiles {
		fileSet[f] = struct{}{}
	}
	for _, f := range delegateFiles {
		fileSet[f] = struct{}{}
	}

	result := make([]string, 0, len(fileSet))
	for f := range fileSet {
		result = append(result, f)
	}
	sort.Strings(result)
	return result, nil
}

// DeleteFile deletes a file from the directory.
func (d *NRTCachingDirectory) DeleteFile(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.EnsureOpen(); err != nil {
		return err
	}

	// Check if file is in cache
	if d.cacheDir.FileExists(name) {
		size, err := d.cacheDir.FileLength(name)
		if err != nil {
			return err
		}
		if err := d.cacheDir.DeleteFile(name); err != nil {
			return err
		}
		newSize := d.cacheSize.Add(-size)
		if newSize < 0 {
			// Defensive: should not happen
			d.cacheSize.Store(0)
		}
		return nil
	}

	// Delete from delegate
	return d.GetDelegate().DeleteFile(name)
}

// FileLength returns the length of a file in bytes.
func (d *NRTCachingDirectory) FileLength(name string) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.EnsureOpen(); err != nil {
		return 0, err
	}

	// Check cache first
	if d.cacheDir.FileExists(name) {
		return d.cacheDir.FileLength(name)
	}

	// Fall back to delegate
	return d.GetDelegate().FileLength(name)
}

// ListCachedFiles returns the names of all files currently in the cache.
func (d *NRTCachingDirectory) ListCachedFiles() ([]string, error) {
	return d.cacheDir.ListAll()
}

// CreateOutput returns an IndexOutput for writing a new file.
func (d *NRTCachingDirectory) CreateOutput(name string, ctx IOContext) (IndexOutput, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	if d.doCacheWrite(name, ctx) {
		// Track the size that will be added when file is closed
		return d.cacheDir.CreateOutput(name, ctx)
	}
	return d.GetDelegate().CreateOutput(name, ctx)
}

// Sync flushes cached files to the delegate and syncs.
func (d *NRTCachingDirectory) Sync(names []string) error {
	if err := d.EnsureOpen(); err != nil {
		return err
	}

	for _, name := range names {
		if err := d.unCache(name); err != nil {
			return err
		}
	}

	// Call Sync on delegate if it has the method
	type syncer interface {
		Sync([]string) error
	}
	if s, ok := d.GetDelegate().(syncer); ok {
		return s.Sync(names)
	}
	return nil
}

// Rename renames a file.
func (d *NRTCachingDirectory) Rename(source, dest string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.EnsureOpen(); err != nil {
		return err
	}

	// Uncache source if needed
	if err := d.unCache(source); err != nil {
		return err
	}

	// Check if dest exists in cache
	if d.cacheDir.FileExists(dest) {
		return fmt.Errorf("%w: target file %s already exists in cache", ErrFileAlreadyExists, dest)
	}

	// Call Rename on delegate if it has the method
	type renamer interface {
		Rename(string, string) error
	}
	if r, ok := d.GetDelegate().(renamer); ok {
		return r.Rename(source, dest)
	}
	return fmt.Errorf("delegate does not support Rename")
}

// OpenInput returns an IndexInput for reading an existing file.
func (d *NRTCachingDirectory) OpenInput(name string, ctx IOContext) (IndexInput, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	// Check cache first
	if d.cacheDir.FileExists(name) {
		return d.cacheDir.OpenInput(name, ctx)
	}

	// Fall back to delegate
	return d.GetDelegate().OpenInput(name, ctx)
}

// Close releases all resources associated with this directory.
func (d *NRTCachingDirectory) Close() error {
	if d.closed.Swap(true) {
		return nil // Already closed
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Uncache all files
	cacheFiles, _ := d.cacheDir.ListAll()
	for _, name := range cacheFiles {
		d.unCache(name)
	}

	// Close cache directory
	d.cacheDir.Close()

	// Close delegate
	return d.GetDelegate().Close()
}

// CreateTempOutput creates a temporary output file with a unique name.
func (d *NRTCachingDirectory) CreateTempOutput(prefix, suffix string, ctx IOContext) (IndexOutput, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	// Determine which directory to use first
	var first, second Directory
	if d.doCacheWrite(prefix, ctx) {
		first = d.cacheDir
		second = d.GetDelegate()
	} else {
		first = d.GetDelegate()
		second = d.cacheDir
	}

	// Try to create a unique temp file
	for {
		// Check if first has CreateTempOutput
		type tempOutputCreator interface {
			CreateTempOutput(string, string, IOContext) (IndexOutput, error)
		}

		var out IndexOutput
		var err error

		if toc, ok := first.(tempOutputCreator); ok {
			out, err = toc.CreateTempOutput(prefix, suffix, ctx)
		} else {
			// Fallback: generate a unique name and use CreateOutput
			name := d.generateTempName(prefix, suffix)
			out, err = first.CreateOutput(name, ctx)
		}

		if err != nil {
			return nil, err
		}

		name := out.GetName()
		exists, _ := d.slowFileExists(second, name)
		if exists {
			out.Close()
			first.DeleteFile(name)
		} else {
			return out, nil
		}
	}
}

// generateTempName generates a unique temporary file name.
func (d *NRTCachingDirectory) generateTempName(prefix, suffix string) string {
	// Simple unique name generation using counter
	// In production, this should use a more robust method
	return fmt.Sprintf("%s_%d%s", prefix, d.cacheSize.Load(), suffix)
}

// slowFileExists returns true if the file exists (can be opened), false if it cannot be opened.
func (d *NRTCachingDirectory) slowFileExists(dir Directory, fileName string) (bool, error) {
	_, err := dir.FileLength(fileName)
	if err != nil {
		if err == ErrFileNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// isCachedFile returns true if the file is in the cache.
func (d *NRTCachingDirectory) isCachedFile(fileName string) bool {
	return d.cacheDir.FileExists(fileName)
}

// unCache moves a file from the cache to the delegate.
func (d *NRTCachingDirectory) unCache(fileName string) error {
	if !d.cacheDir.FileExists(fileName) {
		return nil // Not in cache
	}

	// Check if file exists in delegate (should not happen)
	exists, _ := d.slowFileExists(d.GetDelegate(), fileName)
	if exists {
		return fmt.Errorf("file %s exists both in cache and in delegate", fileName)
	}

	// Copy from cache to delegate
	size, err := d.cacheDir.FileLength(fileName)
	if err != nil {
		return err
	}

	// Manual copy
	in, err := d.cacheDir.OpenInput(fileName, IOContextRead)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := d.GetDelegate().CreateOutput(fileName, IOContextWrite)
	if err != nil {
		return err
	}
	defer out.Close()

	// Copy data
	buf := make([]byte, 8192)
	for {
		err := in.ReadBytes(buf)
		if err != nil {
			// Check if we've reached EOF
			if err.Error() == "EOF" || err.Error() == "not enough data available" {
				break
			}
			return err
		}
		if err := out.WriteBytes(buf); err != nil {
			return err
		}
	}

	// Update cache size and delete from cache
	d.cacheSize.Add(-size)
	return d.cacheDir.DeleteFile(fileName)
}

// doCacheWrite returns true if this file should be written to the RAM-based cache first.
// Subclasses can override this to customize logic.
func (d *NRTCachingDirectory) doCacheWrite(name string, ctx IOContext) bool {
	var bytes int64
	if ctx.MergeInfo != nil {
		bytes = ctx.MergeInfo.EstimatedMergeBytes
	} else if ctx.FlushInfo != nil {
		bytes = ctx.FlushInfo.EstimatedSegmentSize
	} else {
		return false
	}

	return (bytes <= d.maxMergeSizeBytes) && (bytes+d.cacheSize.Load()) <= d.maxCachedBytes
}

// RamBytesUsed returns the current size of the cache in bytes.
func (d *NRTCachingDirectory) RamBytesUsed() int64 {
	return d.cacheSize.Load()
}

// Ensure that NRTCachingDirectory implements Directory
var _ Directory = (*NRTCachingDirectory)(nil)
