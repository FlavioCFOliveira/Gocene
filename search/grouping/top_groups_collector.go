// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"errors"
	"math"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TopGroupsCollector is a second-pass collector that collects the TopDocs for
// each group, and returns them as a TopGroups object.
//
// Mirrors org.apache.lucene.search.grouping.TopGroupsCollector<T>, which
// extends SecondPassGroupingCollector<T>.
type TopGroupsCollector[T any] struct {
	*SecondPassGroupingCollector[T]

	groupSort       *search.Sort
	withinGroupSort *search.Sort
	maxDocsPerGroup int
}

// NewTopGroupsCollector creates a new TopGroupsCollector.
//
// groupSelector is the group selector used to define groups, groups are the
// groups to collect TopDocs for, groupSort the order in which groups are
// returned, withinGroupSort the order in which documents are sorted in each
// group, maxDocsPerGroup the maximum number of docs to collect for each group
// and getMaxScores records the maximum score for each group when true.
//
// Mirrors TopGroupsCollector(GroupSelector, Collection, Sort, Sort, int, boolean).
func NewTopGroupsCollector[T any](
	groupSelector GroupSelector[T],
	groups []*SearchGroup[T],
	groupSort *search.Sort,
	withinGroupSort *search.Sort,
	maxDocsPerGroup int,
	getMaxScores bool,
) (*TopGroupsCollector[T], error) {
	if groupSort == nil {
		return nil, errors.New("groupSort must not be null")
	}
	if withinGroupSort == nil {
		return nil, errors.New("withinGroupSort must not be null")
	}
	second, err := NewSecondPassGroupingCollector(
		groupSelector,
		groups,
		newTopDocsReducer[T](withinGroupSort, maxDocsPerGroup, getMaxScores))
	if err != nil {
		return nil, err
	}
	return &TopGroupsCollector[T]{
		SecondPassGroupingCollector: second,
		groupSort:                   groupSort,
		withinGroupSort:             withinGroupSort,
		maxDocsPerGroup:             maxDocsPerGroup,
	}, nil
}

// maxScoreCollector mirrors the private static class
// TopGroupsCollector.MaxScoreCollector.
type maxScoreCollector struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector

	scorer           search.Scorable
	maxScore         float32
	collectedAnyHits bool
}

// newMaxScoreCollector mirrors MaxScoreCollector().
func newMaxScoreCollector() *maxScoreCollector {
	c := &maxScoreCollector{maxScore: math.SmallestNonzeroFloat32}
	c.Outer = c
	return c
}

// getMaxScore mirrors float getMaxScore().
func (c *maxScoreCollector) getMaxScore() float32 {
	if c.collectedAnyHits {
		return c.maxScore
	}
	return float32(math.NaN())
}

// ScoreMode mirrors MaxScoreCollector.scoreMode().
func (c *maxScoreCollector) ScoreMode() search.ScoreMode {
	return search.COMPLETE
}

// SetScorer mirrors MaxScoreCollector.setScorer(Scorable).
func (c *maxScoreCollector) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	return nil
}

// Collect mirrors MaxScoreCollector.collect(int).
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

// CollectRange mirrors the default body of LeafCollector.collectRange(int, int).
func (c *maxScoreCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream mirrors the default body of LeafCollector.collect(DocIdStream).
func (c *maxScoreCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// topDocsCollectorRef renders the Java field type TopDocsCollector<?>: the
// abstract base of TopScoreDocCollector and TopFieldCollector. Gocene's
// search package does not expose that base as a type, so the two members
// TopGroupsCollector needs are named here.
type topDocsCollectorRef interface {
	search.Collector
	TopDocs() *search.TopDocs
}

// rangedTopDocsCollector adds the TopDocsCollector.topDocs(int, int) member,
// which only the field-sorted branch of TopGroupsCollector calls and which
// Gocene declares on TopFieldCollector.
type rangedTopDocsCollector interface {
	topDocsCollectorRef
	TopDocsRange(start, howMany int) *search.TopDocs
}

// topDocsAndMaxScoreCollector mirrors the private static class
// TopGroupsCollector.TopDocsAndMaxScoreCollector, which extends
// FilterCollector.
type topDocsAndMaxScoreCollector struct {
	search.FilterCollector

	topDocsCollector  topDocsCollectorRef
	maxScoreCollector *maxScoreCollector
	sortedByScore     bool
}

// newTopDocsAndMaxScoreCollector mirrors
// TopDocsAndMaxScoreCollector(boolean, TopDocsCollector<?>, MaxScoreCollector).
func newTopDocsAndMaxScoreCollector(
	sortedByScore bool,
	topDocs topDocsCollectorRef,
	maxScore *maxScoreCollector,
) (*topDocsAndMaxScoreCollector, error) {
	var wrapped search.Collector
	var err error
	if maxScore == nil {
		// Java passes a null MaxScoreCollector and MultiCollector.wrap drops
		// it; a typed nil pointer would survive Go's nil check, so the nil
		// case is spelled out.
		wrapped, err = search.MultiCollectorWrap(topDocs)
	} else {
		wrapped, err = search.MultiCollectorWrap(topDocs, maxScore)
	}
	if err != nil {
		return nil, err
	}
	c := &topDocsAndMaxScoreCollector{
		sortedByScore:     sortedByScore,
		topDocsCollector:  topDocs,
		maxScoreCollector: maxScore,
	}
	c.In = wrapped
	return c, nil
}

// topDocsReducer mirrors the private static class
// TopGroupsCollector.TopDocsReducer<T>, which extends
// GroupReducer<T, TopDocsAndMaxScoreCollector>.
type topDocsReducer[T any] struct {
	BaseGroupReducer[T]

	supplier    func() (*topDocsAndMaxScoreCollector, error)
	needsScores bool
}

// newTopDocsReducer mirrors TopDocsReducer(Sort, int, boolean).
func newTopDocsReducer[T any](withinGroupSort *search.Sort, maxDocsPerGroup int, getMaxScores bool) *topDocsReducer[T] {
	r := &topDocsReducer[T]{}
	r.Outer = r
	r.needsScores = getMaxScores || withinGroupSort.NeedsScores()
	if withinGroupSort == search.RELEVANCE {
		r.supplier = func() (*topDocsAndMaxScoreCollector, error) {
			manager, err := search.NewTopScoreDocCollectorManager(maxDocsPerGroup, nil, math.MaxInt32)
			if err != nil {
				return nil, err
			}
			collector, err := manager.NewCollector()
			if err != nil {
				return nil, err
			}
			return newTopDocsAndMaxScoreCollector(true, collector, nil)
		}
	} else {
		r.supplier = func() (*topDocsAndMaxScoreCollector, error) {
			manager, err := search.NewTopFieldCollectorManager(withinGroupSort, maxDocsPerGroup, nil, math.MaxInt32)
			if err != nil {
				return nil, err
			}
			// TODO: disable exact counts?
			topDocsCollector, err := manager.NewCollector()
			if err != nil {
				return nil, err
			}
			var maxScoreCollector *maxScoreCollector
			if getMaxScores {
				maxScoreCollector = newMaxScoreCollector()
			}
			return newTopDocsAndMaxScoreCollector(false, topDocsCollector, maxScoreCollector)
		}
	}
	return r
}

// NeedsScores mirrors TopDocsReducer.needsScores().
func (r *topDocsReducer[T]) NeedsScores() bool {
	return r.needsScores
}

// NewCollector mirrors TopDocsReducer.newCollector().
func (r *topDocsReducer[T]) NewCollector() (search.Collector, error) {
	return r.supplier()
}

// GetTopGroups gets the TopGroups recorded by this collector.
// withinGroupOffset is the offset within each group to start collecting
// documents.
//
// Mirrors TopGroups<T> getTopGroups(int withinGroupOffset).
func (c *TopGroupsCollector[T]) GetTopGroups(withinGroupOffset int) (*TopGroups[T], error) {
	groupDocsResult := make([]*GroupDocs[T], len(c.groups))

	groupIDX := 0
	maxScore := float32(math.SmallestNonzeroFloat32)
	for _, group := range c.groups {
		collector, ok := c.groupReducer.GetCollector(group.GroupValue).(*topDocsAndMaxScoreCollector)
		if !ok {
			return nil, errors.New("group collector is not a TopDocsAndMaxScoreCollector")
		}
		var topDocs *search.TopDocs
		var groupMaxScore float32
		if collector.sortedByScore {
			allTopDocs := collector.topDocsCollector.TopDocs()
			if len(allTopDocs.ScoreDocs) == 0 {
				groupMaxScore = float32(math.NaN())
			} else {
				groupMaxScore = allTopDocs.ScoreDocs[0].Score
			}
			if len(allTopDocs.ScoreDocs) <= withinGroupOffset {
				topDocs = search.NewTopDocs(allTopDocs.TotalHits, []*search.ScoreDoc{})
			} else {
				end := withinGroupOffset + c.maxDocsPerGroup
				if len(allTopDocs.ScoreDocs) < end {
					end = len(allTopDocs.ScoreDocs)
				}
				topDocs = search.NewTopDocs(
					allTopDocs.TotalHits,
					util.CopyOfSubArrayGeneric(allTopDocs.ScoreDocs, withinGroupOffset, end))
			}
		} else {
			ranged, ok := collector.topDocsCollector.(rangedTopDocsCollector)
			if !ok {
				return nil, errors.New("top docs collector does not implement TopDocsCollector.topDocs(int, int)")
			}
			topDocs = ranged.TopDocsRange(withinGroupOffset, c.maxDocsPerGroup)
			if collector.maxScoreCollector == nil {
				groupMaxScore = float32(math.NaN())
			} else {
				groupMaxScore = collector.maxScoreCollector.getMaxScore()
			}
		}

		groupDocsResult[groupIDX] = NewGroupDocs(
			float32(math.NaN()),
			groupMaxScore,
			topDocs.TotalHits,
			topDocs.ScoreDocs,
			group.GroupValue,
			group.SortValues)
		groupIDX++
		maxScore = nonNANmax(maxScore, groupMaxScore)
	}

	return NewTopGroups(
		c.groupSort.GetSort(),
		c.withinGroupSort.GetSort(),
		c.totalHitCount,
		c.totalGroupedHitCount,
		groupDocsResult,
		maxScore), nil
}
