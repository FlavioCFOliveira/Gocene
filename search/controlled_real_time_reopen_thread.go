package search

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// ControlledRealTimeReopenThread is a utility class that runs a thread to manage periodic reopens
// of a ReferenceManager, with methods to wait for a specific index change to become visible.
//
// This is the Go port of Lucene's org.apache.lucene.search.ControlledRealTimeReopenThread.
type ControlledRealTimeReopenThread[T any] struct {
	manager          *index.ReferenceManager[T]
	targetMaxStaleNS int64
	targetMinStaleNS int64
	writer           *index.IndexWriter
	finish           atomic.Bool
	waitingGen       atomic.Int64
	searchingGen     atomic.Int64
	refreshStartGen   int64

	// mu protects searchingGen and the notification channel
	mu             sync.Mutex
	searchingGenNotify chan struct{}

	// reopenMu protects the reopen cycle
	reopenMu   sync.Mutex
	reopenSignal chan struct{}
}

// NewControlledRealTimeReopenThread creates a ControlledRealTimeReopenThread to periodically
// reopen the ReferenceManager.
func NewControlledRealTimeReopenThread[T any](
	writer *index.IndexWriter,
	manager *index.ReferenceManager[T],
	targetMaxStaleSec float64,
	targetMinStaleSec float64,
) *ControlledRealTimeReopenThread[T] {
	if targetMaxStaleSec < targetMinStaleSec {
		panic(fmt.Sprintf("targetMaxStaleSec (= %f) < targetMinStaleSec (= %f)", targetMaxStaleSec, targetMinStaleSec))
	}

	crt := &ControlledRealTimeReopenThread[T]{
		writer:           writer,
		manager:          manager,
		targetMaxStaleNS: int64(1000000000 * targetMaxStaleSec),
		targetMinStaleNS: int64(1000000000 * targetMinStaleSec),
		searchingGenNotify: make(chan struct{}),
		reopenSignal:       make(chan struct{}, 1),
	}

	manager.AddRefreshListener(crt)

	go crt.run()

	return crt
}

// BeforeRefresh implements index.RefreshListener.
func (crt *ControlledRealTimeReopenThread[T]) BeforeRefresh() {
	crt.refreshStartGen = crt.writer.GetMaxCompletedSequenceNumber()
}

// AfterRefresh implements index.RefreshListener.
func (crt *ControlledRealTimeReopenThread[T]) AfterRefresh(generation int64) {
	crt.mu.Lock()
	crt.searchingGen.Store(crt.refreshStartGen)
	// Notify all waiting threads
	close(crt.searchingGenNotify)
	crt.searchingGenNotify = make(chan struct{})
	crt.mu.Unlock()
}

// Close stops the reopen thread and notifies any waiting search threads.
func (crt *ControlledRealTimeReopenThread[T]) Close() error {
	crt.finish.Store(true)

	// Wake up the reopen thread
	crt.reopenMu.Lock()
	select {
	case crt.reopenSignal <- struct{}{}:
	default:
	}
	crt.reopenMu.Unlock()

	// Max it out so any waiting search threads will return:
	crt.searchingGen.Store(math.MaxInt64)
	crt.mu.Lock()
	close(crt.searchingGenNotify)
	crt.searchingGenNotify = make(chan struct{})
	crt.mu.Unlock()

	return nil
}

// WaitForGeneration waits for the target generation to become visible in the searcher.
func (crt *ControlledRealTimeReopenThread[T]) WaitForGeneration(targetGen int64) (bool, error) {
	return crt.WaitForGenerationWithTimeout(targetGen, -1)
}

// WaitForGenerationWithTimeout waits for the target generation to become visible in the searcher,
// up to a maximum specified milliseconds.
func (crt *ControlledRealTimeReopenThread[T]) WaitForGenerationWithTimeout(targetGen int64, maxMS int) (bool, error) {
	if targetGen > crt.searchingGen.Load() {
		// Notify the reopen thread that the waitingGen has changed
		crt.reopenMu.Lock()
		currentWaiting := crt.waitingGen.Load()
		if targetGen > currentWaiting {
			crt.waitingGen.Store(targetGen)
		}
		select {
		case crt.reopenSignal <- struct{}{}:
		default:
		}
		crt.reopenMu.Unlock()

		var timeout <-chan time.Time
		if maxMS >= 0 {
			timeout = time.After(time.Duration(maxMS) * time.Millisecond)
		}

		for {
			crt.mu.Lock()
			if targetGen <= crt.searchingGen.Load() {
				crt.mu.Unlock()
				return true, nil
			}
			notify := crt.searchingGenNotify
			crt.mu.Unlock()

			select {
			case <-notify:
				// woken up, check again
			case <-timeout:
				return false, nil
			}
		}
	}

	return true, nil
}

func (crt *ControlledRealTimeReopenThread[T]) run() {
	lastReopenStartNS := time.Now().UnixNano()

	for !crt.finish.Load() {
		// Loop until we've waited long enough before the next reopen:
		for !crt.finish.Load() {
			crt.reopenMu.Lock()

			hasWaiting := crt.waitingGen.Load() > crt.searchingGen.Load()
			staleNS := crt.targetMaxStaleNS
			if hasWaiting {
				staleNS = crt.targetMinStaleNS
			}

			nextReopenStartNS := lastReopenStartNS + staleNS
			sleepNS := nextReopenStartNS - time.Now().UnixNano()

			if sleepNS > 0 {
				timer := time.NewTimer(time.Duration(sleepNS) * time.Nanosecond)

				crt.reopenMu.Unlock()
				select {
				case <-timer.C:
					// timeout reached
				case <-crt.reopenSignal:
					// signaled
					timer.Stop()
				}
			} else {
				crt.reopenMu.Unlock()
				break
			}
		}

		if crt.finish.Load() {
			break
		}

		lastReopenStartNS = time.Now().UnixNano()

		// In Lucene this is manager.maybeRefreshBlocking()
		// We use MaybeRefresh() here as a faithful approximation.
		_, err := crt.manager.MaybeRefresh()
		if err != nil {
			panic(err)
		}
	}
}

func (crt *ControlledRealTimeReopenThread[T]) SearchingGen() int64 {
	return crt.searchingGen.Load()
}

func (crt *ControlledRealTimeReopenThread[T]) Equals(other any) bool {
	o, ok := other.(*ControlledRealTimeReopenThread[T])
	if !ok {
		return false
	}
	return crt == o
}

func (crt *ControlledRealTimeReopenThread[T]) HashCode() uint32 {
	return uint32(uintptr(unsafe.Pointer(crt)))
}

func (crt *ControlledRealTimeReopenThread[T]) String() string {
	return fmt.Sprintf("ControlledRealTimeReopenThread{searchingGen=%d, waitingGen=%d}",
		crt.searchingGen.Load(), crt.waitingGen.Load())
}
