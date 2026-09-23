// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockGroupingCollector performs grouping with a single pass collector, as
// long as you are grouping by a doc block field, ie all documents sharing a
// given group value were indexed as a doc block using the atomic
// IndexWriter.addDocuments() or IndexWriter.updateDocuments() API.
//
// This results in faster performance (~25% faster QPS) than the two-pass
// grouping collectors, with the tradeoff being that the documents in each
// group must always be indexed as a block. This collector also fills in
// TopGroups.totalGroupCount without requiring the separate
// AllGroupsCollector. However, this collector does not fill in the groupValue
// of each group; this field will always be null.
//
// NOTE: this collector makes no effort to verify the docs were in fact
// indexed as a block, so it's up to you to ensure this was the case.
//
// Mirrors org.apache.lucene.search.grouping.BlockGroupingCollector (Apache
// Lucene 10.5.0), which extends SimpleCollector.
//
// lucene.experimental
type BlockGroupingCollector struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector

	pendingSubDocs   []int
	pendingSubScores []float32
	subDocUpto       int

	groupSort       *search.Sort
	topNGroups      int
	lastDocPerGroup search.Weight

	// TODO: specialize into 2 classes, static "create" method:
	needsScores bool

	comparators     []search.FieldComparator
	leafComparators []search.LeafFieldComparator
	reversed        []int
	compIDXEnd      int
	bottomSlot      int
	queueFull       bool

	currentReaderContext *index.LeafReaderContext

	topGroupDoc         int
	totalHitCount       int
	totalGroupCount     int
	docBase             int
	groupEndDocID       int
	lastDocPerGroupBits search.DocIdSetIterator
	scorer              search.Scorable
	groupQueue          *util.PriorityQueue[*oneGroup]
	groupCompetes       bool
}

// oneGroup renders the private static final class OneGroup.
type oneGroup struct {
	readerContext *index.LeafReaderContext
	// int groupOrd;
	topGroupDoc    int
	docs           []int
	scores         []float32
	count          int
	comparatorSlot int
}

// NewBlockGroupingCollector creates the single pass collector.
//
// groupSort is the Sort used to sort the groups; the top sorted document
// within each group according to groupSort determines how that group sorts
// against other groups. This must be non-null, ie, if you want to groupSort
// by relevance use search.RELEVANCE. topNGroups is how many top groups to
// keep. needsScores is true if the collected documents require scores, either
// because relevance is included in the withinGroupSort or because you plan
// to pass true for either getScores or getMaxScores to GetTopGroups.
// lastDocPerGroup is a Weight that marks the last document in each group.
//
// Mirrors BlockGroupingCollector(Sort, int, boolean, Weight); Java's
// IllegalArgumentException is returned as an error.
func NewBlockGroupingCollector(groupSort *search.Sort, topNGroups int, needsScores bool,
	lastDocPerGroup search.Weight) (*BlockGroupingCollector, error) {
	if topNGroups < 1 {
		return nil, fmt.Errorf("topNGroups must be >= 1 (got %d)", topNGroups)
	}

	c := &BlockGroupingCollector{}
	pendingSubDocs := make([]int, 10)
	c.pendingSubDocs = pendingSubDocs
	if needsScores {
		c.pendingSubScores = make([]float32, 10)
	}

	c.needsScores = needsScores
	c.lastDocPerGroup = lastDocPerGroup

	c.groupSort = groupSort

	c.topNGroups = topNGroups

	sortFields := groupSort.GetSort()
	c.comparators = make([]search.FieldComparator, len(sortFields))
	c.leafComparators = make([]search.LeafFieldComparator, len(sortFields))
	c.compIDXEnd = len(c.comparators) - 1
	c.reversed = make([]int, len(sortFields))
	for i, sortField := range sortFields {
		c.comparators[i] = search.SortFieldGetComparator(sortField, topNGroups, search.PruningNone)
		if sortField.Reverse {
			c.reversed[i] = -1
		} else {
			c.reversed[i] = 1
		}
	}

	groupQueue, err := util.NewPriorityQueue(topNGroups, c.groupQueueLessThan)
	if err != nil {
		return nil, err
	}
	c.groupQueue = groupQueue
	c.Outer = c
	return c, nil
}

// groupQueueLessThan renders GroupQueue.lessThan(OneGroup, OneGroup): sorts
// by groupSort. Not static -- uses comparators, reversed.
func (c *BlockGroupingCollector) groupQueueLessThan(group1, group2 *oneGroup) bool {
	if util.AssertsEnabled() {
		if group1 == group2 {
			panic(util.NewAssertionError("group1 != group2"))
		}
		if group1.comparatorSlot == group2.comparatorSlot {
			panic(util.NewAssertionError("group1.comparatorSlot != group2.comparatorSlot"))
		}
	}

	numComparators := len(c.comparators)
	for compIDX := 0; compIDX < numComparators; compIDX++ {
		cmp := c.reversed[compIDX] * c.comparators[compIDX].Compare(group1.comparatorSlot, group2.comparatorSlot)
		if cmp != 0 {
			// Short circuit
			return cmp > 0
		}
	}

	// Break ties by docID; lower docID is always sorted first
	return group1.topGroupDoc > group2.topGroupDoc
}

// processGroup is called when we transition to another group; if the group is
// competitive we insert into the group queue.
func (c *BlockGroupingCollector) processGroup() error {
	c.totalGroupCount++
	if c.groupCompetes {
		if !c.queueFull {
			// Startup transient: always add a new OneGroup
			og := &oneGroup{}
			og.count = c.subDocUpto
			og.topGroupDoc = c.docBase + c.topGroupDoc
			og.docs = c.pendingSubDocs
			c.pendingSubDocs = make([]int, 10)
			if c.needsScores {
				og.scores = c.pendingSubScores
				c.pendingSubScores = make([]float32, 10)
			}
			og.readerContext = c.currentReaderContext
			// og.groupOrd = lastGroupOrd;
			og.comparatorSlot = c.bottomSlot
			c.groupQueue.Add(og)
			bottomGroup := c.groupQueue.Top()
			c.queueFull = c.groupQueue.Size() == c.topNGroups
			if c.queueFull {
				// Queue just became full; now set the real bottom
				// in the comparators:
				c.bottomSlot = bottomGroup.comparatorSlot
				for i := range c.comparators {
					if err := c.leafComparators[i].SetBottom(c.bottomSlot); err != nil {
						return err
					}
				}
			} else {
				// Queue not full yet -- just advance bottomSlot:
				c.bottomSlot = c.groupQueue.Size()
			}
		} else {
			// Replace bottom element in PQ and then updateTop
			og := c.groupQueue.Top()
			if util.AssertsEnabled() && og == nil {
				return util.NewAssertionError("og != null")
			}
			og.count = c.subDocUpto
			og.topGroupDoc = c.docBase + c.topGroupDoc
			// Swap pending docs
			savDocs := og.docs
			og.docs = c.pendingSubDocs
			c.pendingSubDocs = savDocs
			if c.needsScores {
				// Swap pending scores
				savScores := og.scores
				og.scores = c.pendingSubScores
				c.pendingSubScores = savScores
			}
			og.readerContext = c.currentReaderContext
			// og.groupOrd = lastGroupOrd;
			c.groupQueue.UpdateTop()
			c.bottomSlot = c.groupQueue.Top().comparatorSlot

			for i := range c.comparators {
				if err := c.leafComparators[i].SetBottom(c.bottomSlot); err != nil {
					return err
				}
			}
		}
	}
	c.subDocUpto = 0
	return nil
}

// topDocsCollectorWithRange is the part of TopDocsCollector<?> that
// GetTopGroups drives: the collector itself and topDocs(int, int). Gocene
// declares TopDocsCollector.topDocs(int, int) as TopDocsRange on both
// TopScoreDocCollector and TopFieldCollector.
type topDocsCollectorWithRange interface {
	search.Collector
	TopDocsRange(start, howMany int) *search.TopDocs
}

// GetTopGroups returns the grouped results. Returns nil if the number of
// groups collected is <= groupOffset.
//
// NOTE: This collector is unable to compute the groupValue per group so it
// will always be null. This is normally not a problem, as you can obtain the
// value just like you obtain other values for each matching document (eg, via
// stored fields, via DocValues, etc.)
//
// withinGroupSort is the Sort used to sort documents within each group;
// groupOffset which group to start from; withinGroupOffset which document to
// start from within each group; maxDocsPerGroup how many top documents to
// keep within each group.
//
// Mirrors TopGroups<?> getTopGroups(Sort, int, int, int); the wildcard
// TopGroups<?> is rendered as TopGroups[any], whose group values are all nil.
func (c *BlockGroupingCollector) GetTopGroups(withinGroupSort *search.Sort, groupOffset, withinGroupOffset,
	maxDocsPerGroup int) (*TopGroups[any], error) {
	if groupOffset >= c.groupQueue.Size() {
		return nil, nil
	}
	totalGroupedHitCount := 0

	fakeScorer := &blockGroupingScore{}

	maxScore := float32(math.SmallestNonzeroFloat32)

	groupSortByRelevance := c.groupSort.Equals(search.RELEVANCE)

	groups := make([]*GroupDocs[any], c.groupQueue.Size()-groupOffset)
	for downTo := c.groupQueue.Size() - groupOffset - 1; downTo >= 0; downTo-- {
		og := c.groupQueue.Pop()

		// At this point we hold all docs w/ in each group,
		// unsorted; we now sort them:
		var collector topDocsCollectorWithRange
		withinGroupSortByRelevance := withinGroupSort.Equals(search.RELEVANCE)
		if withinGroupSortByRelevance {
			// Sort by score
			if !c.needsScores {
				return nil, fmt.Errorf("cannot sort by relevance within group: needsScores=false")
			}
			manager, err := search.NewTopScoreDocCollectorManager(maxDocsPerGroup, nil, math.MaxInt32)
			if err != nil {
				return nil, err
			}
			tsdc, err := manager.NewCollector()
			if err != nil {
				return nil, err
			}
			collector = tsdc
		} else {
			// Sort by fields
			manager, err := search.NewTopFieldCollectorManager(withinGroupSort, maxDocsPerGroup, nil, math.MaxInt32)
			if err != nil {
				return nil, err
			}
			tfc, err := manager.NewCollector() // TODO: disable exact counts?
			if err != nil {
				return nil, err
			}
			collector = tfc
		}

		groupMaxScore := float32(math.NaN())
		if c.needsScores {
			groupMaxScore = float32(math.Inf(-1))
		}
		leafCollector, err := collector.GetLeafCollector(og.readerContext)
		if err != nil {
			return nil, err
		}
		if err := leafCollector.SetScorer(fakeScorer); err != nil {
			return nil, err
		}
		for docIDX := 0; docIDX < og.count; docIDX++ {
			doc := og.docs[docIDX]
			if c.needsScores {
				fakeScorer.score = og.scores[docIDX]
				if !withinGroupSortByRelevance {
					groupMaxScore = javaMathMaxFloat(groupMaxScore, fakeScorer.score)
				}
			}
			if err := leafCollector.Collect(doc); err != nil {
				return nil, err
			}
		}
		totalGroupedHitCount += og.count

		groupSortValues := make([]any, len(c.comparators))
		for sortFieldIDX := range c.comparators {
			groupSortValues[sortFieldIDX] = c.comparators[sortFieldIDX].Value(og.comparatorSlot)
		}

		topDocs := collector.TopDocsRange(withinGroupOffset, maxDocsPerGroup)
		if withinGroupSortByRelevance && len(topDocs.ScoreDocs) > 0 {
			groupMaxScore = topDocs.ScoreDocs[0].Score
		}

		// TODO: we could aggregate scores across children
		// by Sum/Avg instead of passing NaN:
		groups[downTo] = NewGroupDocs[any](
			float32(math.NaN()),
			groupMaxScore,
			search.NewTotalHits(int64(og.count), search.EQUAL_TO),
			topDocs.ScoreDocs,
			nil,
			groupSortValues)
		if !groupSortByRelevance {
			maxScore = javaMathMaxFloat(maxScore, groupMaxScore)
		}
	}

	if groupSortByRelevance {
		maxScore = groups[0].MaxScore
	}

	totalGroupCount := c.totalGroupCount
	return NewTopGroupsWithTotalGroupCount(
		NewTopGroups(
			c.groupSort.GetSort(),
			withinGroupSort.GetSort(),
			c.totalHitCount,
			totalGroupedHitCount,
			groups,
			maxScore),
		&totalGroupCount), nil
}

// javaMathMaxFloat renders Math.max(float, float): NaN if either argument is
// NaN, and +0 over -0.
func javaMathMaxFloat(a, b float32) float32 {
	if a != a {
		return a
	}
	if b != b {
		return b
	}
	if a == 0 && b == 0 {
		if math.Signbit(float64(a)) {
			return b
		}
		return a
	}
	if a >= b {
		return a
	}
	return b
}

// GetLeafCollector renders SimpleCollector.getLeafCollector: doSetNextReader,
// then return this.
func (c *BlockGroupingCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if err := c.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return c, nil
}

// SetScorer renders setScorer(Scorable).
func (c *BlockGroupingCollector) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	for _, comparator := range c.leafComparators {
		if err := comparator.SetScorer(scorer); err != nil {
			return err
		}
	}
	return nil
}

// Collect renders collect(int).
func (c *BlockGroupingCollector) Collect(doc int) error {
	if doc > c.groupEndDocID {
		// Group changed
		if c.subDocUpto != 0 {
			if err := c.processGroup(); err != nil {
				return err
			}
		}
		groupEndDocID, err := c.lastDocPerGroupBits.Advance(doc)
		if err != nil {
			return err
		}
		c.groupEndDocID = groupEndDocID
		c.subDocUpto = 0
		c.groupCompetes = !c.queueFull
	}

	c.totalHitCount++

	// Always cache doc/score within this group:
	if c.subDocUpto == len(c.pendingSubDocs) {
		c.pendingSubDocs = util.GrowInRange(c.pendingSubDocs, len(c.pendingSubDocs)+1, math.MaxInt32)
	}
	c.pendingSubDocs[c.subDocUpto] = doc
	if c.needsScores {
		if c.subDocUpto == len(c.pendingSubScores) {
			c.pendingSubScores = util.GrowInRangeFloat(c.pendingSubScores, len(c.pendingSubScores)+1, math.MaxInt32)
		}
		score, err := c.scorer.Score()
		if err != nil {
			return err
		}
		c.pendingSubScores[c.subDocUpto] = score
	}
	c.subDocUpto++

	if c.groupCompetes {
		if c.subDocUpto == 1 {
			if util.AssertsEnabled() && c.queueFull {
				return util.NewAssertionError("!queueFull")
			}

			for _, fc := range c.leafComparators {
				if err := fc.Copy(c.bottomSlot, doc); err != nil {
					return err
				}
				if err := fc.SetBottom(c.bottomSlot); err != nil {
					return err
				}
			}
			c.topGroupDoc = doc
		} else {
			// Compare to bottomSlot
			for compIDX := 0; ; compIDX++ {
				cb, err := c.leafComparators[compIDX].CompareBottom(doc)
				if err != nil {
					return err
				}
				cmp := c.reversed[compIDX] * cb
				if cmp < 0 {
					// Definitely not competitive -- done
					return nil
				} else if cmp > 0 {
					// Definitely competitive.
					break
				} else if compIDX == c.compIDXEnd {
					// Ties with bottom, except we know this docID is
					// > docID in the queue (docs are visited in
					// order), so not competitive:
					return nil
				}
			}

			for _, fc := range c.leafComparators {
				if err := fc.Copy(c.bottomSlot, doc); err != nil {
					return err
				}
				// Necessary because some comparators cache
				// details of bottom slot; this forces them to
				// re-cache:
				if err := fc.SetBottom(c.bottomSlot); err != nil {
					return err
				}
			}
			c.topGroupDoc = doc
		}
	} else {
		// We're not sure this group will make it into the
		// queue yet
		for compIDX := 0; ; compIDX++ {
			cb, err := c.leafComparators[compIDX].CompareBottom(doc)
			if err != nil {
				return err
			}
			cmp := c.reversed[compIDX] * cb
			if cmp < 0 {
				// Definitely not competitive -- done
				return nil
			} else if cmp > 0 {
				// Definitely competitive.
				break
			} else if compIDX == c.compIDXEnd {
				// Ties with bottom, except we know this docID is
				// > docID in the queue (docs are visited in
				// order), so not competitive:
				return nil
			}
		}
		c.groupCompetes = true
		for _, fc := range c.leafComparators {
			if err := fc.Copy(c.bottomSlot, doc); err != nil {
				return err
			}
			// Necessary because some comparators cache
			// details of bottom slot; this forces them to
			// re-cache:
			if err := fc.SetBottom(c.bottomSlot); err != nil {
				return err
			}
		}
		c.topGroupDoc = doc
	}
	return nil
}

// DoSetNextReader renders doSetNextReader(LeafReaderContext).
func (c *BlockGroupingCollector) DoSetNextReader(readerContext *index.LeafReaderContext) error {
	c.subDocUpto = 0
	c.docBase = readerContext.DocBase
	s, err := c.lastDocPerGroup.Scorer(readerContext)
	if err != nil {
		return err
	}
	if s == nil {
		c.lastDocPerGroupBits = nil
	} else {
		c.lastDocPerGroupBits = s.Iterator()
	}
	c.groupEndDocID = -1

	c.currentReaderContext = readerContext
	for i := range c.comparators {
		leaf, err := c.comparators[i].GetLeafComparator(readerContext)
		if err != nil {
			return err
		}
		c.leafComparators[i] = leaf
	}
	return nil
}

// Finish renders finish(): the last pending group is processed.
func (c *BlockGroupingCollector) Finish() error {
	if c.subDocUpto != 0 {
		return c.processGroup()
	}
	return nil
}

// ScoreMode renders scoreMode().
func (c *BlockGroupingCollector) ScoreMode() search.ScoreMode {
	if c.needsScores {
		return search.COMPLETE
	}
	return search.COMPLETE_NO_SCORES
}

// CollectRange renders the default LeafCollector.collectRange(int, int).
func (c *BlockGroupingCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream renders the default LeafCollector.collect(DocIdStream).
func (c *BlockGroupingCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// blockGroupingScore renders the private static class Score, which extends
// Scorable.
type blockGroupingScore struct {
	search.BaseScorable
	score float32
}

// Score renders score().
func (s *blockGroupingScore) Score() (float32, error) {
	return s.score, nil
}
