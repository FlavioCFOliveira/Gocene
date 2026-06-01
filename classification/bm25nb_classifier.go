// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classification

import (
	"math"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BM25NBClassifier approximates Naive Bayes classification by using BM25
// scoring rather than raw term frequencies.  Each P(w|c) is estimated as
// the BM25 score of a TermQuery for word w inside the class-restricted
// document set, which provides a more accurate relevance signal than simple
// TF counting.
//
// Port of org.apache.lucene.classification.BM25NBClassifier.
type BM25NBClassifier struct {
	reader         termsProvider
	searcher       *search.IndexSearcher
	analyzer       analysis.Analyzer
	query          search.Query
	classFieldName string
	textFieldNames []string
}

// NewBM25NBClassifier creates the classifier.
//
//   - reader: an index.IndexReaderInterface over the training index.
//   - analyzer: the Analyzer used to tokenize text.
//   - query: an optional filter query (pass nil to use all indexed docs).
//   - classFieldName: the field that holds the class label.
//   - textFieldNames: the text fields used for classification.
func NewBM25NBClassifier(
	reader interface{},
	analyzer analysis.Analyzer,
	query interface{},
	classFieldName string,
	textFieldNames ...string,
) *BM25NBClassifier {
	c := &BM25NBClassifier{
		analyzer:       analyzer,
		classFieldName: classFieldName,
		textFieldNames: textFieldNames,
	}
	if tp, ok := reader.(termsProvider); ok {
		c.reader = tp
	}
	if ri, ok := reader.(index.IndexReaderInterface); ok {
		searcher := search.NewIndexSearcher(ri)
		searcher.SetSimilarity(search.NewBM25Similarity())
		c.searcher = searcher
	}
	if q, ok := query.(search.Query); ok {
		c.query = q
	}
	return c
}

// AssignClass returns the class with the highest posterior log-probability.
func (c *BM25NBClassifier) AssignClass(inputDocument string) (*ClassificationResult[*util.BytesRef], error) {
	list, err := c.assignClassNormalizedList(inputDocument)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

// GetClasses returns all classes sorted by descending score.
func (c *BM25NBClassifier) GetClasses(text string) ([]*ClassificationResult[*util.BytesRef], error) {
	list, err := c.assignClassNormalizedList(text)
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Score > list[j].Score })
	return list, nil
}

// GetClassesMax returns the top max classes sorted by descending score.
func (c *BM25NBClassifier) GetClassesMax(text string, max int) ([]*ClassificationResult[*util.BytesRef], error) {
	list, err := c.GetClasses(text)
	if err != nil {
		return nil, err
	}
	if max > len(list) {
		max = len(list)
	}
	return list[:max], nil
}

// assignClassNormalizedList computes the posterior score for every known class
// and returns the softmax-normalised result list.
func (c *BM25NBClassifier) assignClassNormalizedList(inputDocument string) ([]*ClassificationResult[*util.BytesRef], error) {
	if c.reader == nil || c.searcher == nil || c.analyzer == nil {
		return nil, nil
	}

	classes, err := c.reader.Terms(c.classFieldName)
	if err != nil || classes == nil {
		return nil, err
	}

	tokens, err := c.tokenizeBM25(inputDocument)
	if err != nil {
		return nil, err
	}

	it, err := classes.GetIterator()
	if err != nil {
		return nil, err
	}

	var assignedClasses []*ClassificationResult[*util.BytesRef]
	for {
		next, err := it.Next()
		if err != nil {
			return nil, err
		}
		if next == nil {
			break
		}
		if len(next.Text()) == 0 {
			continue
		}
		term := index.NewTerm(c.classFieldName, next.Text())
		logPrior, err := c.calculateLogPriorBM25(term)
		if err != nil {
			return nil, err
		}
		logLikelihood, err := c.calculateLogLikelihoodBM25(tokens, term)
		if err != nil {
			return nil, err
		}
		assignedClasses = append(assignedClasses, &ClassificationResult[*util.BytesRef]{
			AssignedClass: util.NewBytesRef([]byte(next.Text())),
			Score:         logPrior + logLikelihood,
		})
	}

	return normClassificationResults(assignedClasses), nil
}

// tokenizeBM25 tokenises text across all configured text fields and returns
// the merged token list.
func (c *BM25NBClassifier) tokenizeBM25(text string) ([]string, error) {
	var result []string
	for _, fieldName := range c.textFieldNames {
		ts, err := c.analyzer.TokenStream(fieldName, strings.NewReader(text))
		if err != nil {
			return nil, err
		}
		src := ts.(interface {
			GetAttributeSource() *util.AttributeSource
		}).GetAttributeSource()
		var termAttr analysis.CharTermAttribute
		if raw := src.GetAttribute(analysis.CharTermAttributeType); raw != nil {
			termAttr, _ = raw.(analysis.CharTermAttribute)
		}
		for {
			ok, err := ts.IncrementToken()
			if err != nil {
				_ = ts.End()
				_ = ts.Close()
				return nil, err
			}
			if !ok {
				break
			}
			if termAttr != nil {
				result = append(result, termAttr.String())
			}
		}
		_ = ts.End()
		if err := ts.Close(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// calculateLogLikelihoodBM25 computes Σ log P(w|c) where P(w|c) is
// approximated by the BM25 score of w against the class-filtered corpus.
// If a word has no score > 0 in the class, P(w|c) defaults to 1 (log = 0).
func (c *BM25NBClassifier) calculateLogLikelihoodBM25(tokens []string, classTerm *index.Term) (float64, error) {
	var result float64
	for _, word := range tokens {
		prob, err := c.getTermProbForClass(classTerm, word)
		if err != nil {
			return 0, err
		}
		result += math.Log(prob)
	}
	return result, nil
}

// getTermProbForClass returns the BM25 score of (classTerm ∧ word-in-text)
// as a probability estimate.  Returns 1.0 (neutral log contribution) when no
// document matches, mirroring Lucene's BM25NBClassifier.getTermProbForClass.
func (c *BM25NBClassifier) getTermProbForClass(classTerm *index.Term, word string) (float64, error) {
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(classTerm), search.MUST)
	subQ := search.NewBooleanQuery()
	for _, fieldName := range c.textFieldNames {
		subQ.Add(search.NewTermQuery(index.NewTerm(fieldName, word)), search.SHOULD)
	}
	bq.Add(subQ, search.SHOULD)
	if c.query != nil {
		bq.Add(c.query, search.MUST)
	}
	topDocs, err := c.searcher.Search(bq, 1)
	if err != nil {
		return 0, err
	}
	if topDocs.TotalHits.Value > 0 && len(topDocs.ScoreDocs) > 0 {
		return float64(topDocs.ScoreDocs[0].Score), nil
	}
	return 1.0, nil
}

// calculateLogPriorBM25 computes log P(c) from the BM25 score of the class
// term query.  Falls back to 0 (log 1) when the class term has no hits.
func (c *BM25NBClassifier) calculateLogPriorBM25(classTerm *index.Term) (float64, error) {
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(classTerm), search.MUST)
	if c.query != nil {
		bq.Add(c.query, search.MUST)
	}
	topDocs, err := c.searcher.Search(bq, 1)
	if err != nil {
		return 0, err
	}
	if topDocs.TotalHits.Value > 0 && len(topDocs.ScoreDocs) > 0 {
		return math.Log(float64(topDocs.ScoreDocs[0].Score)), nil
	}
	return 0, nil
}
