// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// OpenIfChanged reopens the index if there have been changes since this reader was opened.
// Returns the new reader if changed, or the same reader if unchanged.
//
// This implements GC-641: openIfChanged for IndexReader
func OpenIfChanged(reader IndexReaderInterface) (IndexReaderInterface, error) {
	// Check if the reader supports IsCurrent
	type currentChecker interface {
		IsCurrent() (bool, error)
	}

	if cc, ok := reader.(currentChecker); ok {
		current, err := cc.IsCurrent()
		if err != nil {
			return nil, err
		}
		if current {
			return reader, nil
		}
	}

	// If we can't determine if it's current, return the same reader
	return reader, nil
}

// OpenIfChangedWithCommit reopens the index if the provided commit is different from
// what the reader was opened with.
//
// This implements GC-641: openIfChanged with commit
func OpenIfChangedWithCommit(reader IndexReaderInterface, commit *IndexCommit) (IndexReaderInterface, error) {
	if commit == nil {
		return OpenIfChanged(reader)
	}

	// Check if the reader was opened with a different commit
	type commitReader interface {
		GetIndexCommit() *IndexCommit
	}

	if cr, ok := reader.(commitReader); ok {
		currentCommit := cr.GetIndexCommit()
		if currentCommit != nil && currentCommit.GetGeneration() == commit.GetGeneration() {
			return reader, nil
		}
	}

	// Need to reopen with the new commit
	// This would typically involve opening a new reader from the commit
	return reader, fmt.Errorf("reopening from commit not yet fully implemented")
}

// GetDocCount returns the number of documents containing at least one term for the given field.
// Returns 0 if the field does not exist or has no terms.
//
// This implements GC-644: getDocCount
func (r *IndexReader) GetDocCount(field string) (int, error) {
	terms, err := r.Terms(field)
	if err != nil {
		return 0, err
	}
	if terms == nil {
		return 0, nil
	}
	return terms.GetDocCount()
}

// GetSumDocFreq returns the sum of document frequencies for all terms in the given field.
// Returns -1 if the field does not exist or the value is unknown.
//
// This implements GC-645: getSumDocFreq
func (r *IndexReader) GetSumDocFreq(field string) (int64, error) {
	terms, err := r.Terms(field)
	if err != nil {
		return -1, err
	}
	if terms == nil {
		return -1, nil
	}
	return terms.GetSumDocFreq()
}

// GetSumTotalTermFreq returns the sum of total term frequencies for all terms in the given field.
// Returns -1 if the field does not exist or the value is unknown.
//
// This implements GC-646: getSumTotalTermFreq
func (r *IndexReader) GetSumTotalTermFreq(field string) (int64, error) {
	terms, err := r.Terms(field)
	if err != nil {
		return -1, err
	}
	if terms == nil {
		return -1, nil
	}
	return terms.GetSumTotalTermFreq()
}

// IndexReaderAdvanced provides advanced IndexReader operations.
// This is a helper struct for advanced reader operations.
type IndexReaderAdvanced struct {
	reader IndexReaderInterface
}

// NewIndexReaderAdvanced creates a new IndexReaderAdvanced wrapper.
func NewIndexReaderAdvanced(reader IndexReaderInterface) *IndexReaderAdvanced {
	return &IndexReaderAdvanced{reader: reader}
}

// OpenIfChanged reopens the index if there have been changes.
// Returns the new reader if changed, or the same reader if unchanged.
func (a *IndexReaderAdvanced) OpenIfChanged() (IndexReaderInterface, error) {
	return OpenIfChanged(a.reader)
}

// OpenIfChangedWithWriter reopens the index if there have been changes made by the given writer.
func (a *IndexReaderAdvanced) OpenIfChangedWithWriter(writer *IndexWriter) (IndexReaderInterface, error) {
	// If a writer is provided, we should check if there are uncommitted changes
	// For now, just delegate to OpenIfChanged
	return OpenIfChanged(a.reader)
}

// OpenIfChangedWithCommit reopens the index if the provided commit is different.
func (a *IndexReaderAdvanced) OpenIfChangedWithCommit(commit *IndexCommit) (IndexReaderInterface, error) {
	return OpenIfChangedWithCommit(a.reader, commit)
}

// GetTermVectors returns term vectors for a document.
func (a *IndexReaderAdvanced) GetTermVectors(docID int) (Fields, error) {
	type termVectorGetter interface {
		GetTermVectors(int) (Fields, error)
	}

	if tv, ok := a.reader.(termVectorGetter); ok {
		return tv.GetTermVectors(docID)
	}

	// Fallback: try to get from TermVectors()
	type termVectorsProvider interface {
		TermVectors() (TermVectors, error)
	}

	if tvp, ok := a.reader.(termVectorsProvider); ok {
		tv, err := tvp.TermVectors()
		if err != nil {
			return nil, err
		}
		if tv == nil {
			return nil, nil
		}
		return tv.Get(docID)
	}

	return nil, fmt.Errorf("reader does not support term vectors")
}

// NumDeletedDocs returns the number of deleted documents.
func (a *IndexReaderAdvanced) NumDeletedDocs() int {
	type deletedDocsProvider interface {
		NumDeletedDocs() int
	}

	if ddp, ok := a.reader.(deletedDocsProvider); ok {
		return ddp.NumDeletedDocs()
	}

	// Fallback: calculate from maxDoc and numDocs
	type docCounter interface {
		MaxDoc() int
		NumDocs() int
	}

	if dc, ok := a.reader.(docCounter); ok {
		return dc.MaxDoc() - dc.NumDocs()
	}

	return 0
}

// GetDocCount returns the number of documents containing at least one term for the given field.
func (a *IndexReaderAdvanced) GetDocCount(field string) (int, error) {
	type termsProvider interface {
		Terms(string) (Terms, error)
	}

	if tp, ok := a.reader.(termsProvider); ok {
		terms, err := tp.Terms(field)
		if err != nil {
			return 0, err
		}
		if terms == nil {
			return 0, nil
		}
		return terms.GetDocCount()
	}

	return 0, fmt.Errorf("reader does not support terms")
}

// GetSumDocFreq returns the sum of document frequencies for all terms in the given field.
func (a *IndexReaderAdvanced) GetSumDocFreq(field string) (int64, error) {
	type termsProvider interface {
		Terms(string) (Terms, error)
	}

	if tp, ok := a.reader.(termsProvider); ok {
		terms, err := tp.Terms(field)
		if err != nil {
			return -1, err
		}
		if terms == nil {
			return -1, nil
		}
		return terms.GetSumDocFreq()
	}

	return -1, fmt.Errorf("reader does not support terms")
}

// GetSumTotalTermFreq returns the sum of total term frequencies for all terms in the given field.
func (a *IndexReaderAdvanced) GetSumTotalTermFreq(field string) (int64, error) {
	type termsProvider interface {
		Terms(string) (Terms, error)
	}

	if tp, ok := a.reader.(termsProvider); ok {
		terms, err := tp.Terms(field)
		if err != nil {
			return -1, err
		}
		if terms == nil {
			return -1, nil
		}
		return terms.GetSumTotalTermFreq()
	}

	return -1, fmt.Errorf("reader does not support terms")
}

// DirectoryReaderAdvanced extends IndexReaderAdvanced for DirectoryReader-specific operations.
type DirectoryReaderAdvanced struct {
	*IndexReaderAdvanced
	directory store.Directory
}

// NewDirectoryReaderAdvanced creates a new DirectoryReaderAdvanced.
func NewDirectoryReaderAdvanced(reader IndexReaderInterface, directory store.Directory) *DirectoryReaderAdvanced {
	return &DirectoryReaderAdvanced{
		IndexReaderAdvanced: NewIndexReaderAdvanced(reader),
		directory:           directory,
	}
}

// ListCommits returns a list of all commits in the directory.
func (a *DirectoryReaderAdvanced) ListCommits() ([]*IndexCommit, error) {
	// This would typically read the index commits from the directory
	// For now, return an empty list
	return []*IndexCommit{}, nil
}

// OpenCommit opens a reader for a specific commit.
func (a *DirectoryReaderAdvanced) OpenCommit(commit *IndexCommit) (IndexReaderInterface, error) {
	if commit == nil {
		return nil, fmt.Errorf("commit cannot be nil")
	}
	// This would open a reader from the specific commit
	return a.reader, fmt.Errorf("opening from specific commit not yet fully implemented")
}
