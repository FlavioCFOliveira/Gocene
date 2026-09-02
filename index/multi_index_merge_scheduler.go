// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// MultiIndexMergeScheduler provides multi-index or multi-tenant merge scheduling.
// This is the Go port of Lucene's org.apache.lucene.index.MultiIndexMergeScheduler.
//
// MultiIndexMergeScheduler builds on existing functionality in ConcurrentMergeScheduler
// by automatically tracking merge sources and the index Directory that each applies to,
// and then shunting all of them into a single shared CombinedMergeScheduler instance.
//
// The multi-tenant merge scheduling can be used easily by creating a
// MultiIndexMergeScheduler instance for each index and then using each instance
// normally, the same way you would use a lone ConcurrentMergeScheduler.
//
// This type is experimental.
type MultiIndexMergeScheduler struct {
	*BaseMergeScheduler

	directory              store.Directory
	combinedMergeScheduler *CombinedMergeScheduler
	manageSingleton        bool
}

// NewMultiIndexMergeScheduler creates a MultiIndexMergeScheduler for the given
// directory. This is the main constructor. The scheduler attaches to (and shares)
// the process-wide CombinedMergeScheduler singleton, acquiring a reference that is
// released on Close.
func NewMultiIndexMergeScheduler(directory store.Directory) *MultiIndexMergeScheduler {
	return &MultiIndexMergeScheduler{
		BaseMergeScheduler:     NewBaseMergeScheduler(),
		directory:              directory,
		combinedMergeScheduler: acquireCombinedSingleton(),
		manageSingleton:        true,
	}
}

// NewMultiIndexMergeSchedulerWith creates a MultiIndexMergeScheduler bound to an
// explicitly supplied CombinedMergeScheduler. This constructor is intended for unit
// testing or tenant partitioning; it does not Close the CombinedMergeScheduler.
func NewMultiIndexMergeSchedulerWith(directory store.Directory, combined *CombinedMergeScheduler) *MultiIndexMergeScheduler {
	return &MultiIndexMergeScheduler{
		BaseMergeScheduler:     NewBaseMergeScheduler(),
		directory:              directory,
		combinedMergeScheduler: combined,
		manageSingleton:        false,
	}
}

// GetDirectory returns the index Directory this scheduler applies to.
func (s *MultiIndexMergeScheduler) GetDirectory() store.Directory {
	return s.directory
}

// GetCombinedMergeScheduler returns the shared CombinedMergeScheduler backing this
// scheduler.
func (s *MultiIndexMergeScheduler) GetCombinedMergeScheduler() *CombinedMergeScheduler {
	return s.combinedMergeScheduler
}

// Merge tags the supplied MergeSource with this scheduler's Directory and forwards
// it to the shared CombinedMergeScheduler.
func (s *MultiIndexMergeScheduler) Merge(source MergeSource, trigger MergeTrigger) error {
	tagged := newTaggedMergeSource(source, s.directory)
	return s.combinedMergeScheduler.Merge(tagged, trigger)
}

// WrapForMerge wraps a Directory for merge operations, delegating to the shared
// CombinedMergeScheduler.
func (s *MultiIndexMergeScheduler) WrapForMerge(merge *OneMerge, in store.Directory) store.Directory {
	return s.combinedMergeScheduler.WrapForMerge(merge, in)
}

// Close closes this scheduler for its one directory/index. It waits for any merge
// threads associated with this Directory to finish, and releases the
// CombinedMergeScheduler singleton reference when this scheduler owns one.
func (s *MultiIndexMergeScheduler) Close() error {
	s.combinedMergeScheduler.Sync(s.directory)
	if s.manageSingleton {
		if err := releaseCombinedSingleton(); err != nil {
			return err
		}
	}
	return s.BaseMergeScheduler.Close()
}

// SetInfoStream sets the info stream on the shared CombinedMergeScheduler.
//
// This exists as a separate method because, unlike Lucene's ConcurrentMergeScheduler
// initialize() path, the dynamic-default initialization is not wanted here; only the
// info stream needs to be propagated.
func (s *MultiIndexMergeScheduler) SetInfoStream(infoStream InfoStream) {
	s.combinedMergeScheduler.SetInfoStream(infoStream)
}

// CombinedMergeScheduler is used internally by MultiIndexMergeScheduler to balance
// resources across multiple indices. Normally you do not need to use this directly.
//
// For testing purposes, or if partitioning of tenants into groups is needed, a
// CombinedMergeScheduler can be provided to NewMultiIndexMergeSchedulerWith.
//
// A CombinedMergeScheduler must NOT be passed directly to IndexWriter.
type CombinedMergeScheduler struct {
	*ConcurrentMergeScheduler

	// trackMu protects the per-directory tracking of active tagged sources.
	trackMu sync.Mutex

	// activeByDir counts in-flight tagged merge calls per Directory, allowing Sync
	// to wait until a given index has no outstanding merges. Gocene's MergeThread,
	// unlike Lucene's, does not retain a reference to its MergeSource, so the
	// per-directory bookkeeping is kept here instead of being read off the threads.
	activeByDir map[store.Directory]int

	// dirDone is broadcast whenever a Directory's active count reaches zero.
	dirDone *sync.Cond
}

// NewCombinedMergeScheduler creates a standalone CombinedMergeScheduler.
func NewCombinedMergeScheduler() *CombinedMergeScheduler {
	c := &CombinedMergeScheduler{
		ConcurrentMergeScheduler: NewConcurrentMergeScheduler(),
		activeByDir:              make(map[store.Directory]int),
	}
	c.dirDone = sync.NewCond(&c.trackMu)
	return c
}

// Merge runs the merges from a TaggedMergeSource, tracking the originating
// Directory so that Sync can wait on a per-index basis.
func (c *CombinedMergeScheduler) Merge(source MergeSource, trigger MergeTrigger) error {
	tagged, ok := source.(*TaggedMergeSource)
	if !ok {
		return fmt.Errorf("CombinedMergeScheduler requires a *TaggedMergeSource")
	}

	c.enter(tagged.directory)
	defer c.leave(tagged.directory)

	return c.ConcurrentMergeScheduler.Merge(source, trigger)
}

// Sync blocks until there are no outstanding merge operations for the given
// Directory. Merges for other directories are left untouched.
func (c *CombinedMergeScheduler) Sync(directory store.Directory) {
	c.trackMu.Lock()
	defer c.trackMu.Unlock()
	for c.activeByDir[directory] > 0 {
		c.dirDone.Wait()
	}
}

// enter records the start of a merge call for directory.
func (c *CombinedMergeScheduler) enter(directory store.Directory) {
	c.trackMu.Lock()
	c.activeByDir[directory]++
	c.trackMu.Unlock()
}

// leave records the completion of a merge call for directory and wakes any
// goroutine blocked in Sync for that Directory.
func (c *CombinedMergeScheduler) leave(directory store.Directory) {
	c.trackMu.Lock()
	c.activeByDir[directory]--
	if c.activeByDir[directory] <= 0 {
		delete(c.activeByDir, directory)
	}
	c.dirDone.Broadcast()
	c.trackMu.Unlock()
}

// TaggedMergeSource decorates a MergeSource with the index Directory it belongs to.
// It is a transparent pass-through filter: every MergeSource method delegates to the
// wrapped source unchanged.
type TaggedMergeSource struct {
	in        MergeSource
	directory store.Directory
}

// newTaggedMergeSource wraps in, tagging it with directory.
func newTaggedMergeSource(in MergeSource, directory store.Directory) *TaggedMergeSource {
	return &TaggedMergeSource{in: in, directory: directory}
}

// GetDirectory returns the Directory this source is tagged with.
func (t *TaggedMergeSource) GetDirectory() store.Directory {
	return t.directory
}

// GetNextMerge returns the next pending merge from the wrapped source.
func (t *TaggedMergeSource) GetNextMerge() *OneMerge {
	return t.in.GetNextMerge()
}

// OnMergeFinished forwards merge completion to the wrapped source.
func (t *TaggedMergeSource) OnMergeFinished(merge *OneMerge) {
	t.in.OnMergeFinished(merge)
}

// HasPendingMerges reports whether the wrapped source has pending merges.
func (t *TaggedMergeSource) HasPendingMerges() bool {
	return t.in.HasPendingMerges()
}

// Merge executes the merge on the wrapped source.
func (t *TaggedMergeSource) Merge(merge *OneMerge) error {
	return t.in.Merge(merge)
}

// combinedSingleton holds the process-wide CombinedMergeScheduler shared by all
// MultiIndexMergeScheduler instances created via NewMultiIndexMergeScheduler.
var (
	combinedSingletonMu       sync.Mutex
	combinedSingleton         *CombinedMergeScheduler
	combinedSingletonRefCount int
)

// acquireCombinedSingleton returns the shared CombinedMergeScheduler, creating it on
// first use, and increments its reference count.
func acquireCombinedSingleton() *CombinedMergeScheduler {
	combinedSingletonMu.Lock()
	defer combinedSingletonMu.Unlock()
	if combinedSingleton == nil {
		combinedSingleton = NewCombinedMergeScheduler()
	}
	combinedSingletonRefCount++
	return combinedSingleton
}

// releaseCombinedSingleton decrements the shared CombinedMergeScheduler reference
// count, closing and discarding the singleton when the last reference is released.
func releaseCombinedSingleton() error {
	combinedSingletonMu.Lock()
	defer combinedSingletonMu.Unlock()
	if combinedSingletonRefCount < 1 {
		return fmt.Errorf("releaseCombinedSingleton called too many times")
	}
	combinedSingletonRefCount--
	if combinedSingletonRefCount == 0 {
		err := combinedSingleton.Close()
		combinedSingleton = nil
		return err
	}
	return nil
}

// PeekCombinedSingleton returns the current shared CombinedMergeScheduler, or nil if
// none is currently allocated. Intended for testing and introspection.
func PeekCombinedSingleton() *CombinedMergeScheduler {
	combinedSingletonMu.Lock()
	defer combinedSingletonMu.Unlock()
	return combinedSingleton
}
