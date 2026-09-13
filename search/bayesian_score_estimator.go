// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"slices"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Parameters contains the estimated parameters for BayesianScoreQuery.
type Parameters struct {
	Alpha    float32
	Beta     float32
	BaseRate float32
}

const (
	defaultNSamples       = 50
	defaultTokensPerQuery = 5
	percentileThreshold   = 0.95
	baseRateMin           = 1e-6
	baseRateMax           = 0.5
)

// Estimate estimates BayesianScoreQuery parameters from the given index.
//
// This is the Go port of org.apache.lucene.search.BayesianScoreEstimator#estimate.
func Estimate(searcher *IndexSearcher, field string, nSamples, tokensPerQuery int, seed int64) (Parameters, error) {
	if nSamples <= 0 {
		return Parameters{}, fmt.Errorf("nSamples must be positive, got %d", nSamples)
	}
	if tokensPerQuery <= 0 {
		return Parameters{}, fmt.Errorf("tokensPerQuery must be positive, got %d", tokensPerQuery)
	}

	reader := searcher.GetIndexReader()
	maxDoc := reader.MaxDoc()
	if maxDoc == 0 {
		return Parameters{Alpha: 1.0, Beta: 0.0, BaseRate: 0.01}, nil
	}

	rng := rand.New(rand.NewSource(seed))
	sampledTerms, err := sampleVocabularyTerms(reader, field, nSamples*tokensPerQuery, rng)
	if err != nil {
		return Parameters{}, err
	}
	if len(sampledTerms) == 0 {
		return Parameters{Alpha: 1.0, Beta: 0.0, BaseRate: 0.01}, nil
	}

	// Create pseudo-queries from indexed vocabulary terms and collect scores.
	var allScoreArrays [][]float32
	var baseRateFractions []float32

	for offset := 0; offset < len(sampledTerms); offset += tokensPerQuery {
		bq := NewBooleanQueryBuilder()
		end := offset + tokensPerQuery
		if end > len(sampledTerms) {
			end = len(sampledTerms)
		}
		for i := offset; i < end; i++ {
			term := index.NewTermFromBytesRef(field, sampledTerms[i])
			bq.Add(NewTermQuery(term), SHOULD)
		}

		// Collect all scores
		scores, err := collectScores(searcher, bq.Build(), maxDoc)
		if err != nil {
			return Parameters{}, err
		}
		if len(scores) == 0 {
			continue
		}
		allScoreArrays = append(allScoreArrays, scores)

		// Base rate: fraction of docs above 95th percentile
		sorted := make([]float32, len(scores))
		copy(sorted, scores)
		slices.Sort(sorted)

		pIdx := int(float64(len(sorted)) * percentileThreshold)
		if pIdx >= len(sorted) {
			pIdx = len(sorted) - 1
		}
		threshold := sorted[pIdx]

		highCount := 0
		for _, s := range scores {
			if s >= threshold {
				highCount++
			}
		}
		baseRateFractions = append(baseRateFractions, float32(highCount)/float32(maxDoc))
	}

	if len(allScoreArrays) == 0 {
		return Parameters{Alpha: 1.0, Beta: 0.0, BaseRate: 0.01}, nil
	}

	// Flatten all scores for global statistics
	totalScores := 0
	for _, arr := range allScoreArrays {
		totalScores += len(arr)
	}
	allScores := make([]float32, totalScores)
	offset := 0
	for _, arr := range allScoreArrays {
		copy(allScores[offset:], arr)
		offset += len(arr)
	}

	// beta = median
	slices.Sort(allScores)
	beta := allScores[len(allScores)/2]

	// alpha = 1 / std
	var mean float64
	for _, s := range allScores {
		mean += float64(s)
	}
	mean /= float64(len(allScores))

	var variance float64
	for _, s := range allScores {
		diff := float64(s) - mean
		variance += diff * diff
	}
	variance /= float64(len(allScores))
	std := math.Sqrt(variance)

	alpha := float32(1.0)
	if std > 0 {
		alpha = float32(1.0 / std)
	}

	// base rate = mean of per-query fractions, clamped
	var baseRate float32
	for _, f := range baseRateFractions {
		baseRate += f
	}
	if len(baseRateFractions) > 0 {
		baseRate /= float32(len(baseRateFractions))
	}

	if baseRate < baseRateMin {
		baseRate = baseRateMin
	} else if baseRate > baseRateMax {
		baseRate = baseRateMax
	}

	return Parameters{Alpha: alpha, Beta: beta, BaseRate: baseRate}, nil
}

// EstimateDefault estimates BayesianScoreQuery parameters with default settings
// (50 samples, 5 tokens per query, seed 42).
func EstimateDefault(searcher *IndexSearcher, field string) (Parameters, error) {
	return Estimate(searcher, field, defaultNSamples, defaultTokensPerQuery, 42)
}

func sampleVocabularyTerms(reader index.IndexReaderInterface, field string, sampleSize int, rng *rand.Rand) ([]*util.BytesRef, error) {
	// In Gocene, DirectoryReader implements Terms(field).
	// We need to cast to the interface that provides Terms.
	termsInterface, ok := reader.(interface {
		Terms(field string) (index.Terms, error)
	})
	if !ok {
		return nil, errors.New("reader does not support Terms()")
	}

	terms, err := termsInterface.Terms(field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}

	reservoir := make([]*util.BytesRef, 0, sampleSize)
	iterator, err := terms.GetIterator()
	if err != nil {
		return nil, err
	}

	var seen int64
	for {
		term, err := iterator.Next()
		if err != nil {
			return nil, err
		}
		if term == nil {
			break
		}
		seen++
		if len(reservoir) < sampleSize {
			reservoir = append(reservoir, term.Bytes)
		} else {
			replacement := nextLong(rng, seen)
			if replacement < int64(sampleSize) {
				reservoir[replacement] = term.Bytes
			}
		}
	}
	return reservoir, nil
}

func nextLong(rng *rand.Rand, bound int64) int64 {
	var bits int64
	var value int64
	for {
		bits = rng.Int63()
		value = bits % bound
		if bits-value+(bound-1) >= 0 {
			break
		}
	}
	return value
}

func collectScores(searcher *IndexSearcher, query Query, maxDoc int) ([]float32, error) {
	topN := maxDoc
	if topN > 10000 {
		topN = 10000
	}
	topDocs, err := searcher.Search(query, topN)
	if err != nil {
		return nil, err
	}
	scores := make([]float32, len(topDocs.ScoreDocs))
	for i, sd := range topDocs.ScoreDocs {
		scores[i] = sd.Score
	}
	return scores, nil
}
