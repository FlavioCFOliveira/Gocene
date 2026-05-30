// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

// Ported from Apache Lucene 10.4.0:
//   lucene/join/src/java/org/apache/lucene/search/join/DiversifyingChildrenFloatKnnVectorQuery.java
//   lucene/join/src/java/org/apache/lucene/search/join/DiversifyingChildrenByteKnnVectorQuery.java
//     (the approximateSearch override that drives reader.searchNearestVectors
//      with a DiversifyingNearestChildrenKnnCollector)

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// knnFloatLeafCollectorSearcher is the structural per-leaf search surface a
// leaf reader exposes to drive a caller-owned KnnCollector through the codec's
// HNSW traversal for float vectors. *index.SegmentReader satisfies it.
type knnFloatLeafCollectorSearcher interface {
	SearchNearestVectorsCollector(field string, target []float32, collector utilhnsw.KnnCollector, acceptDocs util.Bits) error
}

// knnByteLeafCollectorSearcher is the byte analogue of
// [knnFloatLeafCollectorSearcher].
type knnByteLeafCollectorSearcher interface {
	SearchNearestVectorsByteCollector(field string, target []byte, collector utilhnsw.KnnCollector, acceptDocs util.Bits) error
}

// diversifyingApproxFloat runs the collector-driven (HNSW) approximate path
// for a float diversifying query on one leaf. It builds a
// DiversifyingNearestChildrenKnnCollector over the leaf's parent bitset and
// drives it through the codec reader's collector-aware float search, then
// returns the collector's TopDocs.
//
// When the leaf reader does not expose the collector-driven search surface
// (e.g. a mock or codec-less test reader), it returns (nil, false, nil) so the
// caller can fall back to the faithful exact diversifying scan. Any other
// missing precondition (no parents on this leaf) yields an empty TopDocs and
// ok=true, matching Lucene's NO_RESULTS contract.
//
// Mirrors DiversifyingChildrenFloatKnnVectorQuery.approximateSearch.
func diversifyingApproxFloat(
	ctx *index.LeafReaderContext,
	field string,
	target []float32,
	k int,
	parents BitSetProducer,
	acceptDocs search.AcceptDocs,
) (td *search.TopDocs, ok bool, err error) {
	searcher, supported := ctx.Reader().(knnFloatLeafCollectorSearcher)
	if !supported {
		return nil, false, nil
	}
	collector, err := newDiversifyingLeafCollector(ctx, k, parents)
	if err != nil {
		return nil, true, err
	}
	if collector == nil {
		// No parents on this leaf: empty result (NO_RESULTS).
		return search.NewTopDocs(search.NewTotalHits(0, search.EQUAL_TO), nil), true, nil
	}
	bits, err := acceptDocs.Bits()
	if err != nil {
		return nil, true, err
	}
	if err := searcher.SearchNearestVectorsCollector(field, target, &diversifyingHnswCollector{inner: collector}, bits); err != nil {
		return nil, true, err
	}
	return collector.TopDocsSearch(), true, nil
}

// diversifyingApproxByte is the byte analogue of [diversifyingApproxFloat].
//
// Mirrors DiversifyingChildrenByteKnnVectorQuery.approximateSearch.
func diversifyingApproxByte(
	ctx *index.LeafReaderContext,
	field string,
	target []byte,
	k int,
	parents BitSetProducer,
	acceptDocs search.AcceptDocs,
) (td *search.TopDocs, ok bool, err error) {
	searcher, supported := ctx.Reader().(knnByteLeafCollectorSearcher)
	if !supported {
		return nil, false, nil
	}
	collector, err := newDiversifyingLeafCollector(ctx, k, parents)
	if err != nil {
		return nil, true, err
	}
	if collector == nil {
		return search.NewTopDocs(search.NewTotalHits(0, search.EQUAL_TO), nil), true, nil
	}
	bits, err := acceptDocs.Bits()
	if err != nil {
		return nil, true, err
	}
	if err := searcher.SearchNearestVectorsByteCollector(field, target, &diversifyingHnswCollector{inner: collector}, bits); err != nil {
		return nil, true, err
	}
	return collector.TopDocsSearch(), true, nil
}

// newDiversifyingLeafCollector resolves the leaf's parent bitset and builds a
// DiversifyingNearestChildrenKnnCollector for it. It returns (nil, nil) when
// the parent filter selects no documents on this leaf, signalling the
// NO_RESULTS case to the caller.
func newDiversifyingLeafCollector(
	ctx *index.LeafReaderContext,
	k int,
	parents BitSetProducer,
) (*DiversifyingNearestChildrenKnnCollector, error) {
	parentBitSet, err := parents.GetBitSet(ctx)
	if err != nil {
		return nil, err
	}
	if parentBitSet == nil {
		return nil, nil
	}
	utilBits, err := joinFixedBitSetToUtil(parentBitSet)
	if err != nil {
		return nil, err
	}
	// visitLimit is unbounded here: the per-leaf k budget already bounds the
	// result heap and Gocene's join queries do not yet thread a visit budget.
	// Mirrors the Java join queries collecting the global per-segment top-k.
	// The search strategy is not forwarded: it does not affect the result set
	// and the inner collector's stored strategy is unused during collection.
	collector, err := NewDiversifyingNearestChildrenKnnCollector(k, int(^uint(0)>>1), utilBits)
	if err != nil {
		return nil, err
	}
	return collector, nil
}

// joinFixedBitSetToUtil converts a join *FixedBitSet (the BitSetProducer
// output, in leaf-local doc-id space) into a util.FixedBitSet so the
// DiversifyingNearestChildrenKnnCollector can call NextSetBitBounded on it.
// Only the set parent bits (one per block, so a small set) are copied.
func joinFixedBitSetToUtil(bs *FixedBitSet) (util.BitSet, error) {
	out, err := util.NewFixedBitSet(bs.Length())
	if err != nil {
		return nil, err
	}
	for b := bs.NextSetBit(0); b >= 0; b = bs.NextSetBit(b + 1) {
		out.Set(b)
	}
	return out, nil
}

// diversifyingHnswCollector adapts a *DiversifyingNearestChildrenKnnCollector
// to the util/hnsw.KnnCollector interface so it can be driven by the codec's
// HNSW graph traversal. It bridges the two contract differences:
//
//   - Collect on the inner collector returns (bool, error); the inner
//     implementation never returns a non-nil error, so this adapter drops it.
//     The HNSW searcher's Collect must return a plain bool.
//   - The inner TopDocs returns []search.ScoreDoc; this adapter exposes the
//     util/hnsw.TopDocs the searcher's interface requires. The query reads
//     results back through the inner collector (TopDocsSearch), so the
//     util/hnsw.TopDocs here is only consumed if the searcher calls it.
//
// Visit-count bookkeeping is kept on the adapter (matching Lucene's
// AbstractKnnCollector contract: incVisitedCount drives visitedCount /
// earlyTerminated; collect does not), so the inner collector's own counter is
// shadowed and never affects the HNSW traversal's early-termination decision.
type diversifyingHnswCollector struct {
	inner        *DiversifyingNearestChildrenKnnCollector
	visitedCount int64
}

// unboundedVisitLimit is the effective ceiling for the adapter: the join
// queries collect the per-segment global top-k without a visit budget, so the
// limit is Integer.MAX_VALUE-equivalent and EarlyTerminated never fires.
const unboundedVisitLimit = int64(^uint64(0) >> 1)

// Collect translates the searcher's accepted (docID, similarity) into an
// inner Collect, dropping the always-nil error.
func (c *diversifyingHnswCollector) Collect(docID int, similarity float32) bool {
	accepted, _ := c.inner.Collect(docID, similarity)
	return accepted
}

// IncVisitedCount records count additional visited vectors.
func (c *diversifyingHnswCollector) IncVisitedCount(count int) {
	c.visitedCount += int64(count)
}

// VisitedCount returns the visited vector count tracked by this adapter.
func (c *diversifyingHnswCollector) VisitedCount() int64 { return c.visitedCount }

// VisitLimit returns the configured ceiling on visited vectors (unbounded).
func (c *diversifyingHnswCollector) VisitLimit() int64 { return unboundedVisitLimit }

// EarlyTerminated reports whether the visit budget has been exhausted.
func (c *diversifyingHnswCollector) EarlyTerminated() bool {
	return c.visitedCount >= unboundedVisitLimit
}

// K delegates to the inner collector's top-K budget.
func (c *diversifyingHnswCollector) K() int { return c.inner.K() }

// MinCompetitiveSimilarity delegates to the inner collector's threshold.
func (c *diversifyingHnswCollector) MinCompetitiveSimilarity() float32 {
	return c.inner.MinCompetitiveSimilarity()
}

// GetSearchStrategy returns nil so the HNSW searcher falls back to its default
// Hnsw strategy. The util/hnsw.KnnSearchStrategy and search.KnnSearchStrategy
// surfaces are distinct stubs today, and the strategy does not alter the
// result set, so the inner collector's stored strategy is not forwarded.
func (c *diversifyingHnswCollector) GetSearchStrategy() utilhnsw.KnnSearchStrategy {
	return nil
}

// TopDocs returns the inner collector's results as a util/hnsw.TopDocs. The
// query reads results through the inner collector directly; this method exists
// only to satisfy the util/hnsw.KnnCollector interface.
func (c *diversifyingHnswCollector) TopDocs() *utilhnsw.TopDocs {
	docs := c.inner.TopDocs()
	scoreDocs := make([]*utilhnsw.ScoreDoc, len(docs))
	for i, d := range docs {
		scoreDocs[i] = utilhnsw.NewScoreDoc(d.Doc, d.Score)
	}
	return utilhnsw.NewTopDocs(utilhnsw.NewTotalHits(c.visitedCount, utilhnsw.EqualTo), scoreDocs)
}

// Compile-time guard: the adapter satisfies the HNSW collector contract.
var _ utilhnsw.KnnCollector = (*diversifyingHnswCollector)(nil)
