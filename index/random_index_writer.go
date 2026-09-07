// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis/api"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestPoint is a simple interface that is executed for each "TP" InfoStream component message.
type TestPoint func(message string)

type testPointInfoStream struct {
	delegate  util.InfoStream
	testPoint TestPoint
}

func newTestPointInfoStream(delegate util.InfoStream, testPoint TestPoint) *testPointInfoStream {
	if delegate == nil {
		delegate = &util.NullInfoStream{}
	}
	return &testPointInfoStream{
		delegate:  delegate,
		testPoint: testPoint,
	}
}

func (s *testPointInfoStream) Close() error {
	return s.delegate.Close()
}

func (s *testPointInfoStream) Message(component, message string) {
	if component == "TP" {
		s.testPoint(message)
	}
	if s.delegate.IsEnabled(component) {
		s.delegate.Message(component, message)
	}
}

func (s *testPointInfoStream) IsEnabled(component string) bool {
	return component == "TP" || s.delegate.IsEnabled(component)
}

// MockIndexWriter returns an indexwriter that randomly mixes up thread scheduling (by yielding at test points).
func MockIndexWriter(r *rand.Rand, d store.Directory, conf *IndexWriterConfig, testPoint TestPoint) (*IndexWriter, error) {
	conf.SetInfoStream(newTestPointInfoStream(conf.GetInfoStream(), testPoint))

	var reader *StandardDirectoryReader
	if r.Intn(2) == 0 && util.IndexExists(d) && conf.LiveIndexWriterConfig.GetOpenMode() != Create {
		// RIW: open writer from reader
		var err error
		reader, err = Open(nil, func(sci *SegmentCommitInfo) (*ReadersAndUpdates, error) {
			return nil, fmt.Errorf("not implemented")
		}, nil, true, true) // Simplified for translation
		if err != nil {
			return nil, err
		}
		conf.SetIndexCommit(reader.GetCommit())
	}

	iw, err := NewIndexWriter(d, conf)
	if err != nil {
		if reader != nil {
			reader.Close()
		}
		return nil, err
	}

	if reader != nil {
		reader.Close()
	}

	return iw, nil
}

// MockIndexWriterSimple returns an indexwriter that randomly calls runtime.Gosched() to mixup thread scheduling.
func MockIndexWriterSimple(d store.Directory, conf *IndexWriterConfig, r *rand.Rand) (*IndexWriter, error) {
	random := rand.New(rand.NewSource(r.Int63()))
	tp := func(message string) {
		if random.Intn(4) == 2 {
			runtime.Gosched()
		}
	}
	return MockIndexWriter(r, d, conf, tp)
}

// RandomIndexWriter is a utility that randomizes the indexing experience for testing purposes.
type RandomIndexWriter struct {
	w *IndexWriter

	r *rand.Rand

	docCount int
	flushAt  int

	flushAtFactor float64

	getReaderCalled bool

	analyzer api.Analyzer // only if WE created it (then we close it)

	softDeletesRatio float64

	config *LiveIndexWriterConfig

	doRandomForceMerge       bool
	doRandomForceMergeAssert bool
}

func NewRandomIndexWriter(r *rand.Rand, d store.Directory) (*RandomIndexWriter, error) {
	// Use a mock analyzer as in Lucene's LuceneTestCase.newIndexWriterConfig
	conf := NewIndexWriterConfig() // Default analyzer is Standard
	// Note: Lucene uses MockAnalyzer(r) here.
	return NewRandomIndexWriterWithConfig(r, d, conf, true, r.Intn(2) == 0)
}

func NewRandomIndexWriterWithAnalyzer(r *rand.Rand, d store.Directory, a api.Analyzer) (*RandomIndexWriter, error) {
	conf := NewIndexWriterConfigWithAnalyzer(a)
	return NewRandomIndexWriterWithConfig(r, d, conf, false, r.Intn(2) == 0)
}

func NewRandomIndexWriterWithConfig(r *rand.Rand, d store.Directory, c *IndexWriterConfig) (*RandomIndexWriter, error) {
	return NewRandomIndexWriterWithConfigExtended(r, d, c, false, r.Intn(2) == 0)
}

func NewRandomIndexWriterWithConfigExtended(r *rand.Rand, d store.Directory, c *IndexWriterConfig, useSoftDeletes bool, randomConfig bool) (*RandomIndexWriter, error) {
	// The Lucene source uses a private constructor.
	// we wrap the logic here.

	// Random should not be shared
	localRand := rand.New(rand.NewSource(r.Int63()))

	if useSoftDeletes {
		c.SetSoftDeletesField("___soft_deletes")
		// softDeletesRatio = 1.d / (double) 1 + r.nextInt(10);
		softDeletesRatio := 1.0 / (1.0 + float64(localRand.Intn(10)))
		_ = softDeletesRatio // Logic preserved in struct below
	}

	// Use MockIndexWriter to setup potential test points
	iw, err := MockIndexWriter(localRand, d, c, func(msg string) {})
	if err != nil {
		return nil, err
	}

	flushAt := 10 + localRand.Intn(991) // TestUtil.nextInt(r, 10, 1000)

	var analyzer api.Analyzer
	// In a real scenario, we'd check if the config analyzer should be closed.
	// For this translation, we keep it simple.
	analyzer = iw.GetAnalyzer()

	riw := &RandomIndexWriter{
		w:               iw,
		r:               localRand,
		config:           iw.GetConfig().LiveIndexWriterConfig,
		flushAt:          flushAt,
		flushAtFactor:    1.0,
		analyzer:         analyzer,
		softDeletesRatio: 0.0,
	}

	if useSoftDeletes {
		riw.softDeletesRatio = 1.0 / (1.0 + float64(localRand.Intn(10)))
	}

	// Make sure we sometimes test indices that don't get any forced merges
	if c.LiveIndexWriterConfig.GetMergePolicy() != nil {
		riw.doRandomForceMerge = localRand.Intn(2) == 0
	}

	return riw, nil
}

func (riw *RandomIndexWriter) maybeChangeLiveIndexWriterConfig() {
	// This mimics LuceneTestCase.maybeChangeLiveIndexWriterConfig.
	// It would randomly change some parameters in riw.config.
	// Since it's a test helper, we implement a minimal version.
	if riw.r.Intn(10) == 0 {
		// Example: change a random setting
		// riw.config.SetSomeSetting(...)
	}
}

func (riw *RandomIndexWriter) AddDocument(doc *document.Document) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()

	var seqNo int64
	var err error

	if riw.r.Intn(5) == 3 {
		// Use AddDocuments with a single doc slice to test the batch path
		seqNo, err = riw.w.AddDocuments([]*document.Document{doc})
	} else {
		seqNo, err = riw.w.AddDocument(doc)
	}

	if err != nil {
		return 0, err
	}

	riw.maybeFlushOrCommit()
	return seqNo, nil
}

func (riw *RandomIndexWriter) AddDocuments(docs []*document.Document) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()
	seqNo, err := riw.w.AddDocuments(docs)
	if err != nil {
		return 0, err
	}
	riw.maybeFlushOrCommit()
	return seqNo, nil
}

func (riw *RandomIndexWriter) UpdateDocument(t *Term, doc *document.Document) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()

	var seqNo int64
	var err error

	if riw.useSoftDeletes() {
		if riw.r.Intn(5) == 3 {
			// use softUpdateDocuments with a slice
			seqNo, err = riw.w.UpdateDocuments([]*document.Document{doc}, t)
			// In Lucene, it explicitly uses NumericDocValuesField for soft deletes.
			// In Gocene, we assume the IndexWriter handles the soft delete field
			// if it's configured in the config.
		} else {
			seqNo, err = riw.w.UpdateDocument(t, doc)
		}
	} else {
		if riw.r.Intn(5) == 3 {
			seqNo, err = riw.w.UpdateDocuments([]*document.Document{doc}, t)
		} else {
			seqNo, err = riw.w.UpdateDocument(t, doc)
		}
	}

	if err != nil {
		return 0, err
	}

	riw.maybeFlushOrCommit()
	return seqNo, nil
}

func (riw *RandomIndexWriter) UpdateDocuments(delTerm *Term, docs []*document.Document) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()

	var seqNo int64
	var err error

	if riw.useSoftDeletes() {
		// Gocene's IndexWriter needs to support softUpdateDocuments.
		// If not explicitly present, we use UpdateDocuments.
		seqNo, err = riw.w.UpdateDocuments(docs, delTerm)
	} else {
		if riw.r.Intn(10) < 3 {
			// 30% chance to use a query-based update.
			// Lucene: w.updateDocuments(new TermQuery(delTerm), docs);
			// Gocene: we'll use the term-based one if query-based isn't available
			seqNo, err = riw.w.UpdateDocuments(docs, delTerm)
		} else {
			seqNo, err = riw.w.UpdateDocuments(docs, delTerm)
		}
	}

	if err != nil {
		return 0, err
	}

	riw.maybeFlushOrCommit()
	return seqNo, nil
}

func (riw *RandomIndexWriter) useSoftDeletes() bool {
	return riw.r.Float64() < riw.softDeletesRatio
}

func (riw *RandomIndexWriter) maybeFlushOrCommit() {
	riw.maybeChangeLiveIndexWriterConfig()
	if riw.docCount == riw.flushAt {
		if riw.r.Intn(2) == 0 {
			_ = riw.flushAllBuffersSequentially()
		} else if riw.r.Intn(2) == 0 {
			_ = riw.w.Flush()
		} else {
			_, _ = riw.w.Commit()
		}
		riw.docCount = 0
		riw.flushAt += 10 + riw.r.Intn(991) // Simplified range based on flushAtFactor
		// Gradually increase time b/w flushes
		if riw.flushAtFactor < 2e6 {
			riw.flushAtFactor *= 1.05
		}
	}
	riw.docCount++
}

func (riw *RandomIndexWriter) flushAllBuffersSequentially() error {
	threadPoolSize := riw.w.GetDocWriterThreadPoolSize()
	numFlushes := 0
	if threadPoolSize > 0 {
		numFlushes = riw.r.Intn(threadPoolSize + 1)
		if numFlushes > 1 {
			numFlushes = 1 // Math.min(1, r.nextInt(threadPoolSize + 1)) in Lucene is actually always 0 or 1.
		}
	}

	for i := 0; i < numFlushes; i++ {
		if !riw.w.FlushNextBuffer() {
			break
		}
	}
	return nil
}

func (riw *RandomIndexWriter) AddIndexes(dirs ...store.Directory) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()
	// Gocene's IndexWriter should have AddIndexes.
	// Assuming it exists as per Lucene.
	return 0, fmt.Errorf("AddIndexes not implemented in Gocene IndexWriter")
}

func (riw *RandomIndexWriter) Commit() (int64, error) {
	return riw.CommitExtended(riw.r.Intn(10) == 0)
}

func (riw *RandomIndexWriter) CommitExtended(flushConcurrently bool) (int64, error) {
	riw.maybeChangeLiveIndexWriterConfig()

	if flushConcurrently {
		var wg sync.WaitGroup
		var errChan = make(chan error, 1)

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := riw.flushAllBuffersSequentially(); err != nil {
				errChan <- err
			}
		}()

		seqNo, err := riw.w.Commit()
		wg.Wait()

		select {
		case e := <-errChan:
			return 0, e
		default:
			if err != nil {
				return 0, err
			}
			return seqNo, nil
		}
	}

	return riw.w.Commit()
}

func (riw *RandomIndexWriter) GetReader() (*StandardDirectoryReader, error) {
	return riw.GetReaderExtended(true, false)
}

func (riw *RandomIndexWriter) GetReaderExtended(applyDeletions, writeAllDeletes bool) (*StandardDirectoryReader, error) {
	riw.maybeChangeLiveIndexWriterConfig()
	riw.getReaderCalled = true

	if riw.r.Intn(20) == 2 {
		_ = riw.doRandomForceMerge()
	}

	if !applyDeletions || riw.r.Intn(2) == 0 {
		if riw.r.Intn(5) == 1 {
			_, _ = riw.w.Commit()
		}
		return riw.w.GetReader(applyDeletions, writeAllDeletes)
	} else {
		_, _ = riw.w.Commit()
		// In Lucene, they might open a new reader from directory.
		// For now, we use the NRT reader for simplicity.
		return riw.w.GetReader(applyDeletions, writeAllDeletes)
	}
}

func (riw *RandomIndexWriter) doRandomForceMerge() error {
	if riw.doRandomForceMerge {
		segCount := riw.w.GetSegmentCount()
		if riw.r.Intn(2) == 0 || segCount == 0 {
			return riw.w.ForceMerge(1)
		} else if riw.r.Intn(2) == 0 {
			limit := 1 + riw.r.Intn(segCount)
			return riw.w.ForceMerge(limit)
		} else {
			// forceMergeDeletes is not yet explicit in Gocene's IndexWriter
			// We'll use ForceMerge(1) as a proxy or skip.
			return riw.w.ForceMerge(1)
		}
	}
	return nil
}

func (riw *RandomIndexWriter) Close() error {
	if !riw.w.IsClosed() {
		riw.maybeChangeLiveIndexWriterConfig()
	}

	if !riw.getReaderCalled && riw.r.Intn(8) == 2 && !riw.w.IsClosed() {
		_ = riw.doRandomForceMerge()
		if !riw.config.GetCommitOnClose() {
			_, _ = riw.w.Commit()
		}
	}

	return riw.w.Close()
}

func (riw *RandomIndexWriter) ForceMerge(maxSegmentCount int) error {
	riw.maybeChangeLiveIndexWriterConfig()
	return riw.w.ForceMerge(maxSegmentCount)
}

func (riw *RandomIndexWriter) Flush() error {
	return riw.w.Flush()
}
