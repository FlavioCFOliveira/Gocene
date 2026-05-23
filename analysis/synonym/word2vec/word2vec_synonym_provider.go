// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package word2vec

import (
	"errors"
	"math"

	codecshnsw "github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// Word2VecSynonymProvider generates synonym lists for a term using a trained
// Word2Vec model. At construction time it builds an HNSW graph over the model
// vectors; at query time it searches that graph for the nearest neighbours.
//
// This is the Go port of
// org.apache.lucene.analysis.synonym.word2vec.Word2VecSynonymProvider from
// Apache Lucene 10.4.0.
//
// Deviation: Lucene's implementation holds the model and the OnHeapHnswGraph
// as separate fields and wraps a DefaultFlatVectorScorer. Gocene preserves
// the same structure but adjusts to the Gocene codecs/hnsw package interfaces.
type Word2VecSynonymProvider struct {
	model                   *Word2VecModel
	hnswGraph               *utilhnsw.OnHeapHnswGraph
	defaultFlatVectorScorer *codecshnsw.DefaultFlatVectorScorer
}

// similarityFunction is the vector similarity function used by the synonym
// provider (dot product on L2-normalized vectors = cosine similarity).
const similarityFunction = index.VectorSimilarityFunctionDotProduct

// NewWord2VecSynonymProvider constructs a Word2VecSynonymProvider and builds
// the HNSW graph from the given model.
func NewWord2VecSynonymProvider(model *Word2VecModel) (*Word2VecSynonymProvider, error) {
	scorer := codecshnsw.NewDefaultFlatVectorScorer()
	scorerSupplier, err := scorer.GetRandomVectorScorerSupplier(similarityFunction, model)
	if err != nil {
		return nil, err
	}
	builder, err := utilhnsw.NewHnswGraphBuilderWithGraphSize(
		scorerSupplier,
		utilhnsw.DefaultMaxConn,
		utilhnsw.DefaultBeamWidth,
		utilhnsw.RandSeed,
		model.Size(),
	)
	if err != nil {
		return nil, err
	}
	graph, err := builder.Build(model.Size())
	if err != nil {
		return nil, err
	}
	return &Word2VecSynonymProvider{
		model:                   model,
		hnswGraph:               graph,
		defaultFlatVectorScorer: scorer,
	}, nil
}

// GetSynonyms returns the synonyms for term, sorted by descending similarity.
// At most maxSynonymsPerTerm synonyms with similarity ≥ minAcceptedSimilarity
// are returned. The query term itself is excluded from the result.
func (p *Word2VecSynonymProvider) GetSynonyms(
	term *util.BytesRef,
	maxSynonymsPerTerm int,
	minAcceptedSimilarity float32,
) ([]*TermAndBoost, error) {
	if term == nil {
		return nil, errors.New("word2vec: term must not be nil")
	}

	query := p.model.VectorValueByTerm(term)
	if query == nil {
		return nil, nil
	}

	scorer, err := p.defaultFlatVectorScorer.GetRandomVectorScorer(
		similarityFunction, p.model, query)
	if err != nil {
		return nil, err
	}

	// We request maxSynonymsPerTerm+1 because the query vector itself is
	// always the nearest neighbour to itself.
	collector, err := utilhnsw.SearchWithOnHeapGraph(
		scorer,
		maxSynonymsPerTerm+1,
		p.hnswGraph,
		nil,
		math.MaxInt32,
	)
	if err != nil {
		return nil, err
	}

	topDocs := collector.TopDocs()
	result := make([]*TermAndBoost, 0, len(topDocs.ScoreDocs))
	for _, sd := range topDocs.ScoreDocs {
		similarity := sd.Score
		synonym := p.model.TermValue(sd.Doc)
		if !util.BytesRefEquals(synonym, term) && similarity >= minAcceptedSimilarity {
			result = append(result, NewTermAndBoost(synonym, similarity))
		}
	}
	return result, nil
}
