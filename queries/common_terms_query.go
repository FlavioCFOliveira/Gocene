// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/CommonTermsQuery.java

package queries

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// CommonTermsQuery executes high-frequency terms in an optional sub-query to prevent
// slow queries due to "common" terms like stopwords.
//
// It builds two sub-queries from the added terms: low-frequency terms are added to a
// required boolean clause and high-frequency terms are added to an optional clause.
// The optional clause is only executed if the required "low-frequency" clause matches.
//
// Note: if the query only contains high-frequency terms the query is rewritten into a
// plain conjunction query — all high-frequency terms need to match in order to match a document.
//
// Mirrors org.apache.lucene.queries.CommonTermsQuery.
//
// Deviations from Java:
//   - collectTermStates / TermStates are omitted; term classification is based on
//     per-field DocFreq from the first leaf that contains the term instead of aggregated
//     cross-leaf TermStates. Gocene's TermQuery does not yet accept pre-built TermStates.
//   - newTermQuery is not overridable (Go has no protected methods); use embedding to override.
type CommonTermsQuery struct {
	search.BaseQuery

	terms                     []*index.Term
	maxTermFrequency          float32
	lowFreqOccur              search.Occur
	highFreqOccur             search.Occur
	lowFreqBoost              float32
	highFreqBoost             float32
	lowFreqMinNrShouldMatch   float32
	highFreqMinNrShouldMatch  float32
}

// NewCommonTermsQuery creates a new CommonTermsQuery.
//
// highFreqOccur is the Occur used for high-frequency terms.
// lowFreqOccur is the Occur used for low-frequency terms.
// maxTermFrequency is a value in [0..1) (relative) or >=1 (absolute) representing
// the maximum document-frequency threshold for a term to be considered low-frequency.
//
// Returns an error if highFreqOccur or lowFreqOccur is MUST_NOT.
func NewCommonTermsQuery(highFreqOccur, lowFreqOccur search.Occur, maxTermFrequency float32) (*CommonTermsQuery, error) {
	if highFreqOccur == search.MUST_NOT {
		return nil, fmt.Errorf("highFreqOccur should be MUST or SHOULD but was MUST_NOT")
	}
	if lowFreqOccur == search.MUST_NOT {
		return nil, fmt.Errorf("lowFreqOccur should be MUST or SHOULD but was MUST_NOT")
	}
	return &CommonTermsQuery{
		maxTermFrequency: maxTermFrequency,
		lowFreqOccur:     lowFreqOccur,
		highFreqOccur:    highFreqOccur,
		lowFreqBoost:     1.0,
		highFreqBoost:    1.0,
	}, nil
}

// Add adds a term to the query.
func (q *CommonTermsQuery) Add(term *index.Term) error {
	if term == nil {
		return fmt.Errorf("term must not be nil")
	}
	q.terms = append(q.terms, term)
	return nil
}

// SetLowFreqMinimumNumberShouldMatch specifies a minimum number of the low-frequency
// optional BooleanClauses which must be satisfied. Accepts a float in [0..1) as a
// fraction of the actual query terms, or a number >=1 as an absolute clause count.
func (q *CommonTermsQuery) SetLowFreqMinimumNumberShouldMatch(min float32) {
	q.lowFreqMinNrShouldMatch = min
}

// GetLowFreqMinimumNumberShouldMatch returns the minimum number of optional low-frequency
// BooleanClauses which must be satisfied.
func (q *CommonTermsQuery) GetLowFreqMinimumNumberShouldMatch() float32 {
	return q.lowFreqMinNrShouldMatch
}

// SetHighFreqMinimumNumberShouldMatch specifies a minimum number of the high-frequency
// optional BooleanClauses which must be satisfied.
func (q *CommonTermsQuery) SetHighFreqMinimumNumberShouldMatch(min float32) {
	q.highFreqMinNrShouldMatch = min
}

// GetHighFreqMinimumNumberShouldMatch returns the minimum number of optional high-frequency
// BooleanClauses which must be satisfied.
func (q *CommonTermsQuery) GetHighFreqMinimumNumberShouldMatch() float32 {
	return q.highFreqMinNrShouldMatch
}

// GetTerms returns an unmodifiable view of the terms list.
func (q *CommonTermsQuery) GetTerms() []*index.Term {
	out := make([]*index.Term, len(q.terms))
	copy(out, q.terms)
	return out
}

// GetMaxTermFrequency returns the maximum threshold of a term's document frequency to
// be considered a low-frequency term.
func (q *CommonTermsQuery) GetMaxTermFrequency() float32 { return q.maxTermFrequency }

// GetLowFreqOccur returns the Occur used for low-frequency terms.
func (q *CommonTermsQuery) GetLowFreqOccur() search.Occur { return q.lowFreqOccur }

// GetHighFreqOccur returns the Occur used for high-frequency terms.
func (q *CommonTermsQuery) GetHighFreqOccur() search.Occur { return q.highFreqOccur }

// GetLowFreqBoost returns the boost used for low-frequency terms.
func (q *CommonTermsQuery) GetLowFreqBoost() float32 { return q.lowFreqBoost }

// GetHighFreqBoost returns the boost used for high-frequency terms.
func (q *CommonTermsQuery) GetHighFreqBoost() float32 { return q.highFreqBoost }

// SetLowFreqBoost sets the boost applied to the low-frequency sub-query.
func (q *CommonTermsQuery) SetLowFreqBoost(boost float32) { q.lowFreqBoost = boost }

// SetHighFreqBoost sets the boost applied to the high-frequency sub-query.
func (q *CommonTermsQuery) SetHighFreqBoost(boost float32) { q.highFreqBoost = boost }

// Clone returns a shallow copy of this query.
func (q *CommonTermsQuery) Clone() search.Query {
	c := *q
	c.terms = make([]*index.Term, len(q.terms))
	copy(c.terms, q.terms)
	return &c
}

// Equals reports structural equality.
func (q *CommonTermsQuery) Equals(other search.Query) bool {
	o, ok := other.(*CommonTermsQuery)
	if !ok {
		return false
	}
	if math.Float32bits(q.highFreqBoost) != math.Float32bits(o.highFreqBoost) {
		return false
	}
	if q.highFreqOccur != o.highFreqOccur {
		return false
	}
	if q.lowFreqOccur != o.lowFreqOccur {
		return false
	}
	if math.Float32bits(q.lowFreqBoost) != math.Float32bits(o.lowFreqBoost) {
		return false
	}
	if math.Float32bits(q.maxTermFrequency) != math.Float32bits(o.maxTermFrequency) {
		return false
	}
	if q.lowFreqMinNrShouldMatch != o.lowFreqMinNrShouldMatch {
		return false
	}
	if q.highFreqMinNrShouldMatch != o.highFreqMinNrShouldMatch {
		return false
	}
	if len(q.terms) != len(o.terms) {
		return false
	}
	for i, t := range q.terms {
		if !t.Equals(o.terms[i]) {
			return false
		}
	}
	return true
}

// HashCode returns a hash code for this query.
func (q *CommonTermsQuery) HashCode() int {
	const prime = 31
	result := 1
	result = prime*result + int(math.Float32bits(q.highFreqBoost))
	result = prime*result + int(q.highFreqOccur)
	result = prime*result + int(q.lowFreqOccur)
	result = prime*result + int(math.Float32bits(q.lowFreqBoost))
	result = prime*result + int(math.Float32bits(q.maxTermFrequency))
	result = prime*result + int(math.Float32bits(q.lowFreqMinNrShouldMatch))
	result = prime*result + int(math.Float32bits(q.highFreqMinNrShouldMatch))
	for _, t := range q.terms {
		result = prime*result + t.HashCode()
	}
	return result
}

// String returns a human-readable representation of the query.
func (q *CommonTermsQuery) String() string {
	buf := ""
	needParens := q.GetLowFreqMinimumNumberShouldMatch() > 0
	if needParens {
		buf += "("
	}
	for i, t := range q.terms {
		buf += search.NewTermQuery(t).String()
		if i != len(q.terms)-1 {
			buf += ", "
		}
	}
	if needParens {
		buf += ")"
	}
	if q.GetLowFreqMinimumNumberShouldMatch() > 0 || q.GetHighFreqMinimumNumberShouldMatch() > 0 {
		buf += "~("
		buf += fmt.Sprintf("%v", q.GetLowFreqMinimumNumberShouldMatch())
		buf += fmt.Sprintf("%v", q.GetHighFreqMinimumNumberShouldMatch())
		buf += ")"
	}
	return buf
}

// Rewrite rewrites the query by classifying terms as high/low frequency and building
// the appropriate boolean query.
func (q *CommonTermsQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	if len(q.terms) == 0 {
		return search.NewMatchNoDocsQueryWithReason("CommonTermsQuery with no terms"), nil
	}
	if len(q.terms) == 1 {
		return search.NewTermQuery(q.terms[0]), nil
	}

	// Collect doc frequencies per term via the leaf readers.
	ireader, ok := reader.(interface {
		Leaves() ([]*index.LeafReaderContext, error)
	})
	if !ok {
		return nil, fmt.Errorf("CommonTermsQuery.Rewrite: reader does not implement Leaves()")
	}
	maxDoc := reader.MaxDoc()
	leaves, err := ireader.Leaves()
	if err != nil {
		return nil, err
	}

	// docFreqs[i] aggregates docFreq for terms[i] across leaves.
	docFreqs := make([]int, len(q.terms))
	for _, lrc := range leaves {
		for i, term := range q.terms {
			terms, err := lrc.LeafReader().Terms(term.Field)
			if err != nil {
				return nil, err
			}
			if terms == nil {
				continue
			}
			te, err := terms.GetIterator()
			if err != nil {
				return nil, err
			}
			found, err := te.SeekExact(term)
			if err != nil {
				return nil, err
			}
			if !found {
				continue
			}
			df, err := te.DocFreq()
			if err != nil {
				return nil, err
			}
			docFreqs[i] += df
		}
	}

	return q.buildQuery(maxDoc, docFreqs)
}

// CreateWeight delegates to the rewritten query for weight creation.
func (q *CommonTermsQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	rewritten, err := q.Rewrite(searcher.GetIndexReader())
	if err != nil {
		return nil, err
	}
	return rewritten.CreateWeight(searcher, needsScores, boost)
}

// buildQuery constructs the final BooleanQuery from classified term lists.
func (q *CommonTermsQuery) buildQuery(maxDoc int, docFreqs []int) (search.Query, error) {
	var lowFreqQueries []search.Query
	var highFreqQueries []search.Query

	for i, term := range q.terms {
		df := docFreqs[i]
		if df == 0 {
			// term not found in index → treat as low-frequency.
			lowFreqQueries = append(lowFreqQueries, search.NewTermQuery(term))
			continue
		}
		threshold := int(math.Ceil(float64(q.maxTermFrequency) * float64(maxDoc)))
		if (q.maxTermFrequency >= 1.0 && float32(df) > q.maxTermFrequency) ||
			(q.maxTermFrequency < 1.0 && df > threshold) {
			highFreqQueries = append(highFreqQueries, search.NewTermQuery(term))
		} else {
			lowFreqQueries = append(lowFreqQueries, search.NewTermQuery(term))
		}
	}

	numLowFreq := len(lowFreqQueries)
	numHighFreq := len(highFreqQueries)
	lowFreqOccur := q.lowFreqOccur
	highFreqOccur := q.highFreqOccur
	lowFreqMinShouldMatch := 0
	highFreqMinShouldMatch := 0

	if lowFreqOccur == search.SHOULD && numLowFreq > 0 {
		lowFreqMinShouldMatch = q.calcLowFreqMinimumNumberShouldMatch(numLowFreq)
	}
	if highFreqOccur == search.SHOULD && numHighFreq > 0 {
		highFreqMinShouldMatch = q.calcHighFreqMinimumNumberShouldMatch(numHighFreq)
	}

	if len(lowFreqQueries) == 0 {
		// Rewrite high-freq terms as a conjunction to prevent slow queries.
		if highFreqMinShouldMatch == 0 && highFreqOccur != search.MUST {
			highFreqOccur = search.MUST
		}
	}

	builder := search.NewBooleanQuery()

	if len(lowFreqQueries) > 0 {
		lowFreq := search.NewBooleanQuery()
		for _, query := range lowFreqQueries {
			lowFreq.Add(query, lowFreqOccur)
		}
		lowFreq.SetMinimumNumberShouldMatch(lowFreqMinShouldMatch)
		builder.Add(search.NewBoostQuery(lowFreq, q.lowFreqBoost), search.MUST)
	}
	if len(highFreqQueries) > 0 {
		highFreq := search.NewBooleanQuery()
		for _, query := range highFreqQueries {
			highFreq.Add(query, highFreqOccur)
		}
		highFreq.SetMinimumNumberShouldMatch(highFreqMinShouldMatch)
		builder.Add(search.NewBoostQuery(highFreq, q.highFreqBoost), search.SHOULD)
	}

	return builder, nil
}

func (q *CommonTermsQuery) calcLowFreqMinimumNumberShouldMatch(numOptional int) int {
	return minNrShouldMatch(q.lowFreqMinNrShouldMatch, numOptional)
}

func (q *CommonTermsQuery) calcHighFreqMinimumNumberShouldMatch(numOptional int) int {
	return minNrShouldMatch(q.highFreqMinNrShouldMatch, numOptional)
}

func minNrShouldMatch(minNrShouldMatch float32, numOptional int) int {
	if minNrShouldMatch >= 1.0 || minNrShouldMatch == 0.0 {
		return int(minNrShouldMatch)
	}
	return int(math.Round(float64(minNrShouldMatch) * float64(numOptional)))
}
