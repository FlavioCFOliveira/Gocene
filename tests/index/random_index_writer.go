// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package index implements org.apache.lucene.tests.index.
package index

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testutil "github.com/FlavioCFOliveira/Gocene/tests/util"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestPoint is a simple interface that is executed for each "TP" InfoStream
// component message.
//
// This is the Go port of org.apache.lucene.tests.index.RandomIndexWriter.TestPoint.
type TestPoint interface {
	Apply(message string)
}

// testPointInfoStream is the Go port of
// org.apache.lucene.tests.index.RandomIndexWriter.TestPointInfoStream.
type testPointInfoStream struct {
	delegate  util.InfoStream
	testPoint TestPoint
}

func newTestPointInfoStream(delegate util.InfoStream, tp TestPoint) util.InfoStream {
	if delegate == nil {
		delegate = testutil.NullInfoStream{}
	}
	return &testPointInfoStream{delegate: delegate, testPoint: tp}
}

func (s *testPointInfoStream) Close() error { return s.delegate.Close() }

func (s *testPointInfoStream) Message(component, message string) {
	if component == "TP" {
		s.testPoint.Apply(message)
	}
	if s.delegate.IsEnabled(component) {
		s.delegate.Message(component, message)
	}
}

func (s *testPointInfoStream) IsEnabled(component string) bool {
	return component == "TP" || s.delegate.IsEnabled(component)
}

// RandomIndexWriter randomizes the indexing experience: it may commit
// periodically, may or may not forceMerge in the end, may flush by doc count
// instead of RAM, etc.
//
// This is the Go port of org.apache.lucene.tests.index.RandomIndexWriter.
type RandomIndexWriter struct {
	// W is the wrapped IndexWriter (Java: the public final field `w`).
	W *index.IndexWriter

	r             *rand.Rand
	docCount      int
	flushAt       int
	flushAtFactor float64

	getReaderCalled  bool
	analyzer         analysis.Analyzer // only if WE created it (then we close it)
	softDeletesRatio float64
	config           *index.LiveIndexWriterConfig

	doRandomForceMergeFlag   bool
	doRandomForceMergeAssert bool
}

// yieldingTestPoint yields to the Go scheduler on one in four test points, so
// that goroutine scheduling gets mixed up. Java: the anonymous TestPoint of
// RandomIndexWriter#mockIndexWriter(Directory, IndexWriterConfig, Random).
type yieldingTestPoint struct {
	r *rand.Rand
}

func (y *yieldingTestPoint) Apply(message string) {
	if y.r.Intn(4) == 2 {
		runtime.Gosched()
	}
}

// MockIndexWriter returns an IndexWriter that randomly mixes up goroutine
// scheduling by yielding at test points.
//
// Java: RandomIndexWriter#mockIndexWriter(Directory, IndexWriterConfig, Random).
func MockIndexWriter(dir store.Directory, conf *index.IndexWriterConfig, r *rand.Rand) (*index.IndexWriter, error) {
	random := rand.New(rand.NewSource(r.Int63()))
	return MockIndexWriterWithTestPoint(r, dir, conf, &yieldingTestPoint{r: random})
}

// MockIndexWriterWithTestPoint returns an IndexWriter that enables the
// specified test point.
//
// Java: RandomIndexWriter#mockIndexWriter(Random, Directory, IndexWriterConfig,
// TestPoint).
func MockIndexWriterWithTestPoint(r *rand.Rand, dir store.Directory, conf *index.IndexWriterConfig, testPoint TestPoint) (*index.IndexWriter, error) {
	conf.SetInfoStream(newTestPointInfoStream(conf.GetInfoStream(), testPoint))
	return index.NewIndexWriter(dir, conf)
}

// NewRandomIndexWriter creates a RandomIndexWriter with a random config,
// using a MockAnalyzer.
//
// Java: RandomIndexWriter(Random, Directory).
func NewRandomIndexWriter(r *rand.Rand, dir store.Directory) (*RandomIndexWriter, error) {
	// Java: new MockAnalyzer(r) == MockAnalyzer(r, WHITESPACE, true, EMPTY_STOPSET).
	a := testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, true, 0, nil, true)
	return newRandomIndexWriter(r, dir, index.NewIndexWriterConfigWithAnalyzer(a), true, r.Intn(2) == 0)
}

// NewRandomIndexWriterWithAnalyzer creates a RandomIndexWriter with a random
// config using the supplied analyzer.
//
// Java: RandomIndexWriter(Random, Directory, Analyzer).
func NewRandomIndexWriterWithAnalyzer(r *rand.Rand, dir store.Directory, a analysis.Analyzer) (*RandomIndexWriter, error) {
	return NewRandomIndexWriterWithConfig(r, dir, index.NewIndexWriterConfigWithAnalyzer(a))
}

// NewRandomIndexWriterWithConfig creates a RandomIndexWriter with the provided
// config.
//
// Java: RandomIndexWriter(Random, Directory, IndexWriterConfig).
func NewRandomIndexWriterWithConfig(r *rand.Rand, dir store.Directory, c *index.IndexWriterConfig) (*RandomIndexWriter, error) {
	return newRandomIndexWriter(r, dir, c, false, r.Intn(2) == 0)
}

// NewRandomIndexWriterWithSoftDeletes creates a RandomIndexWriter with the
// provided config and soft-deletes choice.
//
// Java: RandomIndexWriter(Random, Directory, IndexWriterConfig, boolean).
func NewRandomIndexWriterWithSoftDeletes(r *rand.Rand, dir store.Directory, c *index.IndexWriterConfig, useSoftDeletes bool) (*RandomIndexWriter, error) {
	return newRandomIndexWriter(r, dir, c, false, useSoftDeletes)
}

// newRandomIndexWriter is the Go port of the private Java constructor
// RandomIndexWriter(Random, Directory, IndexWriterConfig, boolean, boolean).
func newRandomIndexWriter(r *rand.Rand, dir store.Directory, c *index.IndexWriterConfig, closeAnalyzer, useSoftDeletes bool) (*RandomIndexWriter, error) {
	// TODO: this should be solved in a different way; Random should not be
	// shared (!). Mirrors the same comment in Lucene.
	rng := rand.New(rand.NewSource(r.Int63()))

	var softDeletesRatio float64
	if useSoftDeletes {
		c.SetSoftDeletesField("___soft_deletes")
		softDeletesRatio = 1.0 / (1.0 + float64(r.Intn(10)))
	}

	w, err := MockIndexWriter(dir, c, r)
	if err != nil {
		return nil, err
	}

	riw := &RandomIndexWriter{
		W:                w,
		r:                rng,
		config:           w.GetConfig().LiveIndexWriterConfig,
		flushAt:          nextInt(r, 10, 1000),
		flushAtFactor:    1.0,
		softDeletesRatio: softDeletesRatio,
	}
	if closeAnalyzer {
		riw.analyzer = w.GetAnalyzer()
	}

	// Make sure we sometimes test indices that don't get any forced merges.
	if _, isNoMerge := c.GetMergePolicy().(*index.NoMergePolicy); !isNoMerge {
		riw.doRandomForceMergeFlag = r.Intn(2) == 0
	}
	return riw, nil
}

// nextInt mirrors org.apache.lucene.tests.util.TestUtil#nextInt(Random,int,int).
func nextInt(r *rand.Rand, start, end int) int {
	if start >= end {
		return start
	}
	return start + r.Intn(end-start+1)
}

// maybeChangeLiveIndexWriterConfig is the Go port of
// org.apache.lucene.tests.util.LuceneTestCase#maybeChangeLiveIndexWriterConfig.
// Lucene hosts it on LuceneTestCase, which is not ported; it is kept here
// because RandomIndexWriter is its only caller in this tree.
func (riw *RandomIndexWriter) maybeChangeLiveIndexWriterConfig() {
	c := riw.config
	r := riw.r

	if rarely(r) {
		// Change flush parameters. The API requires the setters be invoked in
		// a magical order (LUCENE-5661).
		if r.Intn(2) == 0 {
			c.SetRAMBufferSizeMB(float64(nextInt(r, 1, 10)))
			c.SetMaxBufferedDocs(index.DisableAutoFlush)
		} else {
			if rarely(r) {
				// crazy value
				c.SetMaxBufferedDocs(nextInt(r, 2, 15))
			} else {
				// reasonable value
				c.SetMaxBufferedDocs(nextInt(r, 16, 1000))
			}
			c.SetRAMBufferSizeMB(index.DisableAutoFlush)
		}
	}

	if rarely(r) {
		c.SetMergedSegmentWarmer(nil)
	}

	if rarely(r) {
		c.SetUseCompoundFile(r.Intn(2) == 0)
	}

	if rarely(r) {
		if cms, ok := c.GetMergeScheduler().(*index.ConcurrentMergeScheduler); ok {
			maxThreadCount := nextInt(r, 1, 4)
			maxMergeCount := nextInt(r, maxThreadCount, maxThreadCount+4)
			cms.SetAutoIOThrottle(r.Intn(2) == 0)
			// The setter validates the pair; a rejected pair leaves the
			// scheduler untouched, exactly as Lucene's does.
			_ = cms.SetMaxMergesAndThreads(maxMergeCount, maxThreadCount)
		}
	}
}

// rarely mirrors org.apache.lucene.tests.util.LuceneTestCase#rarely(Random).
func rarely(r *rand.Rand) bool { return r.Intn(100) < 5 }

// AddDocument adds a document.
//
// Java: RandomIndexWriter#addDocument(Iterable).
func (riw *RandomIndexWriter) AddDocument(doc *document.Document) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()

	var seqNo int64
	var err error
	if riw.r.Intn(5) == 3 {
		seqNo, err = riw.W.AddDocuments([]*document.Document{doc})
	} else {
		seqNo, err = riw.W.AddDocument(doc)
	}
	if err != nil {
		return 0, err
	}
	if err := riw.maybeFlushOrCommit(); err != nil {
		return 0, err
	}
	return seqNo, nil
}

func (riw *RandomIndexWriter) maybeFlushOrCommit() error {
	riw.maybeChangeLiveIndexWriterConfig()
	docCount := riw.docCount
	riw.docCount++
	if docCount != riw.flushAt {
		return nil
	}

	var err error
	switch {
	case riw.r.Intn(2) == 0:
		err = riw.flushAllBuffersSequentially()
	case riw.r.Intn(2) == 0:
		err = riw.W.Flush()
	default:
		_, err = riw.W.Commit()
	}
	if err != nil {
		return err
	}

	riw.flushAt += nextInt(riw.r, int(riw.flushAtFactor*10), int(riw.flushAtFactor*1000))
	if riw.flushAtFactor < 2e6 {
		// gradually but exponentially increase time b/w flushes
		riw.flushAtFactor *= 1.05
	}
	return nil
}

func (riw *RandomIndexWriter) flushAllBuffersSequentially() error {
	threadPoolSize := riw.W.GetDocWriterThreadPoolSize()
	numFlushes := min(1, riw.r.Intn(threadPoolSize+1))
	for i := 0; i < numFlushes; i++ {
		flushed, err := riw.W.FlushNextBuffer()
		if err != nil {
			return err
		}
		if !flushed {
			break // stop once we didn't flush anything
		}
	}
	return nil
}

// AddDocuments adds a block of documents.
//
// Java: RandomIndexWriter#addDocuments(Iterable).
func (riw *RandomIndexWriter) AddDocuments(docs []*document.Document) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()
	seqNo, err := riw.W.AddDocuments(docs)
	if err != nil {
		return 0, err
	}
	if err := riw.maybeFlushOrCommit(); err != nil {
		return 0, err
	}
	return seqNo, nil
}

// UpdateDocuments replaces the block of documents matching delTerm.
//
// Java: RandomIndexWriter#updateDocuments(Term, Iterable).
func (riw *RandomIndexWriter) UpdateDocuments(delTerm *index.Term, docs []*document.Document) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()

	var seqNo int64
	var err error
	if riw.useSoftDeletes() {
		var softDeletes []*document.Field
		if softDeletes, err = riw.softDeleteField(); err != nil {
			return 0, err
		}
		seqNo, err = riw.W.SoftUpdateDocuments(delTerm, docs, softDeletes)
	} else {
		seqNo, err = riw.W.UpdateDocuments(delTerm, docs)
	}
	if err != nil {
		return 0, err
	}
	if err := riw.maybeFlushOrCommit(); err != nil {
		return 0, err
	}
	return seqNo, nil
}

// softDeleteField builds the NumericDocValuesField that marks a soft delete,
// mirroring `new NumericDocValuesField(config.getSoftDeletesField(), 1)`.
func (riw *RandomIndexWriter) softDeleteField() ([]*document.Field, error) {
	f, err := document.NewNumericDocValuesField(riw.config.GetSoftDeletesField(), 1)
	if err != nil {
		return nil, err
	}
	return []*document.Field{f.Field}, nil
}

func (riw *RandomIndexWriter) useSoftDeletes() bool {
	return riw.r.Float64() < riw.softDeletesRatio
}

// UpdateDocument updates a document.
//
// Java: RandomIndexWriter#updateDocument(Term, Iterable).
func (riw *RandomIndexWriter) UpdateDocument(t *index.Term, doc *document.Document) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()

	var seqNo int64
	var err error
	if riw.useSoftDeletes() {
		var softDeletes []*document.Field
		if softDeletes, err = riw.softDeleteField(); err != nil {
			return 0, err
		}
		if riw.r.Intn(5) == 3 {
			seqNo, err = riw.W.SoftUpdateDocuments(t, []*document.Document{doc}, softDeletes)
		} else {
			seqNo, err = riw.W.SoftUpdateDocument(t, doc, softDeletes)
		}
	} else {
		if riw.r.Intn(5) == 3 {
			seqNo, err = riw.W.UpdateDocuments(t, []*document.Document{doc})
		} else {
			seqNo, err = riw.W.UpdateDocument(t, doc)
		}
	}
	if err != nil {
		return 0, err
	}
	if err := riw.maybeFlushOrCommit(); err != nil {
		return 0, err
	}
	return seqNo, nil
}

// AddIndexes merges the given directories into this index.
//
// Java: RandomIndexWriter#addIndexes(Directory...).
func (riw *RandomIndexWriter) AddIndexes(dirs ...store.Directory) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()
	return riw.W.AddIndexes(dirs...)
}

// UpdateNumericDocValue updates a numeric doc-values field.
//
// Java: RandomIndexWriter#updateNumericDocValue(Term, String, Long).
func (riw *RandomIndexWriter) UpdateNumericDocValue(term *index.Term, field string, value int64) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()
	return riw.W.UpdateNumericDocValue(term, field, value)
}

// UpdateBinaryDocValue updates a binary doc-values field.
//
// Java: RandomIndexWriter#updateBinaryDocValue(Term, String, BytesRef).
func (riw *RandomIndexWriter) UpdateBinaryDocValue(term *index.Term, field string, value []byte) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()
	return riw.W.UpdateBinaryDocValue(term, field, value)
}

// UpdateDocValues updates the given doc-values fields.
//
// Java: RandomIndexWriter#updateDocValues(Term, Field...).
func (riw *RandomIndexWriter) UpdateDocValues(term *index.Term, updates ...*document.Field) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()
	return riw.W.UpdateDocValues(term, updates)
}

// DeleteDocuments deletes the documents matching term.
//
// Java: RandomIndexWriter#deleteDocuments(Term).
func (riw *RandomIndexWriter) DeleteDocuments(term *index.Term) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()
	return riw.W.DeleteDocuments([]index.Term{*term})
}

// DeleteDocumentsQuery deletes the documents matching q.
//
// Java: RandomIndexWriter#deleteDocuments(Query).
func (riw *RandomIndexWriter) DeleteDocumentsQuery(q index.Query) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()
	return riw.W.DeleteDocumentsQuery([]index.Query{q})
}

// Commit commits, sometimes flushing concurrently.
//
// Java: RandomIndexWriter#commit().
func (riw *RandomIndexWriter) Commit() (int64, error) {
	return riw.CommitConcurrently(riw.r.Intn(10) == 0)
}

// CommitConcurrently commits, optionally flushing the buffers from another
// goroutine at the same time.
//
// Java: RandomIndexWriter#commit(boolean).
func (riw *RandomIndexWriter) CommitConcurrently(flushConcurrently bool) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()
	if !flushConcurrently {
		return riw.W.Commit()
	}

	var mu sync.Mutex
	var errs []error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := riw.flushAllBuffersSequentially(); err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
		}
	}()

	seqNo, err := riw.W.Commit()
	if err != nil {
		errs = append(errs, err)
	}
	// Wait for the goroutine to finish, otherwise it might still be processing
	// events and the IndexWriter would not be fully closed after a fatal error.
	wg.Wait()

	if len(errs) != 0 {
		return 0, errors.Join(errs...)
	}
	return seqNo, nil
}

// GetDocStats returns the writer's document statistics.
//
// Java: RandomIndexWriter#getDocStats().
func (riw *RandomIndexWriter) GetDocStats() (index.DocStats, error) {
	return riw.W.GetDocStats()
}

// DeleteAll deletes every document.
//
// Java: RandomIndexWriter#deleteAll().
func (riw *RandomIndexWriter) DeleteAll() (int64, error) {
	return riw.W.DeleteAll()
}

// GetReader returns a reader over the current index state.
//
// Java: RandomIndexWriter#getReader().
func (riw *RandomIndexWriter) GetReader() (*index.DirectoryReader, error) {
	riw.maybeChangeLiveIndexWriterConfig()
	return riw.GetReaderWithDeletes(true, false)
}

// ForceMergeDeletesWithWait forces the deleted documents to be merged away.
//
// Java: RandomIndexWriter#forceMergeDeletes(boolean).
func (riw *RandomIndexWriter) ForceMergeDeletesWithWait(doWait bool) (*index.MergeObserver, error) {
	riw.maybeChangeLiveIndexWriterConfig()
	return riw.W.ForceMergeDeletesWithObserver(doWait)
}

// ForceMergeDeletes forces the deleted documents to be merged away.
//
// Java: RandomIndexWriter#forceMergeDeletes().
func (riw *RandomIndexWriter) ForceMergeDeletes() error {
	riw.maybeChangeLiveIndexWriterConfig()
	return riw.W.ForceMergeDeletes()
}

// SetDoRandomForceMerge enables or disables the random forceMerge calls.
func (riw *RandomIndexWriter) SetDoRandomForceMerge(v bool) { riw.doRandomForceMergeFlag = v }

// SetDoRandomForceMergeAssert enables or disables the segment-count assertion
// made after a partial random forceMerge.
func (riw *RandomIndexWriter) SetDoRandomForceMergeAssert(v bool) { riw.doRandomForceMergeAssert = v }

// doRandomForceMerge is the Go port of RandomIndexWriter#doRandomForceMerge().
func (riw *RandomIndexWriter) doRandomForceMerge() error {
	if !riw.doRandomForceMergeFlag {
		return nil
	}
	segCount := riw.W.GetSegmentCount()
	switch {
	case riw.r.Intn(2) == 0 || segCount == 0:
		// full forceMerge
		return riw.W.ForceMerge(1)
	case riw.r.Intn(2) == 0:
		// partial forceMerge
		limit := nextInt(riw.r, 1, segCount)
		if err := riw.W.ForceMerge(limit); err != nil {
			return err
		}
		if _, tiered := riw.config.GetMergePolicy().(*index.TieredMergePolicy); limit == 1 || !tiered {
			if riw.doRandomForceMergeAssert && riw.W.GetSegmentCount() > limit {
				return fmt.Errorf("limit=%d actual=%d", limit, riw.W.GetSegmentCount())
			}
		}
		return nil
	default:
		return riw.W.ForceMergeDeletes()
	}
}

// GetReaderWithDeletes returns a reader, either a near-real-time one from the
// writer or a freshly opened DirectoryReader.
//
// Java: RandomIndexWriter#getReader(boolean, boolean).
func (riw *RandomIndexWriter) GetReaderWithDeletes(applyDeletions, writeAllDeletes bool) (*index.DirectoryReader, error) {
	riw.maybeChangeLiveIndexWriterConfig()
	riw.getReaderCalled = true
	if riw.r.Intn(20) == 2 {
		if err := riw.doRandomForceMerge(); err != nil {
			return nil, err
		}
	}

	if !applyDeletions || riw.r.Intn(2) == 0 {
		// if we have soft deletes we can't open from a directory
		if riw.r.Intn(5) == 1 {
			if _, err := riw.W.Commit(); err != nil {
				return nil, err
			}
		}
		return riw.nrtReader(applyDeletions, writeAllDeletes)
	}

	if _, err := riw.W.Commit(); err != nil {
		return nil, err
	}
	if riw.r.Intn(2) == 0 {
		reader, err := index.OpenDirectoryReader(riw.W.GetDirectory())
		if err != nil {
			return nil, err
		}
		if riw.config.GetSoftDeletesField() != "" {
			wrapped, err := index.NewSoftDeletesDirectoryReaderWrapper(reader, riw.config.GetSoftDeletesField())
			if err != nil {
				return nil, err
			}
			return wrapped.DirectoryReader, nil
		}
		return reader, nil
	}
	return riw.nrtReader(applyDeletions, writeAllDeletes)
}

func (riw *RandomIndexWriter) nrtReader(applyDeletions, writeAllDeletes bool) (*index.DirectoryReader, error) {
	reader, err := riw.W.GetReader(applyDeletions, writeAllDeletes)
	if err != nil {
		return nil, err
	}
	return reader.DirectoryReader, nil
}

// Close closes this writer.
//
// Java: RandomIndexWriter#close().
func (riw *RandomIndexWriter) Close() error {
	var err error
	if !riw.W.IsClosed() {
		riw.maybeChangeLiveIndexWriterConfig()
	}
	// If someone isn't using getReader() API, we want to be sure to forceMerge
	// since presumably they might open a reader on the dir.
	if !riw.getReaderCalled && riw.r.Intn(8) == 2 && !riw.W.IsClosed() {
		if err = riw.doRandomForceMerge(); err == nil && !riw.config.GetCommitOnClose() {
			// index may have changed, must commit the changes, or otherwise
			// they are discarded by the call to close()
			_, err = riw.W.Commit()
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

// ForceMerge forces a merge down to maxSegmentCount segments.
//
// NOTE: this should be avoided in tests unless absolutely necessary, as it
// will result in less test coverage.
//
// Java: RandomIndexWriter#forceMerge(int).
func (riw *RandomIndexWriter) ForceMerge(maxSegmentCount int) error {
	riw.maybeChangeLiveIndexWriterConfig()
	return riw.W.ForceMerge(maxSegmentCount)
}

// Flush writes all in-memory segments to the Directory.
//
// Java: RandomIndexWriter#flush().
func (riw *RandomIndexWriter) Flush() error {
	return riw.W.Flush()
}
