// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"context"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// RefreshCommitSupplier provides the commit to refresh the searcher on.
type RefreshCommitSupplier interface {
	// GetSearcherRefreshCommit returns the commit to refresh the searcher on.
	// Returns nil if no refresh is needed.
	GetSearcherRefreshCommit(reader *index.DirectoryReader) *index.IndexCommit
}

type defaultRefreshCommitSupplier struct{}

func (s *defaultRefreshCommitSupplier) GetSearcherRefreshCommit(reader *index.DirectoryReader) *index.IndexCommit {
	return nil
}

// SearcherManager is a utility class to safely share IndexSearcher instances across multiple threads,
// while periodically reopening. It ensures each searcher is closed only once all threads have
// finished using it.
type SearcherManager struct {
	*index.ReferenceManager[*IndexSearcher]
	searcherFactory       SearcherFactory
	refreshCommitSupplier RefreshCommitSupplier
}

// NewSearcherManager creates and returns a new SearcherManager from the given IndexWriter.
func NewSearcherManager(writer *index.IndexWriter, factory SearcherFactory) (*SearcherManager, error) {
	return NewSearcherManagerWithOptions(writer, true, false, factory, nil)
}

// NewSearcherManagerWithOptions creates and returns a new SearcherManager from the given IndexWriter,
// controlling whether past deletions should be applied.
func NewSearcherManagerWithOptions(
	writer *index.IndexWriter,
	applyAllDeletes bool,
	writeAllDeletes bool,
	factory SearcherFactory,
	supplier RefreshCommitSupplier,
) (*SearcherManager, error) {
	if factory == nil {
		factory = NewDefaultSearcherFactory()
	}

	reader, err := index.OpenDirectoryReaderFromWriterWithOptions(writer, applyAllDeletes, writeAllDeletes)
	if err != nil {
		return nil, err
	}

	searcher, err := GetSearcher(context.Background(), factory, reader, nil)
	if err != nil {
		reader.DecRef()
		return nil, err
	}

	if supplier == nil {
		supplier = &defaultRefreshCommitSupplier{}
	}

	sm := &SearcherManager{
		searcherFactory:       factory,
		refreshCommitSupplier: supplier,
	}

	sm.ReferenceManager = index.NewReferenceManagerWithFuncs(
		searcher,
		func(s *IndexSearcher) *IndexSearcher {
			s.GetIndexReader().IncRef()
			return s
		},
		func(s *IndexSearcher) error {
			return s.GetIndexReader().DecRef()
		},
	)

	return sm, nil
}

// NewSearcherManagerFromDir creates and returns a new SearcherManager from the given Directory.
func NewSearcherManagerFromDir(dir store.Directory, factory SearcherFactory) (*SearcherManager, error) {
	if factory == nil {
		factory = NewDefaultSearcherFactory()
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		return nil, err
	}

	searcher, err := GetSearcher(context.Background(), factory, reader, nil)
	if err != nil {
		reader.DecRef()
		return nil, err
	}

	sm := &SearcherManager{
		searcherFactory:       factory,
		refreshCommitSupplier: &defaultRefreshCommitSupplier{},
	}

	sm.ReferenceManager = index.NewReferenceManagerWithFuncs(
		searcher,
		func(s *IndexSearcher) *IndexSearcher {
			s.GetIndexReader().IncRef()
			return s
		},
		func(s *IndexSearcher) error {
			return s.GetIndexReader().DecRef()
		},
	)

	return sm, nil
}

// NewSearcherManagerFromReader creates and returns a new SearcherManager from an existing DirectoryReader.
// Note that this steals the incoming reference.
func NewSearcherManagerFromReader(reader *index.DirectoryReader, factory SearcherFactory, supplier RefreshCommitSupplier) (*SearcherManager, error) {
	if factory == nil {
		factory = NewDefaultSearcherFactory()
	}

	searcher, err := GetSearcher(context.Background(), factory, reader, nil)
	if err != nil {
		return nil, err
	}

	if supplier == nil {
		supplier = &defaultRefreshCommitSupplier{}
	}

	sm := &SearcherManager{
		searcherFactory:       factory,
		refreshCommitSupplier: supplier,
	}

	sm.ReferenceManager = index.NewReferenceManagerWithFuncs(
		searcher,
		func(s *IndexSearcher) *IndexSearcher {
			s.GetIndexReader().IncRef()
			return s
		},
		func(s *IndexSearcher) error {
			return s.GetIndexReader().DecRef()
		},
	)

	return sm, nil
}

// MaybeRefresh checks if a refresh is needed and performs it if so.
// Returns true if a refresh was performed, false otherwise.
func (sm *SearcherManager) MaybeRefresh() (bool, error) {
	// We can't call the embedded ReferenceManager.MaybeRefresh because it's a dummy.
	// We implement the logic here and use Swap.

	searcher := sm.GetCurrent()
	reader := searcher.GetIndexReader()
	dr, ok := reader.(*index.DirectoryReader)
	if !ok {
		return false, fmt.Errorf("searcher's IndexReader should be a DirectoryReader, but got %T", reader)
	}

	refreshCommit := sm.refreshCommitSupplier.GetSearcherRefreshCommit(dr)

	// To simulate openIfChanged(dr, refreshCommit):
	// 1. Open from commit
	newReader, err := dr.ReopenFromCommit(refreshCommit)
	if err != nil {
		return false, err
	}

	// 2. If the new reader is the same as the old one, it's current.
	if newReader == dr {
		newReader.DecRef()
		return false, nil
	}

	// 3. Create searcher and swap
	newSearcher, err := GetSearcher(context.Background(), sm.searcherFactory, newReader, reader)
	if err != nil {
		newReader.DecRef()
		return false, err
	}

	oldSearcher := sm.Swap(newSearcher)

	// The Swap method in ReferenceManager increments generation and updates current.
	// It returns the old reference. We must release it.
	sm.Release(oldSearcher)

	return true, nil
}

// Refresh refreshes the searcher and returns the new generation.
func (sm *SearcherManager) Refresh() (int64, error) {
	// In Java, this calls maybeRefresh() and returns the generation.
	// Since MaybeRefresh in Go returns bool, we can call it and then get generation.

	refreshed, err := sm.MaybeRefresh()
	if err != nil {
		return 0, err
	}

	if !refreshed {
		// If not refreshed, we still return the current generation.
		return sm.GetGeneration(), nil
	}

	return sm.GetGeneration(), nil
}

// GetSearcherCommitGeneration returns index commit generation for current searcher.
func (sm *SearcherManager) GetSearcherCommitGeneration() (int64, error) {
	s, err := sm.Acquire()
	if err != nil {
		return 0, err
	}
	defer sm.Release(s)

	reader := s.GetIndexReader()
	dr, ok := reader.(*index.DirectoryReader)
	if !ok {
		return 0, fmt.Errorf("searcher's IndexReader should be a DirectoryReader, but got %T", reader)
	}

	return dr.GetIndexCommit().Generation(), nil
}

// IsSearcherCurrent returns true if no changes have occurred since this searcher was opened.
func (sm *SearcherManager) IsSearcherCurrent() (bool, error) {
	s, err := sm.Acquire()
	if err != nil {
		return false, err
	}
	defer sm.Release(s)

	reader := s.GetIndexReader()
	dr, ok := reader.(*index.DirectoryReader)
	if !ok {
		return false, fmt.Errorf("searcher's IndexReader should be a DirectoryReader, but got %T", reader)
	}

	return dr.IsCurrent()
}

// GetSearcher creates a searcher from the provided IndexReader using the provided SearcherFactory.
func GetSearcher(ctx context.Context, factory SearcherFactory, reader index.IndexReaderInterface, previousReader index.IndexReaderInterface) (*IndexSearcher, error) {
	success := false
	var searcher *IndexSearcher
	var err error

	defer func() {
		if !success {
			reader.DecRef()
		}
	}()

	searcher, err = factory.NewSearcher(ctx, reader)
	if err != nil {
		return nil, err
	}

	if searcher.GetIndexReader() != reader {
		return nil, fmt.Errorf("SearcherFactory must wrap exactly the provided reader (got %v but expected %v)", searcher.GetIndexReader(), reader)
	}

	success = true
	return searcher, nil
}
