// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "fmt"

// LuceneLMCollectionModel mirrors LMSimilarity.CollectionModel — a
// strategy interface for computing the collection language model P(w|C).
type LuceneLMCollectionModel interface {
	// ComputeProbability returns P(w|C) for the term encoded in stats.
	ComputeProbability(stats *LuceneBasicStats) float64

	// Name returns the model name (used in LMSimilarity.toString). May
	// return the empty string to omit the suffix.
	Name() string
}

// LuceneDefaultCollectionModel mirrors DefaultCollectionModel:
// P(w|C) = (totalTermFreq + 1) / (numberOfFieldTokens + 1).
type LuceneDefaultCollectionModel struct{}

// NewLuceneDefaultCollectionModel constructs the parameter-free default model.
func NewLuceneDefaultCollectionModel() *LuceneDefaultCollectionModel {
	return &LuceneDefaultCollectionModel{}
}

// ComputeProbability implements LuceneLMCollectionModel.
func (LuceneDefaultCollectionModel) ComputeProbability(stats *LuceneBasicStats) float64 {
	return (float64(stats.TotalTermFreq()) + 1.0) / (float64(stats.NumberOfFieldTokens()) + 1.0)
}

// Name returns the empty string — DefaultCollectionModel suppresses the
// model suffix in LMSimilarity.toString.
func (LuceneDefaultCollectionModel) Name() string { return "" }

// LuceneIndriCollectionModel mirrors IndriDirichletSimilarity.IndriCollectionModel:
// P(w|C) = totalTermFreq / numberOfFieldTokens (no +1 smoothing).
type LuceneIndriCollectionModel struct{}

// NewLuceneIndriCollectionModel constructs the parameter-free Indri model.
func NewLuceneIndriCollectionModel() *LuceneIndriCollectionModel {
	return &LuceneIndriCollectionModel{}
}

// ComputeProbability implements LuceneLMCollectionModel.
func (LuceneIndriCollectionModel) ComputeProbability(stats *LuceneBasicStats) float64 {
	tokens := float64(stats.NumberOfFieldTokens())
	if tokens == 0 {
		return 0
	}
	return float64(stats.TotalTermFreq()) / tokens
}

// Name returns the empty string — matches Java's null.
func (LuceneIndriCollectionModel) Name() string { return "" }

// LuceneLMSimilarity mirrors org.apache.lucene.search.similarities.
// LMSimilarity — the abstract superclass for language modeling
// similarities. It folds the collection probability into the per-term
// stats during fillBasicStats so subclasses (Dirichlet, Jelinek-Mercer,
// Indri) can read it from the hot path.
//
// Subclasses configure the score/subExplain/toString hooks via the
// constructor.
type LuceneLMSimilarity struct {
	*LuceneSimilarityBase

	collectionModel LuceneLMCollectionModel
	name            string // e.g. "Dirichlet(2000.000000)"
}

// NewLuceneLMSimilarity constructs an LM similarity with the given
// collection model, name, score/subExplain hooks, and discountOverlaps.
// The name is used in LMSimilarity.toString.
func NewLuceneLMSimilarity(collectionModel LuceneLMCollectionModel, name string, score LuceneScoreFunc, subExplain LuceneSubExplainFunc, discountOverlaps bool) *LuceneLMSimilarity {
	if collectionModel == nil {
		collectionModel = NewLuceneDefaultCollectionModel()
	}
	lm := &LuceneLMSimilarity{
		collectionModel: collectionModel,
		name:            name,
	}
	toString := func() string {
		coll := collectionModel.Name()
		if coll != "" {
			return fmt.Sprintf("LM %s - %s", name, coll)
		}
		return fmt.Sprintf("LM %s", name)
	}
	// Append the collection probability detail at the end of the
	// subExplain output, mirroring LMSimilarity.explain.
	wrappedSubExplain := func(stats *LuceneBasicStats, freq, docLen float64) []Explanation {
		var subs []Explanation
		if subExplain != nil {
			subs = subExplain(stats, freq, docLen)
		}
		subs = append(subs, NewExplanation(true,
			float32(collectionModel.ComputeProbability(stats)),
			"collection probability"))
		return subs
	}
	lm.LuceneSimilarityBase = NewLuceneSimilarityBaseWithDiscount(discountOverlaps, score, wrappedSubExplain, toString)
	lm.SetFillExtra(func(stats *LuceneBasicStats, _ *CollectionStatistics, _ *TermStatistics) {
		stats.SetCollectionProbability(collectionModel.ComputeProbability(stats))
	})
	return lm
}

// GetCollectionModel returns the configured collection model.
func (s *LuceneLMSimilarity) GetCollectionModel() LuceneLMCollectionModel { return s.collectionModel }

// GetName returns the LM method name.
func (s *LuceneLMSimilarity) GetName() string { return s.name }

// String mirrors LMSimilarity.toString.
func (s *LuceneLMSimilarity) String() string {
	coll := s.collectionModel.Name()
	if coll != "" {
		return fmt.Sprintf("LM %s - %s", s.name, coll)
	}
	return fmt.Sprintf("LM %s", s.name)
}

// Compile-time guarantees.
var (
	_ LuceneLMCollectionModel = LuceneDefaultCollectionModel{}
	_ LuceneLMCollectionModel = LuceneIndriCollectionModel{}
	_ LuceneSimilarity        = (*LuceneLMSimilarity)(nil)
)
