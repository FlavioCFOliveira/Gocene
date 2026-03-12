// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"container/heap"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// MoreLikeThis implements the "more like this" search functionality.
// It finds documents similar to a reference document based on term frequency analysis.
//
// This is the Go port of Lucene's org.apache.lucene.queries.mlt.MoreLikeThis.
type MoreLikeThis struct {
	// MinTermFreq is the minimum term frequency for a term to be considered.
	// Default is 2.
	MinTermFreq int

	// MinDocFreq is the minimum document frequency for a term to be considered.
	// Terms that appear in fewer documents are ignored.
	// Default is 5.
	MinDocFreq int

	// MaxDocFreq is the maximum document frequency for a term to be considered.
	// Terms that appear in more than this percentage of documents are ignored.
	// Default is 95 (95%).
	MaxDocFreq int

	// MaxQueryTerms is the maximum number of query terms to include.
	// Only the most interesting terms are selected.
	// Default is 25.
	MaxQueryTerms int

	// MaxNumTokensParsed is the maximum number of tokens to parse from the input.
	// Default is 5000.
	MaxNumTokensParsed int

	// MinWordLen is the minimum word length for a term to be considered.
	// Default is 0 (no minimum).
	MinWordLen int

	// MaxWordLen is the maximum word length for a term to be considered.
	// Default is 0 (no maximum).
	MaxWordLen int

	// StopWords is a set of words that should be ignored.
	StopWords map[string]bool

	// FieldNames are the field names to use for similarity.
	// If empty, uses all fields.
	FieldNames []string

	// Analyzer is used to analyze the source document text.
	Analyzer analysis.Analyzer
}

// NewMoreLikeThis creates a new MoreLikeThis with default settings.
func NewMoreLikeThis(analyzer analysis.Analyzer) *MoreLikeThis {
	return &MoreLikeThis{
		MinTermFreq:        2,
		MinDocFreq:         5,
		MaxDocFreq:         95,
		MaxQueryTerms:      25,
		MaxNumTokensParsed: 5000,
		MinWordLen:         0,
		MaxWordLen:         0,
		StopWords:          make(map[string]bool),
		Analyzer:           analyzer,
	}
}

// SetStopWords sets the stop words as a slice of strings.
func (mlt *MoreLikeThis) SetStopWords(words []string) {
	mlt.StopWords = make(map[string]bool)
	for _, w := range words {
		mlt.StopWords[strings.ToLower(w)] = true
	}
}

// IsStopWord checks if a word is a stop word.
func (mlt *MoreLikeThis) IsStopWord(word string) bool {
	return mlt.StopWords[strings.ToLower(word)]
}

// termFreq represents term frequency information for a single term.
type termFreq struct {
	term  string
	freq  int
	field string
}

// interestingTerm represents a term with its score for selection.
type interestingTerm struct {
	term  string
	field string
	score float64
}

// interestingTermHeap implements heap.Interface for selecting top terms.
type interestingTermHeap []*interestingTerm

func (h interestingTermHeap) Len() int           { return len(h) }
func (h interestingTermHeap) Less(i, j int) bool { return h[i].score < h[j].score }
func (h interestingTermHeap) Swap(i, j int)    { h[i], h[j] = h[j], h[i] }

func (h *interestingTermHeap) Push(x interface{}) {
	*h = append(*h, x.(*interestingTerm))
}

func (h *interestingTermHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// Like finds documents similar to the given document ID.
func (mlt *MoreLikeThis) Like(reader IndexReader, docID int) (Query, error) {
	if mlt.Analyzer == nil {
		return nil, fmt.Errorf("analyzer is required")
	}

	// Retrieve term vectors for the document
	termFreqs := make(map[string]*termFreq)

	// For each field, retrieve term frequencies
	for _, fieldName := range mlt.FieldNames {
		if err := mlt.retrieveTerms(reader, docID, fieldName, termFreqs); err != nil {
			return nil, err
		}
	}

	// If no field names specified, try all fields
	if len(mlt.FieldNames) == 0 {
		// Get all field names from the index
		// For now, use a simplified approach
		if err := mlt.retrieveTerms(reader, docID, "", termFreqs); err != nil {
			return nil, err
		}
	}

	// Select the most interesting terms
	terms := mlt.selectInterestingTerms(reader, termFreqs)

	// Create a boolean query from the selected terms
	if len(terms) == 0 {
		return nil, fmt.Errorf("no interesting terms found")
	}

	return mlt.createQuery(terms), nil
}

// LikeText finds documents similar to the given text.
func (mlt *MoreLikeThis) LikeText(text string) (Query, error) {
	if mlt.Analyzer == nil {
		return nil, fmt.Errorf("analyzer is required")
	}

	// Analyze the text and extract term frequencies
	termFreqs := make(map[string]*termFreq)

	// Create a token stream from the text
	ts, err := mlt.Analyzer.TokenStream("", strings.NewReader(text))
	if err != nil {
		return nil, fmt.Errorf("failed to create token stream: %w", err)
	}
	defer ts.Close()

	// Extract terms from the token stream
	termCount := 0
	// Simplified approach: split by spaces
	words := strings.Fields(text)
	for _, word := range words {
		word = strings.ToLower(strings.TrimSpace(word))
		if word == "" {
			continue
		}

		// Check word length constraints
		if mlt.MinWordLen > 0 && len(word) < mlt.MinWordLen {
			continue
		}
		if mlt.MaxWordLen > 0 && len(word) > mlt.MaxWordLen {
			continue
		}

		// Check stop words
		if mlt.IsStopWord(word) {
			continue
		}

		if tf, ok := termFreqs[word]; ok {
			tf.freq++
		} else {
			termFreqs[word] = &termFreq{
				term:  word,
				freq:  1,
				field: "",
			}
		}
		termCount++
		if termCount >= mlt.MaxNumTokensParsed {
			break
		}
	}

	// Filter by minTermFreq
	filtered := make(map[string]*termFreq)
	for key, tf := range termFreqs {
		if tf.freq >= mlt.MinTermFreq {
			filtered[key] = tf
		}
	}

	// Select interesting terms (simplified - no IDF calculation without reader)
	terms := make([]*interestingTerm, 0, len(filtered))
	for _, tf := range filtered {
		terms = append(terms, &interestingTerm{
			term:  tf.term,
			field: tf.field,
			score: float64(tf.freq),
		})
	}

	// Sort by score and take top terms
	sort.Slice(terms, func(i, j int) bool {
		return terms[i].score > terms[j].score
	})

	if len(terms) > mlt.MaxQueryTerms {
		terms = terms[:mlt.MaxQueryTerms]
	}

	if len(terms) == 0 {
		return nil, fmt.Errorf("no interesting terms found")
	}

	return mlt.createQuery(terms), nil
}

// retrieveTerms retrieves term frequencies from the index for a document.
func (mlt *MoreLikeThis) retrieveTerms(reader IndexReader, docID int, fieldName string, termFreqs map[string]*termFreq) error {
	// In a full implementation, this would:
	// 1. Get term vectors for the document
	// 2. Extract terms and frequencies
	// 3. Filter based on constraints

	// For now, return nil as this is a simplified implementation
	return nil
}

// selectInterestingTerms selects the most interesting terms based on TF/IDF scoring.
func (mlt *MoreLikeThis) selectInterestingTerms(reader IndexReader, termFreqs map[string]*termFreq) []*interestingTerm {
	// Create a min-heap to keep top terms
	h := &interestingTermHeap{}
	heap.Init(h)

	numDocs := reader.NumDocs()
	if numDocs == 0 {
		numDocs = 1
	}

	for _, tf := range termFreqs {
		// Check minimum term frequency
		if tf.freq < mlt.MinTermFreq {
			continue
		}

		// Get document frequency for the term
		// In a full implementation, this would query the index
		docFreq := tf.freq // Simplified

		// Check document frequency constraints
		if docFreq < mlt.MinDocFreq {
			continue
		}
		if mlt.MaxDocFreq > 0 && docFreq*100/numDocs > mlt.MaxDocFreq {
			continue
		}

		// Calculate TF/IDF score
		// score = tf * log(numDocs / (docFreq + 1))
		tfScore := 1 + math.Log10(float64(tf.freq))
		idfScore := math.Log10(float64(numDocs) / float64(docFreq+1))
		score := tfScore * idfScore

		term := &interestingTerm{
			term:  tf.term,
			field: tf.field,
			score: score,
		}

		// Add to heap
		if h.Len() < mlt.MaxQueryTerms {
			heap.Push(h, term)
		} else if (*h)[0].score < score {
			heap.Pop(h)
			heap.Push(h, term)
		}
	}

	// Convert heap to slice
	result := make([]*interestingTerm, h.Len())
	for i := len(result) - 1; i >= 0; i-- {
		result[i] = heap.Pop(h).(*interestingTerm)
	}

	return result
}

// createQuery creates a boolean OR query from the selected terms.
func (mlt *MoreLikeThis) createQuery(terms []*interestingTerm) Query {
	if len(terms) == 0 {
		return nil
	}

	// Create term queries for each interesting term
	queries := make([]Query, 0, len(terms))
	for _, term := range terms {
		// Create a term query for this term
		// Use empty field if not specified, otherwise use the term's field
		field := term.field
		if field == "" {
			field = ""
		}
		termObj := index.NewTerm(field, term.term)
		queries = append(queries, NewTermQuery(termObj))
	}

	// Combine with OR (BooleanQuery with should clauses)
	return NewBooleanQueryOrWithQueries(queries...)
}

// MoreLikeThisQuery is a query that wraps the MoreLikeThis functionality.
type MoreLikeThisQuery struct {
	BaseQuery
	mlt    *MoreLikeThis
	docID  int
	text   string
	isText bool
}

// NewMoreLikeThisQuery creates a new MoreLikeThisQuery for a document ID.
func NewMoreLikeThisQuery(mlt *MoreLikeThis, docID int) *MoreLikeThisQuery {
	return &MoreLikeThisQuery{
		mlt:    mlt,
		docID:  docID,
		isText: false,
	}
}

// NewMoreLikeThisQueryFromText creates a new MoreLikeThisQuery from text.
func NewMoreLikeThisQueryFromText(mlt *MoreLikeThis, text string) *MoreLikeThisQuery {
	return &MoreLikeThisQuery{
		mlt:    mlt,
		text:   text,
		isText: true,
	}
}

// Rewrite rewrites this query into a boolean query.
func (q *MoreLikeThisQuery) Rewrite(reader IndexReader) (Query, error) {
	if q.isText {
		return q.mlt.LikeText(q.text)
	}
	return q.mlt.Like(reader, q.docID)
}
