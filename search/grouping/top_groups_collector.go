// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// Ported from Apache Lucene 10.5.0:
//   lucene/grouping/src/java/org/apache/lucene/search/grouping/TopGroupsCollector.java

// maxScoreCollector renders the private static nested class
// TopGroupsCollector.MaxScoreCollector.
type maxScoreCollector struct {
	search.BaseSimpleCollector

	scorer           search.Scorable
	maxScore         float32
	collectedAnyHits bool
}

func newMaxScoreCollector() *maxScoreCollector {
	// Java initialises maxScore to Float.MIN_VALUE, the smallest positive
	// float, not the most negative one.
	c := &maxScoreCollector{maxScore: math.SmallestNonzeroFloat32}
	c.BaseSimpleCollector.Outer = c
	return c
}

// GetMaxScore mirrors MaxScoreCollector.getMaxScore().
func (c *maxScoreCollector) GetMaxScore() float32 {
	if !c.collectedAnyHits {
		return float32(math.NaN())
	}
	return c.maxScore
}

// ScoreMode mirrors MaxScoreCollector.scoreMode().
func (c *maxScoreCollector) ScoreMode() search.ScoreMode { return search.COMPLETE }

// SetScorer mirrors MaxScoreCollector.setScorer(Scorable).
func (c *maxScoreCollector) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	return nil
}

// Collect mirrors MaxScoreCollector.collect(int), whose body is
// maxScore = Math.max(scorer.score(), maxScore).
func (c *maxScoreCollector) Collect(doc int) error {
	c.collectedAnyHits = true
	score, err := c.scorer.Score()
	if err != nil {
		return err
	}
	if score > c.maxScore {
		c.maxScore = score
	}
	return nil
}

// CollectRange mirrors the LeafCollector default collectRange(int, int).
func (c *maxScoreCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream mirrors the LeafCollector default collect(DocIdStream).
func (c *maxScoreCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// CompetitiveIterator mirrors the LeafCollector default, which returns null.
func (c *maxScoreCollector) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return nil, nil
}

// Finish mirrors the LeafCollector default, whose body is empty.
func (c *maxScoreCollector) Finish() error { return nil }

var _ search.SimpleCollector = (*maxScoreCollector)(nil)

// topDocsAndMaxScoreCollector renders the private static nested class
// TopGroupsCollector.TopDocsAndMaxScoreCollector, a FilterCollector over
// MultiCollector.wrap(topDocsCollector, maxScoreCollector).
type topDocsAndMaxScoreCollector struct {
	search.FilterCollector

	topDocsCollector  search.Collector
	maxScoreCollector *maxScoreCollector
	sortedByScore     bool
}

func newTopDocsAndMaxScoreCollector(sortedByScore bool, topDocsCollector search.Collector, maxScore *maxScoreCollector) (*topDocsAndMaxScoreCollector, error) {
	// MultiCollector.wrap drops nulls; a typed nil pointer is not a nil
	// interface in Go, so the absent collector is passed as an untyped nil.
	var maxScoreAsCollector search.Collector
	if maxScore != nil {
		maxScoreAsCollector = maxScore
	}
	wrapped, err := search.MultiCollectorWrap(topDocsCollector, maxScoreAsCollector)
	if err != nil {
		return nil, err
	}
	return &topDocsAndMaxScoreCollector{
		FilterCollector:   search.FilterCollector{In: wrapped},
		topDocsCollector:  topDocsCollector,
		maxScoreCollector: maxScore,
		sortedByScore:     sortedByScore,
	}, nil
}

var _ search.Collector = (*topDocsAndMaxScoreCollector)(nil)

// newTopDocsReducer renders the private static nested class
// TopGroupsCollector.TopDocsReducer<T>, whose only members are the overrides of
// needsScores() and newCollector(); in Go those are the two values a
// [GroupReducer] is constructed with.
func newTopDocsReducer[T any](withinGroupSort *search.Sort, maxDocsPerGroup int, getMaxScores bool) *GroupReducer[T] {
	needsScores := getMaxScores || withinGroupSort.NeedsScores()

	var supplier func() search.Collector
	if withinGroupSort == search.RELEVANCE {
		// Java tests identity against Sort.RELEVANCE here, not equality.
		supplier = func() search.Collector {
			collector, err := newTopDocsAndMaxScoreCollector(true, search.NewTopScoreDocCollector(maxDocsPerGroup), nil)
			if err != nil {
				panic(err)
			}
			return collector
		}
	} else {
		supplier = func() search.Collector {
			var maxScore *maxScoreCollector
			if getMaxScores {
				maxScore = newMaxScoreCollector()
			}
			collector, err := newTopDocsAndMaxScoreCollector(false, search.NewTopFieldCollector(maxDocsPerGroup, withinGroupSort), maxScore)
			if err != nil {
				panic(err)
			}
			return collector
		}
	}

	return NewGroupReducer[T](supplier, needsScores)
}

// TopGroupsCollector is a second-pass collector that collects the TopDocs for
// each group and returns them as a [TopGroups].
//
// Mirrors org.apache.lucene.search.grouping.TopGroupsCollector<T>.
type TopGroupsCollector[T any] struct {
	*SecondPassGroupingCollector[T]

	groupSort       *search.Sort
	withinGroupSort *search.Sort
	maxDocsPerGroup int
}

// NewTopGroupsCollector creates a new TopGroupsCollector. groupSelector defines
// the groups, groups are the groups to collect TopDocs for, groupSort is the
// order in which groups are returned, withinGroupSort the order in which
// documents are sorted in each group, maxDocsPerGroup the maximum number of
// docs to collect for each group, and getMaxScores records the maximum score
// for each group when true.
func NewTopGroupsCollector[T any](groupSelector GroupSelector[T], groups []SearchGroup[T], groupSort, withinGroupSort *search.Sort, maxDocsPerGroup int, getMaxScores bool) *TopGroupsCollector[T] {
	reducer := newTopDocsReducer[T](withinGroupSort, maxDocsPerGroup, getMaxScores)
	if groupSort == nil {
		panic("groupSort must not be nil")
	}
	if withinGroupSort == nil {
		panic("withinGroupSort must not be nil")
	}
	c := &TopGroupsCollector[T]{
		SecondPassGroupingCollector: NewSecondPassGroupingCollector(groupSelector, groups, reducer),
		groupSort:                   groupSort,
		withinGroupSort:             withinGroupSort,
		maxDocsPerGroup:             maxDocsPerGroup,
	}
	c.SecondPassGroupingCollector.BaseSimpleCollector.Outer = c
	return c
}

// GetTopGroups returns the TopGroups recorded by this collector, starting at
// withinGroupOffset within each group.
//
// Mirrors getTopGroups(int).
func (c *TopGroupsCollector[T]) GetTopGroups(withinGroupOffset int) *TopGroups[T] {
	groupDocsResult := make([]*GroupDocs[T], len(c.groups))

	maxScore := float32(math.SmallestNonzeroFloat32)
	for groupIDX, group := range c.groups {
		collector := c.groupReducer.GetCollector(group.GroupValue).(*topDocsAndMaxScoreCollector)

		var topDocs *search.TopDocs
		var groupMaxScore float32
		var fieldDocs []*search.FieldDoc

		if collector.sortedByScore {
			allTopDocs := collector.topDocsCollector.(*search.TopScoreDocCollector).TopDocs()
			if len(allTopDocs.ScoreDocs) == 0 {
				groupMaxScore = float32(math.NaN())
			} else {
				groupMaxScore = allTopDocs.ScoreDocs[0].Score
			}
			if len(allTopDocs.ScoreDocs) <= withinGroupOffset {
				topDocs = search.NewTopDocs(allTopDocs.TotalHits, []*search.ScoreDoc{})
			} else {
				end := withinGroupOffset + c.maxDocsPerGroup
				if end > len(allTopDocs.ScoreDocs) {
					end = len(allTopDocs.ScoreDocs)
				}
				sliced := make([]*search.ScoreDoc, end-withinGroupOffset)
				copy(sliced, allTopDocs.ScoreDocs[withinGroupOffset:end])
				topDocs = search.NewTopDocs(allTopDocs.TotalHits, sliced)
			}
		} else {
			fieldCollector := collector.topDocsCollector.(*search.TopFieldCollector)
			topDocs = fieldCollector.TopDocsRange(withinGroupOffset, c.maxDocsPerGroup)
			fieldDocs = topFieldDocsSlice(fieldCollector, withinGroupOffset, len(topDocs.ScoreDocs))
			if collector.maxScoreCollector == nil {
				groupMaxScore = float32(math.NaN())
			} else {
				groupMaxScore = collector.maxScoreCollector.GetMaxScore()
			}
		}

		groupDocsResult[groupIDX] = &GroupDocs[T]{
			Score:           float32(math.NaN()),
			MaxScore:        groupMaxScore,
			TotalHits:       topDocs.TotalHits,
			ScoreDocs:       topDocs.ScoreDocs,
			FieldDocs:       fieldDocs,
			GroupValue:      group.GroupValue,
			GroupSortValues: group.SortValues,
		}
		maxScore = nonNANmax(maxScore, groupMaxScore)
	}

	return NewTopGroups(
		c.groupSort.GetSort(),
		c.withinGroupSort.GetSort(),
		c.totalHitCount,
		c.totalGroupedHitCount,
		groupDocsResult,
		maxScore,
	)
}

// topFieldDocsSlice returns the per-hit FieldDocs matching the same window that
// TopDocsRange returned.
//
// In Lucene the ScoreDoc[] of a field-sorted TopDocs already holds FieldDoc
// instances; Gocene's invariant []*ScoreDoc cannot, so [search.TopFieldDocs]
// carries them in a parallel slice and [GroupDocs] does the same.
func topFieldDocsSlice(collector *search.TopFieldCollector, start, howMany int) []*search.FieldDoc {
	all := collector.TopFieldDocs()
	if start < 0 || start >= len(all.FieldDocs) || howMany <= 0 {
		return []*search.FieldDoc{}
	}
	if howMany > len(all.FieldDocs)-start {
		howMany = len(all.FieldDocs) - start
	}
	out := make([]*search.FieldDoc, howMany)
	copy(out, all.FieldDocs[start:start+howMany])
	return out
}

// DoSetNextReader mirrors SecondPassGroupingCollector.doSetNextReader; it is
// re-declared so the embedded base's Outer wiring reaches this type.
func (c *TopGroupsCollector[T]) DoSetNextReader(readerContext *index.LeafReaderContext) error {
	return c.SecondPassGroupingCollector.DoSetNextReader(readerContext)
}
