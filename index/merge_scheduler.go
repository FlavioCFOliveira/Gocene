// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file is the Go port of org.apache.lucene.index.MergeScheduler from
// Apache Lucene 10.5.0, together with its nested MergeSource interface, and of
// the merge bookkeeping helpers Gocene's schedulers build on top of it
// (MergeThread, MergeProgress, MergeException).
//
// PORT NOTE: Java's MergeScheduler is an abstract Closeable class with one
// abstract method (merge) and concrete defaults for the rest. Go splits that
// into the MergeScheduler interface and BaseMergeScheduler, which every
// scheduler embeds the way Java subclasses extend the abstract base.

// InfoStream is the Go port of org.apache.lucene.util.InfoStream, re-exported
// here under the name the index package uses. The canonical declaration lives
// in util, matching the Java package layout.
type InfoStream = util.InfoStream

// MergeSource provides access to new merges and executes the actual merge.
// Mirrors org.apache.lucene.index.MergeScheduler.MergeSource, which
// IndexWriter implements.
type MergeSource interface {
	// GetNextMerge returns the next merge the MergePolicy requested, or nil
	// when there is none.
	GetNextMerge() *OneMerge

	// OnMergeFinished does the finishing bookkeeping for a merge.
	OnMergeFinished(merge *OneMerge)

	// HasPendingMerges reports whether merges are waiting to be scheduled.
	HasPendingMerges() bool

	// Merge merges the indicated segments, replacing them in the stack with a
	// single segment.
	Merge(merge *OneMerge) error
}

// MergeScheduler is used by IndexWriter to execute the merges a MergePolicy
// selects. The default MergeScheduler is ConcurrentMergeScheduler.
//
// Mirrors org.apache.lucene.index.MergeScheduler. Implementations embed
// *BaseMergeScheduler to inherit the concrete defaults.
type MergeScheduler interface {
	// Merge runs the merges provided by mergeSource.GetNextMerge until it
	// returns nil. trigger names the event that caused the merge.
	Merge(mergeSource MergeSource, trigger MergeTrigger) error

	// MergeWithSpec runs exactly the merges named by spec, optionally waiting
	// for them to finish.
	//
	// PORT NOTE: Java drives this through IndexWriter, which registers the
	// spec's merges and then calls merge(); Gocene gives the scheduler the
	// spec directly so a scheduler can size its own concurrency to it.
	MergeWithSpec(mergeSource MergeSource, spec *MergeSpecification, doWait bool) error

	// Initialize is called by IndexWriter on construction, handing the
	// scheduler the writer's info stream and directory.
	Initialize(infoStream util.InfoStream, directory store.Directory) error

	// WrapForMerge wraps the incoming Directory so the merge can be
	// rate-limited, as RateLimitedIndexOutput does in Java. The default is a
	// no-op.
	WrapForMerge(merge *OneMerge, in store.Directory) store.Directory

	// GetIntraMergeExecutor provides an executor for parallelism within a
	// single merge operation. The default runs every intra-merge action on the
	// calling goroutine.
	GetIntraMergeExecutor(merge *OneMerge) util.ExecutorLike

	// AbortAll aborts every merge currently running or pending.
	AbortAll()

	// GetRunningMergeCount returns how many merges are currently running.
	GetRunningMergeCount() int

	// SetMaxMerges sets the maximum number of concurrent merges.
	SetMaxMerges(maxMerges int)

	// GetMaxMerges returns the maximum number of concurrent merges.
	GetMaxMerges() int

	// Close releases any resources the scheduler holds.
	Close() error
}

// BaseMergeScheduler supplies the concrete behaviour every MergeScheduler
// inherits. It is the Go counterpart of the non-abstract half of Java's
// abstract MergeScheduler class.
type BaseMergeScheduler struct {
	mu sync.RWMutex

	// infoStream is used for messages about merge scheduling. Java declares it
	// a protected field of MergeScheduler.
	infoStream util.InfoStream

	// directory is the index directory, handed over by Initialize.
	directory store.Directory

	// executor backs GetIntraMergeExecutor. Java allocates a
	// SameThreadExecutorService per scheduler; Gocene does the same.
	executor util.ExecutorLike

	// maxMerges caps the number of concurrent merges.
	maxMerges int

	// runningMerges counts the merges currently executing.
	runningMerges atomic.Int64

	// closed records that Close has run.
	closed atomic.Bool
}

// NewBaseMergeScheduler creates a BaseMergeScheduler with Lucene's defaults: a
// same-goroutine intra-merge executor and no info stream.
func NewBaseMergeScheduler() *BaseMergeScheduler {
	return &BaseMergeScheduler{
		executor:  &util.SameThreadExecutorService{},
		maxMerges: 1,
	}
}

// Merge runs no merges. Java declares this abstract; every concrete scheduler
// in this package overrides it.
func (s *BaseMergeScheduler) Merge(mergeSource MergeSource, trigger MergeTrigger) error {
	return nil
}

// MergeWithSpec runs no merges. Concrete schedulers override it.
func (s *BaseMergeScheduler) MergeWithSpec(mergeSource MergeSource, spec *MergeSpecification, doWait bool) error {
	return nil
}

// Initialize records the info stream and directory. Mirrors
// MergeScheduler.initialize, which IndexWriter calls on construction.
func (s *BaseMergeScheduler) Initialize(infoStream util.InfoStream, directory store.Directory) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.infoStream = infoStream
	s.directory = directory
	return nil
}

// WrapForMerge returns the directory unchanged. Schedulers that throttle
// merge I/O override it.
func (s *BaseMergeScheduler) WrapForMerge(merge *OneMerge, in store.Directory) store.Directory {
	return in
}

// GetIntraMergeExecutor returns an executor that runs every intra-merge action
// on the calling goroutine, mirroring Java's SameThreadExecutorService default.
func (s *BaseMergeScheduler) GetIntraMergeExecutor(merge *OneMerge) util.ExecutorLike {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.executor
}

// AbortAll aborts nothing. Schedulers that track running merges override it.
func (s *BaseMergeScheduler) AbortAll() {}

// Close marks the scheduler closed and shuts its intra-merge executor down.
func (s *BaseMergeScheduler) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	s.mu.RLock()
	executor := s.executor
	s.mu.RUnlock()
	if executor != nil {
		executor.Shutdown()
	}
	return nil
}

// IsClosed reports whether Close has been called.
func (s *BaseMergeScheduler) IsClosed() bool { return s.closed.Load() }

// SetMaxMerges sets the maximum number of concurrent merges.
func (s *BaseMergeScheduler) SetMaxMerges(maxMerges int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxMerges = maxMerges
}

// GetMaxMerges returns the maximum number of concurrent merges.
func (s *BaseMergeScheduler) GetMaxMerges() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maxMerges
}

// GetRunningMergeCount returns how many merges are currently running.
func (s *BaseMergeScheduler) GetRunningMergeCount() int {
	return int(s.runningMerges.Load())
}

// IncrementRunningMerges records that a merge has started, returning the new
// running count.
func (s *BaseMergeScheduler) IncrementRunningMerges() int {
	return int(s.runningMerges.Add(1))
}

// DecrementRunningMerges records that a merge has finished, returning the new
// running count.
func (s *BaseMergeScheduler) DecrementRunningMerges() int {
	return int(s.runningMerges.Add(-1))
}

// SetInfoStream sets the info stream used for merge-scheduling messages.
func (s *BaseMergeScheduler) SetInfoStream(infoStream util.InfoStream) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.infoStream = infoStream
}

// GetInfoStream returns the info stream used for merge-scheduling messages.
func (s *BaseMergeScheduler) GetInfoStream() util.InfoStream {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.infoStream
}

// GetDirectory returns the directory recorded by Initialize.
func (s *BaseMergeScheduler) GetDirectory() store.Directory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.directory
}

// Verbose reports whether info-stream messages are enabled for the "MS"
// component. Mirrors MergeScheduler.verbose.
func (s *BaseMergeScheduler) Verbose() bool {
	is := s.GetInfoStream()
	return is != nil && is.IsEnabled("MS")
}

// Message writes the given message to the info stream. Callers are expected to
// have checked Verbose first, matching Java's contract.
func (s *BaseMergeScheduler) Message(msg string) {
	if is := s.GetInfoStream(); is != nil {
		is.Message("MS", msg)
	}
}

// MergeException reports a problem encountered while executing a merge. It is
// the Go port of org.apache.lucene.index.MergePolicy.MergeException, which
// Java throws as an unchecked RuntimeException.
type MergeException struct {
	// Message describes the failure.
	Message string

	// Cause is the underlying error, if any.
	Cause error

	// Merge is the merge that failed, if known.
	Merge *OneMerge
}

// NewMergeException builds a MergeException for the given merge.
func NewMergeException(message string, cause error, merge *OneMerge) *MergeException {
	return &MergeException{Message: message, Cause: cause, Merge: merge}
}

// Error implements the error interface.
func (e *MergeException) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", ErrMerge, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", ErrMerge, e.Message)
}

// Unwrap exposes the underlying cause to errors.Is and errors.As.
func (e *MergeException) Unwrap() error {
	if e.Cause != nil {
		return e.Cause
	}
	return ErrMerge
}

// MergeProgress tracks how far a single merge has progressed, in documents.
//
// PORT NOTE: Java has no such class; ConcurrentMergeScheduler infers progress
// from the merge's own counters. Gocene models it explicitly so schedulers can
// report progress without reaching into OneMerge's internals.
type MergeProgress struct {
	mu sync.RWMutex

	// TotalDocs is the number of documents the merge must process.
	TotalDocs int

	// MergedDocs is the number of documents processed so far.
	MergedDocs int

	// IsAborted records that the merge was aborted.
	IsAborted bool

	// err records a failure observed while merging.
	err error
}

// NewMergeProgress creates a MergeProgress over the given total document count.
func NewMergeProgress(totalDocs int) *MergeProgress {
	return &MergeProgress{TotalDocs: totalDocs}
}

// GetProgress returns the fraction of documents merged so far, in [0, 1]. A
// merge with no documents is reported as complete.
func (p *MergeProgress) GetProgress() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.TotalDocs <= 0 {
		return 1.0
	}
	progress := float64(p.MergedDocs) / float64(p.TotalDocs)
	if progress > 1.0 {
		return 1.0
	}
	return progress
}

// SetProgress records the absolute number of documents merged so far.
func (p *MergeProgress) SetProgress(mergedDocs int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.MergedDocs = mergedDocs
}

// IncrementProgress adds delta to the number of documents merged so far.
func (p *MergeProgress) IncrementProgress(delta int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.MergedDocs += delta
}

// Abort marks the merge this progress tracks as aborted.
func (p *MergeProgress) Abort() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.IsAborted = true
}

// SetError records a failure observed while merging.
func (p *MergeProgress) SetError(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.err = err
}

// GetError returns the failure recorded by SetError, or nil.
func (p *MergeProgress) GetError() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.err
}

// MergeThread tracks one merge executing on its own goroutine.
//
// PORT NOTE: Java's ConcurrentMergeScheduler.MergeThread extends Thread.
// Gocene models the merge goroutine as a value the scheduler owns, since a
// goroutine is not itself an object that can be named or joined.
type MergeThread struct {
	// Name identifies the merge goroutine in info-stream messages.
	Name string

	// Merge is the merge this goroutine executes.
	Merge *OneMerge

	mu      sync.RWMutex
	running bool
	err     error

	// done is closed by the owning scheduler once the merge goroutine exits.
	done chan struct{}
}

// NewMergeThread creates a MergeThread for the given merge. The goroutine
// itself is started by the scheduler that owns it.
func NewMergeThread(name string, merge *OneMerge) *MergeThread {
	return &MergeThread{
		Name:  name,
		Merge: merge,
		done:  make(chan struct{}),
	}
}

// IsRunning reports whether the merge goroutine is currently executing.
func (t *MergeThread) IsRunning() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.running
}

// SetRunning records whether the merge goroutine is currently executing.
func (t *MergeThread) SetRunning(running bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.running = running
}

// Cancel aborts the merge this goroutine is executing.
func (t *MergeThread) Cancel() {
	if t.Merge != nil {
		t.Merge.Abort()
	}
}

// Abort aborts the merge this goroutine is executing. It is a synonym of
// Cancel, kept because both spellings appear at Gocene call sites.
func (t *MergeThread) Abort() {
	t.Cancel()
}

// Done returns a channel closed once the merge goroutine has exited.
func (t *MergeThread) Done() <-chan struct{} { return t.done }

// Wait blocks until the merge goroutine exits and returns the error it
// recorded, if any.
func (t *MergeThread) Wait() error {
	<-t.done
	return t.GetError()
}

// GetError returns the error recorded by SetError, or nil.
func (t *MergeThread) GetError() error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.err
}

// SetError records the error the merge goroutine finished with.
func (t *MergeThread) SetError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.err = err
}
