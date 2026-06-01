// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package document provides document-level classifiers that extend the base
// classification classifiers to operate on Document objects rather than plain
// strings.
//
// Port of org.apache.lucene.classification.document.
package document

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/classification"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// KNearestNeighborDocumentClassifier classifies documents using a
// k-nearest-neighbor approach backed by a Lucene index.  It extends
// KNearestNeighborClassifier to operate on Document objects.
//
// Port of org.apache.lucene.classification.document.KNearestNeighborDocumentClassifier.
type KNearestNeighborDocumentClassifier struct {
	// base provides the index access infrastructure shared with the
	// text-string KNN classifier.
	base           *classification.KNearestNeighborClassifier
	mlt            *search.MoreLikeThis
	searcher       *search.IndexSearcher
	query          search.Query
	classFieldName string
	textFieldNames []string
	field2analyzer map[string]analysis.Analyzer
	k              int
}

// NewKNearestNeighborDocumentClassifier creates the document classifier.
//
//   - reader: an index.IndexReaderInterface over the training index.
//   - similarity: the search.Similarity to use (nil defaults to BM25).
//   - query: an optional filter query (pass nil for no filter).
//   - k: number of nearest neighbours to retrieve.
//   - minDocsFreq: MoreLikeThis MinDocFreq threshold (≤ 0 keeps default).
//   - minTermFreq: MoreLikeThis MinTermFreq threshold (≤ 0 keeps default).
//   - classFieldName: the field holding the class label.
//   - field2analyzer: per-field analyzers for document field tokenization.
//   - textFieldNames: the text fields used for classification (may include
//     boost syntax, e.g. "title^2").
func NewKNearestNeighborDocumentClassifier(
	reader interface{},
	_ interface{}, // similarity (reserved for future use)
	query interface{},
	k, minDocsFreq, minTermFreq int,
	classFieldName string,
	field2analyzer map[string]analysis.Analyzer,
	textFieldNames ...string,
) *KNearestNeighborDocumentClassifier {
	c := &KNearestNeighborDocumentClassifier{
		classFieldName: classFieldName,
		textFieldNames: textFieldNames,
		field2analyzer: field2analyzer,
		k:              k,
	}
	if q, ok := query.(search.Query); ok {
		c.query = q
	}
	if ri, ok := reader.(index.IndexReaderInterface); ok {
		searcher := search.NewIndexSearcher(ri)
		searcher.SetSimilarity(search.NewBM25Similarity())
		c.searcher = searcher

		// Build a default analyzer from the first available field analyzer.
		var defaultAnalyzer analysis.Analyzer
		for _, a := range field2analyzer {
			defaultAnalyzer = a
			break
		}
		if defaultAnalyzer != nil {
			mlt := search.NewMoreLikeThis(defaultAnalyzer)
			plainFields := plainFieldNames(textFieldNames)
			mlt.FieldNames = plainFields
			if minDocsFreq > 0 {
				mlt.MinDocFreq = minDocsFreq
			}
			if minTermFreq > 0 {
				mlt.MinTermFreq = minTermFreq
			}
			c.mlt = mlt
		}
	}
	return c
}

// AssignClass assigns the most-probable class to the given document.
func (c *KNearestNeighborDocumentClassifier) AssignClass(doc interface{}) (*classification.ClassificationResult[*util.BytesRef], error) {
	d, ok := doc.(*document.Document)
	if !ok || d == nil || c.searcher == nil {
		return nil, nil
	}
	topDocs, err := c.knnSearchDocument(d)
	if err != nil {
		return nil, err
	}
	return c.classifyFromTopDocs(topDocs)
}

// GetClasses returns all classes sorted by descending score.
func (c *KNearestNeighborDocumentClassifier) GetClasses(doc interface{}) ([]*classification.ClassificationResult[*util.BytesRef], error) {
	d, ok := doc.(*document.Document)
	if !ok || d == nil || c.searcher == nil {
		return nil, nil
	}
	topDocs, err := c.knnSearchDocument(d)
	if err != nil {
		return nil, err
	}
	list, err := c.buildListFromTopDocs(topDocs)
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Score > list[j].Score })
	return list, nil
}

// GetClassesMax returns the top max classes sorted by descending score.
func (c *KNearestNeighborDocumentClassifier) GetClassesMax(doc interface{}, max int) ([]*classification.ClassificationResult[*util.BytesRef], error) {
	list, err := c.GetClasses(doc)
	if err != nil {
		return nil, err
	}
	if max > len(list) {
		max = len(list)
	}
	return list[:max], nil
}

// knnSearchDocument builds a MoreLikeThis query from the document's field
// values and returns the top-k hits.
func (c *KNearestNeighborDocumentClassifier) knnSearchDocument(doc *document.Document) (*search.TopDocs, error) {
	if c.mlt == nil {
		return nil, nil
	}
	mltQuery := search.NewBooleanQuery()
	for _, fieldName := range c.textFieldNames {
		plain, boost := splitFieldBoost(fieldName)
		fieldAnalyzer := c.field2analyzer[plain]
		if fieldAnalyzer == nil {
			continue
		}
		c.mlt.Analyzer = fieldAnalyzer
		c.mlt.FieldNames = []string{plain}

		for _, fieldVal := range doc.GetValues(plain) {
			q, err := c.mlt.LikeText(fieldVal)
			if err != nil {
				continue
			}
			if boost > 0 {
				q = search.NewBoostQuery(q, boost)
			}
			mltQuery.Add(q, search.SHOULD)
		}
		c.mlt.Analyzer = nil
	}
	// See KNearestNeighborClassifier.knnSearch for the rationale behind
	// omitting the class-field wildcard constraint.
	if c.query != nil {
		mltQuery.Add(c.query, search.MUST)
	}
	return c.searcher.Search(mltQuery, c.k)
}

// classifyFromTopDocs returns the class with the highest score from the kNN
// result set.
func (c *KNearestNeighborDocumentClassifier) classifyFromTopDocs(topDocs *search.TopDocs) (*classification.ClassificationResult[*util.BytesRef], error) {
	list, err := c.buildListFromTopDocs(topDocs)
	if err != nil {
		return nil, err
	}
	var best *classification.ClassificationResult[*util.BytesRef]
	for _, r := range list {
		if best == nil || r.Score > best.Score {
			best = r
		}
	}
	return best, nil
}

// buildListFromTopDocs tallies per-class counts and score boosts.
func (c *KNearestNeighborDocumentClassifier) buildListFromTopDocs(topDocs *search.TopDocs) ([]*classification.ClassificationResult[*util.BytesRef], error) {
	if topDocs == nil || len(topDocs.ScoreDocs) == 0 {
		return nil, nil
	}
	classCounts := make(map[string]int)
	classBoosts := make(map[string]float64)
	var maxScore float32
	if topDocs.TotalHits.Value > 0 {
		maxScore = topDocs.ScoreDocs[0].Score
	}
	for _, sd := range topDocs.ScoreDocs {
		doc, err := c.searcher.Doc(sd.Doc)
		if err != nil || doc == nil {
			continue
		}
		for _, f := range doc.GetFieldsByName(c.classFieldName) {
			if f == nil {
				continue
			}
			cl := f.StringValue()
			if cl == "" {
				continue
			}
			classCounts[cl]++
			classBoosts[cl] += float64(sd.Score) / float64(maxScore)
		}
	}
	var temp []*classification.ClassificationResult[*util.BytesRef]
	var sumdoc int
	for cl, count := range classCounts {
		normBoost := classBoosts[cl] / float64(count)
		temp = append(temp, &classification.ClassificationResult[*util.BytesRef]{
			AssignedClass: util.NewBytesRef([]byte(cl)),
			Score:         float64(count) * normBoost / float64(c.k),
		})
		sumdoc += count
	}
	if sumdoc < c.k {
		for i, cr := range temp {
			temp[i] = &classification.ClassificationResult[*util.BytesRef]{
				AssignedClass: cr.AssignedClass,
				Score:         cr.Score * float64(c.k) / float64(sumdoc),
			}
		}
	}
	return temp, nil
}

// ---- helpers ----------------------------------------------------------------

func splitFieldBoost(fieldName string) (string, float32) {
	if idx := strings.Index(fieldName, "^"); idx >= 0 {
		plain := fieldName[:idx]
		var b float32
		if _, err := fmt.Sscanf(fieldName[idx+1:], "%f", &b); err == nil && b > 0 {
			return plain, b
		}
		return plain, 0
	}
	return fieldName, 0
}

func plainFieldNames(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		plain, _ := splitFieldBoost(n)
		out[i] = plain
	}
	return out
}
