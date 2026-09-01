package search

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

const globalOrdinalArraySize = 4096

// Scores handles score storage for global ordinals using blocks to avoid huge arrays.
type Scores struct {
	blocks [][]float32
	unset  float32
}

func NewScores(valueCount int, unset float32) *Scores {
	numBlocks := (valueCount + globalOrdinalArraySize - 1) / globalOrdinalArraySize
	return &Scores{
		blocks: make([][]float32, numBlocks),
		unset:  unset,
	}
}

func (s *Scores) SetScore(globalOrdinal int, score float32) {
	block := globalOrdinal / globalOrdinalArraySize
	offset := globalOrdinal % globalOrdinalArraySize
	if s.blocks[block] == nil {
		s.blocks[block] = make([]float32, globalOrdinalArraySize)
		if s.unset != 0 {
			for i := range s.blocks[block] {
				s.blocks[block][i] = s.unset
			}
		}
	}
	s.blocks[block][offset] = score
}

func (s *Scores) GetScore(globalOrdinal int) float32 {
	block := globalOrdinal / globalOrdinalArraySize
	offset := globalOrdinal % globalOrdinalArraySize
	if s.blocks[block] == nil {
		return s.unset
	}
	return s.blocks[block][offset]
}

// Occurrences handles occurrence counts for global ordinals using blocks.
type Occurrences struct {
	blocks [][]int
}

func NewOccurrences(valueCount int) *Occurrences {
	numBlocks := (valueCount + globalOrdinalArraySize - 1) / globalOrdinalArraySize
	return &Occurrences{
		blocks: make([][]int, numBlocks),
	}
}

func (o *Occurrences) Increment(globalOrdinal int) {
	block := globalOrdinal / globalOrdinalArraySize
	offset := globalOrdinal % globalOrdinalArraySize
	if o.blocks[block] == nil {
		o.blocks[block] = make([]int, globalOrdinalArraySize)
	}
	o.blocks[block][offset]++
}

func (o *Occurrences) GetOccurrence(globalOrdinal int) int {
	block := globalOrdinal / globalOrdinalArraySize
	offset := globalOrdinal % globalOrdinalArraySize
	if o.blocks[block] == nil {
		return 0
	}
	return o.blocks[block][offset]
}

// GlobalOrdinalsWithScoreCollector is the base for collectors that aggregate scores for global ordinals.
// It mirrors Lucene's org.apache.lucene.search.join.GlobalOrdinalsWithScoreCollector.
type GlobalOrdinalsWithScoreCollector struct {
	field         string
	mode          ScoreMode
	doMinMax      bool
	min           int
	max           int
	ordinalMap    index.OrdinalMap
	collectedOrds util.BitSet
	scores        *Scores
	occurrences   *Occurrences
	doScoreFunc   func(globalOrd int)
	unsetScore    float32
}

func NewGlobalOrdinalsWithScoreCollector(field string, ordinalMap index.OrdinalMap, valueCount int, mode ScoreMode, min, max int) (*GlobalOrdinalsWithScoreCollector, error) {
	if valueCount > math.MaxInt32 {
		return nil, fmt.Errorf("can't collect more than [%d] ids", math.MaxInt32)
	}

	doMinMax := min > 1 || max < math.MaxInt32
	var scores *Scores
	var occurrences *Occurrences

	var unset float32
	var doScore func(int)

	c := &GlobalOrdinalsWithScoreCollector{
		field:         field,
		mode:          mode,
		doMinMax:      doMinMax,
		min:           min,
		max:           max,
		ordinalMap:    ordinalMap,
		collectedOrds: util.NewBitSet(valueCount),
	}

	switch mode {
	case ScoreModeMin:
		unset = math.MaxFloat32
		scores = NewScores(valueCount, unset)
		c.doScoreFunc = func(globalOrd int) {
			// This is a closure that will be called by the leaf collector.
			// We need to access the current score from the scorer.
			// I'll handle this in the leaf collector's Collect method.
		}
	case ScoreModeMax:
		unset = -math.MaxFloat32
		scores = NewScores(valueCount, unset)
	case ScoreModeTotal:
		unset = 0
		scores = NewScores(valueCount, unset)
	case ScoreModeAvg:
		unset = 0
		scores = NewScores(valueCount, unset)
	case ScoreModeNone:
		unset = 0
	default:
		unset = 0
	}

	c.scores = scores
	c.unsetScore = unset

	if mode == ScoreModeAvg || doMinMax {
		occurrences = NewOccurrences(valueCount)
	}
	c.occurrences = occurrences

	return c, nil
}

// Match returns whether the global ordinal matches the criteria.
func (c *GlobalOrdinalsWithScoreCollector) Match(globalOrd int) bool {
	if c.collectedOrds.Get(globalOrd) {
		if c.doMinMax {
			occ := c.occurrences.GetOccurrence(globalOrd)
			return occ >= c.min && occ <= c.max
		}
		return true
	}
	return false
}

// Score returns the aggregated score for the global ordinal.
func (c *GlobalOrdinalsWithScoreCollector) Score(globalOrdinal int) float32 {
	if c.scores == nil {
		return 1.0
	}
	score := c.scores.GetScore(globalOrdinal)
	if c.mode == ScoreModeAvg && c.occurrences != nil {
		occ := c.occurrences.GetOccurrence(globalOrdinal)
		if occ > 0 {
			return score / float32(occ)
		}
	}
	return score
}

func (c *GlobalOrdinalsWithScoreCollector) ScoreMode() ScoreMode {
	return c.mode
}

func (c *GlobalOrdinalsWithScoreCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	docTermOrds, err := index.GetSorted(context.Reader(), c.field)
	if err != nil {
		return nil, err
	}

	if c.ordinalMap != nil {
		segmentOrdToGlobalOrdLookup := c.ordinalMap.GetGlobalOrds(context.Ord)
		return &ordinalMapScoreCollector{
			docTermOrds:                 docTermOrds,
			segmentOrdToGlobalOrdLookup: segmentOrdToGlobalOrdLookup,
			collector:                   c,
		}, nil
	}
	return &segmentOrdinalScoreCollector{
		docTermOrds: docTermOrds,
		collector:   c,
	}, nil
}

func (c *GlobalOrdinalsWithScoreCollector) SetWeight(weight Weight) {}

type ordinalMapScoreCollector struct {
	docTermOrds                 index.SortedDocValues
	segmentOrdToGlobalOrdLookup util.LongValues
	collector                   *GlobalOrdinalsWithScoreCollector
	scorer                      Scorable
}

func (c *ordinalMapScoreCollector) SetScorer(scorer Scorable) error {
	c.scorer = scorer
	return nil
}

func (c *ordinalMapScoreCollector) Collect(doc int) error {
	if c.docTermOrds.AdvanceExact(doc) {
		globalOrd := int(c.segmentOrdToGlobalOrdLookup.Get(c.docTermOrds.OrdValue()))
		c.collector.collectedOrds.Set(globalOrd)

		if c.collector.scores != nil && c.scorer != nil {
			existingScore := c.collector.scores.GetScore(globalOrd)
			newScore, err := c.scorer.Score()
			if err != nil {
				return err
			}
			c.collector.doScore(globalOrd, existingScore, newScore)
		}

		if c.collector.occurrences != nil {
			c.collector.occurrences.Increment(globalOrd)
		}
	}
	return nil
}

func (c *ordinalMapScoreCollector) CollectRange(min, max int) error {
	return nil
}

func (c *ordinalMapScoreCollector) CollectStream(stream DocIdStream) error {
	return nil
}

func (c *ordinalMapScoreCollector) CompetitiveIterator() (DocIdSetIterator, error) {
	return nil, nil
}

func (c *ordinalMapScoreCollector) Finish() error {
	return nil
}

type segmentOrdinalScoreCollector struct {
	docTermOrds index.SortedDocValues
	collector   *GlobalOrdinalsWithScoreCollector
	scorer      Scorable
}

func (c *segmentOrdinalScoreCollector) SetScorer(scorer Scorable) error {
	c.scorer = scorer
	return nil
}

func (c *segmentOrdinalScoreCollector) Collect(doc int) error {
	if c.docTermOrds.AdvanceExact(doc) {
		globalOrd := c.docTermOrds.OrdValue()
		c.collector.collectedOrds.Set(globalOrd)

		if c.collector.scores != nil && c.scorer != nil {
			existingScore := c.collector.scores.GetScore(globalOrd)
			newScore, err := c.scorer.Score()
			if err != nil {
				return err
			}
			c.collector.doScore(globalOrd, existingScore, newScore)
		}

		if c.collector.occurrences != nil {
			c.collector.occurrences.Increment(globalOrd)
		}
	}
	return nil
}

func (c *segmentOrdinalScoreCollector) CollectRange(min, max int) error {
	return nil
}

func (c *segmentOrdinalScoreCollector) CollectStream(stream DocIdStream) error {
	return nil
}

func (c *segmentOrdinalScoreCollector) CompetitiveIterator() (DocIdSetIterator, error) {
	return nil, nil
}

func (c *segmentOrdinalScoreCollector) Finish() error {
	return nil
}

func (c *GlobalOrdinalsWithScoreCollector) doScore(globalOrd int, existingScore, newScore float32) {
	switch c.mode {
	case ScoreModeMin:
		c.scores.SetScore(globalOrd, float32(math.Min(float64(existingScore), float64(newScore))))
	case ScoreModeMax:
		c.scores.SetScore(globalOrd, float32(math.Max(float64(existingScore), float64(newScore))))
	case ScoreModeTotal:
		c.scores.SetScore(globalOrd, existingScore+newScore)
	case ScoreModeAvg:
		c.scores.SetScore(globalOrd, existingScore+newScore)
	}
}
