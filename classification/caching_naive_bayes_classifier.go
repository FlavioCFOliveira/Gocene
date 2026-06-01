// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classification

import (
	"math"
	"sort"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// CachingNaiveBayesClassifier extends SimpleNaiveBayesClassifier by caching
// per-word, per-class hit counts so that repeated classifications over the
// same vocabulary avoid redundant index searches.
//
// Port of org.apache.lucene.classification.CachingNaiveBayesClassifier.
//
// This is NOT an online classifier: the cache is built once at construction
// time (via reInitCache) and is not updated when new documents are indexed.
type CachingNaiveBayesClassifier struct {
	SimpleNaiveBayesClassifier

	// mu guards cclasses, termCClassHitCache and classTermFreq, which are
	// populated once during reInitCache but may be read concurrently.
	mu sync.RWMutex

	// cclasses is the ordered list of all class BytesRef values found in
	// the index.
	cclasses []*util.BytesRef

	// termCClassHitCache maps a word (token) to a map from class BytesRef
	// to the hit count (docs in that class containing the word).  An entry
	// with a nil inner map means the word is not cached; a non-nil (possibly
	// empty) inner map is the cached answer.
	termCClassHitCache map[string]map[string]int

	// classTermFreq maps a class BytesRef string to the pre-computed
	// denominator: avg unique terms per doc × docs with that class.
	classTermFreq map[string]float64

	// justCachedTerms controls whether words absent from the skeleton cache
	// are looked up live (false) or simply ignored (true).
	justCachedTerms bool

	// docsWithClassSize is cached at init time by countDocsWithClass.
	docsWithClassSize int
}

// NewCachingNaiveBayesClassifier creates the caching classifier and eagerly
// populates the cache from the index.
//
// A cache-init failure (e.g. the index is empty or the reader is nil) is
// silently swallowed and results in a classifier whose methods return nil,
// matching the behaviour of a not-yet-trained classifier.
func NewCachingNaiveBayesClassifier(
	reader interface{},
	analyzer analysis.Analyzer,
	query interface{},
	classFieldName string,
	textFieldNames ...string,
) *CachingNaiveBayesClassifier {
	c := &CachingNaiveBayesClassifier{
		SimpleNaiveBayesClassifier: *NewSimpleNaiveBayesClassifier(
			reader, analyzer, query, classFieldName, textFieldNames...),
		termCClassHitCache: make(map[string]map[string]int),
		classTermFreq:      make(map[string]float64),
	}
	// Build the cache; ignore errors (nil reader etc.) gracefully.
	_ = c.reInitCache(0, true)
	return c
}

// ReInitCache rebuilds the in-memory term→class hit cache.
//
//   - minTermOccurrenceInCache: terms with a total doc-frequency ≤ this value
//     are excluded from the skeleton.  Use 0 to include every term.
//   - justCachedTerms: when true, words not in the skeleton are skipped
//     entirely during classification; when false they are looked up live in
//     the index (but not stored in the cache).
func (c *CachingNaiveBayesClassifier) ReInitCache(minTermOccurrenceInCache int, justCachedTerms bool) error {
	return c.reInitCache(minTermOccurrenceInCache, justCachedTerms)
}

func (c *CachingNaiveBayesClassifier) reInitCache(minTermOccurrenceInCache int, justCachedTerms bool) error {
	if c.reader == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.justCachedTerms = justCachedTerms

	// Count docs with a class field value.
	classesTerms, err := c.reader.Terms(c.classFieldName)
	if err != nil || classesTerms == nil {
		return err
	}
	docsWithClass, err := c.SimpleNaiveBayesClassifier.countDocsWithClass(classesTerms)
	if err != nil {
		return err
	}
	c.docsWithClassSize = docsWithClass

	// Reset maps.
	c.termCClassHitCache = make(map[string]map[string]int)
	c.cclasses = c.cclasses[:0]
	c.classTermFreq = make(map[string]float64)

	// Build skeleton: collect every term that appears in any text field with
	// a doc-frequency above the threshold.
	freqMap := make(map[string]int64)
	for _, fieldName := range c.textFieldNames {
		terms, err := c.reader.Terms(fieldName)
		if err != nil || terms == nil {
			continue
		}
		it, err := terms.GetIterator()
		if err != nil {
			return err
		}
		for {
			t, err := it.Next()
			if err != nil {
				return err
			}
			if t == nil {
				break
			}
			df, err := it.DocFreq()
			if err != nil {
				return err
			}
			freqMap[t.Text()] += int64(df)
		}
	}
	for word, freq := range freqMap {
		if freq > int64(minTermOccurrenceInCache) {
			c.termCClassHitCache[word] = nil // nil = slot reserved, not yet populated
		}
	}

	// Enumerate all class values.
	classIt, err := classesTerms.GetIterator()
	if err != nil {
		return err
	}
	for {
		t, err := classIt.Next()
		if err != nil {
			return err
		}
		if t == nil {
			break
		}
		c.cclasses = append(c.cclasses, util.NewBytesRef([]byte(t.Text())))
	}

	// Pre-compute classTermFreq for each class.
	for _, cclass := range c.cclasses {
		var avgUniqueTerms float64
		for _, fieldName := range c.textFieldNames {
			terms, err := c.reader.Terms(fieldName)
			if err != nil || terms == nil {
				continue
			}
			numPostings, err := terms.GetSumDocFreq()
			if err != nil {
				return err
			}
			docCount, err := terms.GetDocCount()
			if err != nil {
				return err
			}
			if docCount > 0 {
				avgUniqueTerms += float64(numPostings) / float64(docCount)
			}
		}
		classTerm := index.NewTerm(c.classFieldName, cclass.String())
		docsWithC, err := docFreqForTerm(c.reader, classTerm)
		if err != nil {
			return err
		}
		c.classTermFreq[cclass.String()] = avgUniqueTerms * float64(docsWithC)
	}

	return nil
}

// AssignClass assigns the most-probable class to inputDocument.
func (c *CachingNaiveBayesClassifier) AssignClass(inputDocument string) (*ClassificationResult[*util.BytesRef], error) {
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
func (c *CachingNaiveBayesClassifier) GetClasses(text string) ([]*ClassificationResult[*util.BytesRef], error) {
	list, err := c.assignClassNormalizedList(text)
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Score > list[j].Score })
	return list, nil
}

// GetClassesMax returns the top max classes sorted by descending score.
func (c *CachingNaiveBayesClassifier) GetClassesMax(text string, max int) ([]*ClassificationResult[*util.BytesRef], error) {
	list, err := c.GetClasses(text)
	if err != nil {
		return nil, err
	}
	if max > len(list) {
		max = len(list)
	}
	return list[:max], nil
}

// assignClassNormalizedList computes posterior probabilities using the
// pre-computed cache and returns the normalised result list.
func (c *CachingNaiveBayesClassifier) assignClassNormalizedList(inputDocument string) ([]*ClassificationResult[*util.BytesRef], error) {
	if c.reader == nil || c.analyzer == nil {
		return nil, nil
	}

	tokenizedText, err := tokenize(c.analyzer, c.textFieldNames, inputDocument)
	if err != nil {
		return nil, err
	}

	raw, err := c.calculateLogLikelihood(tokenizedText)
	if err != nil {
		return nil, err
	}
	return normClassificationResults(raw), nil
}

// calculateLogLikelihood accumulates Σ log P(w|c) for each class c.
func (c *CachingNaiveBayesClassifier) calculateLogLikelihood(tokenizedText []string) ([]*ClassificationResult[*util.BytesRef], error) {
	c.mu.RLock()
	cclasses := c.cclasses
	docsWithClass := c.docsWithClassSize
	c.mu.RUnlock()

	// Initialise result list with score 0.
	ret := make([]*ClassificationResult[*util.BytesRef], len(cclasses))
	for i, cclass := range cclasses {
		ret[i] = &ClassificationResult[*util.BytesRef]{
			AssignedClass: util.NewBytesRef(cclass.ValidBytes()),
			Score:         0,
		}
	}

	for _, word := range tokenizedText {
		hitsInClasses, err := c.getWordFreqForClasses(word)
		if err != nil {
			return nil, err
		}
		for i, cclass := range cclasses {
			hits := hitsInClasses[cclass.String()]
			num := float64(hits + 1) // add-1 smoothing

			c.mu.RLock()
			den := c.classTermFreq[cclass.String()] + float64(docsWithClass)
			c.mu.RUnlock()

			ret[i] = &ClassificationResult[*util.BytesRef]{
				AssignedClass: ret[i].AssignedClass,
				Score:         ret[i].Score + math.Log(num/den),
			}
		}
	}
	return ret, nil
}

// getWordFreqForClasses returns, for each class, the count of docs in that
// class that contain word.  Results come from the cache when available;
// otherwise (and when justCachedTerms is false) a live index search is run.
func (c *CachingNaiveBayesClassifier) getWordFreqForClasses(word string) (map[string]int, error) {
	c.mu.RLock()
	cached, inSkeleton := c.termCClassHitCache[word]
	justCachedTerms := c.justCachedTerms
	cclasses := c.cclasses
	c.mu.RUnlock()

	// Cache hit (non-nil map means already computed).
	if inSkeleton && cached != nil {
		return cached, nil
	}

	// Word is either in the skeleton (nil = reserved slot) or we allow live
	// lookup regardless of skeleton membership.
	if !inSkeleton && justCachedTerms {
		return map[string]int{}, nil
	}

	// Run live searches for each class.
	searched := make(map[string]int)
	for _, cclass := range cclasses {
		classTerm := index.NewTerm(c.classFieldName, cclass.String())
		subQuery := search.NewBooleanQuery()
		for _, fieldName := range c.textFieldNames {
			subQuery.Add(search.NewTermQuery(index.NewTerm(fieldName, word)), search.SHOULD)
		}
		bq := search.NewBooleanQuery()
		bq.Add(subQuery, search.MUST)
		bq.Add(search.NewTermQuery(classTerm), search.MUST)
		if c.query != nil {
			bq.Add(c.query, search.MUST)
		}
		cnt, err := countQuery(c.searcher, bq)
		if err != nil {
			return nil, err
		}
		if cnt > 0 {
			searched[cclass.String()] = cnt
		}
	}

	// Populate the skeleton slot if the word was in the skeleton.
	if inSkeleton {
		c.mu.Lock()
		c.termCClassHitCache[word] = searched
		c.mu.Unlock()
	}
	return searched, nil
}
