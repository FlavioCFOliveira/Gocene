package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// GlobalOrdinalsCollector collects all ordinals from a specified field matching the query.
// It mirrors Lucene's org.apache.lucene.search.join.GlobalOrdinalsCollector.
type GlobalOrdinalsCollector struct {
	field         string
	collectedOrds util.BitSet
	ordinalMap    index.OrdinalMap
}

// NewGlobalOrdinalsCollector creates a new GlobalOrdinalsCollector.
func NewGlobalOrdinalsCollector(field string, ordinalMap index.OrdinalMap, valueCount int) *GlobalOrdinalsCollector {
	return &GlobalOrdinalsCollector{
		field:         field,
		ordinalMap:    ordinalMap,
		collectedOrds: util.NewBitSet(valueCount),
	}
}

// GetCollectorOrdinals returns the collected ordinals.
func (c *GlobalOrdinalsCollector) GetCollectorOrdinals() util.BitSet {
	return c.collectedOrds
}

// ScoreMode indicates what features are required from the scorer.
func (c *GlobalOrdinalsCollector) ScoreMode() ScoreMode {
	return ScoreModeCompleteNoScores
}

// GetLeafCollector creates a new LeafCollector to collect the given context.
func (c *GlobalOrdinalsCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	docTermOrds, err := index.GetSorted(context.Reader(), c.field)
	if err != nil {
		return nil, err
	}

	if c.ordinalMap != nil {
		segmentOrdToGlobalOrdLookup := c.ordinalMap.GetGlobalOrds(context.Ord)
		return &ordinalMapCollector{
			docTermOrds:                 docTermOrds,
			segmentOrdToGlobalOrdLookup: segmentOrdToGlobalOrdLookup,
			collector:                   c,
		}, nil
	}
	return &segmentOrdinalCollector{
		docTermOrds: docTermOrds,
		collector:   c,
	}, nil
}

// SetWeight sets the weight that will be used to produce scorers.
func (c *GlobalOrdinalsCollector) SetWeight(weight Weight) {}

type ordinalMapCollector struct {
	docTermOrds                 index.SortedDocValues
	segmentOrdToGlobalOrdLookup util.LongValues
	collector                   *GlobalOrdinalsCollector
}

func (c *ordinalMapCollector) SetScorer(scorer Scorable) error {
	return nil
}

func (c *ordinalMapCollector) Collect(doc int) error {
	if c.docTermOrds.AdvanceExact(doc) {
		segmentOrd := c.docTermOrds.OrdValue()
		globalOrd := c.segmentOrdToGlobalOrdLookup.Get(segmentOrd)
		c.collector.collectedOrds.Set(int(globalOrd))
	}
	return nil
}

func (c *ordinalMapCollector) CollectRange(min, max int) error {
	return nil
}

func (c *ordinalMapCollector) CollectStream(stream DocIdStream) error {
	return nil
}

func (c *ordinalMapCollector) CompetitiveIterator() (DocIdSetIterator, error) {
	return nil, nil
}

func (c *ordinalMapCollector) Finish() error {
	return nil
}

type segmentOrdinalCollector struct {
	docTermOrds index.SortedDocValues
	collector   *GlobalOrdinalsCollector
}

func (c *segmentOrdinalCollector) SetScorer(scorer Scorable) error {
	return nil
}

func (c *segmentOrdinalCollector) Collect(doc int) error {
	if c.docTermOrds.AdvanceExact(doc) {
		c.collector.collectedOrds.Set(int(c.docTermOrds.OrdValue()))
	}
	return nil
}

func (c *segmentOrdinalCollector) CollectRange(min, max int) error {
	return nil
}

func (c *segmentOrdinalCollector) CollectStream(stream DocIdStream) error {
	return nil
}

func (c *segmentOrdinalCollector) CompetitiveIterator() (DocIdSetIterator, error) {
	return nil, nil
}

func (c *segmentOrdinalCollector) Finish() error {
	return nil
}
