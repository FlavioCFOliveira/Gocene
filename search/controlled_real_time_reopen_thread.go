// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// ControlledRealTimeReopenThread is the Go port of
// org.apache.lucene.search.ControlledRealTimeReopenThread (Apache Lucene
// 10.5.0): utility class that runs a thread to manage periodic reopens of a
// [ReferenceManager], with methods to wait for a specific index change to
// become visible. When a given search request needs to see a specific index
// change, call the WaitForGeneration method to wait for that change to be
// visible. Note that this will only scale well if most searches do not need
// to wait for a specific index generation.
//
// Java's class extends Thread. The Go rendering keeps the Thread members the
// class relies on: [ControlledRealTimeReopenThread.Start] (Thread.start()),
// [ControlledRealTimeReopenThread.Run] (the overridden run()) and
// [ControlledRealTimeReopenThread.Join] (Thread.join()). An exception that
// escapes run() terminates the thread and is reported the way the JVM's
// default uncaught-exception handler reports it, on standard error.
//
// @lucene.experimental
type ControlledRealTimeReopenThread[T comparable] struct {
	manager          *ReferenceManager[T]
	targetMaxStaleNS int64
	targetMinStaleNS int64
	writer           *index.IndexWriter
	finish           atomic.Bool
	waitingGen       atomic.Int64
	searchingGen     atomic.Int64
	refreshStartGen  int64

	// reopenLock renders `private final ReentrantLock reopenLock`; reopenCond
	// renders `reopenLock.newCondition()`: a signal is a token in the
	// buffered channel, which a waiter consumes (a stale token only causes
	// the spurious wake-up Condition.awaitNanos allows).
	reopenLock sync.Mutex
	reopenCond chan struct{}

	// monitor renders the object monitor (the synchronized methods and
	// wait()/notifyAll()); notifyAll closes monitorCh and installs a new one.
	monitor   sync.Mutex
	monitorCh chan struct{}

	// startOnce / done render the Thread life cycle.
	startOnce sync.Once
	started   atomic.Bool
	done      chan struct{}
}

// NewControlledRealTimeReopenThread creates ControlledRealTimeReopenThread, to
// periodically reopen the [ReferenceManager].
//
// targetMaxStaleSec is the maximum time until a new reader must be opened;
// this sets the upper bound on how slowly reopens may occur, when no caller
// is waiting for a specific generation to become visible.
//
// targetMinStaleSec is the minimum time until a new reader can be opened;
// this sets the lower bound on how quickly reopens may occur, when a caller
// is waiting for a specific generation to become visible.
func NewControlledRealTimeReopenThread[T comparable](
	writer *index.IndexWriter,
	manager *ReferenceManager[T],
	targetMaxStaleSec float64,
	targetMinStaleSec float64,
) (*ControlledRealTimeReopenThread[T], error) {
	if targetMaxStaleSec < targetMinStaleSec {
		return nil, fmt.Errorf("targetMaxScaleSec (= %v) < targetMinStaleSec (=%v)", targetMaxStaleSec, targetMinStaleSec)
	}
	t := &ControlledRealTimeReopenThread[T]{
		writer:           writer,
		manager:          manager,
		targetMaxStaleNS: int64(1000000000 * targetMaxStaleSec),
		targetMinStaleNS: int64(1000000000 * targetMinStaleSec),
		reopenCond:       make(chan struct{}, 1),
		monitorCh:        make(chan struct{}),
		done:             make(chan struct{}),
	}
	manager.AddListener(&handleRefresh[T]{t: t})
	return t, nil
}

// handleRefresh renders the private inner class HandleRefresh.
type handleRefresh[T comparable] struct {
	t *ControlledRealTimeReopenThread[T]
}

// BeforeRefresh renders HandleRefresh.beforeRefresh().
func (h *handleRefresh[T]) BeforeRefresh() error {
	// Save the gen as of when we started the reopen; the
	// listener (HandleRefresh above) copies this to
	// searchingGen once the reopen completes:
	h.t.refreshStartGen = h.t.writer.GetMaxCompletedSequenceNumber()
	return nil
}

// AfterRefresh renders HandleRefresh.afterRefresh(boolean didRefresh).
func (h *handleRefresh[T]) AfterRefresh(didRefresh bool) error {
	t := h.t
	t.monitor.Lock()
	defer t.monitor.Unlock()
	t.searchingGen.Store(t.refreshStartGen)
	t.notifyAllLocked()
	return nil
}

// notifyAllLocked renders notifyAll(); the caller holds the monitor.
func (t *ControlledRealTimeReopenThread[T]) notifyAllLocked() {
	close(t.monitorCh)
	t.monitorCh = make(chan struct{})
}

// waitLocked renders wait() (timeout <= 0) and wait(timeout); the caller
// holds the monitor, which is released while waiting and re-acquired before
// returning.
func (t *ControlledRealTimeReopenThread[T]) waitLocked(timeout time.Duration) {
	ch := t.monitorCh
	t.monitor.Unlock()
	if timeout <= 0 {
		<-ch
	} else {
		timer := time.NewTimer(timeout)
		select {
		case <-ch:
		case <-timer.C:
		}
		timer.Stop()
	}
	t.monitor.Lock()
}

// signalReopen renders reopenCond.signal(); the caller holds reopenLock.
func (t *ControlledRealTimeReopenThread[T]) signalReopen() {
	select {
	case t.reopenCond <- struct{}{}:
	default:
	}
}

// awaitReopenNanos renders reopenCond.awaitNanos(nanos); the caller holds
// reopenLock, which is released while waiting and re-acquired before
// returning.
func (t *ControlledRealTimeReopenThread[T]) awaitReopenNanos(nanos int64) {
	t.reopenLock.Unlock()
	timer := time.NewTimer(time.Duration(nanos))
	select {
	case <-t.reopenCond:
	case <-timer.C:
	}
	timer.Stop()
	t.reopenLock.Lock()
}

// Close renders the synchronized close(): it stops the reopen thread, waits
// for it to terminate and releases every thread waiting for a generation.
func (t *ControlledRealTimeReopenThread[T]) Close() error {
	t.monitor.Lock()
	defer t.monitor.Unlock()
	// System.out.println("NRT: set finish");

	t.finish.Store(true)

	// So thread wakes up and notices it should finish:
	t.reopenLock.Lock()
	t.signalReopen()
	t.reopenLock.Unlock()

	// join(): the Java class extends Thread, so Thread.join() waits on this
	// very monitor and releases it while waiting; the reopen thread may need
	// it to run HandleRefresh.afterRefresh before it can terminate.
	t.monitor.Unlock()
	t.Join()
	t.monitor.Lock()

	// Max it out so any waiting search threads will return:
	t.searchingGen.Store(math.MaxInt64)
	t.notifyAllLocked()
	return nil
}

// WaitForGeneration waits for the target generation to become visible in the
// searcher. If the current searcher is older than the target generation, this
// method will block until the searcher is reopened, by another via
// [ReferenceManager.MaybeRefresh] or until the [ReferenceManager] is closed.
func (t *ControlledRealTimeReopenThread[T]) WaitForGeneration(targetGen int64) {
	t.WaitForGenerationWithTimeout(targetGen, -1)
}

// WaitForGenerationWithTimeout waits for the target generation to become
// visible in the searcher, up to a maximum specified milli-seconds. If the
// current searcher is older than the target generation, this method will
// block until the searcher has been reopened by another thread via
// [ReferenceManager.MaybeRefresh], the given waiting time has elapsed, or
// until the [ReferenceManager] is closed.
//
// NOTE: if the waiting time elapses before the requested target generation is
// available the current [SearcherManager] is returned instead.
//
// maxMS is the maximum milliseconds to wait, or -1 to wait indefinitely. It
// returns true if the targetGen is now available, or false if maxMS wait time
// was exceeded.
func (t *ControlledRealTimeReopenThread[T]) WaitForGenerationWithTimeout(targetGen int64, maxMS int) bool {
	t.monitor.Lock()
	defer t.monitor.Unlock()
	if targetGen > t.searchingGen.Load() {
		// Notify the reopen thread that the waitingGen has
		// changed, so it may wake up and realize it should
		// not sleep for much or any longer before reopening:
		t.reopenLock.Lock()

		// Need to find waitingGen inside lock as it's used to determine
		// stale time
		// Math.max(waitingGen, targetGen)
		if w := t.waitingGen.Load(); w > targetGen {
			t.waitingGen.Store(w)
		} else {
			t.waitingGen.Store(targetGen)
		}

		t.signalReopen()
		t.reopenLock.Unlock()

		startMS := time.Duration(nanoTime()).Milliseconds()

		for targetGen > t.searchingGen.Load() {
			if maxMS < 0 {
				t.waitLocked(0)
			} else {
				msLeft := (startMS + int64(maxMS)) - time.Duration(nanoTime()).Milliseconds()
				if msLeft <= 0 {
					return false
				}
				t.waitLocked(time.Duration(msLeft) * time.Millisecond)
			}
		}
	}

	return true
}

// Start renders Thread.start(): it causes this thread to begin execution,
// running [ControlledRealTimeReopenThread.Run] in its own goroutine.
func (t *ControlledRealTimeReopenThread[T]) Start() {
	t.startOnce.Do(func() {
		t.started.Store(true)
		go func() {
			defer close(t.done)
			if err := t.Run(); err != nil {
				// Thread.getDefaultUncaughtExceptionHandler: the exception
				// escaping run() terminates the thread and is printed.
				fmt.Fprintf(os.Stderr, "Exception in thread \"ControlledRealTimeReopenThread\" %v\n", err)
			}
		}()
	})
}

// Join renders Thread.join(): it waits for this thread to die. It returns
// immediately when the thread was never started.
func (t *ControlledRealTimeReopenThread[T]) Join() {
	if !t.started.Load() {
		return
	}
	<-t.done
}

// Run renders the overridden Thread.run(). The error it returns renders the
// RuntimeException wrapping the IOException of maybeRefreshBlocking, which
// escapes run() in Java.
func (t *ControlledRealTimeReopenThread[T]) Run() error {
	// TODO: maybe use private thread ticktock timer, in
	// case clock shift messes up nanoTime?
	lastReopenStartNS := nanoTime()

	// System.out.println("reopen: start");
	for !t.finish.Load() {

		// TODO: try to guestimate how long reopen might
		// take based on past data?

		// Loop until we've waiting long enough before the
		// next reopen:
		for !t.finish.Load() {

			// Need lock before finding out if has waiting
			t.reopenLock.Lock()
			// True if we have someone waiting for reopened searcher:
			hasWaiting := t.waitingGen.Load() > t.searchingGen.Load()
			var stale int64
			if hasWaiting {
				stale = t.targetMinStaleNS
			} else {
				stale = t.targetMaxStaleNS
			}
			nextReopenStartNS := lastReopenStartNS + stale

			sleepNS := nextReopenStartNS - nanoTime()

			if sleepNS > 0 {
				t.awaitReopenNanos(sleepNS)
				t.reopenLock.Unlock()
			} else {
				t.reopenLock.Unlock()
				break
			}
		}

		if t.finish.Load() {
			break
		}

		lastReopenStartNS = nanoTime()
		if err := t.manager.MaybeRefreshBlocking(); err != nil {
			return fmt.Errorf("java.lang.RuntimeException: %w", err)
		}
	}
	return nil
}

// GetSearchingGen renders public long getSearchingGen().
func (t *ControlledRealTimeReopenThread[T]) GetSearchingGen() int64 {
	return t.searchingGen.Load()
}
