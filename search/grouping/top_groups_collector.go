package grouping

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/search"
)

type maxScoreCollector struct {
	search.BaseSimpleCollector
	scorer          search.Scorable
	maxScore        float32
	collectedAnyHits bool
}

func (c *maxScoreCollector) ScoreMode() search.ScoreMode {
	return search.ScoreModeComplete
}

func (c *maxScoreCollector) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	return nil
}

func (c *maxScoreCollector) Collect(doc int) error {
	c.collectedAnyHits = true
	score := c.scorer.Score()
	if score > c.maxScore || c.maxScore == -math.MaxFloat32 {
		c.maxScore = score
	}
	return nil
}

func (c *maxScoreCollector) GetMaxScore() float32 {
	if !c.collectedAnyHits {
		return float32(math.NaN())
	}
	return c.maxScore
}

type topDocsAndMaxScoreCollector struct {
	sortedByScore      bool
	topDocsCollector    search.Collector
	maxScoreCollector   *maxScoreCollector
}

type topDocsReducer[T any] struct {
	GroupReducer[T]
	needsScores bool
	supplier    func() *topDocsAndMaxScoreCollector
}

func newTopDocsReducer[T any](withinGroupSort *search.Sort, maxDocsPerGroup int, getMaxScores bool) *topDocsReducer[T] {
	needsScores := getMaxScores || withinGroupSort.NeedsScores()
	var supplier func() *topDocsAndMaxScoreCollector

	if withinGroupSort.IsRelevance() {
		supplier = func() *topDocsAndMaxScoreCollector {
			return &topDocsAndMaxScoreCollector{
				sortedByScore: true,
				topDocsCollector: search.NewTopScoreDocCollector(maxDocsPerGroup, nil),
				maxScoreCollector: nil, // Not needed for relevance? In Java it is null.
			}
		}
	} else {
		supplier = func() *topDocsAndMaxScoreCollector {
			var maxScoreColl *maxScoreCollector
			if getMaxScores {
				maxScoreColl = &maxScoreCollector{maxScore: -math.MaxFloat32}
			}
			return &topDocsAndMaxScoreCollector{
				sortedByScore: false,
				topDocsCollector: search.NewTopFieldCollector(withinGroupSort, maxDocsPerGroup),
				maxScoreCollector: maxScoreColl,
			}
		}
	}

	return &topDocsReducer[T]{
		needsScores: needsScores,
		supplier:    supplier,
	}
}

func (r *topDocsReducer[T]) NeedsScores() bool {
	return r.needsScores
}

func (r *topDocsReducer[T]) newCollector() search.Collector {
	return r.supplier()
}

type TopGroupsCollector[T any] struct {
	SecondPassGroupingCollector[T]
	groupSort        *search.Sort
	withinGroupSort  *search.Sort
	maxDocsPerGroup  int
}

func NewTopGroupsCollector[T any](groupSelector GroupSelector[T], groups []SearchGroup[T], groupSort, withinGroupSort *search.Sort, maxDocsPerGroup int, getMaxScores bool) *TopGroupsCollector[T] {
	reducer := newTopDocsReducer[T](withinGroupSort, maxDocsPerGroup, getMaxScores)
	return &TopGroupsCollector[T]{
		SecondPassGroupingCollector: *NewSecondPassGroupingCollector(groupSelector, groups, &reducer.GroupReducer),
		groupSort:                   groupSort,
		withinGroupSort:             withinGroupSort,
		maxDocsPerGroup:             maxDocsPerGroup,
	}
}

func (c *TopGroupsCollector[T]) GetTopGroups(withinGroupOffset int) *TopGroups[T] {
	groupDocsResult := make([]*GroupDocs[T], len(c.groups))
	maxScore := float32(-math.MaxFloat32)

	for i, group := range c.groups {
		collector := c.groupReducer.GetCollector(group.GroupValue)
		tdmc := collector.(*topDocsAndMaxScoreCollector)

		var topDocs search.TopDocs
		var groupMaxScore float32

		if tdmc.sortedByScore {
			allTopDocs := tdmc.topDocsCollector.(*search.TopScoreDocCollector).TopDocs()
			groupMaxScore = float32(math.NaN())
			if len(allTopDocs.ScoreDocs) > 0 {
				groupMaxScore = allTopDocs.ScoreDocs[0].Score
			}

			if len(allTopDocs.ScoreDocs) <= withinGroupOffset {
				topDocs = search.TopDocs{
					TotalHits:  allTopDocs.TotalHits,
					ScoreDocs: []search.ScoreDoc{},
				}
			} else {
				end := withinGroupOffset + c.maxDocsPerGroup
				if end > len(allTopDocs.ScoreDocs) {
					end = len(allTopDocs.ScoreDocs)
				}
				topDocs = search.TopDocs{
					TotalHits:  allTopDocs.TotalHits,
					ScoreDocs: allTopDocs.ScoreDocs[withinGroupOffset:end],
				}
			}
		} else {
			topDocs = tdmc.topDocsCollector.(*search.TopFieldCollector).TopDocs(withinGroupOffset, c.maxDocsPerGroup)
			if tdmc.maxScoreCollector == nil {
				groupMaxScore = float32(math.NaN())
			} else {
				groupMaxScore = tdmc.maxScoreCollector.GetMaxScore()
			}
		}

		groupDocsResult[i] = &GroupDocs[T]{
			Score:           float32(math.NaN()),
			MaxScore:       groupMaxScore,
			TotalHits:       search.TotalHits{Value: topDocs.TotalHits, Relation: search.TotalHitsEqual},
			ScoreDocs:       topDocs.ScoreDocs,
			GroupValue:      *group.GroupValue,
			GroupSortValues: group.SortValues,
		}
		maxScore = nonNANmax(maxScore, groupMaxScore)
	}

	return NewTopGroups(c.groupSort.Fields, c.withinGroupSort.Fields, c.TotalHitCount, c.TotalGroupedHitCount, groupDocsResult, maxScore)
}
