// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"math"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/classification"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// termsProvider is the same duck-typed interface used in the base
// classification package (copied here to avoid a cross-package dependency on
// an unexported type).
type termsProvider interface {
	Terms(field string) (index.Terms, error)
}

// SimpleNaiveBayesDocumentClassifier classifies documents using a Naive Bayes
// model backed by a Lucene index.  It extends SimpleNaiveBayesClassifier to
// operate on Document objects.
//
// Port of org.apache.lucene.classification.document.SimpleNaiveBayesDocumentClassifier.
type SimpleNaiveBayesDocumentClassifier struct {
	reader         termsProvider
	searcher       *search.IndexSearcher
	query          search.Query
	classFieldName string
	textFieldNames []string
	field2analyzer map[string]analysis.Analyzer
}

// NewSimpleNaiveBayesDocumentClassifier creates the document classifier.
//
//   - reader: an index.IndexReaderInterface over the training index.
//   - query: an optional filter query (pass nil for no filter).
//   - classFieldName: the field holding the class label.
//   - field2analyzer: per-field analyzers for document field tokenization.
//   - textFieldNames: the text fields used for classification.
func NewSimpleNaiveBayesDocumentClassifier(
	reader interface{},
	query interface{},
	classFieldName string,
	field2analyzer map[string]analysis.Analyzer,
	textFieldNames ...string,
) *SimpleNaiveBayesDocumentClassifier {
	c := &SimpleNaiveBayesDocumentClassifier{
		classFieldName: classFieldName,
		textFieldNames: textFieldNames,
		field2analyzer: field2analyzer,
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

// AssignClass assigns the most-probable class to the given document.
func (c *SimpleNaiveBayesDocumentClassifier) AssignClass(doc interface{}) (*classification.ClassificationResult[*util.BytesRef], error) {
	d, ok := doc.(*document.Document)
	if !ok || d == nil || c.reader == nil {
		return nil, nil
	}
	list, err := c.assignNormClasses(d)
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

// GetClasses returns all classes sorted by descending score.
func (c *SimpleNaiveBayesDocumentClassifier) GetClasses(doc interface{}) ([]*classification.ClassificationResult[*util.BytesRef], error) {
	d, ok := doc.(*document.Document)
	if !ok || d == nil || c.reader == nil {
		return nil, nil
	}
	list, err := c.assignNormClasses(d)
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Score > list[j].Score })
	return list, nil
}

// GetClassesMax returns the top max classes sorted by descending score.
func (c *SimpleNaiveBayesDocumentClassifier) GetClassesMax(doc interface{}, max int) ([]*classification.ClassificationResult[*util.BytesRef], error) {
	list, err := c.GetClasses(doc)
	if err != nil {
		return nil, err
	}
	if max > len(list) {
		max = len(list)
	}
	return list[:max], nil
}

// assignNormClasses computes posterior log-probabilities for every class and
// returns the softmax-normalised results.
func (c *SimpleNaiveBayesDocumentClassifier) assignNormClasses(inputDocument *document.Document) ([]*classification.ClassificationResult[*util.BytesRef], error) {
	classes, err := c.reader.Terms(c.classFieldName)
	if err != nil || classes == nil {
		return nil, err
	}

	// Analyse the seed document: for each text field, tokenize every stored
	// value and collect the results alongside a per-field boost hint.
	fieldName2tokens := make(map[string][][]string)  // field → list-of-token-arrays
	fieldName2boost := make(map[string]float32)

	for i := 0; i < len(c.textFieldNames); i++ {
		rawName := c.textFieldNames[i]
		plain, boost := splitFieldBoost(rawName)
		if boost == 0 {
			boost = 1
		}
		c.textFieldNames[i] = plain // strip boost for downstream searches

		analyzer := c.field2analyzer[plain]
		if analyzer == nil {
			continue
		}
		var tokenizedValues [][]string
		for _, fieldValue := range inputDocument.GetFieldsByName(plain) {
			if fieldValue == nil {
				continue
			}
			tokens, err := tokenizeField(analyzer, plain, fieldValue.StringValue())
			if err != nil {
				return nil, err
			}
			tokenizedValues = append(tokenizedValues, tokens)
		}
		fieldName2tokens[plain] = tokenizedValues
		fieldName2boost[plain] = boost
	}

	docsWithClass, err := countDocsWithClass(c.reader, c.searcher, c.classFieldName, c.query)
	if err != nil {
		return nil, err
	}

	it, err := classes.GetIterator()
	if err != nil {
		return nil, err
	}

	var assignedClasses []*classification.ClassificationResult[*util.BytesRef]
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
		classScore := 0.0
		for _, fieldName := range c.textFieldNames {
			tokensArrays := fieldName2tokens[fieldName]
			fieldScore := 0.0
			for _, tokensArray := range tokensArrays {
				logPrior, err := c.calculateLogPrior(term, docsWithClass)
				if err != nil {
					return nil, err
				}
				ll, err := c.calculateLogLikelihood(tokensArray, fieldName, term, docsWithClass)
				if err != nil {
					return nil, err
				}
				fieldScore += logPrior + ll*float64(fieldName2boost[fieldName])
			}
			classScore += fieldScore
		}
		assignedClasses = append(assignedClasses, &classification.ClassificationResult[*util.BytesRef]{
			AssignedClass: util.NewBytesRef([]byte(next.Text())),
			Score:         classScore,
		})
	}

	return normDocClassResults(assignedClasses), nil
}

// calculateLogPrior computes log P(c).
func (c *SimpleNaiveBayesDocumentClassifier) calculateLogPrior(term *index.Term, docsWithClassSize int) (float64, error) {
	df, err := docFreqForTerm(c.reader, term)
	if err != nil {
		return 0, err
	}
	return math.Log(float64(df)) - math.Log(float64(docsWithClassSize)), nil
}

// calculateLogLikelihood computes Σ log P(w|c) for a specific field.
func (c *SimpleNaiveBayesDocumentClassifier) calculateLogLikelihood(tokens []string, fieldName string, term *index.Term, docsWithClass int) (float64, error) {
	var result float64
	for _, word := range tokens {
		hits, err := c.getWordFreqForClass(word, fieldName, term)
		if err != nil {
			return 0, err
		}
		num := float64(hits + 1)
		den, err := c.getTextTermFreqForClass(term, fieldName)
		if err != nil {
			return 0, err
		}
		den += float64(docsWithClass)
		result += math.Log(num / den)
	}
	if len(tokens) > 0 {
		result /= float64(len(tokens))
	}
	return result, nil
}

// getTextTermFreqForClass returns the average unique-term-per-doc count times
// the number of docs in class c, for the specified field.
func (c *SimpleNaiveBayesDocumentClassifier) getTextTermFreqForClass(term *index.Term, fieldName string) (float64, error) {
	terms, err := c.reader.Terms(fieldName)
	if err != nil || terms == nil {
		return 0, err
	}
	numPostings, err := terms.GetSumDocFreq()
	if err != nil {
		return 0, err
	}
	docCount, err := terms.GetDocCount()
	if err != nil {
		return 0, err
	}
	if docCount == 0 {
		return 0, nil
	}
	avgUniqueTerms := float64(numPostings) / float64(docCount)
	docsWithC, err := docFreqForTerm(c.reader, term)
	if err != nil {
		return 0, err
	}
	return avgUniqueTerms * float64(docsWithC), nil
}

// getWordFreqForClass counts docs labelled term that contain word in fieldName.
func (c *SimpleNaiveBayesDocumentClassifier) getWordFreqForClass(word, fieldName string, term *index.Term) (int, error) {
	subQ := search.NewBooleanQuery()
	subQ.Add(search.NewTermQuery(index.NewTerm(fieldName, word)), search.SHOULD)
	bq := search.NewBooleanQuery()
	bq.Add(subQ, search.MUST)
	bq.Add(search.NewTermQuery(term), search.MUST)
	if c.query != nil {
		bq.Add(c.query, search.MUST)
	}
	return countQuery(c.searcher, bq)
}

// ---- local helpers ----------------------------------------------------------

// countQuery is the count(query) equivalent.
func countQuery(searcher *search.IndexSearcher, q search.Query) (int, error) {
	top, err := searcher.Search(q, 1)
	if err != nil {
		return 0, err
	}
	return int(top.TotalHits.Value), nil
}

// docFreqForTerm looks up the per-term document-frequency via the terms API.
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

// countDocsWithClass returns the number of docs that have at least one value
// in the class field.
//
// Primary path: the per-field docCount stored in the index metadata.
// Fallback A (docCount == -1): wildcard search (Lucene convention).
// Fallback B (docCount == 0): sumDocFreq or enumerated per-class sum
// (Gocene block-tree limitation for DOCS-only indexed fields).
func countDocsWithClass(reader termsProvider, searcher *search.IndexSearcher, classFieldName string, filterQuery search.Query) (int, error) {
	classTerms, err := reader.Terms(classFieldName)
	if err != nil || classTerms == nil {
		return 0, err
	}
	cnt, err := classTerms.GetDocCount()
	if err != nil {
		return 0, err
	}
	if cnt == -1 {
		wq := search.NewWildcardQuery(index.NewTerm(classFieldName, "*"))
		bq := search.NewBooleanQuery()
		bq.Add(wq, search.MUST)
		if filterQuery != nil {
			bq.Add(filterQuery, search.MUST)
		}
		return countQuery(searcher, bq)
	}
	if cnt == 0 {
		// Use sumDocFreq as proxy for a single-valued class field.
		sumDocFreq, err := classTerms.GetSumDocFreq()
		if err != nil {
			return 0, err
		}
		if sumDocFreq > 0 {
			return int(sumDocFreq), nil
		}
		// Last resort: sum per-class docFreq.
		it, err := classTerms.GetIterator()
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
	return cnt, nil
}

// tokenizeField tokenises value using the given analyzer for fieldName and
// returns the collected token strings.
func tokenizeField(analyzer analysis.Analyzer, fieldName, value string) ([]string, error) {
	ts, err := analyzer.TokenStream(fieldName, strings.NewReader(value))
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
	var result []string
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
	_ = ts.Close()
	return result, nil
}

// normDocClassResults normalises log-space scores to [0,1] via log-sum-exp.
func normDocClassResults(list []*classification.ClassificationResult[*util.BytesRef]) []*classification.ClassificationResult[*util.BytesRef] {
	if len(list) == 0 {
		return list
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Score > list[j].Score })
	smax := list[0].Score
	var sumLog float64
	for _, cr := range list {
		sumLog += math.Exp(cr.Score - smax)
	}
	loga := smax + math.Log(sumLog)
	out := make([]*classification.ClassificationResult[*util.BytesRef], len(list))
	for i, cr := range list {
		out[i] = &classification.ClassificationResult[*util.BytesRef]{
			AssignedClass: cr.AssignedClass,
			Score:         math.Exp(cr.Score - loga),
		}
	}
	return out
}
