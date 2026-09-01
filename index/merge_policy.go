package index

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// PauseReason describes why a merge thread was paused.
type PauseReason int

const (
	PauseReasonStopped PauseReason = iota
	PauseReasonPaused
	PauseReasonOther
)

func (pr PauseReason) String() string {
	switch pr {
	case PauseReasonStopped:
		return "STOPPED"
	case PauseReasonPaused:
		return "PAUSED"
	case PauseReasonOther:
		return "OTHER"
	default:
		return "UNKNOWN"
	}
}

// OneMergeProgress encapsulates the logic to pause, resume, or abort a merge thread.
type OneMergeProgress struct {
	wakeupChan   chan struct{}
	pauseTimesNS map[PauseReason]*atomic.Int64
	aborted      atomic.Bool
}

func NewOneMergeProgress() *OneMergeProgress {
	p := &OneMergeProgress{
		wakeupChan:   make(chan struct{}, 1),
		pauseTimesNS: make(map[PauseReason]*atomic.Int64),
	}
	for _, reason := range []PauseReason{PauseReasonStopped, PauseReasonPaused, PauseReasonOther} {
		p.pauseTimesNS[reason] = &atomic.Int64{}
	}
	return p
}

// Abort the merge this progress tracks at the next possible moment.
func (p *OneMergeProgress) Abort() {
	p.aborted.Store(true)
	p.Wakeup()
}

// IsAborted returns the aborted state of this merge.
func (p *OneMergeProgress) IsAborted() bool {
	return p.aborted.Load()
}

// PauseNanos pauses the calling goroutine for at least pauseNanos nanoseconds unless
// the merge is aborted or the external condition returns false.
func (p *OneMergeProgress) PauseNanos(pauseNanos int64, reason PauseReason, condition func() bool) {
	start := time.Now()
	timeUpdate := p.pauseTimesNS[reason]

	for pauseNanos > 0 && !p.aborted.Load() && condition() {
		timeout := time.Duration(pauseNanos)
		
		timer := time.NewTimer(timeout)
		select {
		case <-p.wakeupChan:
			timer.Stop()
			elapsed := time.Since(start).Nanoseconds()
			pauseNanos -= elapsed
		case <-timer.C:
			pauseNanos = 0
		}
	}
	
	timeUpdate.Add(time.Since(start).Nanoseconds())
}

// Wakeup requests a wakeup for any threads stalled in PauseNanos.
func (p *OneMergeProgress) Wakeup() {
	select {
	case p.wakeupChan <- struct{}{}:
	default:
	}
}

// GetPauseTimes returns pause reasons and associated times in nanoseconds.
func (p *OneMergeProgress) GetPauseTimes() map[PauseReason]int64 {
	res := make(map[PauseReason]int64)
	for k, v := range p.pauseTimesNS {
		res[k] = v.Load()
	}
	return res
}

// OneMerge provides the information necessary to perform an individual primitive merge operation.
type OneMerge struct {
	// MergeCompleted is used to signal when the merge is done.
	MergeCompleted chan bool
	
	// Internal fields used by IndexWriter
	Info              SegmentCommitInfo
	RegisterDone      bool
	MergeGen          int64
	IsExternal        bool
	MaxNumSegments    int
	UsesPooledReaders bool
	
	EstimatedMergeBytes atomic.Int64
	TotalMergeBytes     atomic.Int64
	
	// MergeReaders is used by IndexWriter
	MergeReaders []any // Replace any with actual MergeReader type when available
	
	// Segments to be merged.
	Segments []SegmentCommitInfo
}

func NewOneMerge(segments []SegmentCommitInfo) *OneMerge {
	return &OneMerge{
		MergeCompleted: make(chan bool, 1),
		Segments:       segments,
		MaxNumSegments:  -1,
	}
}

// MergeSpecification describes the set of merges that should be done.
type MergeSpecification struct {
	Merges []*OneMerge
}

func (ms *MergeSpecification) Add(merge *OneMerge) {
	ms.Merges = append(ms.Merges, merge)
}

// MergePolicy determines the sequence of primitive merge operations.
type MergePolicy interface {
	FindMerges(trigger MergeTrigger, infos SegmentInfos, ctx MergeContext) (*MergeSpecification, error)
	FindForcedMerges(infos SegmentInfos, maxNumSegments int, segmentsToMerge map[SegmentCommitInfo]bool, ctx MergeContext) (*MergeSpecification, error)
	FindForcedDeletesMerges(infos SegmentInfos, ctx MergeContext) (*MergeSpecification, error)
	FindFullFlushMerges(trigger MergeTrigger, infos SegmentInfos, ctx MergeContext) (*MergeSpecification, error)
	UseCompoundFile(infos SegmentInfos, mergedInfo SegmentCommitInfo, ctx MergeContext) (bool, error)
}

// BaseMergePolicy provides default implementations for MergePolicy.
type BaseMergePolicy struct {
	noCFSRatio       float64
	maxCFSSegmentSize int64
}

func NewBaseMergePolicy(noCFSRatio float64, maxCFSSegmentSize int64) *BaseMergePolicy {
	return &BaseMergePolicy{
		noCFSRatio:       noCFSRatio,
		maxCFSSegmentSize: maxCFSSegmentSize,
	}
}

// Default constants
const (
	DefaultNoCFSRatio       = 1.0
	DefaultMaxCFSSegmentSize = math.MaxInt64
)

// UseCompoundFile returns true if a new segment should use the compound file format.
func (bmp *BaseMergePolicy) UseCompoundFile(infos SegmentInfos, mergedInfo SegmentCommitInfo, ctx MergeContext) (bool, error) {
	if bmp.noCFSRatio == 0.0 {
		return false, nil
	}
	
	mergedInfoSize, err := bmp.size(mergedInfo, ctx)
	if err != nil {
		return false, err
	}
	
	if bmp.maxCFSSegmentSize != DefaultMaxCFSSegmentSize && mergedInfoSize > bmp.maxCFSSegmentSize {
		return false, nil
	}
	
	if bmp.noCFSRatio >= 1.0 {
		return true, nil
	}
	
	var totalSize int64
	for _, info := range infos {
		s, err := bmp.size(info, ctx)
		if err != nil {
			return false, err
		}
		totalSize += s
	}
	
	return float64(mergedInfoSize) <= bmp.noCFSRatio*float64(totalSize), nil
}

func (bmp *BaseMergePolicy) size(info SegmentCommitInfo, ctx MergeContext) (int64, error) {
	byteSize := info.SizeInBytes()
	delCount := ctx.NumDeletesToMerge(info)
	
	if !bmp.assertDelCount(delCount, info) {
		return 0, fmt.Errorf("invalid delete count %d for segment %v", delCount, info)
	}
	
	maxDoc := info.Info().MaxDoc()
	if maxDoc <= 0 {
		return byteSize, nil
	}
	
	delRatio := float64(delCount) / float64(maxDoc)
	
	return int64(float64(byteSize) * (1.0 - delRatio)), nil
}

func (bmp *BaseMergePolicy) assertDelCount(delCount int, info SegmentCommitInfo) bool {
	return delCount <= info.Info().MaxDoc()
}

func (bmp *BaseMergePolicy) MaxFullFlushMergeSize() int64 {
	return 0
}
