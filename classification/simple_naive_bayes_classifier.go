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

// termsProvider is satisfied by any reader that exposes Terms(field) — both
// DirectoryReader (StandardDirectoryReader) and LeafReader implement it.
type termsProvider interface {
	Terms(field string) (index.Terms, error)
}

// countQuery executes query and returns the total hit count.
// It is the Go equivalent of IndexSearcher.count(query) from Lucene.
func countQuery(searcher *search.IndexSearcher, query search.Query) (int, error) {
	topDocs, err := searcher.Search(query, 1)
	if err != nil {
		return 0, err
	}
	return int(topDocs.TotalHits.Value), nil
}

// docFreqForTerm returns the number of documents that contain term in the
// index accessible via reader.  It is the Go equivalent of
// IndexReader.docFreq(term) from Lucene.
func docFreqForTerm(reader termsProvider, term *index.Term) (int, error) {
	terms, err := reader.Terms(term.Field)
	if err != nil {
		return 0, err
	}
	if terms == nil {
		return 0, nil
	}
	it, err := terms.GetIterator()
	if err != nil {
		return 0, err
	}
	found, err := it.SeekExact(term)
	if err != nil || !found {
		return 0, err
	}
	return it.DocFreq()
}

// tokenize runs text through the analyzer for each text field and returns all
// resulting token strings.  Mirrors SimpleNaiveBayesClassifier.tokenize.
func tokenize(analyzer analysis.Analyzer, textFieldNames []string, text string) ([]string, error) {
	var result []string
	for _, fieldName := range textFieldNames {
		ts, err := analyzer.TokenStream(fieldName, strings.NewReader(text))
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
		if err := ts.End(); err != nil {
			_ = ts.Close()
			return nil, err
		}
		if err := ts.Close(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// normClassificationResults normalises a raw log-space result list to [0,1]
// using the log-sum-exp trick.  Mirrors
// SimpleNaiveBayesClassifier.normClassificationResults.
func normClassificationResults(list []*ClassificationResult[*util.BytesRef]) []*ClassificationResult[*util.BytesRef] {
	if len(list) == 0 {
		return list
	}
	// Sort descending so list[0] has the largest (least-negative) log score.
	sort.Slice(list, func(i, j int) bool { return list[i].Score > list[j].Score })
	smax := list[0].Score
	var sumLog float64
	for _, cr := range list {
		sumLog += math.Exp(cr.Score - smax)
	}
	loga := smax + math.Log(sumLog)
	out := make([]*ClassificationResult[*util.BytesRef], len(list))
	for i, cr := range list {
		out[i] = &ClassificationResult[*util.BytesRef]{
			AssignedClass: cr.AssignedClass,
			Score:         math.Exp(cr.Score - loga),
		}
	}
	return out
}

// SimpleNaiveBayesClassifier classifies text using a Naive Bayes model
// backed by a Lucene index.
//
// Port of org.apache.lucene.classification.SimpleNaiveBayesClassifier.
//
// The algorithm computes, for each known class c:
//
//	log P(c|d) ∝ log P(c) + Σ_w log P(w|c)
//
// where P(c) = docs_with_c / total_docs_with_class and P(w|c) uses
// Laplace (add-1) smoothing.  Scores are then softmax-normalised to [0,1].
type SimpleNaiveBayesClassifier struct {
	reader         termsProvider
	searcher       *search.IndexSearcher
	analyzer       analysis.Analyzer
	query          search.Query
	classFieldName string
	textFieldNames []string
}

// NewSimpleNaiveBayesClassifier creates the classifier.
//
//   - reader: an index.IndexReaderInterface (or any termsProvider) over the
//     training index.
//   - analyzer: the Analyzer used to tokenize text.
//   - query: an optional filter query (pass nil to use all indexed docs).
//   - classFieldName: the field that holds the class label.
//   - textFieldNames: the text fields used for classification.
func NewSimpleNaiveBayesClassifier(
	reader interface{},
	analyzer analysis.Analyzer,
	query interface{},
	classFieldName string,
	textFieldNames ...string,
) *SimpleNaiveBayesClassifier {
	c := &SimpleNaiveBayesClassifier{
		analyzer:       analyzer,
		classFieldName: classFieldName,
		textFieldNames: textFieldNames,
	}
	if tp, ok := reader.(termsProvider); ok {
		c.reader = tp
	}
	if ri, ok := reader.(index.IndexReaderInterface); ok {
		c.searcher = search.NewIndexSearcher(ri)
	}
	if q, ok := query.(search.Query); ok {
		c.query = q
	}
	return c
}

// AssignClass assigns the most-probable class to inputDocument.
func (c *SimpleNaiveBayesClassifier) AssignClass(inputDocument string) (*ClassificationResult[*util.BytesRef], error) {
	list, err := c.assignClassNormalizedList(inputDocument)
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

// GetClasses returns all classes sorted by descending score.
func (c *SimpleNaiveBayesClassifier) GetClasses(text string) ([]*ClassificationResult[*util.BytesRef], error) {
	list, err := c.assignClassNormalizedList(text)
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Score > list[j].Score })
	return list, nil
}

// GetClassesMax returns the top max classes sorted by descending score.
func (c *SimpleNaiveBayesClassifier) GetClassesMax(text string, max int) ([]*ClassificationResult[*util.BytesRef], error) {
	list, err := c.GetClasses(text)
	if err != nil {
		return nil, err
	}
	if max > len(list) {
		max = len(list)
	}
	return list[:max], nil
}

// assignClassNormalizedList computes the posterior probability for every
// known class and returns the normalised results.
func (c *SimpleNaiveBayesClassifier) assignClassNormalizedList(inputDocument string) ([]*ClassificationResult[*util.BytesRef], error) {
	if c.reader == nil || c.searcher == nil || c.analyzer == nil {
		return nil, nil
	}

	classes, err := c.reader.Terms(c.classFieldName)
	if err != nil || classes == nil {
		return nil, err
	}

	tokenizedText, err := tokenize(c.analyzer, c.textFieldNames, inputDocument)
	if err != nil {
		return nil, err
	}

	docsWithClassSize, err := c.countDocsWithClass(classes)
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
		logPrior, err := c.calculateLogPrior(term, docsWithClassSize)
		if err != nil {
			return nil, err
		}
		logLikelihood, err := c.calculateLogLikelihood(tokenizedText, term, docsWithClassSize)
		if err != nil {
			return nil, err
		}
		clVal := logPrior + logLikelihood
		assignedClasses = append(assignedClasses, &ClassificationResult[*util.BytesRef]{
			AssignedClass: util.NewBytesRef([]byte(next.Text())),
			Score:         clVal,
		})
	}

	return normClassificationResults(assignedClasses), nil
}

// countDocsWithClass returns the number of indexed documents that have at
// least one value in the class field.  Mirrors
// SimpleNaiveBayesClassifier.countDocsWithClass.
//
// Primary path: use the per-field docCount stored in the index metadata.
// Fallback A (docCount == -1, Lucene convention for unknown): wildcard search.
// Fallback B (docCount == 0, Gocene block-tree writer limitation for
// DOCS-only index options): sum docFreq across all class terms — valid
// because each document has exactly one class label.
func (c *SimpleNaiveBayesClassifier) countDocsWithClass(classes index.Terms) (int, error) {
	docCount, err := classes.GetDocCount()
	if err != nil {
		return 0, err
	}
	if docCount == -1 {
		// Codec returns -1 (unknown) — fall back to a wildcard search as Java does.
		wq := search.NewWildcardQuery(index.NewTerm(c.classFieldName, "*"))
		bq := search.NewBooleanQuery()
		bq.Add(wq, search.MUST)
		if c.query != nil {
			bq.Add(c.query, search.MUST)
		}
		return countQuery(c.searcher, bq)
	}
	if docCount == 0 {
		// Gocene block-tree writer does not populate docCount for DOCS-only
		// indexed fields; use sumDocFreq as the proxy.  sumDocFreq equals the
		// total number of (doc, term) pairs; for a single-valued class field
		// this equals the number of classified documents.
		sumDocFreq, err := classes.GetSumDocFreq()
		if err != nil {
			return 0, err
		}
		if sumDocFreq > 0 {
			return int(sumDocFreq), nil
		}
		// Last resort: enumerate and sum per-class docFreq values.
		total, err := c.sumClassDocFreqs(classes)
		if err != nil {
			return 0, err
		}
		return total, nil
	}
	return docCount, nil
}

// sumClassDocFreqs iterates every class term and sums their docFreq values.
// This is the last-resort fallback when neither docCount nor sumDocFreq are
// available.
func (c *SimpleNaiveBayesClassifier) sumClassDocFreqs(classes index.Terms) (int, error) {
	it, err := classes.GetIterator()
	if err != nil {
		return 0, err
	}
	total := 0
	for {
		t, err := it.Next()
		if err != nil {
			return 0, err
		}
		if t == nil {
			break
		}
		df, err := it.DocFreq()
		if err != nil {
			return 0, err
		}
		total += df
	}
	return total, nil
}

// calculateLogPrior computes log P(c) = log(docs_with_c / total_docs_with_class).
func (c *SimpleNaiveBayesClassifier) calculateLogPrior(term *index.Term, docsWithClassSize int) (float64, error) {
	df, err := docFreqForTerm(c.reader, term)
	if err != nil {
		return 0, err
	}
	return math.Log(float64(df)) - math.Log(float64(docsWithClassSize)), nil
}

// calculateLogLikelihood computes log P(d|c) = Σ_w log P(w|c) using add-1
// Laplace smoothing.  Mirrors
// SimpleNaiveBayesClassifier.calculateLogLikelihood.
func (c *SimpleNaiveBayesClassifier) calculateLogLikelihood(tokenizedText []string, term *index.Term, docsWithClass int) (float64, error) {
	var result float64
	for _, word := range tokenizedText {
		hits, err := c.getWordFreqForClass(word, term)
		if err != nil {
			return 0, err
		}
		num := float64(hits + 1) // add-1 smoothing
		den, err := c.getTextTermFreqForClass(term)
		if err != nil {
			return 0, err
		}
		den += float64(docsWithClass)
		result += math.Log(num / den)
	}
	return result, nil
}

// getTextTermFreqForClass returns the average number of unique terms across
// all text fields multiplied by the number of docs in class c.
func (c *SimpleNaiveBayesClassifier) getTextTermFreqForClass(term *index.Term) (float64, error) {
	var avgUniqueTerms float64
	for _, fieldName := range c.textFieldNames {
		terms, err := c.reader.Terms(fieldName)
		if err != nil || terms == nil {
			continue
		}
		numPostings, err := terms.GetSumDocFreq()
		if err != nil {
			return 0, err
		}
		docCount, err := terms.GetDocCount()
		if err != nil {
			return 0, err
		}
		if docCount > 0 {
			avgUniqueTerms += float64(numPostings) / float64(docCount)
		}
	}
	docsWithC, err := docFreqForTerm(c.reader, term)
	if err != nil {
		return 0, err
	}
	return avgUniqueTerms * float64(docsWithC), nil
}

// getWordFreqForClass returns the number of documents labelled with term that
// also contain word in any text field.
func (c *SimpleNaiveBayesClassifier) getWordFreqForClass(word string, term *index.Term) (int, error) {
	subQuery := search.NewBooleanQuery()
	for _, fieldName := range c.textFieldNames {
		subQuery.Add(search.NewTermQuery(index.NewTerm(fieldName, word)), search.SHOULD)
	}
	bq := search.NewBooleanQuery()
	bq.Add(subQuery, search.MUST)
	bq.Add(search.NewTermQuery(term), search.MUST)
	if c.query != nil {
		bq.Add(c.query, search.MUST)
	}
	return countQuery(c.searcher, bq)
}
