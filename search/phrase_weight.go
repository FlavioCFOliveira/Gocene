// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// PhraseWeight is the Weight implementation for PhraseQuery.
// This is the Go port of Lucene's org.apache.lucene.search.PhraseWeight.
type PhraseWeight struct {
	*BaseWeight
	query       *PhraseQuery
	searcher    *IndexSearcher
	needsScores bool
	similarity  Similarity
	simScorer   SimScorer
}

// NewPhraseWeight creates a new PhraseWeight.
func NewPhraseWeight(query *PhraseQuery, searcher *IndexSearcher, needsScores bool) (*PhraseWeight, error) {
	w := &PhraseWeight{
		BaseWeight:  NewBaseWeight(query),
		query:       query,
		searcher:    searcher,
		needsScores: needsScores,
		similarity:  NewClassicSimilarity(),
	}

	if needsScores && len(query.terms) > 0 {
		collectionStats := w.getCollectionStats(searcher)
		termStats := w.getTermStats(searcher)
		w.simScorer = w.similarity.Scorer(collectionStats, termStats)
	}

	return w, nil
}

// getCollectionStats returns collection statistics for the phrase's field.
func (w *PhraseWeight) getCollectionStats(searcher *IndexSearcher) *CollectionStatistics {
	reader := searcher.GetIndexReader()
	return NewCollectionStatistics(w.query.field, reader.MaxDoc(), reader.NumDocs(), -1, -1)
}

// getTermStats returns term statistics for the phrase.
func (w *PhraseWeight) getTermStats(searcher *IndexSearcher) *TermStatistics {
	if len(w.query.terms) == 0 {
		return nil
	}
	term := w.query.terms[0]
	docFreq := 0
	if reader, ok := searcher.GetIndexReader().(index.IndexReaderInterface); ok {
		if leafReader, ok := reader.(*index.LeafReader); ok {
			terms, err := leafReader.Terms(w.query.field)
			if err == nil && terms != nil {
				docFreq, _ = terms.GetDocCount()
			}
		}
	}
	return NewTermStatistics(term, docFreq, -1)
}

// Scorer creates a scorer for this weight.
//
// It fetches postings-with-positions for every phrase term and builds a
// conjunction scorer that only yields documents where the phrase actually
// matches (exact or within slop).
func (w *PhraseWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	leafReader := context.LeafReader()
	if leafReader == nil {
		return nil, nil
	}
	if len(w.query.terms) == 0 {
		return nil, nil
	}

	terms, err := leafReader.Terms(w.query.field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}

	// Collect postings-with-positions for each phrase term.
	// Duplicate terms (same text) can appear (e.g. "A A A"); each slot needs
	// its own independent PostingsEnum so we obtain a fresh one per slot.
	n := len(w.query.terms)
	postings := make([]index.PostingsEnum, n)

	for i, term := range w.query.terms {
		termsEnum, err := terms.GetIterator()
		if err != nil {
			return nil, err
		}
		found, err := termsEnum.SeekExact(term)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil // any missing term → no matches
		}
		pe, err := termsEnum.Postings(index.PostingsFlagPositions)
		if err != nil {
			return nil, err
		}
		if pe == nil {
			return nil, nil
		}
		postings[i] = pe
	}

	queryPositions := w.query.Positions()

	if w.query.slop == 0 {
		return NewPhraseScorer(w, postings, queryPositions, w.simScorer), nil
	}
	return NewSloppyPhraseScorer(w, postings, queryPositions, w.simScorer, w.query.slop), nil
}

// ScorerSupplier creates a scorer supplier for this weight.
func (w *PhraseWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewScorerSupplierAdapter(scorer), nil
}

// Explain returns an explanation of the score for the given document.
func (w *PhraseWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return NewExplanation(false, 0, "PhraseWeight explanation not implemented"), nil
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *PhraseWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// IsCacheable returns true if this weight can be cached for the given leaf.
func (w *PhraseWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// Count returns the count of matching documents in sub-linear time.
func (w *PhraseWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns the matches for a specific document.
func (w *PhraseWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}

// Ensure PhraseWeight implements Weight
var _ Weight = (*PhraseWeight)(nil)

// ---------------------------------------------------------------------------
// postingsAdvanceTo — sequential advance for PostingsEnums without Advance()
// ---------------------------------------------------------------------------

// postingsAdvanceTo advances pe to the first document with doc ID ≥ target
// using NextDoc() calls.  This is necessary because FreqProxPostingsEnum
// does not implement Advance(); it can only be scanned forward sequentially.
//
// Returns the doc ID in index-space (i.e. index.NO_MORE_DOCS = -1 at end).
func postingsAdvanceTo(pe index.PostingsEnum, target int) (int, error) {
	current := pe.DocID()
	if current >= target {
		// Already past or at target; return current position.
		// Note: index.NO_MORE_DOCS (-1) is < any valid target, so this branch
		// is only taken when the enum is already on a valid doc ≥ target.
		return current, nil
	}
	for {
		doc, err := pe.NextDoc()
		if err != nil {
			return index.NO_MORE_DOCS, err
		}
		if doc >= target || doc == index.NO_MORE_DOCS {
			return doc, nil
		}
	}
}

// ---------------------------------------------------------------------------
// PhraseScorer — exact phrase matching (slop == 0)
// ---------------------------------------------------------------------------

// PhraseScorer scores documents for exact phrase queries (slop=0).
//
// It maintains a conjunction over all term postings and, for each candidate
// document, verifies that the terms appear at the required positions.
type PhraseScorer struct {
	*BaseScorer
	postings       []index.PostingsEnum
	queryPositions []int // expected query position for each term slot
	simScorer      SimScorer
	doc            int
	cachedFreq     int // phrase frequency for the current doc; computed once
}

// NewPhraseScorer creates a new PhraseScorer.
func NewPhraseScorer(weight Weight, postings []index.PostingsEnum, queryPositions []int, simScorer SimScorer) *PhraseScorer {
	return &PhraseScorer{
		BaseScorer:     NewBaseScorer(weight),
		postings:       postings,
		queryPositions: queryPositions,
		simScorer:      simScorer,
		doc:            -1,
	}
}

// DocID returns the current document ID.
func (s *PhraseScorer) DocID() int {
	return s.doc
}

// NextDoc advances to the next matching document.
func (s *PhraseScorer) NextDoc() (int, error) {
	doc, err := s.postings[0].NextDoc()
	if err != nil {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, err
	}
	return s.doAdvance(doc)
}

// Advance advances to the first matching document ≥ target.
func (s *PhraseScorer) Advance(target int) (int, error) {
	doc, err := postingsAdvanceTo(s.postings[0], target)
	if err != nil {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, err
	}
	return s.doAdvance(doc)
}

// doAdvance drives the zigzag conjunction from a candidate doc in index-space.
func (s *PhraseScorer) doAdvance(candidate int) (int, error) {
	for {
		if candidate == index.NO_MORE_DOCS {
			s.doc = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}

		// Sync all other enums to candidate.
		allPresent := true
		maxDoc := candidate
		for i := 1; i < len(s.postings); i++ {
			d, err := postingsAdvanceTo(s.postings[i], candidate)
			if err != nil {
				s.doc = NO_MORE_DOCS
				return NO_MORE_DOCS, err
			}
			if d != candidate {
				if d > maxDoc {
					maxDoc = d
				}
				allPresent = false
			}
		}

		if !allPresent {
			// Advance lead to maxDoc and retry.
			d, err := postingsAdvanceTo(s.postings[0], maxDoc)
			if err != nil {
				s.doc = NO_MORE_DOCS
				return NO_MORE_DOCS, err
			}
			candidate = d
			continue
		}

		// All terms present — verify phrase positions.
		freq, err := s.phraseFreq()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if freq > 0 {
			s.cachedFreq = freq
			s.doc = postingsDocToSearchDoc(candidate)
			return s.doc, nil
		}

		// Phrase not found at the right positions; advance lead.
		d, err := s.postings[0].NextDoc()
		if err != nil {
			s.doc = NO_MORE_DOCS
			return NO_MORE_DOCS, err
		}
		candidate = d
	}
}

// phraseFreq counts how many times the exact phrase occurs in the current doc.
func (s *PhraseScorer) phraseFreq() (int, error) {
	if len(s.postings) == 1 {
		freq, err := s.postings[0].Freq()
		if err != nil {
			return 0, err
		}
		return freq, nil
	}

	termPositions, err := collectAllPositions(s.postings)
	if err != nil {
		return 0, err
	}
	return countExactMatches(termPositions, s.queryPositions), nil
}

// Cost returns the estimated cost.
func (s *PhraseScorer) Cost() int64 {
	return s.postings[0].Cost()
}

// DocIDRunEnd returns the end of the current run.
func (s *PhraseScorer) DocIDRunEnd() int {
	return s.doc + 1
}

// Score returns the score for the current document.
// The phrase frequency was already computed and cached by doAdvance.
func (s *PhraseScorer) Score() float32 {
	freq := s.cachedFreq
	if freq == 0 {
		freq = 1
	}
	if s.simScorer != nil {
		return s.simScorer.Score(s.doc, float32(freq))
	}
	return float32(freq)
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *PhraseScorer) GetMaxScore(upTo int) float32 {
	return 1.0
}

// Ensure PhraseScorer implements Scorer
var _ Scorer = (*PhraseScorer)(nil)

// ---------------------------------------------------------------------------
// SloppyPhraseScorer — sloppy phrase matching (slop > 0)
// ---------------------------------------------------------------------------

// SloppyPhraseScorer scores documents for sloppy phrase queries (slop > 0).
//
// It uses the same conjunction approach as PhraseScorer but accepts matches
// where the phrase terms appear within slop extra moves of their ideal
// positions.  Each match contributes 1/(1+matchDistance) to the document's
// phrase frequency, where matchDistance = (maxPos - minPos) - expectedSpan
// and expectedSpan = max(queryPositions) - min(queryPositions).
//
// The algorithm mirrors Lucene's SloppyPhraseMatcher.phraseFreq():
//   - Collect all positions for each term slot from the current document.
//   - Build a min-heap (one entry per term slot).
//   - Repeatedly pop the minimum-position entry, compute the window
//     [minPos, maxPos], test against slop, accumulate, then advance.
type SloppyPhraseScorer struct {
	*BaseScorer
	postings       []index.PostingsEnum
	queryPositions []int
	simScorer      SimScorer
	slop           int
	doc            int
	cachedFreq     float32 // sloppy phrase frequency for the current doc
}

// NewSloppyPhraseScorer creates a new SloppyPhraseScorer.
func NewSloppyPhraseScorer(weight Weight, postings []index.PostingsEnum, queryPositions []int, simScorer SimScorer, slop int) *SloppyPhraseScorer {
	return &SloppyPhraseScorer{
		BaseScorer:     NewBaseScorer(weight),
		postings:       postings,
		queryPositions: queryPositions,
		simScorer:      simScorer,
		slop:           slop,
		doc:            -1,
	}
}

// DocID returns the current document ID.
func (s *SloppyPhraseScorer) DocID() int {
	return s.doc
}

// NextDoc advances to the next matching document.
func (s *SloppyPhraseScorer) NextDoc() (int, error) {
	d, err := s.postings[0].NextDoc()
	if err != nil {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, err
	}
	return s.doAdvance(d)
}

// Advance advances to the first matching document ≥ target.
func (s *SloppyPhraseScorer) Advance(target int) (int, error) {
	d, err := postingsAdvanceTo(s.postings[0], target)
	if err != nil {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, err
	}
	return s.doAdvance(d)
}

// doAdvance drives the conjunction + slop check from candidate (index-space).
func (s *SloppyPhraseScorer) doAdvance(candidate int) (int, error) {
	for {
		if candidate == index.NO_MORE_DOCS {
			s.doc = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}

		allPresent := true
		maxDoc := candidate
		for i := 1; i < len(s.postings); i++ {
			d, err := postingsAdvanceTo(s.postings[i], candidate)
			if err != nil {
				s.doc = NO_MORE_DOCS
				return NO_MORE_DOCS, err
			}
			if d != candidate {
				if d > maxDoc {
					maxDoc = d
				}
				allPresent = false
			}
		}

		if !allPresent {
			d, err := postingsAdvanceTo(s.postings[0], maxDoc)
			if err != nil {
				s.doc = NO_MORE_DOCS
				return NO_MORE_DOCS, err
			}
			candidate = d
			continue
		}

		freq, err := s.sloppyFreq()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		if freq > 0 {
			s.cachedFreq = freq
			s.doc = postingsDocToSearchDoc(candidate)
			return s.doc, nil
		}

		d, err := s.postings[0].NextDoc()
		if err != nil {
			s.doc = NO_MORE_DOCS
			return NO_MORE_DOCS, err
		}
		candidate = d
	}
}

// Cost returns the estimated cost.
func (s *SloppyPhraseScorer) Cost() int64 {
	return s.postings[0].Cost()
}

// DocIDRunEnd returns the end of the current run.
func (s *SloppyPhraseScorer) DocIDRunEnd() int {
	return s.doc + 1
}

// sloppyFreq computes the accumulated slop-weighted frequency for the current
// document by enumerating all valid combinations of one position per slot.
//
// A combination (p[0], p[1], ..., p[n-1]) is valid when:
//   - All positions are distinct (repeated terms cannot share a position).
//   - matchDist = max(p[i]-qOffset[i]) - min(p[i]-qOffset[i]) ≤ slop,
//     where qOffset[i] = queryPositions[i] - queryPositions[0].
//
// Each valid combination contributes 1/(1+matchDist) to the document's phrase
// frequency.  This guarantees correct results for repeated terms and holes.
func (s *SloppyPhraseScorer) sloppyFreq() (float32, error) {
	if len(s.postings) == 1 {
		cnt, err := s.postings[0].Freq()
		if err != nil {
			return 0, err
		}
		return float32(cnt), nil
	}

	termPositions, err := collectAllPositions(s.postings)
	if err != nil {
		return 0, err
	}
	for _, positions := range termPositions {
		if len(positions) == 0 {
			return 0, nil
		}
	}

	// Build per-slot offset relative to the first query position.
	baseQPos := s.queryPositions[0]
	offsets := make([]int, len(s.queryPositions))
	for i, qp := range s.queryPositions {
		offsets[i] = qp - baseQPos
	}

	return sloppyCombinations(termPositions, offsets, s.slop, 0, make([]int, len(termPositions)))
}

// sloppyCombinations recursively enumerates all combinations of one position
// per slot and accumulates the slop-weighted frequency.
//
// combo holds the chosen position for slots already filled (indices < depth).
func sloppyCombinations(termPositions [][]int, offsets []int, slop, depth int, combo []int) (float32, error) {
	n := len(termPositions)
	if depth == n {
		// All slots filled — evaluate the combination.
		// Normalize: subtract the per-slot offset to bring terms to the same
		// reference frame.  The match distance is the span of normalised values.
		normalized := make([]int, n)
		for i, pos := range combo {
			normalized[i] = pos - offsets[i]
		}
		minN, maxN := normalized[0], normalized[0]
		for _, v := range normalized {
			if v < minN {
				minN = v
			}
			if v > maxN {
				maxN = v
			}
		}
		matchDist := maxN - minN
		if matchDist < 0 {
			matchDist = 0
		}
		if matchDist > slop {
			return 0, nil
		}
		return 1.0 / float32(1+matchDist), nil
	}

	var total float32
	for _, pos := range termPositions[depth] {
		// Reject positions already used by an earlier slot (repeated terms must
		// occupy distinct document positions).
		used := false
		for i := 0; i < depth; i++ {
			if combo[i] == pos {
				used = true
				break
			}
		}
		if used {
			continue
		}
		combo[depth] = pos
		sub, err := sloppyCombinations(termPositions, offsets, slop, depth+1, combo)
		if err != nil {
			return 0, err
		}
		total += sub
	}
	return total, nil
}

// Score returns the score for the current document.
// The sloppy phrase frequency was computed and cached by doAdvance.
func (s *SloppyPhraseScorer) Score() float32 {
	if s.simScorer != nil {
		return s.simScorer.Score(s.doc, s.cachedFreq)
	}
	return s.cachedFreq
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *SloppyPhraseScorer) GetMaxScore(upTo int) float32 {
	return 1.0
}

// Ensure SloppyPhraseScorer implements Scorer
var _ Scorer = (*SloppyPhraseScorer)(nil)

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// collectAllPositions drains all positions for each PostingsEnum slot.
// The caller must have already positioned all enums on the same document.
// After this call each enum's position stream is exhausted for the current doc.
func collectAllPositions(postings []index.PostingsEnum) ([][]int, error) {
	result := make([][]int, len(postings))
	for i, pe := range postings {
		freq, err := pe.Freq()
		if err != nil {
			return nil, err
		}
		positions := make([]int, 0, freq)
		for j := 0; j < freq; j++ {
			pos, err := pe.NextPosition()
			if err != nil {
				return nil, err
			}
			if pos == index.NO_MORE_POSITIONS {
				break
			}
			positions = append(positions, pos)
		}
		sort.Ints(positions)
		result[i] = positions
	}
	return result, nil
}

// countExactMatches counts how many distinct anchor positions exist such that
// for every term slot i there is a position in termPositions[i] equal to
// anchorPos + (queryPositions[i] - queryPositions[0]).
func countExactMatches(termPositions [][]int, queryPositions []int) int {
	if len(termPositions) == 0 {
		return 0
	}

	posSets := make([]map[int]struct{}, len(termPositions))
	for i, positions := range termPositions {
		m := make(map[int]struct{}, len(positions))
		for _, p := range positions {
			m[p] = struct{}{}
		}
		posSets[i] = m
	}

	baseOffset := queryPositions[0]
	count := 0
	for anchorPos := range posSets[0] {
		match := true
		for i := 1; i < len(termPositions); i++ {
			expected := anchorPos + (queryPositions[i] - baseOffset)
			if _, ok := posSets[i][expected]; !ok {
				match = false
				break
			}
		}
		if match {
			count++
		}
	}
	return count
}
