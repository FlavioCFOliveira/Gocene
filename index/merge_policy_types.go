// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// MergePolicy selects which segments to merge. Mirrors
// org.apache.lucene.index.MergePolicy from Apache Lucene 10.5.0.
type MergePolicy interface {
	// FindMerges identifies merges to perform, based on the given SegmentInfos.
	FindMerges(trigger MergeTrigger, infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error)

	// FindForcedMerges identifies merges needed to reduce the index down to maxSegmentCount segments.
	FindForcedMerges(infos *SegmentInfos, maxSegmentCount int, segmentsToMerge map[*SegmentCommitInfo]bool, mergeContext MergeContext) (*MergeSpecification, error)

	// FindForcedDeletesMerges identifies merges needed to expunge all deletes from the index.
	FindForcedDeletesMerges(infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error)
}

// MergeScheduler is responsible for running merges. Mirrors
// org.apache.lucene.index.MergeScheduler.
type MergeScheduler interface {
	// Merge runs the merges provided by mergeSource.GetNextMerge() until it returns nil.
	Merge(source MergeSource, trigger MergeTrigger) error

	// Close releases any resources held by this scheduler.
	Close() error
}

// MergeSource is the callback surface a MergeScheduler uses to pull and
// execute merges. Mirrors org.apache.lucene.index.MergeScheduler.MergeSource.
type MergeSource interface {
	// GetNextMerge returns the next merge requested by the MergePolicy, or nil.
	GetNextMerge() *OneMerge

	// OnMergeFinished does finishing work for a merge.
	OnMergeFinished(merge *OneMerge)

	// HasPendingMerges reports whether merges are waiting to be scheduled.
	HasPendingMerges() bool

	// Merge merges the indicated segments, replacing them with a single segment.
	Merge(merge *OneMerge) error
}

// MergeSpecification is a set of merges to perform. Mirrors
// org.apache.lucene.index.MergePolicy.MergeSpecification.
type MergeSpecification struct {
	// Merges is the subset of merges included in this specification.
	Merges []*OneMerge
}

// Add appends merge to this specification.
func (s *MergeSpecification) Add(merge *OneMerge) {
	s.Merges = append(s.Merges, merge)
}

// PauseReason is the reason a merge thread is paused. Mirrors
// org.apache.lucene.index.MergePolicy.OneMergeProgress.PauseReason.
type PauseReason int

const (
	// STOPPED means the merge is stopped, typically because the throughput
	// rate is set to 0.
	STOPPED PauseReason = iota
	// PAUSED means the merge is temporarily paused because it exceeded its
	// throughput rate.
	PAUSED
	// OTHER covers any other pause reason.
	OTHER
)

// OneMergeProgress tracks pausing/stopping/resuming of a single merge, and
// records how long it spent paused for each PauseReason. Mirrors
// org.apache.lucene.index.MergePolicy.OneMergeProgress.
type OneMergeProgress struct {
	mu        sync.Mutex
	cond      *sync.Cond
	pauseTNS  map[PauseReason]int64
	aborted   bool
}

// NewOneMergeProgress creates a new, unaborted OneMergeProgress.
func NewOneMergeProgress() *OneMergeProgress {
	p := &OneMergeProgress{
		pauseTNS: map[PauseReason]int64{STOPPED: 0, PAUSED: 0, OTHER: 0},
	}
	p.cond = sync.NewCond(&p.mu)
	return p
}

// Abort marks this merge as aborted at the next possible moment.
func (p *OneMergeProgress) Abort() {
	p.mu.Lock()
	p.aborted = true
	p.mu.Unlock()
	p.Wakeup()
}

// IsAborted reports whether this merge was aborted.
func (p *OneMergeProgress) IsAborted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.aborted
}

// PauseNanos pauses the calling goroutine for at least pauseNanos unless the
// merge is aborted or condition returns false, mirroring
// OneMergeProgress.pauseNanos.
func (p *OneMergeProgress) PauseNanos(pauseNanos int64, reason PauseReason, condition func() bool) error {
	start := time.Now()
	p.mu.Lock()
	deadline := start.Add(time.Duration(pauseNanos))
	for pauseNanos > 0 && !p.aborted && (condition == nil || condition()) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		timer := time.AfterFunc(remaining, p.Wakeup)
		p.cond.Wait()
		timer.Stop()
		pauseNanos = int64(time.Until(deadline))
	}
	p.mu.Unlock()
	p.pauseTNS[reason] += time.Since(start).Nanoseconds()
	return nil
}

// Wakeup wakes any goroutines stalled in PauseNanos.
func (p *OneMergeProgress) Wakeup() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cond.Broadcast()
}

// GetPauseTimes returns the accumulated pause time, in nanoseconds, per reason.
func (p *OneMergeProgress) GetPauseTimes() map[PauseReason]int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make(map[PauseReason]int64, len(p.pauseTNS))
	for k, v := range p.pauseTNS {
		out[k] = v
	}
	return out
}

// MergeReader pairs a CodecReader taking part in a merge with the hard
// (non-soft-deleted) live docs to use for it. Mirrors
// org.apache.lucene.index.MergePolicy.MergeReader.
type MergeReader struct {
	Reader       CodecReader
	HardLiveDocs util.Bits
}

// OneMerge provides the information necessary to perform an individual
// primitive merge operation, resulting in a single new segment. Mirrors
// org.apache.lucene.index.MergePolicy.OneMerge.
type OneMerge struct {
	// Segments to be merged.
	Segments []*SegmentCommitInfo

	// Info is the SegmentCommitInfo for the merged segment, set once the
	// merge has produced it.
	Info *SegmentCommitInfo

	// MaxNumSegments is the maxNumSegments requested by a forceMerge, or -1.
	MaxNumSegments int

	// IsExternal records whether the merge involves segments coming from an
	// external index (e.g. via AddIndexes).
	IsExternal bool

	// EstimatedMergeBytes is the estimated size in bytes of the merged segment.
	EstimatedMergeBytes int64

	// TotalMergeBytes is the sum of sizeInBytes of all merged SegmentInfos.
	TotalMergeBytes int64

	// TotalMaxDoc is the total number of documents in the segments to be
	// merged, not accounting for deletions.
	TotalMaxDoc int

	// MergeStartNS is the time.Now().UnixNano() the merge started at, or -1.
	MergeStartNS int64

	// Progress controls pausing/stopping/resuming of this merge's thread.
	Progress *OneMergeProgress

	// Error records an error that occurred while executing this merge.
	Error error

	mu           sync.Mutex
	mergeReaders []*MergeReader
	completed    chan struct{}
	success      bool
}

// NewOneMerge creates a OneMerge over the given segments. Mirrors
// OneMerge(List<SegmentCommitInfo>).
func NewOneMerge(segments []*SegmentCommitInfo) *OneMerge {
	totalMaxDoc := 0
	for _, si := range segments {
		if si != nil && si.Info != nil {
			totalMaxDoc += si.Info.MaxDoc()
		}
	}
	return &OneMerge{
		Segments:       append([]*SegmentCommitInfo(nil), segments...),
		MaxNumSegments: -1,
		TotalMaxDoc:    totalMaxDoc,
		MergeStartNS:   -1,
		Progress:       NewOneMergeProgress(),
		completed:      make(chan struct{}),
	}
}

// NewOneMergeFromReaders creates a OneMerge directly from CodecReaders,
// mirroring OneMerge(CodecReader...) — used by IndexWriter.AddIndexes.
func NewOneMergeFromReaders(readers []CodecReader, hardLiveDocs []util.Bits) *OneMerge {
	mergeReaders := make([]*MergeReader, len(readers))
	totalDocs := 0
	for i, r := range readers {
		var hld util.Bits
		if i < len(hardLiveDocs) {
			hld = hardLiveDocs[i]
		}
		mergeReaders[i] = &MergeReader{Reader: r, HardLiveDocs: hld}
	}
	return &OneMerge{
		MaxNumSegments: -1,
		TotalMaxDoc:    totalDocs,
		MergeStartNS:   -1,
		Progress:       NewOneMergeProgress(),
		completed:      make(chan struct{}),
		mergeReaders:   mergeReaders,
	}
}

// InitMerge is called after the merge started, from the goroutine executing it.
func (m *OneMerge) InitMerge() {}

// GetMergeReader returns the CodecReaders participating in this merge (for
// the AddIndexes(CodecReader...) path).
func (m *OneMerge) GetMergeReader() []*MergeReader {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mergeReaders
}

// WrapForMerge wraps a reader prior to merging. The base implementation is
// the identity function, mirroring OneMerge.wrapForMerge.
func (m *OneMerge) WrapForMerge(reader CodecReader) CodecReader {
	return reader
}

// SetMergeInfo sets the SegmentCommitInfo of the merged segment.
func (m *OneMerge) SetMergeInfo(info *SegmentCommitInfo) {
	m.Info = info
}

// GetMergeInfo returns the SegmentCommitInfo for the merged segment, or nil.
func (m *OneMerge) GetMergeInfo() *SegmentCommitInfo {
	return m.Info
}

// SetException records that an error occurred while executing this merge.
func (m *OneMerge) SetException(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Error = err
}

// GetException retrieves the previous error set by SetException.
func (m *OneMerge) GetException() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Error
}

// IsAborted returns true if this merge was or should be aborted.
func (m *OneMerge) IsAborted() bool {
	return m.Progress.IsAborted()
}

// Abort marks this merge as aborted; the merge thread should terminate at
// the soonest possible moment.
func (m *OneMerge) Abort() {
	m.Progress.Abort()
}

// CheckAborted returns an error if this merge has been aborted, mirroring
// OneMerge.checkAborted (which throws MergeAbortedException in Lucene).
func (m *OneMerge) CheckAborted() error {
	if m.IsAborted() {
		return fmt.Errorf("merge aborted: %s", m.String())
	}
	return nil
}

// TotalBytesSize returns the total input size in bytes of this merge.
func (m *OneMerge) TotalBytesSize() int64 {
	return m.TotalMergeBytes
}

// TotalNumDocs returns the total number of documents included in this merge.
func (m *OneMerge) TotalNumDocs() int {
	return m.TotalMaxDoc
}

// GetStoreMergeInfo returns a MergeInfo describing this merge.
func (m *OneMerge) GetStoreMergeInfo() *store.MergeInfo {
	return &store.MergeInfo{
		TotalMaxDoc:          m.TotalMaxDoc,
		EstimatedMergeBytes:  m.EstimatedMergeBytes,
		IsExternal:           m.IsExternal,
		MergeMaxNumSegments:  m.MaxNumSegments,
	}
}

// SegmentsSize returns the number of segments participating in this merge.
func (m *OneMerge) SegmentsSize() int {
	return len(m.Segments)
}

// Close closes this merge, invoking readerConsumer on every merge reader and
// recording whether the merge succeeded. Mirrors OneMerge.close.
func (m *OneMerge) Close(success bool, segmentDropped bool, readerConsumer func(mr *OneMerge)) {
	m.mu.Lock()
	select {
	case <-m.completed:
		// already closed
		m.mu.Unlock()
		return
	default:
		m.success = success
		close(m.completed)
	}
	readers := m.mergeReaders
	m.mergeReaders = nil
	m.mu.Unlock()

	if readerConsumer != nil {
		readerConsumer(m)
	}
	for _, r := range readers {
		if r != nil && r.Reader != nil {
			_ = r.Reader.Close()
		}
	}
}

// Complete marks the merge as finished (success or failure), unblocking any
// MergeObserver waiting on it. This is Gocene test/observer-facing sugar,
// not present on Lucene's OneMerge (which is driven by IndexWriter.merge).
func (m *OneMerge) Complete(success bool) {
	m.Close(success, false, nil)
}

// Done reports whether this merge has completed (successfully or not).
func (m *OneMerge) Done() <-chan struct{} {
	return m.completed
}

// Succeeded reports whether the merge finished successfully. Only valid
// after Done() is closed.
func (m *OneMerge) Succeeded() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.success
}

// String returns a readable description of the current merge state.
func (m *OneMerge) String() string {
	s := fmt.Sprintf("%d segments", len(m.Segments))
	if m.Info != nil && m.Info.Info != nil {
		s += fmt.Sprintf(" into %s", m.Info.Info.Name())
	}
	if m.MaxNumSegments != -1 {
		s += fmt.Sprintf(" [maxNumSegments=%d]", m.MaxNumSegments)
	}
	if m.IsAborted() {
		s += " [ABORTED]"
	}
	return s
}

// MergeObserver watches the merges in a MergeSpecification and lets callers
// wait for all of them to finish. This is Gocene glue (not a direct Lucene
// port) used by IndexWriter.ForceMergeDeletesWithObserver.
type MergeObserver struct {
	merges []*OneMerge
}

// NewMergeObserver creates a MergeObserver over spec's merges. A nil spec
// yields an observer with no merges, whose Await* methods return
// immediately/true.
func NewMergeObserver(spec *MergeSpecification) *MergeObserver {
	o := &MergeObserver{}
	if spec != nil {
		o.merges = spec.Merges
	}
	return o
}

// NumMerges returns the number of merges being observed.
func (o *MergeObserver) NumMerges() int {
	return len(o.merges)
}

// NumCompletedMerges returns how many of the observed merges have finished.
func (o *MergeObserver) NumCompletedMerges() int {
	n := 0
	for _, m := range o.merges {
		select {
		case <-m.Done():
			n++
		default:
		}
	}
	return n
}

// Await blocks until every observed merge has finished, returning true iff
// all of them succeeded.
func (o *MergeObserver) Await() bool {
	ok := true
	for _, m := range o.merges {
		<-m.Done()
		ok = ok && m.Succeeded()
	}
	return ok
}

// AwaitWithTimeout blocks until every observed merge has finished or timeout
// elapses, returning true iff all merges finished successfully within it.
func (o *MergeObserver) AwaitWithTimeout(timeout time.Duration) bool {
	deadline := time.After(timeout)
	for _, m := range o.merges {
		select {
		case <-m.Done():
			if !m.Succeeded() {
				return false
			}
		case <-deadline:
			return false
		}
	}
	return true
}

// AwaitAsync returns a channel that closes once every observed merge has
// finished.
func (o *MergeObserver) AwaitAsync() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, m := range o.merges {
			<-m.Done()
		}
	}()
	return done
}
