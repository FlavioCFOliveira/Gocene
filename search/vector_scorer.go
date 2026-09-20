// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

// Ported from Apache Lucene 10.5.0:
//
//	lucene/core/src/java/org/apache/lucene/search/VectorScorer.java

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// VectorScorer computes the similarity score between a given query vector and
// different document vectors.
//
// Mirrors org.apache.lucene.search.VectorScorer. The interface's two abstract
// members, score() and iterator(), live on [util.VectorScorer] because
// org.apache.lucene.util types consume them and Go cannot close that import
// cycle; the alias keeps the Lucene name available in its declaring package.
// The interface's default method bulk(DocIdSetIterator) is rendered as the
// package function [VectorScorerBulk], the Gocene spelling for a Java default
// method that a concrete type may override.
type VectorScorer = util.VectorScorer

// Bulk is the bulk scorer interface to score multiple vectors at once.
//
// Mirrors the nested interface VectorScorer.Bulk; it lives on
// [util.Bulk] for the same reason [VectorScorer] does. Its two static
// factories are [BulkFromRandomScorerDense] and [BulkFromRandomScorerSparse].
type Bulk = util.Bulk

// DefaultBulkBatchSize is the number of documents a [Bulk] scores per call.
//
// Mirrors {@code int DEFAULT_BULK_BATCH_SIZE = 64} of VectorScorer.
const DefaultBulkBatchSize = 64

// DocAndFloatFeatureBuffer is NOT declared by VectorScorer.java; it is its own
// Lucene class, org.apache.lucene.search.DocAndFloatFeatureBuffer, and is
// declared in doc_feature_buffers.go.

// vectorScorerBulkProvider is the optional interface a VectorScorer satisfies
// when it overrides VectorScorer#bulk(DocIdSetIterator).
type vectorScorerBulkProvider interface {
	Bulk(matchingDocs DocIdSetIterator) (Bulk, error)
}

// VectorScorerBulk renders {@code default Bulk bulk(DocIdSetIterator
// matchingDocs)} of VectorScorer:
//
//	final DocIdSetIterator iterator =
//	    matchingDocs == null
//	        ? iterator()
//	        : ConjunctionUtils.createConjunction(List.of(matchingDocs, iterator()), List.of());
//	if (iterator.docID() == -1) {
//	  iterator.nextDoc();
//	}
//	return (upTo, liveDocs, buffer) -> { ... };
//
// It first honours a scorer that overrides the member, mirroring Java's
// virtual dispatch, and otherwise runs the default body.
func VectorScorerBulk(scorer VectorScorer, matchingDocs DocIdSetIterator) (Bulk, error) {
	if provider, ok := scorer.(vectorScorerBulkProvider); ok {
		return provider.Bulk(matchingDocs)
	}
	var iterator DocIdSetIterator
	if matchingDocs == nil {
		iterator = scorer.Iterator()
	} else {
		iterator = CreateConjunction([]DocIdSetIterator{matchingDocs, scorer.Iterator()}, nil)
	}
	if iterator.DocID() == -1 {
		if _, err := iterator.NextDoc(); err != nil {
			return nil, err
		}
	}
	return &defaultVectorScorerBulk{scorer: scorer, iterator: iterator}, nil
}

// defaultVectorScorerBulk is the lambda VectorScorer#bulk returns.
type defaultVectorScorerBulk struct {
	scorer   VectorScorer
	iterator DocIdSetIterator
}

// NextDocsAndScores reproduces the lambda body of VectorScorer#bulk.
func (b *defaultVectorScorerBulk) NextDocsAndScores(
	upTo int, liveDocs interface{ Get(int) bool }, buffer *util.DocAndFloatFeatureBuffer,
) (float32, error) {
	buffer.GrowNoCopy(DefaultBulkBatchSize)
	size := 0
	// float maxScore = Float.NEGATIVE_INFINITY;
	maxScore := float32(math.Inf(-1))
	for doc := b.iterator.DocID(); doc < upTo && size < DefaultBulkBatchSize; {
		if liveDocs == nil || liveDocs.Get(doc) {
			buffer.Docs[size] = doc
			score, err := b.scorer.Score()
			if err != nil {
				return 0, err
			}
			buffer.Features[size] = score
			if buffer.Features[size] > maxScore {
				maxScore = buffer.Features[size]
			}
			size++
		}
		next, err := b.iterator.NextDoc()
		if err != nil {
			return 0, err
		}
		doc = next
	}
	buffer.Size = size
	return maxScore, nil
}

// BulkFromRandomScorerDense renders the static factory
// {@code VectorScorer.Bulk#fromRandomScorerDense(RandomVectorScorer,
// KnnVectorValues.DocIndexIterator, DocIdSetIterator)}.
func BulkFromRandomScorerDense(
	scorer hnsw.RandomVectorScorer, iterator spi.DocIndexIterator, matchingDocs DocIdSetIterator,
) Bulk {
	var matches DocIdSetIterator
	if matchingDocs == nil {
		matches = iterator
	} else {
		matches = CreateConjunction([]DocIdSetIterator{matchingDocs, iterator}, nil)
	}
	return &denseRandomScorerBulk{scorer: scorer, matches: matches}
}

// denseRandomScorerBulk is the lambda returned by fromRandomScorerDense.
type denseRandomScorerBulk struct {
	scorer  hnsw.RandomVectorScorer
	matches DocIdSetIterator
}

// NextDocsAndScores reproduces
//
//	if (matches.docID() == -1) { matches.nextDoc(); }
//	buffer.growNoCopy(DEFAULT_BULK_BATCH_SIZE);
//	int size = 0;
//	for (int doc = matches.docID(); doc < upTo && size < DEFAULT_BULK_BATCH_SIZE; doc = matches.nextDoc()) {
//	  if (liveDocs == null || liveDocs.get(doc)) { buffer.docs[size++] = doc; }
//	}
//	buffer.size = size;
//	return scorer.bulkScore(buffer.docs, buffer.features, size);
func (b *denseRandomScorerBulk) NextDocsAndScores(
	upTo int, liveDocs interface{ Get(int) bool }, buffer *util.DocAndFloatFeatureBuffer,
) (float32, error) {
	if b.matches.DocID() == -1 {
		if _, err := b.matches.NextDoc(); err != nil {
			return 0, err
		}
	}
	buffer.GrowNoCopy(DefaultBulkBatchSize)
	size := 0
	for doc := b.matches.DocID(); doc < upTo && size < DefaultBulkBatchSize; {
		if liveDocs == nil || liveDocs.Get(doc) {
			buffer.Docs[size] = doc
			size++
		}
		next, err := b.matches.NextDoc()
		if err != nil {
			return 0, err
		}
		doc = next
	}
	buffer.Size = size
	return b.scorer.BulkScore(buffer.Docs, buffer.Features, size)
}

// BulkFromRandomScorerSparse renders the static factory
// {@code VectorScorer.Bulk#fromRandomScorerSparse(RandomVectorScorer,
// KnnVectorValues.DocIndexIterator, DocIdSetIterator)}.
func BulkFromRandomScorerSparse(
	scorer hnsw.RandomVectorScorer, iterator spi.DocIndexIterator, matchingDocs DocIdSetIterator,
) Bulk {
	var matches DocIdSetIterator
	if matchingDocs == nil {
		matches = iterator
	} else {
		matches = CreateConjunction([]DocIdSetIterator{matchingDocs, iterator}, nil)
	}
	return &sparseRandomScorerBulk{
		scorer:   scorer,
		iterator: iterator,
		matches:  matches,
		docIds:   make([]int, 0),
	}
}

// sparseRandomScorerBulk is the anonymous Bulk returned by
// fromRandomScorerSparse. Unlike the dense factory it must buffer the real
// doc IDs separately, because scorer.bulkScore consumes vector ordinals.
type sparseRandomScorerBulk struct {
	scorer   hnsw.RandomVectorScorer
	iterator spi.DocIndexIterator
	matches  DocIdSetIterator
	docIds   []int
}

// NextDocsAndScores reproduces the body of the anonymous Bulk declared by
// fromRandomScorerSparse.
func (b *sparseRandomScorerBulk) NextDocsAndScores(
	upTo int, liveDocs interface{ Get(int) bool }, buffer *util.DocAndFloatFeatureBuffer,
) (float32, error) {
	if b.matches.DocID() == -1 {
		if _, err := b.matches.NextDoc(); err != nil {
			return 0, err
		}
	}
	buffer.GrowNoCopy(DefaultBulkBatchSize)
	b.docIds = growNoCopyInt(b.docIds, DefaultBulkBatchSize)
	size := 0
	for doc := b.matches.DocID(); doc < upTo && size < DefaultBulkBatchSize; {
		if liveDocs == nil || liveDocs.Get(doc) {
			buffer.Docs[size] = b.iterator.Index()
			b.docIds[size] = doc
			size++
		}
		next, err := b.matches.NextDoc()
		if err != nil {
			return 0, err
		}
		doc = next
	}
	buffer.Size = size
	maxScore, err := b.scorer.BulkScore(buffer.Docs, buffer.Features, size)
	if err != nil {
		return 0, err
	}
	// copy back the real doc IDs
	copy(buffer.Docs[:size], b.docIds[:size])
	return maxScore, nil
}

// growNoCopyInt renders org.apache.lucene.util.ArrayUtil#growNoCopy(int[],
// int): a new array of ArrayUtil.oversize(minSize, Integer.BYTES) elements
// when the current one is too small, and the current one otherwise. The
// contents are not preserved, exactly as the Java name states.
func growNoCopyInt(array []int, minSize int) []int {
	if len(array) < minSize {
		return make([]int, util.Oversize(minSize, 4))
	}
	return array
}
