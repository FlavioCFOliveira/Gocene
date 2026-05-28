// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/document"
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
	if n < 0 {
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

// SearchAfter finds the top n hits for query, restricted to documents that
// sort strictly after the given ScoreDoc in the (score desc, docID asc)
// ordering. Passing the bottom result of a previous page as after enables
// cursor-based pagination ("deep paging") that returns non-overlapping,
// globally ordered pages.
//
// This is the Go port of org.apache.lucene.search.IndexSearcher#searchAfter
// (Lucene 10.4.0, IndexSearcher.java lines 582-596). As in Lucene:
//   - the effective limit is max(1, reader.MaxDoc());
//   - after.Doc must be < limit, otherwise an error is returned;
//   - n is capped to min(n, limit);
//   - n must be > 0 (Lucene's TopScoreDocCollectorManager rejects numHits<=0),
//     so a non-positive n yields an error rather than empty results.
func (s *IndexSearcher) SearchAfter(after *ScoreDoc, query Query, n int) (*TopDocs, error) {
	limit := s.reader.MaxDoc()
	if limit < 1 {
		limit = 1
	}

	if after != nil && after.Doc >= limit {
		return nil, fmt.Errorf(
			"after.doc exceeds the number of documents in the reader: after.doc=%d limit=%d",
			after.Doc, limit)
	}

	if n <= 0 {
		return nil, fmt.Errorf("numHits must be > 0, got %d", n)
	}

	cappedNumHits := n
	if cappedNumHits > limit {
		cappedNumHits = limit
	}

	collector := NewTopDocsCollectorAfter(cappedNumHits, after)
	if err := s.SearchWithCollector(query, collector); err != nil {
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

// asLeafReader extracts a *index.LeafReader from an IndexReaderInterface.
// SegmentReader embeds *LeafReader, so we must handle that case explicitly.
func asLeafReader(r index.IndexReaderInterface) *index.LeafReader {
	switch v := r.(type) {
	case *index.LeafReader:
		return v
	case *index.SegmentReader:
		return v.LeafReader
	default:
		return nil
	}
}

func (s *IndexSearcher) searchLeaf(reader index.IndexReaderInterface, docBase int, weight Weight, collector Collector) error {
	// Create a LeafReaderContext for the reader.
	// Pass reader directly (may be *SegmentReader which overrides Terms()); do
	// NOT unwrap to the embedded *LeafReader, which would lose the override.
	ctx := index.NewLeafReaderContext(reader, nil, 0, docBase)

	leafCollector, err := collector.GetLeafCollector(reader)
	if err != nil {
		return err
	}

	// If it's a TopDocsLeafCollector, set the docBase
	if tdc, ok := leafCollector.(*TopDocsLeafCollector); ok {
		tdc.SetDocBase(docBase)
	}

	scorer, err := weight.Scorer(ctx)
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
func (s *IndexSearcher) Doc(docID int) (*document.Document, error) {
	// Find the segment that contains this document
	if dr, ok := interface{}(s.reader).(*index.DirectoryReader); ok {
		readers := dr.GetSegmentReaders()
		docBase := 0

		for _, sr := range readers {
			maxDoc := sr.MaxDoc()
			if docID >= docBase && docID < docBase+maxDoc {
				// Found the segment - convert to segment-local doc ID
				segmentDocID := docID - docBase
				return s.docFromSegment(sr, segmentDocID)
			}
			docBase += maxDoc
		}
		return nil, nil
	}

	// For single segment readers
	if sr, ok := interface{}(s.reader).(*index.SegmentReader); ok {
		return s.docFromSegment(sr, docID)
	}

	// For LeafReader
	if lr, ok := interface{}(s.reader).(*index.LeafReader); ok {
		return s.docFromLeafReader(lr, docID)
	}

	return document.NewDocument(), nil
}

// docFromSegment retrieves a document from a SegmentReader.
func (s *IndexSearcher) docFromSegment(sr *index.SegmentReader, docID int) (*document.Document, error) {
	storedFields, err := sr.StoredFields()
	if err != nil {
		return nil, err
	}

	visitor := NewDocumentVisitor()
	err = storedFields.Document(docID, visitor)
	if err != nil {
		return nil, err
	}

	return visitor.Document(), nil
}

// docFromLeafReader retrieves a document from a LeafReader.
func (s *IndexSearcher) docFromLeafReader(lr *index.LeafReader, docID int) (*document.Document, error) {
	storedFields, err := lr.StoredFields()
	if err != nil {
		return nil, err
	}

	visitor := NewDocumentVisitor()
	err = storedFields.Document(docID, visitor)
	if err != nil {
		return nil, err
	}

	return visitor.Document(), nil
}

// GetIndexReader returns the IndexReader.
func (s *IndexSearcher) GetIndexReader() index.IndexReaderInterface {
	return s.reader
}

// Close closes the searcher.
func (s *IndexSearcher) Close() error {
	return nil
}

// DocumentVisitor is a StoredFieldVisitor that collects fields into a Document.
type DocumentVisitor struct {
	doc *document.Document
}

// NewDocumentVisitor creates a new DocumentVisitor.
func NewDocumentVisitor() *DocumentVisitor {
	return &DocumentVisitor{
		doc: document.NewDocument(),
	}
}

// StringField is called for a stored string field.
func (v *DocumentVisitor) StringField(field string, value string) {
	sf, _ := document.NewStoredField(field, value)
	v.doc.Add(sf)
}

// BinaryField is called for a stored binary field.
func (v *DocumentVisitor) BinaryField(field string, value []byte) {
	sf, _ := document.NewStoredFieldFromBytes(field, value)
	v.doc.Add(sf)
}

// IntField is called for a stored int field.
func (v *DocumentVisitor) IntField(field string, value int) {
	sf, _ := document.NewStoredFieldFromInt(field, value)
	v.doc.Add(sf)
}

// LongField is called for a stored long field.
func (v *DocumentVisitor) LongField(field string, value int64) {
	sf, _ := document.NewStoredFieldFromInt64(field, value)
	v.doc.Add(sf)
}

// FloatField is called for a stored float field.
func (v *DocumentVisitor) FloatField(field string, value float32) {
	sf, _ := document.NewStoredFieldFromFloat64(field, float64(value))
	v.doc.Add(sf)
}

// DoubleField is called for a stored double field.
func (v *DocumentVisitor) DoubleField(field string, value float64) {
	sf, _ := document.NewStoredFieldFromFloat64(field, value)
	v.doc.Add(sf)
}

// Document returns the collected Document.
func (v *DocumentVisitor) Document() *document.Document {
	return v.doc
}
