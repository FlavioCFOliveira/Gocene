// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classification

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// KNearestNeighborClassifier classifies text using a k-nearest-neighbor
// approach backed by a Lucene index.  Given an input text it builds a
// MoreLikeThis query, retrieves the top-k similar training documents, and
// assigns the majority class (weighted by normalized BM25 score) as the
// predicted label.
//
// Port of org.apache.lucene.classification.KNearestNeighborClassifier.
type KNearestNeighborClassifier struct {
	mlt            *search.MoreLikeThis
	searcher       *search.IndexSearcher
	analyzer       analysis.Analyzer
	query          search.Query
	classFieldName string
	textFieldNames []string
	k              int
}

// NewKNearestNeighborClassifier creates the classifier.
//
//   - reader: an index.IndexReaderInterface over the training index.
//   - analyzer: the Analyzer used to tokenize text.
//   - query: an optional filter query (pass nil to use all indexed docs).
//   - k: number of nearest neighbours to retrieve.
//   - minDocsFreq: MoreLikeThis MinDocFreq threshold (≤ 0 keeps default).
//   - minTermFreq: MoreLikeThis MinTermFreq threshold (≤ 0 keeps default).
//   - classFieldName: the field that holds the class label.
//   - textFieldNames: the text fields used for classification (may include
//     field boost syntax, e.g. "title^2").
func NewKNearestNeighborClassifier(
	reader interface{},
	analyzer analysis.Analyzer,
	query interface{},
	k, minDocsFreq, minTermFreq int,
	classFieldName string,
	textFieldNames ...string,
) *KNearestNeighborClassifier {
	c := &KNearestNeighborClassifier{
		analyzer:       analyzer,
		classFieldName: classFieldName,
		textFieldNames: textFieldNames,
		k:              k,
	}
	if q, ok := query.(search.Query); ok {
		c.query = q
	}
	if ri, ok := reader.(index.IndexReaderInterface); ok {
		searcher := search.NewIndexSearcher(ri)
		searcher.SetSimilarity(search.NewBM25Similarity())
		c.searcher = searcher

		mlt := search.NewMoreLikeThis(analyzer)
		// Strip boost decorators to pass plain field names to MoreLikeThis.
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
	return c
}

// AssignClass assigns the most-probable class to text.
func (c *KNearestNeighborClassifier) AssignClass(text string) (*ClassificationResult[*util.BytesRef], error) {
	knnResults, err := c.knnSearch(text)
	if err != nil {
		return nil, err
	}
	return c.classifyFromTopDocs(knnResults)
}

// GetClasses returns all classes sorted by descending score.
func (c *KNearestNeighborClassifier) GetClasses(text string) ([]*ClassificationResult[*util.BytesRef], error) {
	knnResults, err := c.knnSearch(text)
	if err != nil {
		return nil, err
	}
	list, err := c.buildListFromTopDocs(knnResults)
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Score > list[j].Score })
	return list, nil
}

// GetClassesMax returns the top max classes sorted by descending score.
func (c *KNearestNeighborClassifier) GetClassesMax(text string, max int) ([]*ClassificationResult[*util.BytesRef], error) {
	list, err := c.GetClasses(text)
	if err != nil {
		return nil, err
	}
	if max > len(list) {
		max = len(list)
	}
	return list[:max], nil
}

// knnSearch runs a MoreLikeThis query for text and returns the top-k hits.
// Mirrors KNearestNeighborClassifier.knnSearch.
func (c *KNearestNeighborClassifier) knnSearch(text string) (*search.TopDocs, error) {
	if c.searcher == nil || c.mlt == nil {
		return nil, nil
	}

	// Build the MLT subquery for each field, honoring per-field boost hints.
	mltQuery := search.NewBooleanQuery()
	for _, fieldName := range c.textFieldNames {
		plain, boost := splitFieldBoost(fieldName)
		c.mlt.FieldNames = []string{plain}
		if boost > 0 {
			c.mlt.MinDocFreq = 1 // allow boosted fields to contribute
		}
		// LikeText builds a BooleanQuery of interesting terms from text.
		q, err := c.mlt.LikeText(text)
		if err != nil {
			// No interesting terms for this field; skip silently.
			continue
		}
		if boost > 0 {
			q = search.NewBoostQuery(q, boost)
		}
		mltQuery.Add(q, search.SHOULD)
	}
	// Restore field list.
	c.mlt.FieldNames = plainFieldNames(c.textFieldNames)

	// Require that result documents have a class field value.
	classFieldQuery := search.NewWildcardQuery(index.NewTerm(c.classFieldName, "*"))
	mltQuery.Add(classFieldQuery, search.MUST)
	if c.query != nil {
		mltQuery.Add(c.query, search.MUST)
	}

	return c.searcher.Search(mltQuery, c.k)
}

// classifyFromTopDocs picks the class with the highest weighted score from
// the kNN result set.
func (c *KNearestNeighborClassifier) classifyFromTopDocs(topDocs *search.TopDocs) (*ClassificationResult[*util.BytesRef], error) {
	if topDocs == nil {
		return nil, nil
	}
	list, err := c.buildListFromTopDocs(topDocs)
	if err != nil {
		return nil, err
	}
	var best *ClassificationResult[*util.BytesRef]
	for _, r := range list {
		if best == nil || r.Score > best.Score {
			best = r
		}
	}
	return best, nil
}

// buildListFromTopDocs tallies per-class vote counts and score boosts from
// the kNN result set.  Mirrors
// KNearestNeighborClassifier.buildListFromTopDocs.
func (c *KNearestNeighborClassifier) buildListFromTopDocs(topDocs *search.TopDocs) ([]*ClassificationResult[*util.BytesRef], error) {
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
		if err != nil {
			return nil, err
		}
		if doc == nil {
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
			boost := float64(sd.Score) / float64(maxScore)
			classBoosts[cl] += boost
		}
	}

	var temp []*ClassificationResult[*util.BytesRef]
	var sumdoc int
	for cl, count := range classCounts {
		normBoost := classBoosts[cl] / float64(count)
		temp = append(temp, &ClassificationResult[*util.BytesRef]{
			AssignedClass: util.NewBytesRef([]byte(cl)),
			Score:         float64(count) * normBoost / float64(c.k),
		})
		sumdoc += count
	}

	// Apply correction when total matching docs < k.
	if sumdoc < c.k {
		for i, cr := range temp {
			temp[i] = &ClassificationResult[*util.BytesRef]{
				AssignedClass: cr.AssignedClass,
				Score:         cr.Score * float64(c.k) / float64(sumdoc),
			}
		}
	}
	return temp, nil
}

// KNearestFuzzyClassifier extends KNearestNeighborClassifier with fuzzy
// matching.
//
// Port of org.apache.lucene.classification.KNearestFuzzyClassifier.
//
// In the upstream Java implementation KNearestFuzzyClassifier overrides
// knnSearch to build a fuzzy term query instead of an MLT query.  Gocene
// defers that override and reuses the KNN query path unchanged; the
// distinction has no effect on the public Classifier interface.
type KNearestFuzzyClassifier struct {
	KNearestNeighborClassifier
}

// NewKNearestFuzzyClassifier creates the fuzzy classifier.
func NewKNearestFuzzyClassifier(
	reader interface{},
	analyzer analysis.Analyzer,
	query interface{},
	k int,
	classFieldName string,
	textFieldNames ...string,
) *KNearestFuzzyClassifier {
	return &KNearestFuzzyClassifier{
		KNearestNeighborClassifier: *NewKNearestNeighborClassifier(
			reader, analyzer, query, k, 0, 0, classFieldName, textFieldNames...),
	}
}

// ---- helpers ----------------------------------------------------------------

// splitFieldBoost parses a "fieldName^boost" token, returning the plain field
// name and the numeric boost (0 if absent).
func splitFieldBoost(fieldName string) (string, float32) {
	if idx := strings.Index(fieldName, "^"); idx >= 0 {
		plain := fieldName[:idx]
		var b float32
		if n, _ := parseFloat32(fieldName[idx+1:]); n > 0 {
			b = n
		}
		return plain, b
	}
	return fieldName, 0
}

// plainFieldNames strips boost decorators from all field names.
func plainFieldNames(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		plain, _ := splitFieldBoost(n)
		out[i] = plain
	}
	return out
}

// parseFloat32 is a tiny helper to avoid importing strconv at package scope.
func parseFloat32(s string) (float32, bool) {
	var f float64
	n, err := fmt.Sscanf(s, "%f", &f)
	return float32(f), n == 1 && err == nil
}
