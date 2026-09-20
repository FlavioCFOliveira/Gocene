// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file is the Go port of org.apache.lucene.index.MergePolicy from Apache
// Lucene 10.5.0, together with the nested classes Java declares inside it:
// OneMergeProgress (and its PauseReason enum), OneMerge, MergeReader,
// MergeSpecification, MergeObserver, MergeException and MergeAbortedException.
// Go has no nested types, so each becomes a top-level declaration here.
//
// PORT NOTE: Java's MergePolicy is an abstract class mixing three abstract
// methods with a dozen concrete ones. Go splits that into the MergePolicy
// interface (the contract callers program against) and BaseMergePolicy (the
// concrete defaults, embedded by every policy the way subclasses inherit them
// in Java). MergeContext, the fourth nested type, lives in merge_context.go.

// DefaultNoCFSRatio is the default ratio for compound file system usage. Set
// to 1.0: always use the compound file system. Mirrors
// MergePolicy.DEFAULT_NO_CFS_RATIO.
const DefaultNoCFSRatio = 1.0

// DefaultMaxCFSSegmentSize is the default maximum segment size for which the
// compound file format is used. Set to math.MaxInt64. Mirrors
// MergePolicy.DEFAULT_MAX_CFS_SEGMENT_SIZE.
const DefaultMaxCFSSegmentSize int64 = math.MaxInt64

// ErrMerge is the sentinel wrapped by errors reporting a problem encountered
// while executing a merge. It stands in for Java's
// MergePolicy.MergeException, which Lucene throws as a RuntimeException.
var ErrMerge = errors.New("merge failed")

// ErrMergeAborted is the sentinel wrapped by errors reporting that a merge was
// aborted before it completed. It stands in for Java's
// MergePolicy.MergeAbortedException, an IOException subclass.
var ErrMergeAborted = errors.New("merge aborted")

// MergePolicy determines the sequence of primitive merge operations.
//
// Whenever the segments in an index have been altered by IndexWriter — a newly
// flushed segment, segments added by AddIndexes, or a previous merge that may
// now need to cascade — IndexWriter invokes FindMerges to give the MergePolicy
// a chance to pick merges that are now required. That returns a
// MergeSpecification describing the merges to run, or nil if none are
// necessary. When IndexWriter.ForceMerge is called, it calls FindForcedMerges
// instead.
//
// A policy may return more than one merge at a time. With SerialMergeScheduler
// they run sequentially; with ConcurrentMergeScheduler they run concurrently.
//
// The default MergePolicy is TieredMergePolicy.
//
// Mirrors org.apache.lucene.index.MergePolicy. Implementations embed
// *BaseMergePolicy to inherit the concrete defaults, mirroring Java's
// subclassing of the abstract base.
type MergePolicy interface {
	// FindMerges determines what set of merge operations is now necessary on
	// the index. IndexWriter calls this whenever the segments change. The call
	// is serialised by IndexWriter, so only one goroutine at a time runs it.
	FindMerges(mergeTrigger MergeTrigger, segmentInfos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error)

	// FindForcedMerges determines what set of merge operations is necessary to
	// merge down to at most maxSegmentCount segments. segmentsToMerge maps the
	// specific segments that must be merged away; a true value marks a segment
	// that was present in the original to-be-merged index, false one produced
	// by a cascaded merge.
	FindForcedMerges(segmentInfos *SegmentInfos, maxSegmentCount int, segmentsToMerge map[*SegmentCommitInfo]bool, mergeContext MergeContext) (*MergeSpecification, error)

	// FindForcedDeletesMerges determines what set of merge operations is
	// necessary to expunge all deletes from the index.
	FindForcedDeletesMerges(segmentInfos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error)

	// FindFullFlushMerges identifies merges to execute synchronously on
	// commit. Merges returned here make Commit, PrepareCommit or GetReader
	// block until they finish or until the configured max full-flush merge
	// wait has elapsed.
	FindFullFlushMerges(mergeTrigger MergeTrigger, segmentInfos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error)

	// UseCompoundFile reports whether a new segment, whatever its origin,
	// should use the compound file format.
	UseCompoundFile(infos *SegmentInfos, mergedInfo *SegmentCommitInfo, mergeContext MergeContext) (bool, error)

	// NumDeletesToMerge returns the number of deletes a merge would claim on
	// the given segment. The default is the supplied delCount; policies that
	// wrap merge readers may adjust it to reflect deletes carried over to the
	// target segment, as soft deletes require.
	NumDeletesToMerge(info *SegmentCommitInfo, delCount int) int

	// KeepFullyDeletedSegment reports whether the segment should be kept even
	// though every document in it is deleted.
	KeepFullyDeletedSegment(info *SegmentCommitInfo) bool

	// GetMaxMergeDocs returns the largest document count a segment may have
	// and still be eligible for merging.
	GetMaxMergeDocs() int

	// SetMaxMergeDocs sets the largest document count a segment may have and
	// still be eligible for merging.
	SetMaxMergeDocs(maxMergeDocs int)

	// GetMaxMergedSegmentBytes returns the largest merged segment size, in
	// bytes, the policy will produce.
	GetMaxMergedSegmentBytes() int64

	// SetMaxMergedSegmentBytes sets the largest merged segment size, in bytes,
	// the policy will produce.
	SetMaxMergedSegmentBytes(maxMergedSegmentBytes int64)
}

// PauseReason is the reason a merge thread is paused. Mirrors
// org.apache.lucene.index.MergePolicy.OneMergeProgress.PauseReason.
type PauseReason int

// The constants keep the Java enum spelling, because PauseReason is scoped by
// its enclosing class in Java and only by the package in Go.
const (
	// STOPPED means the merge is stopped, typically because the throughput
	// rate was set to zero.
	STOPPED PauseReason = iota

	// PAUSED means the merge is temporarily paused because it exceeded its
	// throughput rate.
	PAUSED

	// OTHER covers every other pause reason.
	OTHER
)

// String returns the Java enum constant name for the pause reason.
func (r PauseReason) String() string {
	switch r {
	case STOPPED:
		return "STOPPED"
	case PAUSED:
		return "PAUSED"
	case OTHER:
		return "OTHER"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(r))
	}
}

// OneMergeProgress carries the progress and state of an executing merge. It
// encapsulates the logic to pause and resume the merge goroutine, or to abort
// the merge entirely. Mirrors
// org.apache.lucene.index.MergePolicy.OneMergeProgress.
type OneMergeProgress struct {
	// pauseMu guards pausing and aborted, and backs the pausing condition
	// variable, mirroring Java's pauseLock/pausing ReentrantLock+Condition.
	pauseMu sync.Mutex
	pausing *sync.Cond

	// aborted mirrors Java's volatile aborted flag.
	aborted bool

	// pauseTimesNS accumulates pause time in nanoseconds per PauseReason.
	// Java uses an EnumMap of AtomicLong; Go uses a fixed array guarded by
	// timesMu, since the reasons are a small dense enum.
	timesMu      sync.Mutex
	pauseTimesNS [3]int64
}

// NewOneMergeProgress creates a new, unaborted merge progress tracker.
func NewOneMergeProgress() *OneMergeProgress {
	p := &OneMergeProgress{}
	p.pausing = sync.NewCond(&p.pauseMu)
	return p
}

// Abort aborts the merge this progress tracks at the next possible moment.
func (p *OneMergeProgress) Abort() {
	p.pauseMu.Lock()
	p.aborted = true
	p.pauseMu.Unlock()
	// Wake up any paused merge goroutine.
	p.Wakeup()
}

// IsAborted reports the aborted state of this merge.
func (p *OneMergeProgress) IsAborted() bool {
	p.pauseMu.Lock()
	defer p.pauseMu.Unlock()
	return p.aborted
}

// PauseNanos pauses the calling goroutine for at least pauseNanos nanoseconds
// unless the merge is aborted or condition returns false, in which case it
// returns immediately.
//
// The external condition is required so that other goroutines can end the
// pause before pauseNanos expires: a condition variable alone cannot be
// trusted, because it can also return from a spurious wakeup.
func (p *OneMergeProgress) PauseNanos(pauseNanos int64, reason PauseReason, condition func() bool) error {
	start := time.Now()

	deadline := start.Add(time.Duration(pauseNanos))
	p.pauseMu.Lock()
	for pauseNanos > 0 && !p.aborted && (condition == nil || condition()) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		// sync.Cond has no timed wait, so a timer supplies the deadline that
		// Java gets from Condition.awaitNanos.
		timer := time.AfterFunc(remaining, p.Wakeup)
		p.pausing.Wait()
		timer.Stop()
		pauseNanos = int64(time.Until(deadline))
	}
	p.pauseMu.Unlock()

	p.timesMu.Lock()
	if int(reason) >= 0 && int(reason) < len(p.pauseTimesNS) {
		p.pauseTimesNS[reason] += time.Since(start).Nanoseconds()
	}
	p.timesMu.Unlock()
	return nil
}

// CheckAborted returns an error wrapping ErrMergeAborted if the merge this
// progress tracks has been aborted.
func (p *OneMergeProgress) CheckAborted() error {
	if p.IsAborted() {
		return ErrMergeAborted
	}
	return nil
}

// Wakeup requests a wakeup for any goroutines stalled in PauseNanos.
func (p *OneMergeProgress) Wakeup() {
	p.pauseMu.Lock()
	defer p.pauseMu.Unlock()
	p.pausing.Broadcast()
}

// GetPauseTimes returns the accumulated pause time, in nanoseconds, per
// PauseReason.
func (p *OneMergeProgress) GetPauseTimes() map[PauseReason]int64 {
	p.timesMu.Lock()
	defer p.timesMu.Unlock()
	return map[PauseReason]int64{
		STOPPED: p.pauseTimesNS[STOPPED],
		PAUSED:  p.pauseTimesNS[PAUSED],
		OTHER:   p.pauseTimesNS[OTHER],
	}
}

// MergeReader pairs a reader taking part in a merge with the hard
// (non-soft-deleted) live docs to use for it. Mirrors
// org.apache.lucene.index.MergePolicy.MergeReader.
type MergeReader struct {
	// Reader is the reader supplying the segment's data for the merge.
	Reader CodecReader

	// HardLiveDocs are the live docs excluding soft deletes, or nil when every
	// document is live.
	HardLiveDocs util.Bits
}

// OneMerge carries the information necessary to perform an individual
// primitive merge operation, resulting in a single new segment. Mirrors
// org.apache.lucene.index.MergePolicy.OneMerge.
type OneMerge struct {
	// Segments are the segments to be merged. Empty for a OneMerge built
	// directly from readers by AddIndexes.
	Segments []*SegmentCommitInfo

	// Info is the SegmentCommitInfo of the merged segment, set once the merge
	// has produced it.
	Info *SegmentCommitInfo

	// MaxNumSegments is the maxNumSegments a forceMerge requested, or -1.
	MaxNumSegments int

	// IsExternal records whether the merge involves segments coming from an
	// external index, as AddIndexes produces.
	IsExternal bool

	// EstimatedMergeBytes is the estimated size in bytes of the merged
	// segment. Java declares it a volatile long; atomic.Int64 is the Go
	// rendering that preserves the cross-goroutine visibility the merge
	// schedulers rely on when they sort running merges by size.
	EstimatedMergeBytes atomic.Int64

	// TotalMergeBytes is the sum of the sizes in bytes of all merged segments,
	// set by IndexWriter during merge init. Volatile in Java, for the same
	// reason as EstimatedMergeBytes.
	TotalMergeBytes atomic.Int64

	// TotalMaxDoc is the total number of documents in the segments to be
	// merged, not accounting for deletions.
	TotalMaxDoc int

	// MergeStartNS is the wall-clock nanosecond timestamp at which the merge
	// started, or -1 before it starts.
	MergeStartNS int64

	// MergeGen is the merge generation IndexWriter assigns when it registers
	// this merge.
	MergeGen int64

	// RegisterDone records that IndexWriter finished registering this merge.
	RegisterDone bool

	// UsesPooledReaders records whether the merge readers come from
	// IndexWriter's reader pool, so it knows to drop them while closing.
	UsesPooledReaders bool

	// Progress controls pausing, stopping and resuming this merge's goroutine.
	Progress *OneMergeProgress

	// Error records an error that occurred while executing this merge.
	Error error

	mu           sync.Mutex
	mergeReaders []*MergeReader

	// completed stands in for Java's CompletableFuture<Boolean>: it is closed
	// exactly once, when the merge finishes, and success records the outcome.
	completed chan struct{}
	success   bool

	// OnMergeFinished is an optional hook called when the merge finishes.
	// It mirrors the custom logic used in point-in-time merges.
	OnMergeFinished func(m *OneMerge, success bool, segmentDropped bool) error

	// OnMergeComplete is an optional hook called when the merge completes
	// and the merged segment is available.
	OnMergeComplete func(m *OneMerge)
}

// NewOneMerge creates a OneMerge over the given segments. Mirrors
// OneMerge(List<SegmentCommitInfo>).
//
// The slice is copied, because the caller's slice may alias a live
// SegmentInfos and be modified afterwards.
func NewOneMerge(segments []*SegmentCommitInfo) *OneMerge {
	totalMaxDoc := 0
	for _, si := range segments {
		if si != nil {
			totalMaxDoc += si.MaxDoc()
		}
	}
	return &OneMerge{
		Segments:          append([]*SegmentCommitInfo(nil), segments...),
		MaxNumSegments:    -1,
		TotalMaxDoc:       totalMaxDoc,
		MergeStartNS:      -1,
		UsesPooledReaders: true,
		Progress:          NewOneMergeProgress(),
		completed:         make(chan struct{}),
	}
}

// NewOneMergeFromReaders creates a OneMerge directly from CodecReaders,
// mirroring OneMerge(CodecReader...). It is used to merge incoming readers in
// IndexWriter.AddIndexes(CodecReader...); such a merge works directly on
// readers and has an empty Segments list.
func NewOneMergeFromReaders(codecReaders []CodecReader) *OneMerge {
	readers := make([]*MergeReader, 0, len(codecReaders))
	totalDocs := 0
	for _, r := range codecReaders {
		readers = append(readers, &MergeReader{Reader: r, HardLiveDocs: r.GetLiveDocs()})
		totalDocs += r.NumDocs()
	}
	return &OneMerge{
		MaxNumSegments:    -1,
		TotalMaxDoc:       totalDocs,
		MergeStartNS:      -1,
		UsesPooledReaders: false,
		Progress:          NewOneMergeProgress(),
		completed:         make(chan struct{}),
		mergeReaders:      readers,
	}
}

// InitMerge is called by IndexWriter after the merge has started, from the
// goroutine that will execute it. Mirrors OneMerge.mergeInit.
func (m *OneMerge) InitMerge() {}

// MergeFinished is called by IndexWriter after the merge is done and all
// readers have been closed. success reports whether the merge was committed;
// segmentDropped reports whether the merged segment was dropped because it was
// fully deleted. Mirrors OneMerge.mergeFinished, whose base implementation is
// empty.
func (m *OneMerge) MergeFinished(success bool, segmentDropped bool) error { return nil }

// WrapForMerge wraps a reader prior to merging, in order to add or remove
// fields or documents. The base implementation is the identity function.
//
// It is illegal to reorder doc IDs here; Reorder exists for that.
func (m *OneMerge) WrapForMerge(reader CodecReader) CodecReader { return reader }

// SetMergeInfo sets the SegmentCommitInfo of the merged segment, allowing
// policies to add diagnostics to it.
func (m *OneMerge) SetMergeInfo(info *SegmentCommitInfo) { m.Info = info }

// GetMergeInfo returns the SegmentCommitInfo of the merged segment, or nil if
// it has not been set yet.
func (m *OneMerge) GetMergeInfo() *SegmentCommitInfo { return m.Info }

// SetException records that an error occurred while executing this merge.
func (m *OneMerge) SetException(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Error = err
}

// GetException retrieves the error previously recorded by SetException.
func (m *OneMerge) GetException() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Error
}

// GetMergeProgress returns the OneMergeProgress for this merge, which supplies
// merge-goroutine statistics (run time against sleep time) when merging is
// throttled.
func (m *OneMerge) GetMergeProgress() *OneMergeProgress { return m.Progress }

// GetProgress is an alias of GetMergeProgress, spelled the way the Gocene
// merge schedulers call it.
func (m *OneMerge) GetProgress() *OneMergeProgress { return m.Progress }

// TotalBytesSize returns the total input size in bytes of this merge. It does
// not describe the size of the merged segment, and is only meaningful once
// IndexWriter has initialised the merge.
func (m *OneMerge) TotalBytesSize() int64 { return m.TotalMergeBytes.Load() }

// TotalNumDocs returns the total number of documents included in this merge.
// It does not describe the document count after the merge.
func (m *OneMerge) TotalNumDocs() int { return m.TotalMaxDoc }

// GetStoreMergeInfo returns the store.MergeInfo describing this merge.
func (m *OneMerge) GetStoreMergeInfo() *store.MergeInfo {
	return &store.MergeInfo{
		TotalMaxDoc:         m.TotalMaxDoc,
		EstimatedMergeBytes: m.EstimatedMergeBytes.Load(),
		IsExternal:          m.IsExternal,
		MergeMaxNumSegments: m.MaxNumSegments,
	}
}

// IsAborted reports whether this merge was, or should be, aborted.
func (m *OneMerge) IsAborted() bool { return m.Progress.IsAborted() }

// SetAborted marks this merge as aborted. The merge goroutine should terminate
// at the soonest possible moment.
func (m *OneMerge) SetAborted() { m.Progress.Abort() }

// Abort marks this merge as aborted, spelled the way the Gocene merge
// schedulers call it. Equivalent to SetAborted.
func (m *OneMerge) Abort() { m.Progress.Abort() }

// CheckAborted returns an error wrapping ErrMergeAborted if this merge has
// been aborted. Mirrors OneMerge.checkAborted, which throws
// MergeAbortedException.
func (m *OneMerge) CheckAborted() error {
	if m.IsAborted() {
		return fmt.Errorf("%w: %s", ErrMergeAborted, m.SegString())
	}
	return nil
}

// SegmentsSize returns the number of segments participating in this merge.
func (m *OneMerge) SegmentsSize() int { return len(m.Segments) }

// InitMergeReaders populates this merge's readers by applying readerFactory to
// every segment, holding on to the live readers so merged deletes can be
// committed. Mirrors OneMerge.initMergeReaders.
func (m *OneMerge) InitMergeReaders(readerFactory func(*SegmentCommitInfo) (*MergeReader, error)) error {
	readers := make([]*MergeReader, 0, len(m.Segments))
	var err error
	for _, info := range m.Segments {
		var mr *MergeReader
		mr, err = readerFactory(info)
		if err != nil {
			break
		}
		readers = append(readers, mr)
	}
	// Assign whatever was built even on failure, so Close can release them.
	m.mu.Lock()
	m.mergeReaders = readers
	m.mu.Unlock()
	return err
}

// GetMergeReader returns the merge readers, or an empty slice if they have not
// been initialised yet.
func (m *OneMerge) GetMergeReader() []*MergeReader {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mergeReaders
}

// Close closes this merge and releases all merge readers, invoking
// readerConsumer on each one first. Mirrors OneMerge.close, which is final in
// Java so that cleanup can never be skipped by a subclass.
func (m *OneMerge) Close(success bool, segmentDropped bool, readerConsumer func(*MergeReader) error) error {
	m.mu.Lock()
	select {
	case <-m.completed:
		m.mu.Unlock()
		return fmt.Errorf("%w: merge has already finished", ErrMerge)
	default:
	}
	m.success = success
	close(m.completed)
	readers := m.mergeReaders
	m.mergeReaders = nil
	m.mu.Unlock()

	var finishedErr error
	if m.OnMergeFinished != nil {
		finishedErr = m.OnMergeFinished(m, success, segmentDropped)
	} else {
		finishedErr = m.MergeFinished(success, segmentDropped)
	}

	var consumerErr error
	if readerConsumer != nil {
		for _, r := range readers {
			if err := readerConsumer(r); err != nil && consumerErr == nil {
				consumerErr = err
			}
		}
	}
	if finishedErr != nil {
		return finishedErr
	}
	return consumerErr
}

// Complete marks the merge as finished, unblocking anything waiting on it.
// It is the Gocene spelling of completing Java's mergeCompleted future without
// running the reader cleanup, used by MergeObserver-driven call sites.
func (m *OneMerge) Complete(success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	select {
	case <-m.completed:
		return
	default:
	}
	m.success = success
	close(m.completed)
}

// Done returns a channel closed once this merge has completed, successfully or
// not. It stands in for polling Java's mergeCompleted future.
func (m *OneMerge) Done() <-chan struct{} { return m.completed }

// HasFinished reports whether the merge has finished. It never blocks.
func (m *OneMerge) HasFinished() bool {
	select {
	case <-m.completed:
		return true
	default:
		return false
	}
}

// Succeeded reports whether the merge finished successfully. It is only
// meaningful once Done is closed.
func (m *OneMerge) Succeeded() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.success
}

// Await blocks until this merge completes or timeout elapses, reporting
// whether it finished within the timeout.
func (m *OneMerge) Await(timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-m.completed:
		return true
	case <-timer.C:
		return false
	}
}

// SegString returns a readable description of the current merge state.
// Mirrors OneMerge.segString.
func (m *OneMerge) SegString() string {
	var b strings.Builder
	for i, s := range m.Segments {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(s.String())
	}
	if m.Info != nil {
		b.WriteString(" into ")
		b.WriteString(m.Info.Name())
	}
	if m.MaxNumSegments != -1 {
		fmt.Fprintf(&b, " [maxNumSegments=%d]", m.MaxNumSegments)
	}
	if m.IsAborted() {
		b.WriteString(" [ABORTED]")
	}
	return b.String()
}

// String returns the same description as SegString, so a OneMerge formats
// usefully with the fmt verbs.
func (m *OneMerge) String() string { return m.SegString() }

// MergeSpecification carries the information necessary to perform multiple
// merges: it is simply a list of OneMerge instances. Mirrors
// org.apache.lucene.index.MergePolicy.MergeSpecification.
type MergeSpecification struct {
	// Merges is the subset of merges included in this specification.
	Merges []*OneMerge
}

// NewMergeSpecification creates an empty MergeSpecification. Use Add to
// populate it.
func NewMergeSpecification() *MergeSpecification {
	return &MergeSpecification{}
}

// Add appends the provided OneMerge to this specification.
func (s *MergeSpecification) Add(merge *OneMerge) {
	s.Merges = append(s.Merges, merge)
}

// Size returns the number of merges in this specification.
func (s *MergeSpecification) Size() int {
	if s == nil {
		return 0
	}
	return len(s.Merges)
}

// String returns a description of the merges in this specification, matching
// the layout of Java's MergeSpecification.toString.
func (s *MergeSpecification) String() string {
	var b strings.Builder
	b.WriteString("MergeSpec:")
	for i, m := range s.Merges {
		fmt.Fprintf(&b, "\n  %d: %s", i+1, m.SegString())
	}
	return b.String()
}

// Await blocks until every merge in this specification has completed,
// reporting whether all of them succeeded.
func (s *MergeSpecification) Await() bool {
	ok := true
	for _, m := range s.Merges {
		<-m.Done()
		if !m.Succeeded() {
			ok = false
		}
	}
	return ok
}

// MergeObserver observes the merge operations returned by
// IndexWriter.ForceMergeDeletes, exposing merge status and completion waits.
//
// When no merges are needed NumMerges returns 0, and Await returns true
// immediately because there is nothing to wait for.
//
// Mirrors org.apache.lucene.index.MergePolicy.MergeObserver.
type MergeObserver struct {
	spec *MergeSpecification
}

// NewMergeObserver creates a MergeObserver over spec's merges. A nil spec
// yields an observer with no merges, whose Await methods return immediately.
func NewMergeObserver(spec *MergeSpecification) *MergeObserver {
	return &MergeObserver{spec: spec}
}

// merges returns the observed merges, tolerating a nil observer or spec.
func (o *MergeObserver) merges() []*OneMerge {
	if o == nil || o.spec == nil {
		return nil
	}
	return o.spec.Merges
}

// NumMerges returns the number of merges being observed.
func (o *MergeObserver) NumMerges() int { return len(o.merges()) }

// NumCompletedMerges returns how many of the observed merges have finished.
func (o *MergeObserver) NumCompletedMerges() int {
	n := 0
	for _, m := range o.merges() {
		if m.HasFinished() {
			n++
		}
	}
	return n
}

// Await blocks until every observed merge has finished, reporting whether all
// of them succeeded.
func (o *MergeObserver) Await() bool {
	ok := true
	for _, m := range o.merges() {
		<-m.Done()
		if !m.Succeeded() {
			ok = false
		}
	}
	return ok
}

// AwaitWithTimeout blocks until every observed merge has finished or timeout
// elapses, reporting whether all merges finished successfully within it.
func (o *MergeObserver) AwaitWithTimeout(timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for _, m := range o.merges() {
		select {
		case <-m.Done():
			if !m.Succeeded() {
				return false
			}
		case <-timer.C:
			return false
		}
	}
	return true
}

// AwaitAsync returns a channel that is closed once every observed merge has
// finished. It stands in for Java's CompletableFuture<Void>.
func (o *MergeObserver) AwaitAsync() <-chan struct{} {
	done := make(chan struct{})
	merges := o.merges()
	go func() {
		defer close(done)
		for _, m := range merges {
			<-m.Done()
		}
	}()
	return done
}

// String returns a readable description of the observed merges.
func (o *MergeObserver) String() string {
	return fmt.Sprintf("MergeObserver[numMerges=%d, completed=%d]", o.NumMerges(), o.NumCompletedMerges())
}

// BaseMergePolicy supplies the concrete behaviour every MergePolicy inherits.
// It is the Go counterpart of the non-abstract half of Java's abstract
// MergePolicy class: policies embed *BaseMergePolicy the way Java subclasses
// extend MergePolicy, and override only what they need.
//
// The three methods Java leaves abstract — FindMerges, FindForcedMerges and
// FindForcedDeletesMerges — return nil here, because Go cannot force an
// embedder to override them. Every concrete policy in this package does.
type BaseMergePolicy struct {
	// noCFSRatio is the fraction of total index size above which a merged
	// segment stays in non-compound format.
	noCFSRatio float64

	// maxCFSSegmentSize is the size in bytes above which a merged segment
	// stays in non-compound format.
	maxCFSSegmentSize int64

	// maxMergeDocs is the largest document count a segment may have and still
	// be merged.
	maxMergeDocs int

	// maxMergedSegmentBytes is the largest merged segment the policy produces.
	maxMergedSegmentBytes int64
}

// NewBaseMergePolicy creates a BaseMergePolicy with Lucene's default settings
// for noCFSRatio and maxCFSSegmentSize. Mirrors the no-argument MergePolicy
// constructor.
func NewBaseMergePolicy() *BaseMergePolicy {
	return NewBaseMergePolicyWithDefaults(DefaultNoCFSRatio, DefaultMaxCFSSegmentSize)
}

// NewBaseMergePolicyWithDefaults creates a BaseMergePolicy with the given
// defaults, for policies that use values other than MergePolicy's. Mirrors
// MergePolicy(double, long).
func NewBaseMergePolicyWithDefaults(defaultNoCFSRatio float64, defaultMaxCFSSegmentSize int64) *BaseMergePolicy {
	return &BaseMergePolicy{
		noCFSRatio:            defaultNoCFSRatio,
		maxCFSSegmentSize:     defaultMaxCFSSegmentSize,
		maxMergeDocs:          math.MaxInt32,
		maxMergedSegmentBytes: DefaultMaxCFSSegmentSize,
	}
}

// FindMerges returns no merges. Java declares this abstract; every concrete
// policy in this package overrides it.
func (p *BaseMergePolicy) FindMerges(MergeTrigger, *SegmentInfos, MergeContext) (*MergeSpecification, error) {
	return nil, nil
}

// FindForcedMerges returns no merges. Java declares this abstract; every
// concrete policy in this package overrides it.
func (p *BaseMergePolicy) FindForcedMerges(*SegmentInfos, int, map[*SegmentCommitInfo]bool, MergeContext) (*MergeSpecification, error) {
	return nil, nil
}

// FindForcedDeletesMerges returns no merges. Java declares this abstract;
// every concrete policy in this package overrides it.
func (p *BaseMergePolicy) FindForcedDeletesMerges(*SegmentInfos, MergeContext) (*MergeSpecification, error) {
	return nil, nil
}

// FindMergesForReaders defines the merges to perform on the readers handed to
// IndexWriter.AddIndexes(CodecReader...). The default builds a single merge
// over all of them, which is the lowest possible concurrency; a custom policy
// may split them to raise it. Mirrors MergePolicy.findMerges(CodecReader...).
func (p *BaseMergePolicy) FindMergesForReaders(readers []CodecReader) (*MergeSpecification, error) {
	spec := NewMergeSpecification()
	spec.Add(NewOneMergeFromReaders(readers))
	return spec, nil
}

// MaxFullFlushMergeSize returns the maximum size of segments to include in
// full-flush merges. The default of zero disables them. Mirrors
// MergePolicy.maxFullFlushMergeSize.
func (p *BaseMergePolicy) MaxFullFlushMergeSize() int64 { return 0 }

// FindFullFlushMerges returns the natural merges whose segments are all
// smaller than MaxFullFlushMergeSize.
//
// PORT NOTE: Java calls the overridden findMerges and maxFullFlushMergeSize
// through virtual dispatch. Go has no virtual dispatch across embedding, so
// the base implementation consults its own FindMerges — which yields no
// merges. Policies that want full-flush merges override this method, as
// MergeOnFlushMergePolicy does.
func (p *BaseMergePolicy) FindFullFlushMerges(mergeTrigger MergeTrigger, segmentInfos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	return nil, nil
}

// Size returns the byte size of the given segment, prorated by the percentage
// of non-deleted documents. Mirrors MergePolicy.size, which Java declares
// protected.
func (p *BaseMergePolicy) Size(info *SegmentCommitInfo, mergeContext MergeContext) (int64, error) {
	byteSize, err := info.SizeInBytes()
	if err != nil {
		return 0, err
	}
	maxDoc := info.MaxDoc()
	if maxDoc <= 0 {
		return byteSize, nil
	}
	delCount := mergeContext.NumDeletesToMerge(info)
	delRatio := float64(delCount) / float64(maxDoc)
	return int64(float64(byteSize) * (1.0 - delRatio)), nil
}

// UseCompoundFile reports whether a new segment should use the compound file
// format. It returns true when the merged segment is no larger than
// GetMaxCFSSegmentSizeMB and no larger than the total index size times
// GetNoCFSRatio.
func (p *BaseMergePolicy) UseCompoundFile(infos *SegmentInfos, mergedInfo *SegmentCommitInfo, mergeContext MergeContext) (bool, error) {
	if p.GetNoCFSRatio() == 0.0 {
		return false, nil
	}
	mergedInfoSize, err := p.Size(mergedInfo, mergeContext)
	if err != nil {
		return false, err
	}
	if mergedInfoSize > p.maxCFSSegmentSize {
		return false, nil
	}
	if p.GetNoCFSRatio() >= 1.0 {
		return true, nil
	}
	var totalSize int64
	for info := range infos.Iterator() {
		size, err := p.Size(info, mergeContext)
		if err != nil {
			return false, err
		}
		totalSize += size
	}
	return float64(mergedInfoSize) <= p.GetNoCFSRatio()*float64(totalSize), nil
}

// IsMerged reports whether this single segment is already fully merged: it has
// no pending deletes and its compound-file setting already matches what the
// policy would choose. Mirrors MergePolicy.isMerged.
func (p *BaseMergePolicy) IsMerged(infos *SegmentInfos, info *SegmentCommitInfo, mergeContext MergeContext) (bool, error) {
	if mergeContext.NumDeletesToMerge(info) != 0 {
		return false, nil
	}
	useCFS, err := p.UseCompoundFile(infos, info, mergeContext)
	if err != nil {
		return false, err
	}
	return useCFS == info.SegmentInfo().IsCompoundFile(), nil
}

// GetNoCFSRatio returns the current noCFSRatio.
func (p *BaseMergePolicy) GetNoCFSRatio() float64 { return p.noCFSRatio }

// SetNoCFSRatio sets the fraction of the total index size above which a merged
// segment stays in non-compound format. Set it to 1.0 to always use the
// compound file format regardless of merge size. Values outside [0.0, 1.0] are
// clamped, where Java throws IllegalArgumentException.
func (p *BaseMergePolicy) SetNoCFSRatio(noCFSRatio float64) {
	if noCFSRatio < 0.0 {
		noCFSRatio = 0.0
	}
	if noCFSRatio > 1.0 {
		noCFSRatio = 1.0
	}
	p.noCFSRatio = noCFSRatio
}

// GetMaxCFSSegmentSizeMB returns the largest size, in megabytes, allowed for a
// compound file segment.
func (p *BaseMergePolicy) GetMaxCFSSegmentSizeMB() float64 {
	return float64(p.maxCFSSegmentSize) / 1024.0 / 1024.0
}

// SetMaxCFSSegmentSizeMB sets the size, in megabytes, above which a merged
// segment stays in non-compound format. A negative value is clamped to zero,
// where Java throws IllegalArgumentException.
func (p *BaseMergePolicy) SetMaxCFSSegmentSizeMB(v float64) {
	if v < 0.0 {
		v = 0.0
	}
	v *= 1024 * 1024
	if v > float64(math.MaxInt64) {
		p.maxCFSSegmentSize = math.MaxInt64
		return
	}
	p.maxCFSSegmentSize = int64(v)
}

// KeepFullyDeletedSegment reports whether a segment should be kept even though
// every document in it is deleted. The default is false; policies that
// implement soft-delete retention override it.
func (p *BaseMergePolicy) KeepFullyDeletedSegment(info *SegmentCommitInfo) bool { return false }

// NumDeletesToMerge returns the number of deletes a merge would claim on the
// given segment. The default is the supplied delCount, which is the sum of the
// on-disk and pending delete counts.
func (p *BaseMergePolicy) NumDeletesToMerge(info *SegmentCommitInfo, delCount int) int {
	return delCount
}

// GetMaxMergeDocs returns the largest document count a segment may have and
// still be eligible for merging.
func (p *BaseMergePolicy) GetMaxMergeDocs() int { return p.maxMergeDocs }

// SetMaxMergeDocs sets the largest document count a segment may have and still
// be eligible for merging.
func (p *BaseMergePolicy) SetMaxMergeDocs(maxMergeDocs int) { p.maxMergeDocs = maxMergeDocs }

// GetMaxMergedSegmentBytes returns the largest merged segment, in bytes, the
// policy will produce.
func (p *BaseMergePolicy) GetMaxMergedSegmentBytes() int64 { return p.maxMergedSegmentBytes }

// SetMaxMergedSegmentBytes sets the largest merged segment, in bytes, the
// policy will produce.
func (p *BaseMergePolicy) SetMaxMergedSegmentBytes(maxMergedSegmentBytes int64) {
	p.maxMergedSegmentBytes = maxMergedSegmentBytes
}

// SegString builds a description of the given segments, annotated with the
// deletes each one would claim. Mirrors MergePolicy.segString.
func (p *BaseMergePolicy) SegString(mergeContext MergeContext, infos []*SegmentCommitInfo) string {
	parts := make([]string, 0, len(infos))
	for _, info := range infos {
		parts = append(parts, fmt.Sprintf("%s(delCount=%d)", info.String(), mergeContext.NumDeletedDocs(info)-info.DelCount()))
	}
	return strings.Join(parts, " ")
}

// Message prints a debug message to the MergeContext's info stream, if it is
// in verbose mode. Mirrors MergePolicy.message.
func (p *BaseMergePolicy) Message(message string, mergeContext MergeContext) {
	if p.Verbose(mergeContext) {
		mergeContext.GetInfoStream().Message("MP", message)
	}
}

// Verbose reports whether the MergeContext's info stream is in verbose mode
// for the "MP" component. Mirrors MergePolicy.verbose.
func (p *BaseMergePolicy) Verbose(mergeContext MergeContext) bool {
	is := mergeContext.GetInfoStream()
	return is != nil && is.IsEnabled("MP")
}
