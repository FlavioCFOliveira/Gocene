// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
)

// TopDocs holds the top scoring documents.
type TopDocs struct {
	TotalHits *TotalHits
	ScoreDocs []*ScoreDoc
	MaxScore  float32
}

// NewTopDocs creates a new TopDocs.
func NewTopDocs(totalHits *TotalHits, scoreDocs []*ScoreDoc) *TopDocs {
	return &TopDocs{
		TotalHits: totalHits,
		ScoreDocs: scoreDocs,
		MaxScore:  0,
	}
}

// Merge merges multiple TopDocs into one.
func Merge(topDocs []*TopDocs, n int) *TopDocs {
	if len(topDocs) == 0 {
		return nil
	}
	if len(topDocs) == 1 {
		return topDocs[0]
	}

	var totalHits int64
	var maxScore float32
	relation := EQUAL_TO

	// Count total hits and find max score
	for _, td := range topDocs {
		if td == nil {
			continue
		}
		totalHits += td.TotalHits.Value
		if td.TotalHits.Relation == GREATER_THAN_OR_EQUAL_TO {
			relation = GREATER_THAN_OR_EQUAL_TO
		}
		if td.MaxScore > maxScore {
			maxScore = td.MaxScore
		}
	}

	// Simple merge for now: collect all and sort
	// In production, use a priority queue for efficiency
	allDocs := make([]*ScoreDoc, 0)
	for _, td := range topDocs {
		if td != nil {
			allDocs = append(allDocs, td.ScoreDocs...)
		}
	}

	// Sort by score descending, then by doc ID ascending
	sort.Slice(allDocs, func(i, j int) bool {
		if allDocs[i].Score != allDocs[j].Score {
			return allDocs[i].Score > allDocs[j].Score
		}
		return allDocs[i].Doc < allDocs[j].Doc
	})

	// Limit to n
	if len(allDocs) > n {
		allDocs = allDocs[:n]
	}

	return &TopDocs{
		TotalHits: NewTotalHits(totalHits, relation),
		ScoreDocs: allDocs,
		MaxScore:  maxScore,
	}
}

// shardIndexAndDoc is the composite key used by RRF to identify a unique
// document across shards. Mirrors org.apache.lucene.search.TopDocs.ShardIndexAndDoc.
type shardIndexAndDoc struct {
	shardIndex int
	doc        int
}

// RRF combines multiple TopDocs into a single ranked list using Reciprocal
// Rank Fusion. This is especially well suited when combining hits computed via
// different methods, whose score distributions are hardly comparable.
//
// topN is the number of top results to return. k is a constant that determines
// how much influence documents in individual rankings have on the final result;
// a higher value gives lower-rank documents more influence. k must be >= 1.
//
// Port of org.apache.lucene.search.TopDocs.rrf(int topN, int k, TopDocs[] hits)
// (Apache Lucene 10.4.0). It returns an error in place of Java's
// IllegalArgumentException for invalid topN/k or a mix of set/unset shardIndex.
func RRF(topN, k int, hits []*TopDocs) (*TopDocs, error) {
	if topN < 1 {
		return nil, fmt.Errorf("topN must be >= 1, got %d", topN)
	}
	if k < 1 {
		return nil, fmt.Errorf("k must be >= 1, got %d", k)
	}

	// All hits must either have shardIndex set on every ScoreDoc, or unset (-1)
	// on every ScoreDoc — not a mix.
	var shardIndexSet *bool
	for _, topDocs := range hits {
		if topDocs == nil {
			continue
		}
		for _, scoreDoc := range topDocs.ScoreDocs {
			thisShardIndexSet := scoreDoc.ShardIndex != -1
			if shardIndexSet == nil {
				v := thisShardIndexSet
				shardIndexSet = &v
			} else if *shardIndexSet != thisShardIndexSet {
				return nil, errors.New(
					"All hits must either have their ScoreDoc#shardIndex set, or unset (-1), not a mix of both.")
			}
		}
	}

	// Compute the RRF score as a float64 to reduce accuracy loss from
	// floating-point arithmetic, exactly like Lucene.
	rrfScore := make(map[shardIndexAndDoc]float64)
	// Preserve first-seen insertion order so that ties break deterministically
	// the same way Lucene's HashMap-then-sort does (the final sort fully orders
	// equal scores by doc then shardIndex, so insertion order only matters for
	// the rare case of fully-equal keys, which cannot occur since keys are
	// unique).
	var order []shardIndexAndDoc

	var totalHitCount int64
	for _, topDoc := range hits {
		if topDoc == nil {
			continue
		}
		// A document is a hit globally if it is a hit for any of the top docs,
		// so the total hit count is the max total hit count.
		if topDoc.TotalHits != nil && topDoc.TotalHits.Value > totalHitCount {
			totalHitCount = topDoc.TotalHits.Value
		}
		for i, scoreDoc := range topDoc.ScoreDocs {
			rank := i + 1
			contribution := 1.0 / float64(k+rank)
			key := shardIndexAndDoc{shardIndex: scoreDoc.ShardIndex, doc: scoreDoc.Doc}
			if _, seen := rrfScore[key]; !seen {
				order = append(order, key)
			}
			rrfScore[key] += contribution
		}
	}

	ranked := make([]shardIndexAndDoc, len(order))
	copy(ranked, order)
	sort.SliceStable(ranked, func(i, j int) bool {
		ki, kj := ranked[i], ranked[j]
		si, sj := rrfScore[ki], rrfScore[kj]
		if si != sj {
			// Sort by descending score.
			return si > sj
		}
		// Tie-break by doc ID, then shard index (like TopDocs.merge).
		if ki.doc != kj.doc {
			return ki.doc < kj.doc
		}
		return ki.shardIndex < kj.shardIndex
	})

	n := topN
	if n > len(ranked) {
		n = len(ranked)
	}
	rrfScoreDocs := make([]*ScoreDoc, n)
	for i := 0; i < n; i++ {
		key := ranked[i]
		rrfScoreDocs[i] = NewScoreDoc(key.doc, float32(rrfScore[key]), key.shardIndex)
	}

	totalHits := NewTotalHits(totalHitCount, GREATER_THAN_OR_EQUAL_TO)
	return &TopDocs{TotalHits: totalHits, ScoreDocs: rrfScoreDocs}, nil
}

// MergeSort merges topN results across the provided sorted TopFieldDocs,
// ordering by the supplied Sort. Each shard's TopFieldDocs must have been
// produced by the same Sort with sort field values filled (FieldDocs populated).
//
// start ignores the top start hits (pagination). Tie-breaking follows Lucene's
// DEFAULT_TIE_BREAKER: by shardIndex, then docID, then intra-shard hit order.
//
// Port of org.apache.lucene.search.TopDocs.merge(Sort, int start, int topN,
// TopFieldDocs[]) / mergeAux with a non-null sort (Apache Lucene 10.4.0).
func MergeSort(sort *Sort, start, topN int, shardHits []*TopFieldDocs) (*TopFieldDocs, error) {
	if sort == nil {
		return nil, errors.New("sort must be non-null when merging field-docs")
	}

	type shardRef struct {
		shardIndex int
		hitIndex   int
	}

	reverseMul := make([]int, len(sort.Fields))
	for i, sf := range sort.Fields {
		if sf.GetReverse() {
			reverseMul[i] = -1
		} else {
			reverseMul[i] = 1
		}
	}

	// hitAt returns the i-th FieldDoc of a shard (panics-free: validated by caller).
	hitAt := func(s, h int) *FieldDoc { return shardHits[s].FieldDocs[h] }

	// lessThan returns true when the hit referenced by a sorts before b.
	lessThan := func(a, b shardRef) bool {
		fa := hitAt(a.shardIndex, a.hitIndex)
		fb := hitAt(b.shardIndex, b.hitIndex)
		for compIDX := range sort.Fields {
			c := reverseMul[compIDX] * compareSortValues(sort.Fields[compIDX].Type, fa.Fields[compIDX], fb.Fields[compIDX])
			if c != 0 {
				return c < 0
			}
		}
		// DEFAULT_TIE_BREAKER: shardIndex, then docID, then intra-shard hit order.
		if fa.ShardIndex != fb.ShardIndex {
			return fa.ShardIndex < fb.ShardIndex
		}
		if fa.Doc != fb.Doc {
			return fa.Doc < fb.Doc
		}
		return a.hitIndex < b.hitIndex
	}

	var totalHitCount int64
	relation := EQUAL_TO
	availHitCount := 0
	var refs []shardRef
	for shardIDX, shard := range shardHits {
		if shard == nil || shard.TopDocs == nil {
			continue
		}
		if shard.TotalHits != nil {
			totalHitCount += shard.TotalHits.Value
			if shard.TotalHits.Relation == GREATER_THAN_OR_EQUAL_TO {
				relation = GREATER_THAN_OR_EQUAL_TO
			}
		}
		n := len(shard.FieldDocs)
		if n == 0 {
			continue
		}
		// Validate the API contract: every hit must carry sort field values.
		for h := 0; h < n; h++ {
			if shard.FieldDocs[h] == nil || shard.FieldDocs[h].Fields == nil {
				return nil, fmt.Errorf("shard %d did not set sort field values (FieldDoc.Fields is nil)", shardIDX)
			}
		}
		availHitCount += n
		refs = append(refs, shardRef{shardIndex: shardIDX, hitIndex: 0})
	}

	var hits []*FieldDoc
	if availHitCount > start {
		want := topN
		if want > availHitCount-start {
			want = availHitCount - start
		}
		hits = make([]*FieldDoc, want)
		requestedWindow := start + topN
		numIter := availHitCount
		if requestedWindow < numIter {
			numIter = requestedWindow
		}
		// A binary heap keyed by lessThan, mirroring Lucene's PriorityQueue.
		heapRefs := make([]shardRef, len(refs))
		copy(heapRefs, refs)
		heapInit(heapRefs, lessThan)

		unsetShardIndex := false
		hitUpto := 0
		for hitUpto < numIter {
			top := heapRefs[0]
			hit := hitAt(top.shardIndex, top.hitIndex)

			if hitUpto > 0 {
				if unsetShardIndex != (hit.ShardIndex == -1) {
					return nil, errors.New("Inconsistent order of shard indices")
				}
			}
			if hit.ShardIndex == -1 {
				unsetShardIndex = true
			}

			if hitUpto >= start {
				hits[hitUpto-start] = hit
			}
			hitUpto++

			heapRefs[0].hitIndex++
			if heapRefs[0].hitIndex < len(shardHits[top.shardIndex].FieldDocs) {
				heapDownShard(heapRefs, 0, lessThan)
			} else {
				// pop
				last := len(heapRefs) - 1
				heapRefs[0] = heapRefs[last]
				heapRefs = heapRefs[:last]
				if len(heapRefs) > 0 {
					heapDownShard(heapRefs, 0, lessThan)
				}
			}
		}
	} else {
		hits = []*FieldDoc{}
	}

	totalHits := NewTotalHits(totalHitCount, relation)
	return NewTopFieldDocsWithFieldDocs(totalHits, hits, sort.Fields), nil
}

// heapInit builds a min-heap (by less) over refs in place.
func heapInit[T any](a []T, less func(a, b T) bool) {
	for i := len(a)/2 - 1; i >= 0; i-- {
		heapDownGen(a, i, less)
	}
}

// heapDownShard restores the heap property at index i for the shardRef heap.
// It is a thin wrapper kept separate so the closure-typed less is monomorphic.
func heapDownShard[T any](a []T, i int, less func(a, b T) bool) { heapDownGen(a, i, less) }

// heapDownGen sifts element i down to restore the min-heap property.
func heapDownGen[T any](a []T, i int, less func(a, b T) bool) {
	n := len(a)
	for {
		l := 2*i + 1
		r := 2*i + 2
		smallest := i
		if l < n && less(a[l], a[smallest]) {
			smallest = l
		}
		if r < n && less(a[r], a[smallest]) {
			smallest = r
		}
		if smallest == i {
			return
		}
		a[i], a[smallest] = a[smallest], a[i]
		i = smallest
	}
}

// compareSortValues compares two cached sort values of the given SortField type,
// returning a Java-style sign: negative if a < b. Mirrors the per-type
// FieldComparator.compareValues used by TopDocs.MergeSortQueue. For STRING,
// missing values (nil) sort last (matching the default DocValues comparator).
func compareSortValues(t SortFieldType, a, b any) int {
	switch t {
	case SortFieldTypeScore:
		// Reversed intentionally: relevance by default sorts descending, so
		// RelevanceComparator.compareValues(first, second) == compare(second,
		// first) (Lucene 10.4.0 FieldComparator.RelevanceComparator). The
		// reverseMul the caller applies (reverse=true for the default score
		// sort) flips this back to descending.
		return compareFloat32(toFloat32(b), toFloat32(a))
	case SortFieldTypeDoc:
		return compareInt(toInt(a), toInt(b))
	case SortFieldTypeInt:
		return cmpInt64(int64(toInt32(a)), int64(toInt32(b)))
	case SortFieldTypeLong:
		return cmpInt64(toInt64(a), toInt64(b))
	case SortFieldTypeFloat:
		return compareFloat32(toFloat32(a), toFloat32(b))
	case SortFieldTypeDouble:
		return compareFloat64(toFloat64(a), toFloat64(b))
	case SortFieldTypeString:
		ba, _ := a.([]byte)
		bb, _ := b.([]byte)
		if ba == nil && bb == nil {
			return 0
		}
		if ba == nil {
			return 1 // missing sorts last
		}
		if bb == nil {
			return -1
		}
		return bytes.Compare(ba, bb)
	default:
		return 0
	}
}

func compareInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	}
	return 0
}

func toInt32(v any) int32 {
	switch x := v.(type) {
	case int32:
		return x
	case int:
		return int32(x)
	case int64:
		return int32(x)
	}
	return 0
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int32:
		return int64(x)
	case int:
		return int64(x)
	}
	return 0
}

func toFloat32(v any) float32 {
	switch x := v.(type) {
	case float32:
		return x
	case float64:
		return float32(x)
	}
	return 0
}

func toFloat64(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	}
	return 0
}

// MergeWithStart merges multiple TopDocs with pagination support (from/size).
// This is equivalent to TopDocs.merge(from, size, topDocs) in Lucene.
// Returns an error if shard indices are inconsistent (some set, some not).
func MergeWithStart(from, size int, topDocs []*TopDocs) (*TopDocs, error) {
	if len(topDocs) == 0 {
		return nil, nil
	}
	if len(topDocs) == 1 {
		return topDocs[0], nil
	}

	// Check for consistent shard indices
	hasSetShardIndex := false
	hasUnsetShardIndex := false
	for _, td := range topDocs {
		if td == nil {
			continue
		}
		for _, sd := range td.ScoreDocs {
			if sd.ShardIndex >= 0 {
				hasSetShardIndex = true
			} else {
				hasUnsetShardIndex = true
			}
		}
	}

	// Inconsistent shard indices - some set, some not
	if hasSetShardIndex && hasUnsetShardIndex {
		return nil, errors.New("inconsistent shard indices: some ScoreDocs have shardIndex set, others do not")
	}

	var totalHits int64
	var maxScore float32
	relation := EQUAL_TO

	// Count total hits and find max score
	for _, td := range topDocs {
		if td == nil {
			continue
		}
		totalHits += td.TotalHits.Value
		if td.TotalHits.Relation == GREATER_THAN_OR_EQUAL_TO {
			relation = GREATER_THAN_OR_EQUAL_TO
		}
		if td.MaxScore > maxScore {
			maxScore = td.MaxScore
		}
	}

	// Collect all docs
	allDocs := make([]*ScoreDoc, 0)
	for _, td := range topDocs {
		if td != nil {
			allDocs = append(allDocs, td.ScoreDocs...)
		}
	}

	// Sort by score descending, then by doc ID ascending
	sort.Slice(allDocs, func(i, j int) bool {
		if allDocs[i].Score != allDocs[j].Score {
			return allDocs[i].Score > allDocs[j].Score
		}
		return allDocs[i].Doc < allDocs[j].Doc
	})

	// Apply from/size pagination
	if from < len(allDocs) {
		end := from + size
		if end > len(allDocs) {
			end = len(allDocs)
		}
		allDocs = allDocs[from:end]
	} else {
		allDocs = make([]*ScoreDoc, 0)
	}

	return &TopDocs{
		TotalHits: NewTotalHits(totalHits, relation),
		ScoreDocs: allDocs,
		MaxScore:  maxScore,
	}, nil
}
