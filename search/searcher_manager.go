// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SearcherManager is the Go port of org.apache.lucene.search.SearcherManager
// (Apache Lucene 10.5.0): utility class to safely share [IndexSearcher]
// instances across multiple threads, while periodically reopening. This class
// ensures each searcher is closed only once all threads have finished using
// it.
//
// Use [ReferenceManager.Acquire] to obtain the current searcher, and
// [ReferenceManager.Release] to release it, like this:
//
//	s, err := manager.Acquire()
//	if err != nil { ... }
//	defer manager.Release(s)
//	// Do searching, doc retrieval, etc. with s
//	// Do not use s after this!
//
// In addition you should periodically call [ReferenceManager.MaybeRefresh].
// While it's possible to call this just before running each query, this is
// discouraged since it penalizes the unlucky queries that need to refresh.
// It's better to use a separate background thread, that periodically calls
// MaybeRefresh. Finally, be sure to call [ReferenceManager.Close] once you
// are done.
type SearcherManager struct {
	*ReferenceManager[*IndexSearcher]

	searcherFactory       SearcherFactory
	refreshCommitSupplier RefreshCommitSupplier
}

// newSearcherManagerBase wires the ReferenceManager base to a fresh
// SearcherManager and installs the default refreshCommitSupplier
// (`new RefreshCommitSupplier() {}`).
func newSearcherManagerBase(searcherFactory SearcherFactory) *SearcherManager {
	sm := &SearcherManager{
		searcherFactory:       searcherFactory,
		refreshCommitSupplier: DefaultRefreshCommitSupplier{},
	}
	sm.ReferenceManager = index.NewReferenceManager[*IndexSearcher](sm)
	return sm
}

// NewSearcherManager creates and returns a new SearcherManager from the given
// [index.IndexWriter]; it renders SearcherManager(IndexWriter, SearcherFactory),
// which applies all deletes and does not write them.
//
// searcherFactory is an optional SearcherFactory; if nil, a default
// [BaseSearcherFactory] is used. It can be used to warm new searchers.
func NewSearcherManager(writer *index.IndexWriter, searcherFactory SearcherFactory) (*SearcherManager, error) {
	return NewSearcherManagerWithDeletes(writer, true, false, searcherFactory)
}

// NewSearcherManagerWithDeletes creates and returns a new SearcherManager from
// the given [index.IndexWriter]; it renders SearcherManager(IndexWriter,
// boolean applyAllDeletes, boolean writeAllDeletes, SearcherFactory).
//
// If applyAllDeletes is true, all buffered deletes will be applied (made
// visible) in the IndexSearcher / DirectoryReader. If false, the deletes may
// or may not be applied, but remain buffered (in IndexWriter) so that they
// will be applied in the future. Applying deletes can be costly, so if your
// app can tolerate deleted documents being returned you might gain some
// performance by passing false. If writeAllDeletes is true, new deletes will
// be forcefully written to index files.
func NewSearcherManagerWithDeletes(writer *index.IndexWriter, applyAllDeletes, writeAllDeletes bool, searcherFactory SearcherFactory) (*SearcherManager, error) {
	return NewSearcherManagerWithDeletesAndRefreshCommitSupplier(writer, applyAllDeletes, writeAllDeletes, searcherFactory, nil)
}

// NewSearcherManagerWithDeletesAndRefreshCommitSupplier creates and returns a
// new SearcherManager from the given [index.IndexWriter]; it renders
// SearcherManager(IndexWriter, boolean applyAllDeletes, boolean
// writeAllDeletes, SearcherFactory, RefreshCommitSupplier).
//
// refreshCommitSupplier supplies the commit to refresh on, when the
// searcher is refreshed; nil keeps the default (the latest commit).
func NewSearcherManagerWithDeletesAndRefreshCommitSupplier(
	writer *index.IndexWriter,
	applyAllDeletes, writeAllDeletes bool,
	searcherFactory SearcherFactory,
	refreshCommitSupplier RefreshCommitSupplier,
) (*SearcherManager, error) {
	if searcherFactory == nil {
		searcherFactory = NewSearcherFactory()
	}
	sm := newSearcherManagerBase(searcherFactory)
	reader, err := index.OpenDirectoryReaderFromWriterWithOptions(writer, applyAllDeletes, writeAllDeletes)
	if err != nil {
		return nil, err
	}
	current, err := GetSearcher(searcherFactory, reader, nil)
	if err != nil {
		return nil, err
	}
	sm.SetCurrent(current)
	if refreshCommitSupplier != nil {
		sm.refreshCommitSupplier = refreshCommitSupplier
	}
	return sm, nil
}

// NewSearcherManagerFromDir creates and returns a new SearcherManager from
// the given [store.Directory]; it renders SearcherManager(Directory,
// SearcherFactory).
func NewSearcherManagerFromDir(dir store.Directory, searcherFactory SearcherFactory) (*SearcherManager, error) {
	if searcherFactory == nil {
		searcherFactory = NewSearcherFactory()
	}
	sm := newSearcherManagerBase(searcherFactory)
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		return nil, err
	}
	current, err := GetSearcher(searcherFactory, reader, nil)
	if err != nil {
		return nil, err
	}
	sm.SetCurrent(current)
	return sm, nil
}

// NewSearcherManagerFromReader creates and returns a new SearcherManager from
// an existing [index.DirectoryReader]; it renders
// SearcherManager(DirectoryReader, SearcherFactory). Note that this steals the
// incoming reference.
func NewSearcherManagerFromReader(reader *index.DirectoryReader, searcherFactory SearcherFactory) (*SearcherManager, error) {
	return NewSearcherManagerFromReaderWithRefreshCommitSupplier(reader, searcherFactory, nil)
}

// NewSearcherManagerFromReaderWithRefreshCommitSupplier creates and returns a
// new SearcherManager from an existing [index.DirectoryReader]; it renders
// SearcherManager(DirectoryReader, SearcherFactory, RefreshCommitSupplier).
// Note that this steals the incoming reference.
func NewSearcherManagerFromReaderWithRefreshCommitSupplier(
	reader *index.DirectoryReader,
	searcherFactory SearcherFactory,
	refreshCommitSupplier RefreshCommitSupplier,
) (*SearcherManager, error) {
	if searcherFactory == nil {
		searcherFactory = NewSearcherFactory()
	}
	sm := newSearcherManagerBase(searcherFactory)
	current, err := GetSearcher(searcherFactory, reader, nil)
	if err != nil {
		return nil, err
	}
	sm.SetCurrent(current)
	if refreshCommitSupplier != nil {
		sm.refreshCommitSupplier = refreshCommitSupplier
	}
	return sm, nil
}

// DecRef renders protected void decRef(IndexSearcher reference).
func (sm *SearcherManager) DecRef(reference *IndexSearcher) error {
	return reference.GetIndexReader().DecRef()
}

// RefreshIfNeeded renders protected IndexSearcher refreshIfNeeded(IndexSearcher
// referenceToRefresh).
func (sm *SearcherManager) RefreshIfNeeded(referenceToRefresh *IndexSearcher) (*IndexSearcher, error) {
	r := referenceToRefresh.GetIndexReader()
	dr, ok := r.(*index.DirectoryReader)
	if util.AssertsEnabled() && !ok {
		panic(util.NewAssertionError(fmt.Sprintf("searcher's IndexReader should be a DirectoryReader, but got %v", r)))
	}
	refreshCommit, err := sm.refreshCommitSupplier.GetSearcherRefreshCommit(dr)
	if err != nil {
		return nil, err
	}
	newReader, err := index.OpenIfChangedWithCommit(dr, refreshCommit)
	if err != nil {
		return nil, err
	}
	if newReader == nil {
		return nil, nil
	}
	return GetSearcher(sm.searcherFactory, newReader, r)
}

// TryIncRef renders protected boolean tryIncRef(IndexSearcher reference).
func (sm *SearcherManager) TryIncRef(reference *IndexSearcher) (bool, error) {
	return reference.GetIndexReader().TryIncRef(), nil
}

// GetRefCount renders protected int getRefCount(IndexSearcher reference).
func (sm *SearcherManager) GetRefCount(reference *IndexSearcher) int {
	return int(reference.GetIndexReader().GetRefCount())
}

// getSearcherCommitGeneration renders the package-private long
// getSearcherCommitGeneration(): the index commit generation for the current
// searcher.
func (sm *SearcherManager) getSearcherCommitGeneration() (int64, error) {
	s, err := sm.Acquire()
	if err != nil {
		return 0, err
	}
	gen := s.GetIndexReader().(*index.DirectoryReader).GetIndexCommit().GetGeneration()
	if err := sm.Release(s); err != nil {
		return 0, err
	}
	return gen, nil
}

// isSearcherCurrent renders the package-private boolean isSearcherCurrent():
// true if no changes have occurred since this searcher ie. reader was opened,
// otherwise false.
func (sm *SearcherManager) isSearcherCurrent() (current bool, err error) {
	searcher, err := sm.Acquire()
	if err != nil {
		return false, err
	}
	defer func() {
		if releaseErr := sm.Release(searcher); releaseErr != nil {
			current, err = false, releaseErr
		}
	}()
	r := searcher.GetIndexReader()
	dr, ok := r.(*index.DirectoryReader)
	if util.AssertsEnabled() && !ok {
		panic(util.NewAssertionError(fmt.Sprintf("searcher's IndexReader should be a DirectoryReader, but got %v", r)))
	}
	return dr.IsCurrent()
}

// GetSearcher renders public static IndexSearcher getSearcher(SearcherFactory
// searcherFactory, IndexReader reader, IndexReader previousReader): expert,
// it creates a searcher from the provided [index.IndexReaderInterface] using
// the provided [SearcherFactory]. NOTE: this decRefs incoming reader on
// throwing an exception.
func GetSearcher(searcherFactory SearcherFactory, reader, previousReader index.IndexReaderInterface) (searcher *IndexSearcher, err error) {
	success := false
	defer func() {
		if !success {
			// an exception thrown by the finally block replaces the pending one
			if decErr := reader.DecRef(); decErr != nil {
				searcher, err = nil, decErr
			}
		}
	}()
	searcher, err = searcherFactory.NewSearcher(reader, previousReader)
	if err != nil {
		return nil, err
	}
	if searcher.GetIndexReader() != reader {
		return nil, fmt.Errorf("SearcherFactory must wrap exactly the provided reader (got %v but expected %v)",
			searcher.GetIndexReader(), reader)
	}
	success = true
	return searcher, nil
}

var _ ReferenceManagerOverrides[*IndexSearcher] = (*SearcherManager)(nil)
