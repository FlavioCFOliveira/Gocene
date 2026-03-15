// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// BulkScorer scores a range of documents.
type BulkScorer interface {
	// Score scores documents from minDoc to maxDoc (exclusive).
	Score(collector Collector, acceptDocs DocIdSetIterator) error
}

// DefaultBulkScorer is a default implementation of BulkScorer.
type DefaultBulkScorer struct {
	scorer Scorer
}

// NewDefaultBulkScorer creates a new DefaultBulkScorer.
func NewDefaultBulkScorer(scorer Scorer) *DefaultBulkScorer {
	return &DefaultBulkScorer{scorer: scorer}
}

// Score scores documents from minDoc to maxDoc.
func (bs *DefaultBulkScorer) Score(collector Collector, acceptDocs DocIdSetIterator) error {
	// Simplified implementation - iterate through all docs
	doc, err := bs.scorer.NextDoc()
	if err != nil {
		return err
	}

	for doc != NO_MORE_DOCS {
		// In a real implementation, we would collect the document here
		doc, err = bs.scorer.NextDoc()
		if err != nil {
			return err
		}
	}

	return nil
}
