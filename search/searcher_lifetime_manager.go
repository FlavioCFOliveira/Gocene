// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// nanosPerSec renders SearcherLifetimeManager.NANOS_PER_SEC.
const nanosPerSec = 1000000000.0

// nanoTimeOrigin anchors nanoTime on Go's monotonic clock.
var nanoTimeOrigin = time.Now()

// nanoTime renders System.nanoTime(): a monotonic nanosecond reading with an
// arbitrary origin, only meaningful as a difference.
func nanoTime() int64 {
	return int64(time.Since(nanoTimeOrigin))
}

// searcherTracker renders the private static class
// SearcherLifetimeManager.SearcherTracker.
type searcherTracker struct {
	searcher      *IndexSearcher
	recordTimeSec float64
	version       int64

	// mu renders the monitor of the synchronized close().
	mu sync.Mutex
}

// newSearcherTracker renders SearcherTracker(IndexSearcher searcher).
func newSearcherTracker(searcher *IndexSearcher) (*searcherTracker, error) {
	t := &searcherTracker{searcher: searcher}
	t.version = searcher.GetIndexReader().(*index.DirectoryReader).GetVersion()
	if err := searcher.GetIndexReader().IncRef(); err != nil {
		return nil, err
	}
	// Use nanoTime not currentTimeMillis since it [in
	// theory] reduces risk from clock shift
	t.recordTimeSec = float64(nanoTime()) / nanosPerSec
	return t, nil
}

// compareTo renders SearcherTracker.compareTo: newer searchers are sort
// before older ones.
func (t *searcherTracker) compareTo(other *searcherTracker) int {
	return cmpFloat64(other.recordTimeSec, t.recordTimeSec)
}

// Close renders the synchronized SearcherTracker.close().
func (t *searcherTracker) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.searcher.GetIndexReader().DecRef()
}

// SearcherLifetimeManager is the Go port of
// org.apache.lucene.search.SearcherLifetimeManager (Apache Lucene 10.5.0):
// keeps track of current plus old IndexSearchers, closing the old ones once
// they have timed out.
//
// Use it like this:
//
//	mgr := search.NewSearcherLifetimeManager()
//
// Per search-request, if it's a "new" search request, then obtain the latest
// searcher you have (for example, by using [SearcherManager]), and then
// record this searcher:
//
//	// Record the current searcher, and save the returend
//	// token into user's search results (eg as a  hidden
//	// HTML form field):
//	token, err := mgr.Record(searcher)
//
// When a follow-up search arrives, for example the user clicks next page,
// drills down/up, etc., take the token that you saved from the previous
// search and:
//
//	// If possible, obtain the same searcher as the last
//	// search:
//	searcher := mgr.Acquire(token)
//	if searcher != nil {
//		// Searcher is still here
//		defer mgr.Release(searcher)
//		// Do searching...
//	} else {
//		// Searcher was pruned -- notify user session timed
//		// out, or, pull fresh searcher again
//	}
//
// Finally, in a separate thread, ideally the same thread that's periodically
// reopening your searchers, you should periodically prune old searchers:
//
//	mgr.Prune(search.NewPruneByAge(600.0))
//
// NOTE: keeping many searchers around means you'll use more resources (open
// files, RAM) than a single searcher. However, as long as you are using
// OpenIfChanged, the searchers will usually share almost all segments and the
// added resource usage is contained. When a large merge has completed, and
// you reopen, because that is a large change, the new searcher will use
// higher additional RAM than other searchers; but large merges don't
// complete very often and it's unlikely you'll hit two of them in your expiration
// window. Still you should budget plenty of heap in the JVM to have a good
// safety margin.
type SearcherLifetimeManager struct {
	closed atomic.Bool

	// TODO: we could get by w/ just a "set"; need to have
	// Tracker hash by its version and have compareTo(Long)
	// compare to its version
	searchers sync.Map // map[int64]*searcherTracker, renders ConcurrentHashMap<Long, SearcherTracker>

	// mu renders the monitor of the synchronized prune and close.
	mu sync.Mutex
}

// NewSearcherLifetimeManager renders new SearcherLifetimeManager().
func NewSearcherLifetimeManager() *SearcherLifetimeManager {
	return &SearcherLifetimeManager{}
}

// ensureOpen renders private void ensureOpen().
func (m *SearcherLifetimeManager) ensureOpen() error {
	if m.closed.Load() {
		return store.NewAlreadyClosedException("this SearcherLifetimeManager instance is closed", nil)
	}
	return nil
}

// Record records that you are now using this IndexSearcher. Always call this
// when you've obtained a possibly new [IndexSearcher], for example from
// [SearcherManager]. It's fine if you already passed the same searcher to this
// method before.
//
// This returns the int64 token that you can later pass to
// [SearcherLifetimeManager.Acquire] to retrieve the same IndexSearcher. You
// should record this int64 token in the search results sent to your user,
// such that if the user performs a follow-on action (clicks next page, drills
// down, etc.) the token is returned.
func (m *SearcherLifetimeManager) Record(searcher *IndexSearcher) (int64, error) {
	if err := m.ensureOpen(); err != nil {
		return 0, err
	}
	// TODO: we don't have to use IR.getVersion to track;
	// could be risky (if it's buggy); we could get better
	// bug isolation if we assign our own private ID:
	version := searcher.GetIndexReader().(*index.DirectoryReader).GetVersion()
	if v, ok := m.searchers.Load(version); !ok {
		tracker, err := newSearcherTracker(searcher)
		if err != nil {
			return 0, err
		}
		if _, loaded := m.searchers.LoadOrStore(version, tracker); loaded {
			// Another thread beat us -- must decRef to undo
			// incRef done by SearcherTracker ctor:
			if err := tracker.Close(); err != nil {
				return 0, err
			}
		}
	} else if tracker := v.(*searcherTracker); tracker.searcher != searcher {
		return 0, fmt.Errorf("the provided searcher has the same underlying reader version yet the searcher instance differs from before (new=%v vs old=%v", searcher, tracker.searcher)
	}

	return version, nil
}

// Acquire retrieves a previously recorded [IndexSearcher], if it has not yet
// been closed.
//
// NOTE: this may return nil when the requested searcher has already timed
// out. When this happens you should notify your user that their session
// timed out and that they'll have to restart their search.
//
// If this returns a non-nil result, you must match later call
// [SearcherLifetimeManager.Release] on this searcher, best from a finally
// clause.
func (m *SearcherLifetimeManager) Acquire(version int64) (*IndexSearcher, error) {
	if err := m.ensureOpen(); err != nil {
		return nil, err
	}
	if v, ok := m.searchers.Load(version); ok {
		tracker := v.(*searcherTracker)
		if tracker.searcher.GetIndexReader().TryIncRef() {
			return tracker.searcher, nil
		}
	}

	return nil, nil
}

// Release releases a searcher previously obtained from
// [SearcherLifetimeManager.Acquire].
//
// NOTE: it's fine to call this after Close.
func (m *SearcherLifetimeManager) Release(s *IndexSearcher) error {
	return s.GetIndexReader().DecRef()
}

// Pruner renders org.apache.lucene.search.SearcherLifetimeManager.Pruner:
// see [SearcherLifetimeManager.Prune].
type Pruner interface {
	// DoPrune returns true if this searcher should be removed.
	//
	// ageSec is how much time has passed since this searcher was the current
	// (live) searcher; searcher is the searcher.
	DoPrune(ageSec float64, searcher *IndexSearcher) bool
}

// PruneByAge renders org.apache.lucene.search.SearcherLifetimeManager.PruneByAge:
// simple pruner that drops any searcher older by more than the specified
// seconds, than the newest searcher.
type PruneByAge struct {
	maxAgeSec float64
}

// NewPruneByAge renders PruneByAge(double maxAgeSec).
func NewPruneByAge(maxAgeSec float64) (*PruneByAge, error) {
	if maxAgeSec < 0 {
		return nil, fmt.Errorf("maxAgeSec must be > 0 (got %v)", maxAgeSec)
	}
	return &PruneByAge{maxAgeSec: maxAgeSec}, nil
}

// DoPrune renders PruneByAge.doPrune(double ageSec, IndexSearcher searcher).
func (p *PruneByAge) DoPrune(ageSec float64, searcher *IndexSearcher) bool {
	return ageSec > p.maxAgeSec
}

var _ Pruner = (*PruneByAge)(nil)

// Prune calls provided [Pruner] to prune entries. The entries are passed to
// the Pruner in sorted (newest to oldest IndexSearcher) order.
//
// NOTE: you must periodically call this, ideally from the same background
// thread that opens new searchers.
func (m *SearcherLifetimeManager) Prune(pruner Pruner) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Cannot just pass searchers.values() to ArrayList ctor
	// (not thread-safe since the values can change while
	// ArrayList is init'ing itself); must instead iterate
	// ourselves:
	var trackers []*searcherTracker
	m.searchers.Range(func(_, v any) bool {
		trackers = append(trackers, v.(*searcherTracker))
		return true
	})
	sort.SliceStable(trackers, func(i, j int) bool {
		return trackers[i].compareTo(trackers[j]) < 0
	})
	lastRecordTimeSec := 0.0
	now := float64(nanoTime()) / nanosPerSec
	for _, tracker := range trackers {
		var ageSec float64
		if lastRecordTimeSec == 0.0 {
			ageSec = 0.0
		} else {
			ageSec = now - lastRecordTimeSec
		}
		// First tracker is always age 0.0 sec, since it's
		// still "live"; second tracker's age (= seconds since
		// it was "live") is now minus first tracker's
		// recordTime, etc:
		if pruner.DoPrune(ageSec, tracker.searcher) {
			m.searchers.Delete(tracker.version)
			if err := tracker.Close(); err != nil {
				return err
			}
		}
		lastRecordTimeSec = tracker.recordTimeSec
	}
	return nil
}

// Close closes this to future searching; any searches that are still in
// process in other threads won't be affected, and they should still call
// [SearcherLifetimeManager.Release] after they are done.
//
// NOTE: you must ensure no other threads are calling
// [SearcherLifetimeManager.Record] while you call close; otherwise it's
// possible not all searcher references will be freed.
func (m *SearcherLifetimeManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed.Store(true)
	var trackers []*searcherTracker
	m.searchers.Range(func(_, v any) bool {
		trackers = append(trackers, v.(*searcherTracker))
		return true
	})

	// Remove up front in case exc below, so we don't
	// over-decRef on double-close:
	for _, tracker := range trackers {
		m.searchers.Delete(tracker.version)
	}

	// IOUtils.close(toClose): close every tracker, then rethrow the first
	// exception hit.
	var firstErr error
	for _, tracker := range trackers {
		if err := tracker.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return firstErr
	}

	// Make some effort to catch mis-use:
	remaining := 0
	m.searchers.Range(func(_, _ any) bool {
		remaining++
		return true
	})
	if remaining != 0 {
		return errors.New("another thread called record while this SearcherLifetimeManager instance was being closed; not all searchers were closed")
	}
	return nil
}
