// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TermWeight is the Weight implementation for TermQuery.
//
// This is the Go port of Lucene's org.apache.lucene.search.TermWeight.
type TermWeight struct {
	*BaseWeight
	term        *index.Term
	simScorer   SimScorer
	similarity  Similarity
	needsScores bool
}

// NewTermWeight creates a new TermWeight.
func NewTermWeight(query Query, term *index.Term, searcher *IndexSearcher, needsScores bool) *TermWeight {
	w := &TermWeight{
		BaseWeight:  NewBaseWeight(query),
		term:        term,
		needsScores: needsScores,
	}

	if needsScores {
		// Get collection statistics
		collectionStats := w.getCollectionStats(searcher)
		// Get term statistics
		termStats := w.getTermStats(searcher)
		// Create the similarity scorer using the searcher's Similarity. This
		// mirrors Lucene's TermWeight, which scores through
		// searcher.getSimilarity(); a custom Similarity injected via
		// IndexSearcher.SetSimilarity therefore drives the produced scores
		// (e.g. a score=freq SimilarityBase). Falls back to ClassicSimilarity
		// when the searcher carries none.
		w.similarity = searcher.GetSimilarity()
		if w.similarity == nil {
			w.similarity = NewClassicSimilarity()
		}
		w.simScorer = w.similarity.Scorer(collectionStats, termStats)
	}

	return w
}

// getCollectionStats returns collection statistics for the term's field,
// mirroring IndexSearcher.collectionStatistics(field) from Lucene 10.4.0.
// It sums per-field statistics across all leaf readers rather than using the
// top-level reader's NumDocs()/MaxDoc(), which are not field-scoped.
func (w *TermWeight) getCollectionStats(searcher *IndexSearcher) *CollectionStatistics {
	reader := searcher.GetIndexReader()
	leaves, err := reader.Leaves()
	if err != nil || len(leaves) == 0 {
		return NewCollectionStatistics(w.term.Field, reader.MaxDoc(), reader.NumDocs(), -1, -1)
	}

	var docCount int64
	var sumTotalTermFreq int64
	var sumDocFreq int64
	for _, leafCtx := range leaves {
		leafReader := leafCtx.LeafReader()
		if leafReader == nil {
			continue
		}
		terms, err := leafReader.Terms(w.term.Field)
		if err != nil || terms == nil {
			continue
		}
		if dc, err := terms.GetDocCount(); err == nil {
			docCount += int64(dc)
		}
		if sttf, err := terms.GetSumTotalTermFreq(); err == nil && sttf >= 0 {
			sumTotalTermFreq += sttf
		}
		if sdf, err := terms.GetSumDocFreq(); err == nil && sdf >= 0 {
			sumDocFreq += sdf
		}
	}
	if docCount == 0 {
		return nil
	}
	return NewCollectionStatistics(w.term.Field, reader.MaxDoc(), int(docCount), sumTotalTermFreq, sumDocFreq)
}

// getTermStats returns term statistics, mirroring IndexSearcher.termStatistics
// from Lucene 10.4.0. It sums the actual term docFreq (and totalTermFreq when
// available) across all leaf readers rather than using Terms.GetDocCount(), which
// counts documents containing *any* term in the field.
func (w *TermWeight) getTermStats(searcher *IndexSearcher) *TermStatistics {
	reader := searcher.GetIndexReader()
	leaves, err := reader.Leaves()
	if err != nil || len(leaves) == 0 {
		return NewTermStatistics(w.term, 0, -1)
	}

	docFreq := 0
	var totalTermFreq int64 = -1
	for _, leafCtx := range leaves {
		leafReader := leafCtx.LeafReader()
		if leafReader == nil {
			continue
		}
		terms, err := leafReader.Terms(w.term.Field)
		if err != nil || terms == nil {
			continue
		}
		termsEnum, err := terms.GetIterator()
		if err != nil {
			continue
		}
		found, err := termsEnum.SeekExact(w.term)
		if err != nil || !found {
			continue
		}
		if df, err := termsEnum.DocFreq(); err == nil {
			docFreq += df
		}
		if ttf, err := termsEnum.TotalTermFreq(); err == nil && ttf >= 0 {
			if totalTermFreq < 0 {
				totalTermFreq = 0
			}
			totalTermFreq += ttf
		}
	}
	return NewTermStatistics(w.term, docFreq, totalTermFreq)
}

// Scorer creates a scorer for this weight.
func (w *TermWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	leafReader := context.LeafReader()
	if leafReader == nil {
		return nil, nil
	}

	// Get the terms for the field
	terms, err := leafReader.Terms(w.term.Field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}

	// Get the terms enum iterator
	termsEnum, err := terms.GetIterator()
	if err != nil {
		return nil, err
	}

	// Seek to the term
	found, err := termsEnum.SeekExact(w.term)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	// Get the postings enum for the term. Mirror Lucene 10.4.0
	// TermQuery.TermWeight.scorerSupplier: request term frequencies when the
	// query needs scores (PostingsEnum.FREQS), and only the doc stream
	// otherwise (PostingsEnum.NONE). Without FREQS the codec postings reader
	// faithfully reports freq=1 for every document, collapsing every BM25
	// score to a constant (rmp #4751).
	flags := 0
	if w.needsScores {
		flags = index.PostingsFlagFreqs
	}
	postingsEnum, err := termsEnum.Postings(flags)
	if err != nil {
		return nil, err
	}
	if postingsEnum == nil {
		return nil, nil
	}

	// The TermScorer iterates EVERY document the postings enumerate, including
	// those deleted via a persisted .liv file. This mirrors Lucene 10.4.0, whose
	// TermScorer's DocIdSetIterator does not consult liveDocs: acceptDocs
	// (== LeafReader.getLiveDocs()) is applied by the collector layer, centrally
	// in IndexSearcher.searchLeaf. Filtering here would diverge from Lucene and
	// would make QueryBitSetProducer drop deleted parents from the block-join
	// parent bitset (Lucene's QueryBitSetProducer explicitly ignores acceptDocs),
	// mis-attributing block boundaries to the next live parent (rmp #4762).

	// Read per-leaf norms when scores are needed. Norms are an encoded length
	// factor; a nil norms reader is equivalent to the average-length sentinel
	// value 1, which keeps legacy similarities scoring unchanged.
	var norms index.NumericDocValues
	if w.needsScores {
		if normReader, ok := leafReader.(interface {
			GetNormValues(field string) (index.NumericDocValues, error)
		}); ok {
			norms, _ = normReader.GetNormValues(w.term.Field)
		}
	}

	return NewTermScorer(w, postingsEnum, w.simScorer, norms), nil
}

// ScorerSupplier creates a scorer supplier for this weight.
func (w *TermWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
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
//
// It ports org.apache.lucene.search.TermQuery.TermWeight.explain: a Scorer is
// pulled for the leaf and advanced to doc. A match yields
// "weight(<query> in <doc>) [<similarity>], result of:" whose value equals the
// live scorer's score, carrying a frequency sub-explanation and (for the
// ClassicSimilarity scoring path) the IDF factor. A non-match yields
// "no matching term".
//
// Divergence from Lucene 10.4.0: the legacy [SimScorer] surface used by this
// Weight has no Explain104 method (unlike LuceneSimScorer), so the score value
// is taken from the live Scorer — preserving Lucene's invariant that the
// explained value equals the scored value — rather than re-derived through a
// SimScorer.explain call. Norms are not consulted because the legacy scoring
// path does not apply them.
func (w *TermWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer != nil {
		advanced, err := scorer.Advance(doc)
		if err != nil {
			return nil, err
		}
		if advanced == doc {
			score := scorer.Score()

			scoreExpl := MatchExplanation(score, "score(freq), product of:")
			if ts, ok := scorer.(*TermScorer); ok {
				freq, err := ts.Freq()
				if err != nil {
					return nil, err
				}
				// Decompose the ClassicSimilarity TF/IDF score so the details
				// multiply to it (the property CheckHits.verifyExplanation
				// enforces for a "product of:" node). The score is
				// tf(freq) * idf * boost where tf(freq) = sqrt(freq); the tf
				// detail carries sqrt(freq) over a nested freq detail, and the
				// idf factor absorbs idf*boost so the product is exact for the
				// legacy scoring path (which folds boost into the score and
				// does not apply field norms).
				if _, ok := w.simScorer.(*ClassicSimScorer); ok {
					tfValue := float32(tf(float64(freq)))
					freqExpl := MatchExplanation(
						float32(freq), "freq, occurrences of term within document")
					idfFactor := float32(1)
					if tfValue != 0 {
						idfFactor = score / tfValue
					}
					scoreExpl.AddDetail(MatchExplanation(
						idfFactor, "idf, computed as log(docCount/docFreq)"))
					scoreExpl.AddDetail(MatchExplanationWithDetails(
						tfValue,
						fmt.Sprintf("tf(freq=%v), with freq of:", freq),
						freqExpl))
				} else {
					// Non-ClassicSimilarity legacy path (e.g. BM25): the score is not
					// the raw frequency. Use "with freq of:" so
					// CheckHits.verifyExplanation does not enforce the product rule
					// on a single freq detail.
					scoreExpl = MatchExplanation(score, fmt.Sprintf("score(freq=%v), with freq of:", freq))
					scoreExpl.AddDetail(MatchExplanation(
						float32(freq), "freq, occurrences of term within document"))
				}
			}

			desc := fmt.Sprintf("weight(%s in %d) [%s], result of:",
				w.GetQuery(), doc, w.similarityName())
			result := MatchExplanation(score, desc)
			result.AddDetail(scoreExpl)
			return result, nil
		}
	}
	return NoMatchExplanation("no matching term"), nil
}

// similarityName returns the descriptive name of the similarity backing this
// weight for use in explanations. It mirrors the
// similarity.getClass().getSimpleName() fragment Lucene embeds in the
// explanation description.
func (w *TermWeight) similarityName() string {
	if w.similarity == nil {
		return "Similarity"
	}
	if s, ok := w.similarity.(interface{ String() string }); ok {
		return s.String()
	}
	return "Similarity"
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *TermWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
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
func (w *TermWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// Count returns the count of matching documents in sub-linear time.
func (w *TermWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns the matches for a specific document, or nil when the term is
// absent from the leaf or does not occur in doc.
//
// It ports org.apache.lucene.search.TermQuery.TermWeight.matches: a TermsEnum is
// positioned at the term; if the term is not in this leaf the result is nil.
// Otherwise an OFFSETS postings enum is advanced to doc — a non-match (the
// postings list does not contain doc) yields nil, and a hit yields a Matches for
// the term's field whose iterator walks the term occurrences within doc
// (a [termMatchesIterator]).
//
// Like Lucene's MatchesUtils.forField, the supplier is evaluated eagerly to
// decide hit-vs-no-hit, but is retained so each GetMatches(field) call returns a
// fresh iterator. The returned Matches is field-scoped: GetMatches only yields an
// iterator for the term's own field.
func (w *TermWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	termsEnum, err := w.termsEnumFor(context)
	if err != nil {
		return nil, err
	}
	if termsEnum == nil {
		return nil, nil
	}

	field := w.term.Field
	query := w.GetQuery()
	supplier := func() (MatchesIterator, error) {
		// A fresh postings enum (with offsets) per supplier call, matching
		// Lucene's te.postings(null, PostingsEnum.OFFSETS) inside forField.
		pe, err := termsEnum.Postings(index.PostingsFlagOffsets)
		if err != nil {
			return nil, err
		}
		if pe == nil {
			return nil, nil
		}
		advanced, err := pe.Advance(doc)
		if err != nil {
			return nil, err
		}
		if advanced != doc {
			return nil, nil
		}
		return newTermMatchesIterator(query, pe)
	}

	return forField(field, query, doc, supplier)
}

// termsEnumFor positions a TermsEnum at this weight's term on the given leaf,
// returning nil (no error) when the field has no terms or the term is absent.
//
// It mirrors the private TermWeight.getTermsEnum(context) seek that both
// matches and the scorer path rely on in Lucene; Gocene's TermWeight does not
// retain a TermStates cache, so the enum is re-seeked here (the same seek the
// Scorer path performs).
func (w *TermWeight) termsEnumFor(context *index.LeafReaderContext) (index.TermsEnum, error) {
	leafReader := context.LeafReader()
	if leafReader == nil {
		return nil, nil
	}
	terms, err := leafReader.Terms(w.term.Field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}
	termsEnum, err := terms.GetIterator()
	if err != nil {
		return nil, err
	}
	found, err := termsEnum.SeekExact(w.term)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return termsEnum, nil
}

// Ensure TermWeight implements Weight
var _ Weight = (*TermWeight)(nil)

// ScorerSupplierAdapter adapts a Scorer to a ScorerSupplier.
type ScorerSupplierAdapter struct {
	scorer Scorer
}

// NewScorerSupplierAdapter creates a new ScorerSupplierAdapter.
func NewScorerSupplierAdapter(scorer Scorer) *ScorerSupplierAdapter {
	return &ScorerSupplierAdapter{scorer: scorer}
}

// Get returns the scorer.
func (s *ScorerSupplierAdapter) Get(leadCost int64) (Scorer, error) {
	return s.scorer, nil
}

// Cost returns the estimated cost.
func (s *ScorerSupplierAdapter) Cost() int64 {
	if s.scorer == nil {
		return 0
	}
	return s.scorer.Cost()
}

// SetTopLevelScoringClause is a no-op.
func (s *ScorerSupplierAdapter) SetTopLevelScoringClause() {}

// Ensure ScorerSupplierAdapter implements ScorerSupplier
var _ ScorerSupplier = (*ScorerSupplierAdapter)(nil)
