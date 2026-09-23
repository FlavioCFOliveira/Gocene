// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestEarlyTermination.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
)

// earlyTerminationTestCase renders the fields of TestEarlyTermination.
type earlyTerminationTestCase struct {
	dir    store.Directory
	writer *testindex.RandomIndexWriter
}

// etSetUp renders setUp(); the returned function renders tearDown().
func etSetUp(t *testing.T) (*earlyTerminationTestCase, func()) {
	t.Helper()
	tc := &earlyTerminationTestCase{}
	dir := newDirectory()
	tc.dir = dir
	tc.writer = newRandomIndexWriter(t, dir)
	numDocs := atLeast(100)
	for i := 0; i < numDocs; i++ {
		mustAddDocument(t, tc.writer, document.NewDocument())
		if rarely() {
			mustCommit(t, tc.writer)
		}
	}
	return tc, func() {
		mustClose(t, tc.writer, dir)
	}
}

func TestEarlyTermination(t *testing.T) {
	tc, tearDown := etSetUp(t)
	defer tearDown()
	iters := atLeast(5)
	reader := mustGetReader(t, tc.writer)

	for i := 0; i < iters; i++ {
		searcher := newSearcher(t, reader)
		if _, err := search.SearchWithCollectorManager[*etCollector, struct{}](searcher, search.NewMatchAllDocsQuery(), &etCollectorManager{t: t}); err != nil {
			t.Fatalf("search: %v", err)
		}
	}
	mustClose(t, reader)
}

// etCollector renders the anonymous SimpleCollector of testEarlyTermination.
type etCollector struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector
	t                    *testing.T
	collectionTerminated bool
}

func (c *etCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if err := c.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *etCollector) Collect(doc int) error {
	if c.collectionTerminated {
		c.t.Error("assertFalse(collectionTerminated)")
	}
	if rarely() {
		c.collectionTerminated = true
		return search.NewCollectionTerminatedException()
	}
	return nil
}

func (c *etCollector) DoSetNextReader(context *index.LeafReaderContext) error {
	if random().Intn(2) == 0 {
		c.collectionTerminated = true
		return search.NewCollectionTerminatedException()
	}
	c.collectionTerminated = false
	return nil
}

func (c *etCollector) ScoreMode() search.ScoreMode {
	return search.COMPLETE_NO_SCORES
}

func (c *etCollector) SetScorer(scorer search.Scorable) error { return nil }

func (c *etCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

func (c *etCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

func (c *etCollector) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return c.BaseLeafCollector.CompetitiveIterator()
}

func (c *etCollector) Finish() error { return c.BaseLeafCollector.Finish() }

// etCollectorManager renders the anonymous CollectorManager<SimpleCollector, Void>.
type etCollectorManager struct {
	t *testing.T
}

func (m *etCollectorManager) NewCollector() (*etCollector, error) {
	c := &etCollector{t: m.t, collectionTerminated: true}
	c.Outer = c
	return c, nil
}

func (m *etCollectorManager) Reduce(collectors []*etCollector) (struct{}, error) {
	return struct{}{}, nil
}
