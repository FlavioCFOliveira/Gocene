// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// referenceManagerIsClosedMsg renders
// ReferenceManager.REFERENCE_MANAGER_IS_CLOSED_MSG.
const referenceManagerIsClosedMsg = "this ReferenceManager is closed"

// ReferenceManagerOverrides carries the abstract and the protected overridable
// members of org.apache.lucene.search.ReferenceManager. Every subclass
// (SearcherManager, ReaderManager, ...) implements it and installs itself as
// [ReferenceManager.Overrides]; the base class calls these members only
// through that back-pointer, which renders Java's virtual dispatch.
//
// A subclass that embeds *ReferenceManager inherits the empty default bodies
// of AfterClose and AfterMaybeRefresh, exactly as a Java subclass inherits the
// base-class bodies it does not override.
type ReferenceManagerOverrides[G comparable] interface {
	// DecRef renders protected abstract void decRef(G reference): decrement
	// the reference count of reference, closing it when the count drops to 0.
	DecRef(reference G) error

	// RefreshIfNeeded renders protected abstract G refreshIfNeeded(G
	// referenceToRefresh): it returns the refreshed reference, or the zero G
	// (Java null) when no refresh was needed.
	RefreshIfNeeded(referenceToRefresh G) (G, error)

	// TryIncRef renders protected abstract boolean tryIncRef(G reference).
	TryIncRef(reference G) (bool, error)

	// GetRefCount renders protected abstract int getRefCount(G reference).
	GetRefCount(reference G) int

	// AfterClose renders protected void afterClose(), called after close().
	AfterClose() error

	// AfterMaybeRefresh renders protected void afterMaybeRefresh(), called
	// after a refresh was attempted, regardless of whether a new reference
	// was in fact created.
	AfterMaybeRefresh() error
}

// RefreshListener renders org.apache.lucene.search.ReferenceManager.RefreshListener:
// use it to receive notification when a refresh has finished.
type RefreshListener interface {
	// BeforeRefresh is called right before a refresh attempt.
	BeforeRefresh() error

	// AfterRefresh is called after the refresh attempt, regardless of
	// whether a new reference was in fact created.
	//
	// didRefresh is true if a new reference was in fact created.
	AfterRefresh(didRefresh bool) error
}

// ReferenceManager is the Go port of org.apache.lucene.search.ReferenceManager
// (Apache Lucene 10.5.0): a utility class to safely share instances of a
// certain type across multiple threads, while periodically refreshing them.
// This class ensures each reference is closed only once all threads have
// finished using it. It is recommended to consult the documentation of
// ReferenceManager implementations for their maybeRefresh semantics.
//
// The class is abstract in Java. A Go subclass embeds *ReferenceManager[G],
// implements [ReferenceManagerOverrides] and installs itself as Overrides
// (see [NewReferenceManager]).
//
// Package placement: Lucene declares the class in org.apache.lucene.search,
// but org.apache.lucene.index.ReaderManager extends it. Go forbids the
// resulting index<->search import cycle, so the class is declared here and
// the search package re-exports it under its Lucene package with a type
// alias (search.ReferenceManager = index.ReferenceManager).
type ReferenceManager[G comparable] struct {
	// Overrides is the back-pointer to the concrete subclass.
	Overrides ReferenceManagerOverrides[G]

	// current renders `protected volatile G current`. The box is never
	// shared: every store publishes a fresh pointer, so a load observes a
	// complete value. A nil box and a box holding the zero G both render
	// Java null.
	current atomic.Pointer[G]

	// mu renders the object monitor that guards the synchronized methods
	// swapReference and close.
	mu sync.Mutex

	// refreshLock renders `private final Lock refreshLock = new ReentrantLock()`.
	refreshLock util.ReentrantLock

	// listenersMu serialises the writers of refreshListeners; readers take a
	// snapshot, which renders CopyOnWriteArrayList.
	listenersMu      sync.Mutex
	refreshListeners atomic.Pointer[[]RefreshListener]
}

// NewReferenceManager returns the base part of a ReferenceManager subclass,
// dispatching its abstract members to overrides. The subclass constructor then
// assigns the initial reference with [ReferenceManager.SetCurrent], as the
// Java subclass constructors assign the protected field current.
func NewReferenceManager[G comparable](overrides ReferenceManagerOverrides[G]) *ReferenceManager[G] {
	return &ReferenceManager[G]{Overrides: overrides}
}

// GetCurrent reads the protected volatile field current (Java null is the
// zero G). It is the subclass-side accessor of that field; applications use
// [ReferenceManager.Acquire].
func (rm *ReferenceManager[G]) GetCurrent() G {
	if p := rm.current.Load(); p != nil {
		return *p
	}
	var zero G
	return zero
}

// SetCurrent writes the protected volatile field current. It is the
// subclass-side accessor of that field, used by subclass constructors.
func (rm *ReferenceManager[G]) SetCurrent(reference G) {
	rm.current.Store(&reference)
}

// isNull reports whether reference renders Java null.
func isNull[G comparable](reference G) bool {
	var zero G
	return reference == zero
}

// ensureOpen renders private void ensureOpen().
func (rm *ReferenceManager[G]) ensureOpen() error {
	if isNull(rm.GetCurrent()) {
		return store.NewAlreadyClosedException(referenceManagerIsClosedMsg, nil)
	}
	return nil
}

// swapReference renders private synchronized void swapReference(G newReference).
func (rm *ReferenceManager[G]) swapReference(newReference G) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.swapReferenceLocked(newReference)
}

// swapReferenceLocked is the body of swapReference; the caller holds rm.mu
// (Java re-enters the monitor when close() calls swapReference).
func (rm *ReferenceManager[G]) swapReferenceLocked(newReference G) error {
	if err := rm.ensureOpen(); err != nil {
		return err
	}
	oldReference := rm.GetCurrent()
	rm.SetCurrent(newReference)
	return rm.Release(oldReference)
}

// Acquire obtains the current reference. You must match every call to
// acquire with one call to [ReferenceManager.Release]; it's best to do so in
// a finally clause, and set the reference to null to prevent accidental usage
// after it has been released.
//
// It returns an [store.AlreadyClosedException] if the reference manager has
// been closed.
func (rm *ReferenceManager[G]) Acquire() (G, error) {
	var zero G
	for {
		ref := rm.GetCurrent()
		if isNull(ref) {
			return zero, store.NewAlreadyClosedException(referenceManagerIsClosedMsg, nil)
		}
		ok, err := rm.Overrides.TryIncRef(ref)
		if err != nil {
			return zero, err
		}
		if ok {
			return ref, nil
		}
		if rm.Overrides.GetRefCount(ref) == 0 && rm.GetCurrent() == ref {
			if util.AssertsEnabled() && isNull(ref) {
				panic(util.NewAssertionError(nil))
			}
			// if we can't increment the reader but we are
			// still the current reference the RM is in a
			// illegal states since we can't make any progress
			// anymore. The reference is closed but the RM still
			// holds on to it as the actual instance.
			// This can only happen if somebody outside of the RM
			// decrements the refcount without a corresponding increment
			// since the RM assigns the new reference before counting down
			// the reference.
			return zero, errors.New("The managed reference has already closed - this is likely a bug when the reference count is modified outside of the ReferenceManager")
		}
	}
}

// Close closes this ReferenceManager to prevent future acquiring. A reference
// manager should be closed if the reference to the managed resource should be
// disposed or the application using the ReferenceManager is shutting down. The
// managed resource might not be released immediately, if the
// ReferenceManager user is holding on to a previously acquired reference. The
// resource will be released once when the last reference is released. Those
// references can still be used as if the manager was still active.
//
// Applications should not acquire new references from this manager once this
// method has been called. Acquiring a resource on a closed ReferenceManager
// returns an [store.AlreadyClosedException].
func (rm *ReferenceManager[G]) Close() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if !isNull(rm.GetCurrent()) {
		// make sure we can call this more than once
		// closeable javadoc says:
		// if this is already closed then invoking this method has no effect.
		var null G
		if err := rm.swapReferenceLocked(null); err != nil {
			return err
		}
		return rm.Overrides.AfterClose()
	}
	return nil
}

// AfterClose is the default (empty) body of protected void afterClose().
func (rm *ReferenceManager[G]) AfterClose() error {
	return nil
}

// doMaybeRefresh renders private void doMaybeRefresh(); owner is the holder
// identity of the calling goroutine, which already holds refreshLock.
func (rm *ReferenceManager[G]) doMaybeRefresh(owner util.LockOwner) error {
	// it's ok to call lock() here (blocking) because we're supposed to get here
	// from either maybeRefresh() or maybeRefreshBlocking(), after the lock has
	// already been obtained. Doing that protects us from an accidental bug
	// where this method will be called outside the scope of refreshLock.
	// Per ReentrantLock's javadoc, calling lock() by the same thread more than
	// once is ok, as long as unlock() is called a matching number of times.
	rm.refreshLock.Lock(owner)
	defer rm.refreshLock.Unlock(owner)

	refreshed := false
	reference, err := rm.Acquire()
	if err != nil {
		return err
	}
	err = func() error {
		if err := rm.notifyRefreshListenersBefore(); err != nil {
			return err
		}
		newReference, err := rm.Overrides.RefreshIfNeeded(reference)
		if err != nil {
			return err
		}
		if !isNull(newReference) {
			if util.AssertsEnabled() && newReference == reference {
				panic(util.NewAssertionError("refreshIfNeeded should return null if refresh wasn't needed"))
			}
			swapErr := rm.swapReference(newReference)
			if swapErr == nil {
				refreshed = true
			}
			// finally
			if !refreshed {
				if err := rm.Release(newReference); err != nil {
					return err
				}
			}
			return swapErr
		}
		return nil
	}()
	// finally
	if releaseErr := rm.Release(reference); releaseErr != nil {
		return releaseErr
	}
	if notifyErr := rm.notifyRefreshListenersRefreshed(refreshed); notifyErr != nil {
		return notifyErr
	}
	if err != nil {
		return err
	}
	return rm.Overrides.AfterMaybeRefresh()
}

// MaybeRefresh refreshes the current reference if needed, returning whether
// the refresh was attempted by this call.
//
// You must call this (or [ReferenceManager.MaybeRefreshBlocking]),
// periodically, if you want that [ReferenceManager.Acquire] will return
// refreshed instances.
//
// Threads: it's fine for more than one thread to call this at once. Only the
// first thread will attempt the refresh; subsequent threads will see that
// another thread is already handling refresh and will return immediately.
// Note that this means if another thread is already refreshing then
// subsequent threads will return right away without waiting for the refresh
// to complete.
//
// If this method returns true it means the calling thread either refreshed or
// that there were no changes to refresh. If it returns false it means another
// thread is currently refreshing.
func (rm *ReferenceManager[G]) MaybeRefresh() (bool, error) {
	if err := rm.ensureOpen(); err != nil {
		return false, err
	}

	// Ensure only 1 thread does refresh at once; other threads just return immediately:
	owner := util.NewLockOwner()
	doTryRefresh := rm.refreshLock.TryLock(owner)
	if doTryRefresh {
		defer rm.refreshLock.Unlock(owner)
		if err := rm.doMaybeRefresh(owner); err != nil {
			return false, err
		}
	}

	return doTryRefresh, nil
}

// MaybeRefreshBlocking refreshes the current reference if needed. It is
// similar to [ReferenceManager.MaybeRefresh], but waits until the refresh
// completes, if another thread is currently refreshing.
//
// This method is useful in case you want to guarantee that the next call to
// [ReferenceManager.Acquire] will return a refreshed instance. Otherwise,
// consider using the non-blocking MaybeRefresh.
func (rm *ReferenceManager[G]) MaybeRefreshBlocking() error {
	if err := rm.ensureOpen(); err != nil {
		return err
	}

	// Ensure only 1 thread does refresh at once
	owner := util.NewLockOwner()
	rm.refreshLock.Lock(owner)
	defer rm.refreshLock.Unlock(owner)
	return rm.doMaybeRefresh(owner)
}

// AfterMaybeRefresh is the default (empty) body of protected void
// afterMaybeRefresh().
func (rm *ReferenceManager[G]) AfterMaybeRefresh() error {
	return nil
}

// Release releases the reference previously obtained via
// [ReferenceManager.Acquire].
//
// NOTE: it's safe to call this after [ReferenceManager.Close].
func (rm *ReferenceManager[G]) Release(reference G) error {
	if util.AssertsEnabled() && isNull(reference) {
		panic(util.NewAssertionError(nil))
	}
	return rm.Overrides.DecRef(reference)
}

// listeners returns the current snapshot of the refresh listeners.
func (rm *ReferenceManager[G]) listeners() []RefreshListener {
	if p := rm.refreshListeners.Load(); p != nil {
		return *p
	}
	return nil
}

// notifyRefreshListenersBefore renders private void notifyRefreshListenersBefore().
func (rm *ReferenceManager[G]) notifyRefreshListenersBefore() error {
	for _, refreshListener := range rm.listeners() {
		if err := refreshListener.BeforeRefresh(); err != nil {
			return err
		}
	}
	return nil
}

// notifyRefreshListenersRefreshed renders private void
// notifyRefreshListenersRefreshed(boolean didRefresh).
func (rm *ReferenceManager[G]) notifyRefreshListenersRefreshed(didRefresh bool) error {
	for _, refreshListener := range rm.listeners() {
		if err := refreshListener.AfterRefresh(didRefresh); err != nil {
			return err
		}
	}
	return nil
}

// AddListener adds a listener, to be notified when a reference is refreshed/swapped.
func (rm *ReferenceManager[G]) AddListener(listener RefreshListener) {
	if listener == nil {
		panic("Listener must not be null")
	}
	rm.listenersMu.Lock()
	defer rm.listenersMu.Unlock()
	old := rm.listeners()
	next := make([]RefreshListener, len(old), len(old)+1)
	copy(next, old)
	next = append(next, listener)
	rm.refreshListeners.Store(&next)
}

// RemoveListener removes a listener added with [ReferenceManager.AddListener].
func (rm *ReferenceManager[G]) RemoveListener(listener RefreshListener) {
	if listener == nil {
		panic("Listener must not be null")
	}
	rm.listenersMu.Lock()
	defer rm.listenersMu.Unlock()
	old := rm.listeners()
	for i, l := range old {
		if l == listener {
			next := make([]RefreshListener, 0, len(old)-1)
			next = append(next, old[:i]...)
			next = append(next, old[i+1:]...)
			rm.refreshListeners.Store(&next)
			return
		}
	}
}
