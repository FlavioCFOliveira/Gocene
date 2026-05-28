// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"errors"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// scoresArraySize is the block size used to split the scores array into
// smaller chunks, avoiding huge single allocations when only a fraction of
// docs match. Mirrors GlobalOrdinalsWithScoreCollector.arraySize in Lucene.
const scoresArraySize = 4096

// ordinalScores stores float32 scores in block-allocated 2-D slices.
// Mirrors org.apache.lucene.search.join.GlobalOrdinalsWithScoreCollector.Scores.
type ordinalScores struct {
	blocks [][]float32
	unset  float32
}

func newOrdinalScores(valueCount int64, unset float32) *ordinalScores {
	nBlocks := int((valueCount+scoresArraySize-1)/scoresArraySize) + 1
	return &ordinalScores{
		blocks: make([][]float32, nBlocks),
		unset:  unset,
	}
}

func (s *ordinalScores) set(ord int, score float32) {
	blk := ord / scoresArraySize
	off := ord % scoresArraySize
	if s.blocks[blk] == nil {
		s.blocks[blk] = make([]float32, scoresArraySize)
		if s.unset != 0 {
			for i := range s.blocks[blk] {
				s.blocks[blk][i] = s.unset
			}
		}
	}
	s.blocks[blk][off] = score
}

func (s *ordinalScores) get(ord int) float32 {
	blk := ord / scoresArraySize
	if s.blocks[blk] == nil {
		return s.unset
	}
	return s.blocks[blk][ord%scoresArraySize]
}

// ordinalOccurrences counts per-ordinal occurrences in block-allocated slices.
// Mirrors org.apache.lucene.search.join.GlobalOrdinalsWithScoreCollector.Occurrences.
type ordinalOccurrences struct {
	blocks [][]int32
}

func newOrdinalOccurrences(valueCount int64) *ordinalOccurrences {
	nBlocks := int((valueCount+scoresArraySize-1)/scoresArraySize) + 1
	return &ordinalOccurrences{blocks: make([][]int32, nBlocks)}
}

func (o *ordinalOccurrences) increment(ord int) {
	blk := ord / scoresArraySize
	if o.blocks[blk] == nil {
		o.blocks[blk] = make([]int32, scoresArraySize)
	}
	o.blocks[blk][ord%scoresArraySize]++
}

func (o *ordinalOccurrences) get(ord int) int32 {
	blk := ord / scoresArraySize
	if o.blocks[blk] == nil {
		return 0
	}
	return o.blocks[blk][ord%scoresArraySize]
}

// GlobalOrdinalsWithScoreCollector is the abstract base collector that
// collects per-global-ordinal scores aggregated across all matching docs.
// Concrete subtypes implement Min, Max, Sum, Avg, and NoScore.
//
// Mirrors org.apache.lucene.search.join.GlobalOrdinalsWithScoreCollector.
type GlobalOrdinalsWithScoreCollector struct {
	field         string
	doMinMax      bool
	min           int
	max           int
	ordinalMap    *index.OrdinalMap
	collectedOrds *util.LongBitSet
	scores        *ordinalScores
	occurrences   *ordinalOccurrences

	// doScore is the per-ordinal score aggregation function.
	doScore func(globalOrd int, existing, newScore float32)
	// scoreOf returns the final score for a global ordinal.
	scoreOf func(globalOrd int) float32
	// scoreMode determines whether scores are collected.
	sMode search.ScoreMode
}

// ErrTooManyGlobalOrdinals is returned when valueCount exceeds math.MaxInt32.
var ErrTooManyGlobalOrdinals = errors.New("GlobalOrdinalsWithScoreCollector: cannot collect more than MaxInt32 ordinals")

func newGlobalOrdinalsWithScoreCollector(
	field string,
	ordinalMap *index.OrdinalMap,
	valueCount int64,
	scoreMode ScoreMode,
	min, max int,
) (*GlobalOrdinalsWithScoreCollector, error) {
	if valueCount > math.MaxInt32 {
		return nil, ErrTooManyGlobalOrdinals
	}
	bs, err := util.NewLongBitSet(valueCount)
	if err != nil {
		return nil, err
	}
	c := &GlobalOrdinalsWithScoreCollector{
		field:         field,
		doMinMax:      min > 1 || max < math.MaxInt32,
		min:           min,
		max:           max,
		ordinalMap:    ordinalMap,
		collectedOrds: bs,
	}
	if scoreMode != None {
		c.scores = newOrdinalScores(valueCount, c.unset(scoreMode))
	}
	if scoreMode == Avg || c.doMinMax {
		c.occurrences = newOrdinalOccurrences(valueCount)
	}
	c.sMode = search.COMPLETE
	return c, nil
}

// unset returns the initial (unset) score value for the given ScoreMode.
func (c *GlobalOrdinalsWithScoreCollector) unset(mode ScoreMode) float32 {
	switch mode {
	case Min:
		return float32(math.Inf(1))
	case Max:
		return float32(math.Inf(-1))
	default:
		return 0
	}
}

// Match reports whether a global ordinal was collected and satisfies the
// occurrence window [min, max].
func (c *GlobalOrdinalsWithScoreCollector) Match(globalOrd int) bool {
	if !c.collectedOrds.Get(int64(globalOrd)) {
		return false
	}
	if c.doMinMax {
		occ := int(c.occurrences.get(globalOrd))
		return occ >= c.min && occ <= c.max
	}
	return true
}

// Score returns the aggregated score for a global ordinal.
func (c *GlobalOrdinalsWithScoreCollector) Score(globalOrd int) float32 {
	if c.scoreOf != nil {
		return c.scoreOf(globalOrd)
	}
	if c.scores != nil {
		return c.scores.get(globalOrd)
	}
	return 1
}

// GetCollectedOrds returns the bitset of collected global ordinals.
func (c *GlobalOrdinalsWithScoreCollector) GetCollectedOrds() *util.LongBitSet {
	return c.collectedOrds
}

// ScoreMode implements search.Collector.
func (c *GlobalOrdinalsWithScoreCollector) ScoreMode() search.ScoreMode {
	return c.sMode
}

// GetLeafCollector implements search.Collector.
func (c *GlobalOrdinalsWithScoreCollector) GetLeafCollector(reader search.IndexReader) (search.LeafCollector, error) {
	var sdv index.SortedDocValues
	var globalOrds []int64

	if lr, ok := reader.(*index.LeafReader); ok {
		var err error
		sdv, err = lr.GetSortedDocValues(c.field)
		if err != nil {
			return nil, err
		}
	}
	if c.ordinalMap != nil {
		// OrdinalMap.Build is now implemented (rmp #4646). Full wiring of
		// GetGlobalOrds(segmentIndex) awaits LeafReaderContext.ord in the
		// search interface (backlog #2703).
		_ = globalOrds
		return &globalOrdsWithScoreLeafCollector{
			sdv:        sdv,
			globalOrds: globalOrds,
			parent:     c,
			withScore:  c.scores != nil,
		}, nil
	}
	return &segmentOrdsWithScoreLeafCollector{
		sdv:       sdv,
		parent:    c,
		withScore: c.scores != nil,
	}, nil
}

// --- leaf collectors ---

type globalOrdsWithScoreLeafCollector struct {
	sdv        index.SortedDocValues
	globalOrds []int64
	parent     *GlobalOrdinalsWithScoreCollector
	scorer     search.Scorer
	withScore  bool
}

func (lc *globalOrdsWithScoreLeafCollector) SetScorer(s search.Scorer) error {
	lc.scorer = s
	return nil
}

func (lc *globalOrdsWithScoreLeafCollector) Collect(doc int) error {
	if lc.sdv == nil {
		return nil
	}
	// Migrated to AdvanceExact + OrdValue (rmp #4709). Collect runs in
	// monotonically increasing doc order, which is the precondition for
	// AdvanceExact.
	ok, err := lc.sdv.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	ord, err := lc.sdv.OrdValue()
	if err != nil {
		return err
	}
	if ord < 0 {
		return nil
	}
	if lc.globalOrds == nil || ord >= len(lc.globalOrds) {
		return nil
	}
	globalOrd := int(lc.globalOrds[ord])
	lc.parent.collectedOrds.Set(int64(globalOrd))
	if lc.withScore && lc.scorer != nil {
		score := lc.scorer.Score()
		existing := lc.parent.scores.get(globalOrd)
		if lc.parent.doScore != nil {
			lc.parent.doScore(globalOrd, existing, score)
		}
	}
	if lc.parent.occurrences != nil {
		lc.parent.occurrences.increment(globalOrd)
	}
	return nil
}

type segmentOrdsWithScoreLeafCollector struct {
	sdv       index.SortedDocValues
	parent    *GlobalOrdinalsWithScoreCollector
	scorer    search.Scorer
	withScore bool
}

func (lc *segmentOrdsWithScoreLeafCollector) SetScorer(s search.Scorer) error {
	lc.scorer = s
	return nil
}

func (lc *segmentOrdsWithScoreLeafCollector) Collect(doc int) error {
	if lc.sdv == nil {
		return nil
	}
	// Migrated to AdvanceExact + OrdValue (rmp #4709). Monotonic Collect.
	ok, err := lc.sdv.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	ord, err := lc.sdv.OrdValue()
	if err != nil {
		return err
	}
	if ord < 0 {
		return nil
	}
	lc.parent.collectedOrds.Set(int64(ord))
	if lc.withScore && lc.scorer != nil {
		score := lc.scorer.Score()
		existing := lc.parent.scores.get(ord)
		if lc.parent.doScore != nil {
			lc.parent.doScore(ord, existing, score)
		}
	}
	if lc.parent.occurrences != nil {
		lc.parent.occurrences.increment(ord)
	}
	return nil
}

// --- concrete subtypes ---

// GlobalOrdinalsWithScoreCollectorMin collects the minimum score per ordinal.
type GlobalOrdinalsWithScoreCollectorMin struct {
	GlobalOrdinalsWithScoreCollector
}

// NewGlobalOrdinalsWithScoreCollectorMin creates a Min collector.
func NewGlobalOrdinalsWithScoreCollectorMin(field string, ordinalMap *index.OrdinalMap, valueCount int64, min, max int) (*GlobalOrdinalsWithScoreCollectorMin, error) {
	base, err := newGlobalOrdinalsWithScoreCollector(field, ordinalMap, valueCount, Min, min, max)
	if err != nil {
		return nil, err
	}
	c := &GlobalOrdinalsWithScoreCollectorMin{*base}
	c.doScore = func(ord int, existing, newScore float32) {
		if newScore < existing {
			c.scores.set(ord, newScore)
		}
	}
	return c, nil
}

// GlobalOrdinalsWithScoreCollectorMax collects the maximum score per ordinal.
type GlobalOrdinalsWithScoreCollectorMax struct {
	GlobalOrdinalsWithScoreCollector
}

// NewGlobalOrdinalsWithScoreCollectorMax creates a Max collector.
func NewGlobalOrdinalsWithScoreCollectorMax(field string, ordinalMap *index.OrdinalMap, valueCount int64, min, max int) (*GlobalOrdinalsWithScoreCollectorMax, error) {
	base, err := newGlobalOrdinalsWithScoreCollector(field, ordinalMap, valueCount, Max, min, max)
	if err != nil {
		return nil, err
	}
	c := &GlobalOrdinalsWithScoreCollectorMax{*base}
	c.doScore = func(ord int, existing, newScore float32) {
		if newScore > existing {
			c.scores.set(ord, newScore)
		}
	}
	return c, nil
}

// GlobalOrdinalsWithScoreCollectorSum accumulates scores per ordinal.
type GlobalOrdinalsWithScoreCollectorSum struct {
	GlobalOrdinalsWithScoreCollector
}

// NewGlobalOrdinalsWithScoreCollectorSum creates a Sum collector.
func NewGlobalOrdinalsWithScoreCollectorSum(field string, ordinalMap *index.OrdinalMap, valueCount int64, min, max int) (*GlobalOrdinalsWithScoreCollectorSum, error) {
	base, err := newGlobalOrdinalsWithScoreCollector(field, ordinalMap, valueCount, Total, min, max)
	if err != nil {
		return nil, err
	}
	c := &GlobalOrdinalsWithScoreCollectorSum{*base}
	c.doScore = func(ord int, existing, newScore float32) {
		c.scores.set(ord, existing+newScore)
	}
	return c, nil
}

// GlobalOrdinalsWithScoreCollectorAvg collects the average score per ordinal.
type GlobalOrdinalsWithScoreCollectorAvg struct {
	GlobalOrdinalsWithScoreCollector
}

// NewGlobalOrdinalsWithScoreCollectorAvg creates an Avg collector.
func NewGlobalOrdinalsWithScoreCollectorAvg(field string, ordinalMap *index.OrdinalMap, valueCount int64, min, max int) (*GlobalOrdinalsWithScoreCollectorAvg, error) {
	base, err := newGlobalOrdinalsWithScoreCollector(field, ordinalMap, valueCount, Avg, min, max)
	if err != nil {
		return nil, err
	}
	c := &GlobalOrdinalsWithScoreCollectorAvg{*base}
	c.doScore = func(ord int, existing, newScore float32) {
		c.scores.set(ord, existing+newScore)
	}
	c.scoreOf = func(ord int) float32 {
		occ := c.occurrences.get(ord)
		if occ == 0 {
			return 0
		}
		return c.scores.get(ord) / float32(occ)
	}
	return c, nil
}

// GlobalOrdinalsWithScoreCollectorNoScore collects without scoring.
type GlobalOrdinalsWithScoreCollectorNoScore struct {
	GlobalOrdinalsWithScoreCollector
}

// NewGlobalOrdinalsWithScoreCollectorNoScore creates a NoScore collector.
func NewGlobalOrdinalsWithScoreCollectorNoScore(field string, ordinalMap *index.OrdinalMap, valueCount int64, min, max int) (*GlobalOrdinalsWithScoreCollectorNoScore, error) {
	base, err := newGlobalOrdinalsWithScoreCollector(field, ordinalMap, valueCount, None, min, max)
	if err != nil {
		return nil, err
	}
	c := &GlobalOrdinalsWithScoreCollectorNoScore{*base}
	c.sMode = search.COMPLETE_NO_SCORES
	c.scoreOf = func(_ int) float32 { return 1 }
	return c, nil
}

// interface compliance
var (
	_ search.Collector     = (*GlobalOrdinalsWithScoreCollectorMin)(nil)
	_ search.Collector     = (*GlobalOrdinalsWithScoreCollectorMax)(nil)
	_ search.Collector     = (*GlobalOrdinalsWithScoreCollectorSum)(nil)
	_ search.Collector     = (*GlobalOrdinalsWithScoreCollectorAvg)(nil)
	_ search.Collector     = (*GlobalOrdinalsWithScoreCollectorNoScore)(nil)
	_ search.LeafCollector = (*globalOrdsWithScoreLeafCollector)(nil)
	_ search.LeafCollector = (*segmentOrdsWithScoreLeafCollector)(nil)
)
