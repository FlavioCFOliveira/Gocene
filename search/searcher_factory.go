// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// SearcherFactory is the Go port of org.apache.lucene.search.SearcherFactory
// (Apache Lucene 10.5.0): a factory class used by [SearcherManager] to create
// new IndexSearchers. The default implementation just creates an
// IndexSearcher with no custom behavior:
//
//	func (f *SearcherFactory) NewSearcher(r, previousReader index.IndexReaderInterface) (*IndexSearcher, error) {
//		return NewIndexSearcher(r), nil
//	}
//
// You can pass your own factory instead if you want custom behavior, such as:
//
//   - Setting a custom scoring model: IndexSearcher.SetSimilarity
//   - Parallel per-segment search: NewIndexSearcherWithExecutor
//   - Return custom subclasses of IndexSearcher (for example that implement
//     distributed scoring)
//   - Run queries to warm your IndexSearcher before it is used. Note: when
//     using near-realtime search you may want to also set
//     IndexWriterConfig.setMergedSegmentWarmer to warm newly merged segments
//     in the background, outside of the reopen path.
//
// Java's SearcherFactory is a concrete, subclassable class. Go renders the
// overridable member as this interface and the concrete class as
// [BaseSearcherFactory] (new SearcherFactory() is [NewSearcherFactory]); a
// subclass embeds *BaseSearcherFactory and redeclares NewSearcher, calling
// f.BaseSearcherFactory.NewSearcher where Java calls super.newSearcher.
type SearcherFactory interface {
	// NewSearcher returns a new IndexSearcher over the given reader.
	//
	// reader is the reader to create a new searcher for. previousReader is
	// the reader previously used to create a new searcher. This can be nil
	// if unknown or if the given reader is the initially opened reader. If
	// this reader is non-nil it can be used to find newly opened segments
	// compared to the new reader to warm the searcher up before returning.
	NewSearcher(reader, previousReader index.IndexReaderInterface) (*IndexSearcher, error)
}

// BaseSearcherFactory is the concrete class org.apache.lucene.search.SearcherFactory.
type BaseSearcherFactory struct{}

// NewSearcherFactory renders new SearcherFactory().
func NewSearcherFactory() *BaseSearcherFactory {
	return &BaseSearcherFactory{}
}

// NewSearcher renders public IndexSearcher newSearcher(IndexReader reader,
// IndexReader previousReader).
func (f *BaseSearcherFactory) NewSearcher(reader, previousReader index.IndexReaderInterface) (*IndexSearcher, error) {
	return NewIndexSearcher(reader), nil
}

var _ SearcherFactory = (*BaseSearcherFactory)(nil)
