// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// init registers the query-delete executor hook so that
// IndexWriter.DeleteDocumentsQuery can use an IndexSearcher to find matching
// documents in committed segments, without a direct search → index import cycle.
func init() {
	index.RegisterQueryDeleteExecutor(executeQueryDeletes)
}

// executeQueryDeletes opens an IndexSearcher over the committed segments in
// directory, executes each query, and returns the matching docIDs per segment.
//
// The return map is keyed by segment name and each value is a slice of
// 0-based per-segment docIDs that match at least one of the queries.
func executeQueryDeletes(dir store.Directory, si *spi.SegmentInfos, queries []interface{}) (map[string][]int, error) {
	if len(queries) == 0 {
		return nil, nil
	}

	reader, err := index.OpenDirectoryReaderWithInfos(dir, si)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// Collect all search.Query values from the interface{} slice.
	var searchQueries []Query
	for _, q := range queries {
		if sq, ok := q.(Query); ok {
			searchQueries = append(searchQueries, sq)
		}
	}
	if len(searchQueries) == 0 {
		return nil, nil
	}

	searcher := NewIndexSearcher(reader)
	maxDoc := reader.MaxDoc()

	// Collect global docIDs matching any query.
	matched := make(map[int]struct{})
	for _, q := range searchQueries {
		topDocs, err := searcher.Search(q, maxDoc+1)
		if err != nil {
			// Skip unexecutable queries (e.g. missing codec, no postings).
			continue
		}
		for _, sd := range topDocs.ScoreDocs {
			matched[sd.Doc] = struct{}{}
		}
	}
	if len(matched) == 0 {
		return nil, nil
	}

	// Map global docIDs to per-segment docIDs using the segment info ordering.
	// The segments in si are in the same order as the leaves in the reader;
	// each segment contributes DocCount() consecutive docIDs starting at docBase.
	result := make(map[string][]int)
	docBase := 0
	for _, sci := range si.List() {
		segName := sci.SegmentInfo().Name()
		segMaxDoc := sci.SegmentInfo().DocCount()
		for globalID := range matched {
			if globalID >= docBase && globalID < docBase+segMaxDoc {
				result[segName] = append(result[segName], globalID-docBase)
			}
		}
		docBase += segMaxDoc
	}
	return result, nil
}
