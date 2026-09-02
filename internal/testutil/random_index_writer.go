// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestPoint is a simple interface that is executed for each "TP" InfoStream component message.
type TestPoint interface {
	Apply(message string)
}

// testPointInfoStream implements util.InfoStream and delegates to a TestPoint for "TP" messages.
type testPointInfoStream struct {
	delegate util.InfoStream
	testPoint TestPoint
}

func newTestPointInfoStream(delegate util.InfoStream, tp TestPoint) util.InfoStream {
	if delegate == nil {
		delegate = &util.NullInfoStream{}
	}
	return &testPointInfoStream{
		delegate:  delegate,
		testPoint: tp,
	}
}

func (s *testPointInfoStream) Close() error {
	return s.delegate.Close()
}

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

// RandomIndexWriter is a test utility that wraps an [index.IndexWriter] and
// randomly interleaves Commit, ForceMerge, and Flush operations with
// document mutations to exercise race conditions and ordering assumptions.
//
// This is a port of org.apache.lucene.tests.index.RandomIndexWriter.
type RandomIndexWriter struct {
	w        *index.IndexWriter
	rng      *rand.Rand
	docCount int
	flushAt  int

	flushAtFactor   float64
	getReaderCalled  bool
	analyzer         analysis.Analyzer
	softDeletesRatio float64
	config           *index.LiveIndexWriterConfig

	doRandomForceMerge       bool
	doRandomForceMergeAssert bool
}

// MockIndexWriter returns an IndexWriter that randomly yields to mix up thread scheduling.
func MockIndexWriter(dir store.Directory, conf *index.IndexWriterConfig, r *rand.Rand) (*index.IndexWriter, error) {
	localRng := rand.New(rand.NewSource(r.Int63()))
	tp := &mockTestPoint{rng: localRng}
	conf.SetInfoStream(newTestPointInfoStream(conf.GetInfoStream(), tp))
	return index.NewIndexWriter(dir, conf)
}

type mockTestPoint struct {
	rng *rand.Rand
}

func (m *mockTestPoint) Apply(message string) {
	if m.rng.Intn(4) == 2 {
		runtime.Gosched()
	}
}

// New creates a RandomIndexWriter with a random config and a MockAnalyzer.
func New(r *rand.Rand, dir store.Directory) (*RandomIndexWriter, error) {
	conf := index.NewIndexWriterConfig(NewMockAnalyzerRandom(r, true, 0, nil, false))
	return NewWithConfig(r, dir, conf, true, r.Intn(2) == 0)
}


// NewWithAnalyzer creates a RandomIndexWriter with the provided analyzer.
func NewWithAnalyzer(r *rand.Rand, dir store.Directory, a analysis.Analyzer) (*RandomIndexWriter, error) {
	conf := index.NewIndexWriterConfig(a)
	return NewWithConfig(r, dir, conf, true, r.Intn(2) == 0)
}

// NewWithConfig creates a RandomIndexWriter with the provided config.
func NewWithConfig(r *rand.Rand, dir store.Directory, c *index.IndexWriterConfig) (*RandomIndexWriter, error) {
	return NewWithConfigExtended(r, dir, c, false, r.Intn(2) == 0)
}

// NewWithConfigExtended creates a RandomIndexWriter with the provided config and soft delete settings.
func NewWithConfigExtended(r *rand.Rand, dir store.Directory, c *index.IndexWriterConfig, closeAnalyzer bool, useSoftDeletes bool) (*RandomIndexWriter, error) {
	rng := rand.New(rand.NewSource(r.Int63()))

	var softDeletesRatio float64
	if useSoftDeletes {
		c.SetSoftDeletesField("___soft_deletes")
		softDeletesRatio = 1.0 / (1.0 + float64(rng.Intn(10)))
	} else {
		softDeletesRatio = 0.0
	}

	w, err := MockIndexWriter(dir, c, rng)
	if err != nil {
		return nil, err
	}

	config := w.GetConfig()
	flushAt := nextInt(rng, 10, 1000)

	var analyzer analysis.Analyzer
	if closeAnalyzer {
		analyzer = w.GetAnalyzer()
	}

	doRandomForceMerge := false
	if c.GetMergePolicy() != nil {
		if _, ok := c.GetMergePolicy().(*index.NoMergePolicy); !ok {
			doRandomForceMerge = rng.Intn(2) == 0
		}
	}

	return &RandomIndexWriter{
		w:                w,
		rng:              rng,
		config:           config,
		flushAt:          flushAt,
		flushAtFactor:    1.0,
		softDeletesRatio: softDeletesRatio,
		analyzer:         analyzer,
		doRandomForceMerge: doRandomForceMerge,
	}, nil
}

func nextInt(r *rand.Rand, min, max int) int {
	if min >= max {
		return min
	}
	return min + r.Intn(max-min)
}

func (r *RandomIndexWriter) maybeChangeLiveIndexWriterConfig() {
	if r.rng.Float64() < 0.05 {
		if r.rng.Intn(2) == 0 {
			r.config.SetRAMBufferSizeMB(float64(nextInt(r.rng, 1, 10)))
			r.config.SetMaxBufferedDocs(index.DisableAutoFlush)
		} else {
			if r.rng.Float64() < 0.05 {
				r.config.SetMaxBufferedDocs(nextInt(r.rng, 2, 15))
			} else {
				r.config.SetMaxBufferedDocs(nextInt(r.rng, 16, 1000))
			}
			r.config.SetRAMBufferSizeMB(index.DisableAutoFlush)
		}
	}

	if r.rng.Float64() < 0.05 {
		r.config.SetMergedSegmentWarmer(nil)
	}

	if r.rng.Float64() < 0.05 {
		r.config.SetUseCompoundFile(r.rng.Intn(2) == 0)
	}

	if r.rng.Float64() < 0.05 {
		ms := r.config.GetMergeScheduler()
		if cms, ok := ms.(*index.ConcurrentMergeScheduler); ok {
			maxThreadCount := nextInt(r.rng, 1, 4)
			maxMergeCount := nextInt(r.rng, maxThreadCount, maxThreadCount+4)
			if r.rng.Intn(2) == 0 {
				cms.EnableAutoIOThrottle()
			} else {
				cms.DisableAutoIOThrottle()
			}
			cms.SetMaxMergesAndThreads(maxMergeCount, maxThreadCount)
		}
	}
}

func (r *RandomIndexWriter) AddDocument(doc []index.IndexableField) (int64, error) {
	r.maybeChangeLiveIndexWriterConfig()
	var seqNo int64
	var err error
	if r.rng.Intn(5) == 3 {
		seqNo, err = r.w.AddDocuments([][]index.IndexableField{doc})
	} else {
		seqNo, err = r.w.AddDocument(doc)
	}
	if err != nil {
		return 0, err
	}

	r.maybeFlushOrCommit()
	return seqNo, nil
}

func (r *RandomIndexWriter) maybeFlushOrCommit() {
	r.maybeChangeLiveIndexWriterConfig()
	r.docCount++
	if r.docCount == r.flushAt {
		if r.rng.Intn(2) == 0 {
			r.flushAllBuffersSequentially()
		} else if r.rng.Intn(2) == 0 {
			r.w.Flush()
		} else {
			r.w.Commit()
		}
		r.flushAt += nextInt(r.rng, int(r.flushAtFactor*10), int(r.flushAtFactor*1000))
		if r.flushAtFactor < 2e6 {
			r.flushAtFactor *= 1.05
		}
	}
}

func (r *RandomIndexWriter) flushAllBuffersSequentially() {
	threadPoolSize := r.w.GetDocWriterThreadPoolSize()
	numFlushes := 0
	if threadPoolSize > 0 {
		numFlushes = r.rng.Intn(threadPoolSize + 1)
		if numFlushes > 1 {
			numFlushes = 1
		}
	}
	for i := 0; i < numFlushes; i++ {
		if !r.w.FlushNextBuffer() {
			break
		}
	}
}

func (r *RandomIndexWriter) AddDocuments(docs [][]index.IndexableField) (int64, error) {
	r.maybeChangeLiveIndexWriterConfig()
	seqNo, err := r.w.AddDocuments(docs)
	if err != nil {
		return 0, err
	}
	r.maybeFlushOrCommit()
	return seqNo, nil
}

func (r *RandomIndexWriter) UpdateDocuments(delTerm *index.Term, docs [][]index.IndexableField) (int64, error) {
	r.maybeChangeLiveIndexWriterConfig()
	var seqNo int64
	var err error
	if r.useSoftDeletes() {
		seqNo, err = r.w.SoftUpdateDocuments(delTerm, docs, document.NewNumericDocValuesField(r.config.GetSoftDeletesField(), 1))
	} else {
		if r.rng.Intn(10) < 3 {
			seqNo, err = r.w.UpdateDocuments(&search.TermQuery{Term: delTerm}, docs)
		} else {
			seqNo, err = r.w.UpdateDocuments(delTerm, docs)
		}
	}
	if err != nil {
		return 0, err
	}
	r.maybeFlushOrCommit()
	return seqNo, nil
}

func (r *RandomIndexWriter) useSoftDeletes() bool {
	return r.rng.Float64() < r.softDeletesRatio
}

func (r *RandomIndexWriter) UpdateDocument(t *index.Term, doc []index.IndexableField) (int64, error) {
	r.maybeChangeLiveIndexWriterConfig()
	var seqNo int64
	var err error
	if r.useSoftDeletes() {
		if r.rng.Intn(5) == 3 {
			seqNo, err = r.w.SoftUpdateDocuments(t, [][]index.IndexableField{doc}, document.NewNumericDocValuesField(r.config.GetSoftDeletesField(), 1))
		} else {
			seqNo, err = r.w.SoftUpdateDocument(t, doc, document.NewNumericDocValuesField(r.config.GetSoftDeletesField(), 1))
		}
	} else {
		if r.rng.Intn(5) == 3 {
			seqNo, err = r.w.UpdateDocuments(t, [][]index.IndexableField{doc})
		} else {
			seqNo, err = r.w.UpdateDocument(t, doc)
		}
	}
	if err != nil {
		return 0, err
	}
	r.maybeFlushOrCommit()
	return seqNo, nil
}

func (r *RandomIndexWriter) AddIndexes(dirs ...store.Directory) (int64, error) {
	r.maybeChangeLiveIndexWriterConfig()
	return r.w.AddIndexes(dirs...)
}

func (r *RandomIndexWriter) UpdateNumericDocValue(term *index.Term, field string, value int64) (int64, error) {
	r.maybeChangeLiveIndexWriterConfig()
	return r.w.UpdateNumericDocValue(term, field, value)
}

func (r *RandomIndexWriter) UpdateBinaryDocValue(term *index.Term, field string, value *util.BytesRef) (int64, error) {
	r.maybeChangeLiveIndexWriterConfig()
	return r.w.UpdateBinaryDocValue(term, field, value)
}

func (r *RandomIndexWriter) UpdateDocValues(term *index.Term, updates ...index.IndexableField) (int64, error) {
	r.maybeChangeLiveIndexWriterConfig()
	return r.w.UpdateDocValues(term, updates...)
}

func (r *RandomIndexWriter) DeleteDocuments(term *index.Term) (int64, error) {
	r.maybeChangeLiveIndexWriterConfig()
	return r.w.DeleteDocuments([]*index.Term{term})
}

func (r *RandomIndexWriter) DeleteDocumentsQuery(q search.Query) (int64, error) {
	r.maybeChangeLiveIndexWriterConfig()
	return r.w.DeleteDocumentsQuery(q)
}

func (r *RandomIndexWriter) Commit() (int64, error) {
	return r.CommitExtended(r.rng.Intn(10) == 0)
}

func (r *RandomIndexWriter) CommitExtended(flushConcurrently bool) (int64, error) {
	r.maybeChangeLiveIndexWriterConfig()
	if flushConcurrently {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.flushAllBuffersSequentially()
		}()

		seqNo, err := r.w.Commit()
		wg.Wait()
		if err != nil {
			return 0, err
		}
		return seqNo, nil
	}
	return r.w.Commit()
}

func (r *RandomIndexWriter) GetDocStats() index.DocStats {
	return r.w.GetDocStats()
}

func (r *RandomIndexWriter) DeleteAll() (int64, error) {
	return r.w.DeleteAll()
}

func (r *RandomIndexWriter) GetReader() (*index.DirectoryReader, error) {
	return r.GetReaderExtended(true, false)
}

func (r *RandomIndexWriter) GetReaderExtended(applyDeletions bool, writeAllDeletes bool) (*index.DirectoryReader, error) {
	r.maybeChangeLiveIndexWriterConfig()
	r.getReaderCalled = true
	if r.rng.Intn(20) == 2 {
		r.doRandomForceMerge()
	}
	if !applyDeletions || r.rng.Intn(2) == 0 {
		if r.rng.Intn(5) == 1 {
			r.w.Commit()
		}
		return r.w.GetReader(applyDeletions)
	} else {
		r.w.Commit()
		if r.rng.Intn(2) == 0 {
			reader, err := index.OpenDirectoryReader(r.w.GetDirectory())
			if err != nil {
				return nil, err
			}
			if r.config.GetSoftDeletesField() != "" {
				return index.NewSoftDeletesDirectoryReaderWrapper(reader, r.config.GetSoftDeletesField()), nil
			}
			return reader, nil
		}
		return r.w.GetReader(applyDeletions)
	}
}

func (r *RandomIndexWriter) setDoRandomForceMerge(v bool) {
	r.doRandomForceMerge = v
}

func (r *RandomIndexWriter) setDoRandomForceMergeAssert(v bool) {
	r.doRandomForceMergeAssert = v
}

func (r *RandomIndexWriter) doRandomForceMerge() {
	if r.doRandomForceMerge {
		segCount := r.w.GetSegmentCount()
		if r.rng.Intn(2) == 0 || segCount == 0 {
			r.w.ForceMerge(1)
		} else if r.rng.Intn(2) == 0 {
			limit := nextInt(r.rng, 1, segCount)
			r.w.ForceMerge(limit)
			if limit == 1 {
				if r.doRandomForceMergeAssert && r.w.GetSegmentCount() > limit {
					panic(fmt.Sprintf("ForceMerge limit=%d actual=%d", limit, r.w.GetSegmentCount()))
				}
			}
		} else {
			r.w.ForceMergeDeletes()
		}
	}
}

func (r *RandomIndexWriter) Close() error {
	success := false
	defer func() {
		if !success {
			r.w.Close()
			if r.analyzer != nil {
				r.analyzer.Close()
			}
		}
	}()

	if !r.w.IsClosed() {
		r.maybeChangeLiveIndexWriterConfig()
	}

	if !r.getReaderCalled && r.rng.Intn(8) == 2 && !r.w.IsClosed() {
		r.doRandomForceMerge()
		if !r.config.GetCommitOnClose() {
			r.w.Commit()
		}
	}
	success = true
	return r.w.Close()
}

func (r *RandomIndexWriter) ForceMerge(maxSegmentCount int) error {
	r.maybeChangeLiveIndexWriterConfig()
	return r.w.ForceMerge(maxSegmentCount)
}

func (r *RandomIndexWriter) Flush() error {
	return r.w.Flush()
}

