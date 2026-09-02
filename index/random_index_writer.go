// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
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
	doRandomForceMerge       bool
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
		config:        w.GetConfig(),
		analyzer:      nil,
	}

	// Randomly decide whether to do force merges
	if w.GetConfig().GetMergePolicy() != nil {
		riw.doRandomForceMerge = newRand.Intn(2) == 0
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
		config:           w.GetConfig(),
		softDeletesRatio: softDeletesRatio,
		analyzer:         config.GetAnalyzer(),
	}

	if w.GetConfig().GetMergePolicy() != nil {
		riw.doRandomForceMerge = newRand.Intn(2) == 0
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
	return riw.W.DeleteDocuments(t)
}

// DeleteDocumentsWithQuery deletes documents matching a query.
// q should implement the Query interface from the search package.
func (riw *RandomIndexWriter) DeleteDocumentsWithQuery(q interface{}) (int64, error) {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	maybeChangeConfig(riw.r, riw.config)
	// Use a type assertion to call the actual method
	// This allows us to avoid the import cycle
	if writerMethod, ok := riw.W.(interface {
		DeleteDocumentsWithQuery(interface{}) (int64, error)
	}); ok {
		return writerMethod.DeleteDocumentsWithQuery(q)
	}
	return 0, fmt.Errorf("IndexWriter does not support DeleteDocumentsWithQuery")
}

// UpdateDocValues updates doc values.
func (riw *RandomIndexWriter) UpdateDocValues(term *Term, updates ...*document.Field) (int64, error) {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	maybeChangeConfig(riw.r, riw.config)
	return riw.W.UpdateDocValues(term, updates...)
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
	riw.doRandomForceMerge = v
}

// SetDoRandomForceMergeAssert sets whether to assert merge limits.
func (riw *RandomIndexWriter) SetDoRandomForceMergeAssert(v bool) {
	riw.doRandomForceMergeAssert = v
}

// GetReader returns a reader.
func (riw *RandomIndexWriter) GetReader() (DirectoryReader, error) {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	maybeChangeConfig(riw.r, riw.config)
	riw.getReaderCalled = true

	if riw.r.Intn(20) == 2 {
		_ = riw.doRandomForceMerge()
	}

	if riw.r.Intn(2) == 0 {
		// Use NRT reader
		if riw.r.Intn(5) == 1 {
			_ = riw.W.Commit()
		}
		// Return NRT reader
		// This is a simplified version; real implementation would use IndexWriter.GetReader()
		reader, err := OpenDirectoryReaderFromWriter(riw.W, true, false)
		return reader, err
	} else {
		// Open new reader from directory
		_ = riw.W.Commit()
		reader, err := DirectoryReaderOpen(riw.W.GetDirectory())
		return reader, err
	}
}

// Close closes the writer.
func (riw *RandomIndexWriter) Close() error {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	if !riw.getReaderCalled && riw.r.Intn(8) == 2 && !riw.W.IsClosed() {
		_ = riw.doRandomForceMerge()
		if !riw.config.GetCommitOnClose() {
			_ = riw.W.Commit()
		}
	}

	err := riw.W.Close()
	if riw.analyzer != nil {
		riw.analyzer.Close()
	}
	return err
}

// Flush flushes the writer.
func (riw *RandomIndexWriter) Flush() error {
	riw.mu.Lock()
	defer riw.mu.Unlock()

	return riw.W.Flush()
}

// maybeFlushOrCommit maybe flushes or commits based on document count.
func (riw *RandomIndexWriter) maybeFlushOrCommit() error {
	riw.docCount++
	if riw.docCount == riw.flushAt {
		if riw.r.Intn(2) == 0 {
			_ = riw.W.Flush()
		} else {
			_ = riw.W.Commit()
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
	if !riw.doRandomForceMerge {
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

// OpenDirectoryReaderFromWriter opens a reader from an IndexWriter.
// This is a placeholder that should be implemented based on Gocene's actual API.
func OpenDirectoryReaderFromWriter(w *IndexWriter, applyDeletions, writeAllDeletes bool) (DirectoryReader, error) {
	return nil, fmt.Errorf("OpenDirectoryReaderFromWriter not yet implemented")
}

// DirectoryReaderOpen opens a directory reader.
// This is a placeholder that should be implemented based on Gocene's actual API.
func DirectoryReaderOpen(dir store.Directory) (DirectoryReader, error) {
	return nil, fmt.Errorf("DirectoryReaderOpen not yet implemented")
}
