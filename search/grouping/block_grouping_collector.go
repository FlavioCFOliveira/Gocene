package grouping

import (
	"container/heap"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

type oneGroup struct {
	readerContext  *index.LeafReaderContext
	topGroupDoc    int
	docs           []int
	scores         []float32
	count          int
	comparatorSlot int
}

type groupQueue struct {
	items       []*oneGroup
	comparators []search.FieldComparator
	reversed    []int
}

func (pq groupQueue) Len() int { return len(pq.items) }
func (pq groupQueue) Less(i, j int) bool {
	g1, g2 := pq.items[i], pq.items[j]
	for compIDX := 0; compIDX < len(pq.comparators); compIDX++ {
		cmp := pq.comparators[compIDX].Compare(g1.comparatorSlot, g2.comparatorSlot)
		res := pq.reversed[compIDX] * cmp
		if res != 0 {
			return res > 0
		}
	}
	return g1.topGroupDoc > g2.topGroupDoc
}
func (pq groupQueue) Swap(i, j int) { pq.items[i], pq.items[j] = pq.items[j], pq.items[i] }
func (pq *groupQueue) Push(x any)   { pq.items = append(pq.items, x.(*oneGroup)) }
func (pq *groupQueue) Pop() any {
	old := pq.items
	n := len(old)
	item := old[n-1]
	pq.items = old[0 : n-1]
	return item
}

type BlockGroupingCollector struct {
	search.BaseSimpleCollector
	pendingSubDocs       []int
	pendingSubScores     []float32
	subDocUpto           int
	groupSort            *search.Sort
	topNGroups           int
	needsScores          bool
	comparators          []search.FieldComparator
	leafComparators      []search.LeafFieldComparator
	reversed             []int
	compIDXEnd           int
	bottomSlot           int
	queueFull            bool
	currentReaderContext *index.LeafReaderContext
	topGroupDoc          int
	totalHitCount        int
	totalGroupCount      int
	docBase              int
	groupEndDocID        int
	lastDocPerGroup      search.Weight
	lastDocPerGroupBits  search.DocIdSetIterator
	scorer               search.Scorable
	groupQueue           *groupQueue
	groupCompetes        bool
}

func NewBlockGroupingCollector(groupSort *search.Sort, topNGroups int, needsScores bool, lastDocPerGroup search.Weight) *BlockGroupingCollector {
	if topNGroups < 1 {
		panic("topNGroups must be >= 1")
	}

	sortFields := groupSort.Fields
	comparators := make([]search.FieldComparator, len(sortFields))
	reversed := make([]int, len(sortFields))
	for i, sf := range sortFields {
		comparators[i] = search.SortFieldGetComparator(sf, topNGroups, search.PruningNone)
		if sf.Reverse {
			reversed[i] = -1
		} else {
			reversed[i] = 1
		}
	}

	c := &BlockGroupingCollector{
		lastDocPerGroup:  lastDocPerGroup,
		pendingSubDocs:   make([]int, 10),
		pendingSubScores: make([]float32, 10),
		groupSort:        groupSort,
		topNGroups:       topNGroups,
		needsScores:      needsScores,
		comparators:      comparators,
		leafComparators:  make([]search.LeafFieldComparator, len(sortFields)),
		reversed:         reversed,
		compIDXEnd:       len(comparators) - 1,
		groupQueue:       &groupQueue{comparators: comparators, reversed: reversed},
	}
	c.BaseSimpleCollector.Outer = c
	return c
}

func (c *BlockGroupingCollector) processGroup() error {
	c.totalGroupCount++
	if c.groupCompetes {
		if !c.queueFull {
			og := &oneGroup{
				count:          c.subDocUpto,
				topGroupDoc:    c.docBase + c.topGroupDoc,
				docs:           c.pendingSubDocs,
				readerContext:  c.currentReaderContext,
				comparatorSlot: c.bottomSlot,
			}
			if c.needsScores {
				og.scores = c.pendingSubScores
			}
			c.pendingSubDocs = make([]int, 10)
			if c.needsScores {
				c.pendingSubScores = make([]float32, 10)
			}
			heap.Push(c.groupQueue, og)
			c.queueFull = c.groupQueue.Len() == c.topNGroups
			if c.queueFull {
				bottomGroup := c.groupQueue.items[0]
				c.bottomSlot = bottomGroup.comparatorSlot
				for _, fc := range c.leafComparators {
					fc.SetBottom(c.bottomSlot)
				}
			} else {
				c.bottomSlot = c.groupQueue.Len()
			}
		} else {
			og := c.groupQueue.items[0]
			og.count = c.subDocUpto
			og.topGroupDoc = c.docBase + c.topGroupDoc
			savDocs := og.docs
			og.docs = c.pendingSubDocs
			c.pendingSubDocs = savDocs
			if c.needsScores {
				savScores := og.scores
				og.scores = c.pendingSubScores
				c.pendingSubScores = savScores
			}
			og.readerContext = c.currentReaderContext
			heap.Fix(c.groupQueue, 0)
			bottomGroup := c.groupQueue.items[0]
			c.bottomSlot = bottomGroup.comparatorSlot
			for _, fc := range c.leafComparators {
				fc.SetBottom(c.bottomSlot)
			}
		}
	}
	c.subDocUpto = 0
	return nil
}

func (c *BlockGroupingCollector) Collect(doc int) error {
	if doc > c.groupEndDocID {
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
	if c.subDocUpto == len(c.pendingSubDocs) {
		// grow slice
		newDocs := make([]int, len(c.pendingSubDocs)*2)
		copy(newDocs, c.pendingSubDocs)
		c.pendingSubDocs = newDocs
		if c.needsScores {
			newScores := make([]float32, len(c.pendingSubScores)*2)
			copy(newScores, c.pendingSubScores)
			c.pendingSubScores = newScores
		}
	}
	c.pendingSubDocs[c.subDocUpto] = doc
	if c.needsScores {
		score, err := c.scorer.Score()
		if err != nil {
			return err
		}
		c.pendingSubScores[c.subDocUpto] = score
	}
	c.subDocUpto++

	if c.groupCompetes {
		if c.subDocUpto == 1 {
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
			for compIDX := 0; ; compIDX++ {
				raw, err := c.leafComparators[compIDX].CompareBottom(doc)
				if err != nil {
					return err
				}
				cmp := c.reversed[compIDX] * raw
				if cmp < 0 {
					return nil
				} else if cmp > 0 {
					break
				} else if compIDX == c.compIDXEnd {
					return nil
				}
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
		}
	} else {
		for compIDX := 0; ; compIDX++ {
			raw, err := c.leafComparators[compIDX].CompareBottom(doc)
			if err != nil {
				return err
			}
			cmp := c.reversed[compIDX] * raw
			if cmp < 0 {
				return nil
			} else if cmp > 0 {
				break
			} else if compIDX == c.compIDXEnd {
				return nil
			}
		}
		c.groupCompetes = true
		for _, fc := range c.leafComparators {
			if err := fc.Copy(c.bottomSlot, doc); err != nil {
				return err
			}
			if err := fc.SetBottom(c.bottomSlot); err != nil {
				return err
			}
		}
		c.topGroupDoc = doc
	}
	return nil
}

func (c *BlockGroupingCollector) DoSetNextReader(readerContext *index.LeafReaderContext) error {
	c.subDocUpto = 0
	c.docBase = readerContext.DocBase

	scorer, err := c.lastDocPerGroup.Scorer(readerContext)
	if err != nil {
		return err
	}
	if scorer == nil {
		c.lastDocPerGroupBits = nil
	} else {
		c.lastDocPerGroupBits = scorer.Iterator()
	}

	c.currentReaderContext = readerContext
	for i := 0; i < len(c.comparators); i++ {
		leaf, err := c.comparators[i].GetLeafComparator(readerContext)
		if err != nil {
			return err
		}
		c.leafComparators[i] = leaf
	}
	return nil
}

// score renders the private static nested class BlockGroupingCollector.Score,
// a Scorable whose score() returns the recorded field.
type score struct {
	search.BaseScorable

	val float32
}

func (s *score) Score() (float32, error) {
	return s.val, nil
}

func (c *BlockGroupingCollector) GetTopGroups(withinGroupSort *search.Sort, groupOffset, withinGroupOffset, maxDocsPerGroup int) (*TopGroups[any], error) {
	if groupOffset >= c.groupQueue.Len() {
		return nil, nil
	}

	totalGroupedHitCount := 0
	// Java seeds maxScore with Float.MIN_VALUE, the smallest positive float.
	maxScore := float32(math.SmallestNonzeroFloat32)
	groupSortByRelevance := c.groupSort.Equals(search.RELEVANCE)

	numGroups := c.groupQueue.Len() - groupOffset
	groups := make([]*GroupDocs[any], numGroups)

	for downTo := numGroups - 1; downTo >= 0; downTo-- {
		og := heap.Pop(c.groupQueue).(*oneGroup)

		var collector search.Collector
		withinGroupSortByRelevance := withinGroupSort.Equals(search.RELEVANCE)

		if withinGroupSortByRelevance {
			if !c.needsScores {
				panic("cannot sort by relevance within group: needsScores=false")
			}
			collector = search.NewTopScoreDocCollector(maxDocsPerGroup)
		} else {
			collector = search.NewTopFieldCollector(maxDocsPerGroup, withinGroupSort)
		}

		fakeScorer := &score{}
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
				fakeScorer.val = og.scores[docIDX]
				if !withinGroupSortByRelevance {
					groupMaxScore = nonNANmax(groupMaxScore, fakeScorer.val)
				}
			}
			if err := leafCollector.Collect(doc); err != nil {
				return nil, err
			}
		}
		totalGroupedHitCount += og.count

		groupSortValues := make([]any, len(c.comparators))
		for sortFieldIDX := 0; sortFieldIDX < len(c.comparators); sortFieldIDX++ {
			groupSortValues[sortFieldIDX] = c.comparators[sortFieldIDX].Value(og.comparatorSlot)
		}

		var topDocs *search.TopDocs
		var fieldDocs []*search.FieldDoc
		if withinGroupSortByRelevance {
			topDocs = topScoreDocsRange(collector.(*search.TopScoreDocCollector), withinGroupOffset, maxDocsPerGroup)
		} else {
			fieldCollector := collector.(*search.TopFieldCollector)
			topDocs = fieldCollector.TopDocsRange(withinGroupOffset, maxDocsPerGroup)
			fieldDocs = topFieldDocsSlice(fieldCollector, withinGroupOffset, len(topDocs.ScoreDocs))
		}
		if withinGroupSortByRelevance && len(topDocs.ScoreDocs) > 0 {
			groupMaxScore = topDocs.ScoreDocs[0].Score
		}

		// TODO (Lucene): scores could be aggregated across children by
		// Sum/Avg instead of passing NaN.
		groups[downTo] = &GroupDocs[any]{
			Score:           float32(math.NaN()),
			MaxScore:        groupMaxScore,
			TotalHits:       search.NewTotalHits(int64(og.count), search.EQUAL_TO),
			ScoreDocs:       topDocs.ScoreDocs,
			FieldDocs:       fieldDocs,
			GroupValue:      nil, // BlockGroupingCollector cannot compute groupValue
			GroupSortValues: groupSortValues,
		}
		if !groupSortByRelevance {
			maxScore = nonNANmax(maxScore, groupMaxScore)
		}
	}

	if groupSortByRelevance {
		maxScore = groups[0].MaxScore
	}

	totalGroupCount := c.totalGroupCount
	return NewTopGroupsWithCount(
		NewTopGroups(
			c.groupSort.GetSort(),
			withinGroupSort.GetSort(),
			c.totalHitCount,
			totalGroupedHitCount,
			groups,
			maxScore,
		),
		&totalGroupCount,
	), nil
}

// topScoreDocsRange returns the hits in [start, start+howMany) of a
// TopScoreDocCollector.
//
// Mirrors TopDocsCollector.topDocs(int, int) for the score-sorted collector,
// which Gocene exposes on TopFieldCollector as TopDocsRange but not yet on
// TopScoreDocCollector.
func topScoreDocsRange(collector *search.TopScoreDocCollector, start, howMany int) *search.TopDocs {
	all := collector.TopDocs()
	size := len(all.ScoreDocs)
	if start < 0 || start >= size || howMany <= 0 {
		return search.NewTopDocs(all.TotalHits, []*search.ScoreDoc{})
	}
	if howMany > size-start {
		howMany = size - start
	}
	results := make([]*search.ScoreDoc, howMany)
	copy(results, all.ScoreDocs[start:start+howMany])
	return search.NewTopDocs(all.TotalHits, results)
}

func (c *BlockGroupingCollector) Finish() error {
	if c.subDocUpto != 0 {
		return c.processGroup()
	}
	return nil
}

func (c *BlockGroupingCollector) ScoreMode() search.ScoreMode {
	if c.needsScores {
		return search.COMPLETE
	}
	return search.COMPLETE_NO_SCORES
}

// SetScorer mirrors BlockGroupingCollector.setScorer(Scorable).
func (c *BlockGroupingCollector) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	for _, fc := range c.leafComparators {
		if err := fc.SetScorer(scorer); err != nil {
			return err
		}
	}
	return nil
}

// CollectRange mirrors the LeafCollector default collectRange(int, int).
func (c *BlockGroupingCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream mirrors the LeafCollector default collect(DocIdStream).
func (c *BlockGroupingCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// CompetitiveIterator mirrors the LeafCollector default, which returns null.
func (c *BlockGroupingCollector) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return nil, nil
}
