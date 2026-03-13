// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// IndexSearcher searches an index.
type IndexSearcher struct {
	reader index.IndexReaderInterface
}

// NewIndexSearcher creates a new IndexSearcher.
func NewIndexSearcher(reader index.IndexReaderInterface) *IndexSearcher {
	return &IndexSearcher{reader: reader}
}

// Search executes a query and returns TopDocs.
func (s *IndexSearcher) Search(query Query, n int) (*TopDocs, error) {
	if n <= 0 {
		return &TopDocs{
			TotalHits: NewTotalHits(0, EQUAL_TO),
			ScoreDocs: make([]*ScoreDoc, 0),
		}, nil
	}

	collector := NewTopDocsCollector(n)
	err := s.SearchWithCollector(query, collector)
	if err != nil {
		return nil, err
	}
	return collector.TopDocs(), nil
}

// SearchWithCollector executes a query and collects results.
func (s *IndexSearcher) SearchWithCollector(query Query, collector Collector) error {
	// Rewrite query
	rewritten, err := query.Rewrite(s.reader)
	if err != nil {
		return err
	}

	// Create weight
	needsScores := collector.ScoreMode() == COMPLETE || collector.ScoreMode() == TOP_SCORES
	weight, err := rewritten.CreateWeight(s, needsScores, 1.0)
	if err != nil {
		return err
	}

	// For now, handle DirectoryReader vs single segment
	if dr, ok := interface{}(s.reader).(*index.DirectoryReader); ok {
		docBase := 0
		for _, sr := range dr.GetSegmentReaders() {
			err = s.searchLeaf(sr, docBase, weight, collector)
			if err != nil {
				return err
			}
			docBase += sr.MaxDoc()
		}
	} else {
		return s.searchLeaf(s.reader, 0, weight, collector)
	}

	return nil
}

func (s *IndexSearcher) searchLeaf(reader index.IndexReaderInterface, docBase int, weight Weight, collector Collector) error {
	leafCollector, err := collector.GetLeafCollector(reader)
	if err != nil {
		return err
	}

	// If it's a TopDocsLeafCollector, set the docBase
	if tdc, ok := leafCollector.(*TopDocsLeafCollector); ok {
		tdc.SetDocBase(docBase)
	}

	scorer, err := weight.Scorer(reader)
	if err != nil {
		return err
	}

	if scorer != nil {
		err = leafCollector.SetScorer(scorer)
		if err != nil {
			return err
		}

		for {
			doc, err := scorer.NextDoc()
			if err != nil {
				return err
			}
			if doc == NO_MORE_DOCS {
				break
			}
			err = leafCollector.Collect(doc)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Doc returns the stored fields for a document.
func (s *IndexSearcher) Doc(docID int) (*index.Document, error) {
	// TODO: Implement
	return nil, nil
}

// GetIndexReader returns the IndexReader.
func (s *IndexSearcher) GetIndexReader() index.IndexReaderInterface {
	return s.reader
}

// Close closes the searcher.
func (s *IndexSearcher) Close() error {
	return nil
}
