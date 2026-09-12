// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"math/rand"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// RandomIndexWriter is a wrapper around IndexWriter that randomizes the indexing experience.
// It may swap in a different merge policy/scheduler, commit periodically, may or may not
// forceMerge in the end, may flush by doc count instead of RAM, etc.
//
// This is the Go port of Lucene's org.apache.lucene.tests.index.RandomIndexWriter.
type RandomIndexWriter struct {
	W                        *IndexWriter
	r                        *rand.Rand
	docCount                 int
	flushAt                  int
	flushAtFactor            float64
	getReaderCalled          bool
	analyzer                 analysis.Analyzer // only if WE created it (then we close it)
	softDeletesRatio         float64
	config                   *LiveIndexWriterConfig
	shouldRandomForceMerge   bool
	doRandomForceMergeAssert bool
	mu                       sync.Mutex
}

// NewRandomIndexWriter creates a new RandomIndexWriter with the provided IndexWriter and Random.
func NewRandomIndexWriter(w *IndexWriter, r *rand.Rand) *RandomIndexWriter {
	if r == nil {
		r = rand.New(rand.NewSource(0))
	}
	// Create a new Random with a different seed derived from the input random
	newRand := rand.New(rand.NewSource(r.Int63()))

	riw := &RandomIndexWriter{
		W:             w,
		r:             newRand,
		flushAt:       nextInt(newRand, 10, 1000),
		flushAtFactor: 1.0,
		config:        w.GetConfig().LiveIndexWriterConfig,
		analyzer:      nil,
	}

	// Randomly decide whether to do force merges
	if w.GetConfig().GetMergePolicy() != nil {
		riw.shouldRandomForceMerge = newRand.Intn(2) == 0
	}

	return riw
}

// NewRandomIndexWriterWithConfig creates a new RandomIndexWriter with a config and random instance.
func NewRandomIndexWriterWithConfig(r *rand.Rand, dir store.Directory, config *IndexWriterConfig) (*RandomIndexWriter, error) {
	if r == nil {
		r = rand.New(rand.NewSource(0))
	}
	// Create a new Random with a different seed derived from the input random
	newRand := rand.New(rand.NewSource(r.Int63()))

	// Optionally set soft deletes
	softDeletesRatio := 0.0
	if newRand.Intn(2) == 0 {
		softDeletesRatio = 1.0 / float64(1+newRand.Intn(10))
		config.SetSoftDeletesField("___soft_deletes")
	}

	w, err := NewIndexWriter(dir, config)
	if err != nil {
		return nil, err
	}

	riw := &RandomIndexWriter{
		W:                w,
		r:                newRand,
		flushAt:          nextInt(newRand, 10, 1000),
		flushAtFactor:    1.0,
		config:           w.GetConfig().LiveIndexWriterConfig,
		softDeletesRatio: softDeletesRatio,
		analyzer:         config.GetAnalyzer(),
	}

	if w.GetConfig().GetMergePolicy() != nil {
		riw.shouldRandomForceMerge = newRand.Intn(2) == 0
	}

	return riw, nil
}

// AddDocument adds a document.
func (riw *RandomIndexWriter) AddDocument(doc *document.Document) (int64, error) {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	maybeChangeConfig(riw.r, riw.config)

	var seqNo int64
	var err error

	if riw.r.Intn(5) == 3 {
		// Sometimes add as addDocuments
		docs := []*document.Document{doc}
		seqNo, err = riw.W.AddDocuments(docs)
	} else {
		seqNo, err = riw.W.AddDocument(doc)
	}

	if err != nil {
		return 0, err
	}

	err = riw.maybeFlushOrCommit()
	return seqNo, err
}

// AddDocuments adds multiple documents.
func (riw *RandomIndexWriter) AddDocuments(docs []*document.Document) (int64, error) {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	maybeChangeConfig(riw.r, riw.config)

	seqNo, err := riw.W.AddDocuments(docs)
	if err != nil {
		return 0, err
	}

	err = riw.maybeFlushOrCommit()
	return seqNo, err
}

// UpdateDocument updates a document.
func (riw *RandomIndexWriter) UpdateDocument(t *Term, doc *document.Document) (int64, error) {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	maybeChangeConfig(riw.r, riw.config)

	var seqNo int64
	var err error

	if riw.useSoftDeletes() {
		if riw.r.Intn(5) == 3 {
			docs := []*document.Document{doc}
			seqNo, err = riw.W.UpdateDocuments(t, docs)
		} else {
			seqNo, err = riw.W.UpdateDocument(t, doc)
		}
	} else {
		if riw.r.Intn(5) == 3 {
			docs := []*document.Document{doc}
			seqNo, err = riw.W.UpdateDocuments(t, docs)
		} else {
			seqNo, err = riw.W.UpdateDocument(t, doc)
		}
	}

	if err != nil {
		return 0, err
	}

	err = riw.maybeFlushOrCommit()
	return seqNo, err
}

// DeleteDocuments deletes documents matching a term.
func (riw *RandomIndexWriter) DeleteDocuments(t *Term) (int64, error) {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	maybeChangeConfig(riw.r, riw.config)
	return riw.W.DeleteDocuments([]Term{*t})
}

// DeleteDocumentsWithQuery deletes documents matching a query.
//
// Mirrors RandomIndexWriter.deleteDocuments(Query); IndexWriter exposes the
// Lucene varargs form as DeleteDocumentsQuery([]Query).
func (riw *RandomIndexWriter) DeleteDocumentsWithQuery(q Query) (int64, error) {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	maybeChangeConfig(riw.r, riw.config)
	return riw.W.DeleteDocumentsQuery([]Query{q})
}

// UpdateDocValues updates doc values.
func (riw *RandomIndexWriter) UpdateDocValues(term *Term, updates ...*document.Field) (int64, error) {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	maybeChangeConfig(riw.r, riw.config)
	return riw.W.UpdateDocValues(term, updates)
}

// Commit commits the changes.
func (riw *RandomIndexWriter) Commit() (int64, error) {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	maybeChangeConfig(riw.r, riw.config)
	flushConcurrently := riw.r.Intn(10) == 0
	if flushConcurrently {
		// In a real implementation, we would spawn a goroutine to flush
		// For now, just do a regular commit
	}
	return riw.W.Commit()
}

// ForceMerge forces a merge to the specified segment count.
func (riw *RandomIndexWriter) ForceMerge(maxSegmentCount int) error {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	maybeChangeConfig(riw.r, riw.config)
	return riw.W.ForceMerge(maxSegmentCount)
}

// SetDoRandomForceMerge sets whether to do random force merges.
func (riw *RandomIndexWriter) SetDoRandomForceMerge(v bool) {
	riw.shouldRandomForceMerge = v
}

// SetDoRandomForceMergeAssert sets whether to assert merge limits.
func (riw *RandomIndexWriter) SetDoRandomForceMergeAssert(v bool) {
	riw.doRandomForceMergeAssert = v
}

// GetReader returns a reader.
func (riw *RandomIndexWriter) GetReader() (*DirectoryReader, error) {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	maybeChangeConfig(riw.r, riw.config)
	riw.getReaderCalled = true

	if riw.r.Intn(20) == 2 {
		if err := riw.doRandomForceMerge(); err != nil {
			return nil, err
		}
	}

	if riw.r.Intn(2) == 0 {
		// Use NRT reader
		if riw.r.Intn(5) == 1 {
			if _, err := riw.W.Commit(); err != nil {
				return nil, err
			}
		}
		// Return an NRT reader reflecting the writer's buffered state.
		return OpenDirectoryReaderFromWriterWithOptions(riw.W, true, false)
	}

	// Open new reader from directory
	if _, err := riw.W.Commit(); err != nil {
		return nil, err
	}
	return DirectoryReaderOpen(riw.W.GetDirectory())
}

// Close closes the writer.
func (riw *RandomIndexWriter) Close() error {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	// If someone isn't using GetReader, force merge so that a reader opened on
	// the directory afterwards sees a merged index; mirrors
	// RandomIndexWriter.close().
	var err error
	if !riw.getReaderCalled && riw.r.Intn(8) == 2 && !riw.W.IsClosed() {
		if mergeErr := riw.doRandomForceMerge(); mergeErr != nil {
			err = mergeErr
		} else if !riw.config.GetCommitOnClose() {
			// The index may have changed; the changes must be committed or
			// they are discarded by the call to Close below.
			if _, commitErr := riw.W.Commit(); commitErr != nil {
				err = commitErr
			}
		}
	}

	if closeErr := riw.W.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if riw.analyzer != nil {
		if closeErr := riw.analyzer.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

// Flush flushes the writer.
//
// Mirrors RandomIndexWriter.flush(), which calls IndexWriter.flush() — itself
// flush(triggerMerge=true, applyAllDeletes=true). IndexWriter.doFlush already
// folds in the merge trigger, so the whole contract is doFlush(true).
func (riw *RandomIndexWriter) Flush() error {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	if err := riw.W.ensureOpen(false); err != nil {
		return err
	}
	_, err := riw.W.doFlush(true)
	return err
}

// maybeFlushOrCommit maybe flushes or commits based on document count.
func (riw *RandomIndexWriter) maybeFlushOrCommit() error {
	riw.docCount++
	if riw.docCount == riw.flushAt {
		if riw.r.Intn(2) == 0 {
			if err := riw.W.ensureOpen(false); err != nil {
				return err
			}
			if _, err := riw.W.doFlush(true); err != nil {
				return err
			}
		} else {
			if _, err := riw.W.Commit(); err != nil {
				return err
			}
		}
		riw.flushAt += nextInt(riw.r, int(riw.flushAtFactor*10), int(riw.flushAtFactor*1000))
		if riw.flushAtFactor < 2e6 {
			riw.flushAtFactor *= 1.05
		}
	}
	return nil
}

// doRandomForceMerge randomly does a force merge.
func (riw *RandomIndexWriter) doRandomForceMerge() error {
	if !riw.shouldRandomForceMerge {
		return nil
	}

	// Get segment count and decide whether to merge
	// In a real implementation, we would call IndexWriter.GetSegmentCount()
	// For now, just do a random decision

	if riw.r.Intn(2) == 0 {
		return riw.W.ForceMerge(1)
	}

	return nil
}

// useSoftDeletes decides whether to use soft deletes.
func (riw *RandomIndexWriter) useSoftDeletes() bool {
	return riw.r.Float64() < riw.softDeletesRatio
}

// Helper functions

func nextInt(r *rand.Rand, min, max int) int {
	if min >= max {
		return min
	}
	return min + r.Intn(max-min)
}

func maybeChangeConfig(r *rand.Rand, config *LiveIndexWriterConfig) {
	// In a real implementation, this would randomly change IndexWriterConfig
	// For now, do nothing
}

// DirectoryReaderOpen opens a directory reader over dir's current commit,
// mirroring Lucene's DirectoryReader.open(Directory).
func DirectoryReaderOpen(dir store.Directory) (*DirectoryReader, error) {
	return OpenDirectoryReader(dir)
}
