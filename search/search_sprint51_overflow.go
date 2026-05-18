// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// The Sprint 51 search-module port surfaces these types as typed stubs
// so dependent packages keep compiling; concrete behaviour ports land
// progressively in follow-up deep-port sprints. Each stub mirrors a
// distinct org.apache.lucene.search.* type referenced by the Lucene
// 10.4.0 source tree.

// AbstractDocIdSetIterator mirrors
// org.apache.lucene.search.AbstractDocIdSetIterator.
type AbstractDocIdSetIterator struct{}

// NewAbstractDocIdSetIterator builds an AbstractDocIdSetIterator.
func NewAbstractDocIdSetIterator() *AbstractDocIdSetIterator { return &AbstractDocIdSetIterator{} }

// AbstractKnnCollector mirrors
// org.apache.lucene.search.AbstractKnnCollector. Distinct from the
// util/hnsw.KnnCollector / util/hnsw.AbstractKnnCollector helpers,
// which live one layer below the search abstraction.
type AbstractKnnCollector struct{}

// NewAbstractKnnCollector builds an AbstractKnnCollector.
func NewAbstractKnnCollector() *AbstractKnnCollector { return &AbstractKnnCollector{} }

// AcceptDocs mirrors org.apache.lucene.search.AcceptDocs.
type AcceptDocs struct{}

// NewAcceptDocs builds an AcceptDocs.
func NewAcceptDocs() *AcceptDocs { return &AcceptDocs{} }

// BoostAttribute mirrors org.apache.lucene.search.BoostAttribute. This
// is the search-side fuzzy-boost attribute, distinct from the
// analysis-side analysis.BoostAttribute used for token streams.
type BoostAttribute struct{}

// NewBoostAttribute builds a BoostAttribute.
func NewBoostAttribute() *BoostAttribute { return &BoostAttribute{} }

// BoostAttributeImpl mirrors org.apache.lucene.search.BoostAttributeImpl.
type BoostAttributeImpl struct{}

// NewBoostAttributeImpl builds a BoostAttributeImpl.
func NewBoostAttributeImpl() *BoostAttributeImpl { return &BoostAttributeImpl{} }

// ByteVectorSimilarityQuery mirrors
// org.apache.lucene.search.ByteVectorSimilarityQuery.
type ByteVectorSimilarityQuery struct{}

// NewByteVectorSimilarityQuery builds a ByteVectorSimilarityQuery.
func NewByteVectorSimilarityQuery() *ByteVectorSimilarityQuery {
	return &ByteVectorSimilarityQuery{}
}

// CachingCollector mirrors org.apache.lucene.search.CachingCollector.
type CachingCollector struct{}

// NewCachingCollector builds a CachingCollector.
func NewCachingCollector() *CachingCollector { return &CachingCollector{} }

// CheckedIntConsumer mirrors org.apache.lucene.search.CheckedIntConsumer.
// Lucene models this as a @FunctionalInterface; Gocene mirrors the
// surface as a struct so it can be referenced as a typed stub today and
// promoted to a function type once consumers materialise.
type CheckedIntConsumer struct{}

// NewCheckedIntConsumer builds a CheckedIntConsumer.
func NewCheckedIntConsumer() *CheckedIntConsumer { return &CheckedIntConsumer{} }

// CollectionTerminatedException mirrors
// org.apache.lucene.search.CollectionTerminatedException. In Java it is
// a checked exception used as a control-flow signal; the typed stub
// preserves the symbol so callers that pattern-match by name can find
// the destination once a behaviour-level port lands.
type CollectionTerminatedException struct{}

// NewCollectionTerminatedException builds a
// CollectionTerminatedException.
func NewCollectionTerminatedException() *CollectionTerminatedException {
	return &CollectionTerminatedException{}
}
