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

// TopDocs represents hits returned by IndexSearcher.Search.
// Mirrors org.apache.lucene.search.TopDocs.
type TopDocs struct {
	TotalHits *TotalHits
	ScoreDocs []*ScoreDoc
}

// NewTopDocs creates a new TopDocs.
func NewTopDocs(totalHits *TotalHits, scoreDocs []*ScoreDoc) *TopDocs {
	return &TopDocs{
		TotalHits: totalHits,
		ScoreDocs: scoreDocs,
	}
}

// TieBreaker is a function that compares two ScoreDocs to break ties.
// Returns negative if a < b, positive if a > b, 0 if equal.
type TieBreaker func(a, b *ScoreDoc) int

// DefaultTieBreaker implements the default Lucene tie-breaking: shardIndex, then docID.
func DefaultTieBreaker(a, b *ScoreDoc) int {
	if a.ShardIndex != b.ShardIndex {
		if a.ShardIndex < b.ShardIndex {
			return -1
		}
		return 1
	}
	if a.Doc < b.Doc {
		return -1
	}
	if a.Doc > b.Doc {
		return 1
	}
	return 0
}

type shardRef struct {
	shardIndex int
	hitIndex   int
}

// tieBreakLessThan returns true if first should come before second.
func tieBreakLessThan(firstRef, secondRef shardRef, firstDoc, secondDoc *ScoreDoc, tieBreaker TieBreaker) bool {
	val := tieBreaker(firstDoc, secondDoc)
	if val == 0 {
		return firstRef.hitIndex < secondRef.hitIndex
	}
	return val < 0
}

// Merge combines multiple TopDocs into one, sorting by score.
// start ignores the top start hits (pagination).
// If tieBreaker is nil, DefaultTieBreaker is used.
func Merge(start, topN int, shardHits []*TopDocs, tieBreaker TieBreaker) *TopDocs {
	if tieBreaker == nil {
		tieBreaker = DefaultTieBreaker
	}

	var totalHitCount int64
	relation := EQUAL_TO
	availHitCount := 0

	// Prepare the heap
	h := &heap{
		less: func(a, b shardRef) bool {
			firstDoc := shardHits[a.shardIndex].ScoreDocs[a.hitIndex]
			secondDoc := shardHits[b.shardIndex].ScoreDocs[b.hitIndex]
			if firstDoc.Score < secondDoc.Score {
				return false
			} else if firstDoc.Score > secondDoc.Score {
				return true
			} else {
				return tieBreakLessThan(a, firstDoc, b, secondDoc, tieBreaker)
			}
		},
	}

	for shardIDX, shard := range shardHits {
		if shard == nil {
			continue
		}
		totalHitCount += shard.TotalHits.Value
		if shard.TotalHits.Relation == GREATER_THAN_OR_EQUAL_TO {
			relation = GREATER_THAN_OR_EQUAL_TO
		}
		if len(shard.ScoreDocs) > 0 {
			availHitCount += len(shard.ScoreDocs)
			h.push(shardRef{shardIndex: shardIDX, hitIndex: 0})
		}
	}

	var hits []*ScoreDoc
	unsetShardIndex := false
	if availHitCount <= start {
		hits = []*ScoreDoc{}
	} else {
		count := topN
		if count > availHitCount-start {
			count = availHitCount - start
		}
		hits = make([]*ScoreDoc, count)
		requestedWindow := start + topN
		numIter := availHitCount
		if requestedWindow < numIter {
			numIter = requestedWindow
		}
		hitUpto := 0
		for hitUpto < numIter {
			ref := h.pop()
			hit := shardHits[ref.shardIndex].ScoreDocs[ref.hitIndex]

			if hitUpto > 0 {
				if unsetShardIndex != (hit.ShardIndex == -1) {
					panic("Inconsistent order of shard indices")
				}
			}
			if hit.ShardIndex == -1 {
				unsetShardIndex = true
			}

			if hitUpto >= start {
				hits[hitUpto-start] = hit
			}
			hitUpto++

			ref.hitIndex++
			if ref.hitIndex < len(shardHits[ref.shardIndex].ScoreDocs) {
				h.push(ref)
			}
		}
	}

	return NewTopDocs(NewTotalHits(totalHitCount, relation), hits)
}

// MergeSimple is a convenience method for merging topN results across provided TopDocs, sorting by score.
func MergeSimple(topN int, shardHits []*TopDocs) *TopDocs {
	return Merge(0, topN, shardHits, nil)
}

// MergeSort merges topN results across the provided sorted TopFieldDocs, ordering by the supplied Sort.
// Each shard's TopFieldDocs must have been produced by the same Sort with sort field values filled.
func MergeSort(sort *Sort, start, topN int, shardHits []*TopFieldDocs) (*TopFieldDocs, error) {
	if sort == nil {
		return nil, errors.New("sort must be non-null when merging field-docs")
	}

	reverseMul := make([]int, len(sort.Fields))
	for i, sf := range sort.Fields {
		if sf.GetReverse() {
			reverseMul[i] = -1
		} else {
			reverseMul[i] = 1
		}
	}

	h := &heap{
		less: func(a, b shardRef) bool {
			fa := shardHits[a.shardIndex].FieldDocs[a.hitIndex]
			fb := shardHits[b.shardIndex].FieldDocs[b.hitIndex]
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
		},
	}

	var totalHitCount int64
	relation := EQUAL_TO
	availHitCount := 0
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
		for h_idx := 0; h_idx < n; h_idx++ {
			if shard.FieldDocs[h_idx] == nil || shard.FieldDocs[h_idx].Fields == nil {
				return nil, fmt.Errorf("shard %d did not set sort field values (FieldDoc.Fields is nil)", shardIDX)
			}
		}
		availHitCount += n
		h.push(shardRef{shardIndex: shardIDX, hitIndex: 0})
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
		unsetShardIndex := false
		hitUpto := 0
		for hitUpto < numIter {
			top := h.pop()
			hit := shardHits[top.shardIndex].FieldDocs[top.hitIndex]

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

			top.hitIndex++
			if top.hitIndex < len(shardHits[top.shardIndex].FieldDocs) {
				h.push(top)
			}
		}
	} else {
		hits = []*FieldDoc{}
	}

	return NewTopFieldDocsWithFieldDocs(NewTotalHits(totalHitCount, relation), hits, sort.Fields), nil
}

// RRF combines multiple TopDocs into a single ranked list using Reciprocal Rank Fusion.
func RRF(topN, k int, hits []*TopDocs) (*TopDocs, error) {
	if topN < 1 {
		return nil, fmt.Errorf("topN must be >= 1, got %d", topN)
	}
	if k < 1 {
		return nil, fmt.Errorf("k must be >= 1, got %d", k)
	}

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
				return nil, errors.New("All hits must either have their ScoreDoc#shardIndex set, or unset (-1), not a mix of both.")
			}
		}
	}

	rrfScore := make(map[shardIndexAndDoc]float64)
	var order []shardIndexAndDoc
	var totalHitCount int64
	for _, topDoc := range hits {
		if topDoc == nil {
			continue
		}
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
			return si > sj
		}
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

	return NewTopDocs(NewTotalHits(totalHitCount, GREATER_THAN_OR_EQUAL_TO), rrfScoreDocs), nil
}

type shardIndexAndDoc struct {
	shardIndex int
	doc        int
}

type heap struct {
	data []shardRef
	less func(a, b shardRef) bool
}

func (h *heap) push(v shardRef) {
	h.data = append(h.data, v)
	h.up(len(h.data) - 1)
}

func (h *heap) pop() shardRef {
	res := h.data[0]
	last := len(h.data) - 1
	h.data[0] = h.data[last]
	h.data = h.data[:last]
	if len(h.data) > 0 {
		h.down(0)
	}
	return res
}

func (h *heap) up(i int) {
	for i > 0 {
		p := (i - 1) / 2
		if h.less(h.data[i], h.data[p]) {
			h.data[i], h.data[p] = h.data[p], h.data[i]
			i = p
		} else {
			break
		}
	}
}

func (h *heap) down(i int) {
	n := len(h.data)
	for {
		l := 2*i + 1
		r := 2*i + 2
		smallest := i
		if l < n && h.less(h.data[l], h.data[smallest]) {
			smallest = l
		}
		if r < n && h.less(h.data[r], h.data[smallest]) {
			smallest = r
		}
		if smallest == i {
			break
		}
		h.data[i], h.data[smallest] = h.data[smallest], h.data[i]
		i = smallest
	}
}

func compareSortValues(t SortFieldType, a, b any) int {
	switch t {
	case SortFieldTypeScore:
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
			return 1
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

func cmpInt64(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func compareFloat32(a, b float32) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func compareFloat64(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
