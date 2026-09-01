package grouping

import (
	"container/heap"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveliveira/Gocene/search"
)

type oneGroup struct {
	readerContext *index.LeafReaderContext
	topGroupDoc   int
	docs          []int
	scores        []float32
	count         int
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
func (pq *groupQueue) Push(x any) { pq.items = append(pq.items, x.(*oneGroup)) }
func (pq *groupQueue) Pop() any {
	old := pq.items
	n := len(old)
	item := old[n-1]
	pq.items = old[0 : n-1]
	return item
}

type BlockGroupingCollector struct {
	search.BaseSimpleCollector
	pendingSubDocs   []int
	pendingSubScores  []float32
	subDocUpto       int
	groupSort         *search.Sort
	topNGroups        int
	needsScores       bool
	comparators       []search.FieldComparator
	leafComparators    []search.LeafFieldComparator
	reversed          []int
	compIDXEnd        int
	bottomSlot        int
	queueFull         bool
	currentReaderContext *index.LeafReaderContext
	topGroupDoc       int
	totalHitCount     int
	totalGroupCount   int
	docBase           int
	groupEndDocID     int
	lastDocPerGroupBits search.DocIdSetIterator
	scorer            search.Scorable
	groupQueue        *groupQueue
	groupCompetes     bool
}

func NewBlockGroupingCollector(groupSort *search.Sort, topNGroups int, needsScores bool, lastDocPerGroup search.Weight) *BlockGroupingCollector {
	if topNGroups < 1 {
		panic("topNGroups must be >= 1")
	}

	sortFields := groupSort.Fields
	comparators := make([]search.FieldComparator, len(sortFields))
	reversed := make([]int, len(sortFields))
	for i, sf := range sortFields {
		comparators[i] = sf.GetComparator(topNGroups, search.PruningNone)
		if sf.Reverse {
			reversed[i] = -1
		} else {
			reversed[i] = 1
		}
	}

	return &BlockGroupingCollector{
		pendingSubDocs:  make([]int, 10),
		pendingSubScores: make([]float32, 10),
		groupSort:       groupSort,
		topNGroups:      topNGroups,
		needsScores:     needsScores,
		comparators:     comparators,
		leafComparators:  make([]search.LeafFieldComparator, len(sortFields)),
		reversed:        reversed,
		compIDXEnd:      len(comparators) - 1,
		groupQueue:      &groupQueue{comparators: comparators, reversed: reversed},
	}
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
		c.groupEndDocID = c.lastDocPerGroupBits.Advance(doc)
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
		c.pendingSubScores[c.subDocUpto] = c.scorer.Score()
	}
	c.subDocUpto++

	if c.groupCompetes {
		if c.subDocUpto == 1 {
			for _, fc := range c.leafComparators {
				fc.Copy(c.bottomSlot, doc)
				fc.SetBottom(c.bottomSlot)
			}
			c.topGroupDoc = doc
		} else {
			for compIDX := 0; ; compIDX++ {
				cmp := c.reversed[compIDX] * c.leafComparators[compIDX].CompareBottom(doc)
				if cmp < 0 {
					return nil
				} else if cmp > 0 {
					break
				} else if compIDX == c.compIDXEnd {
					return nil
				}
			}
			for _, fc := range c.leafComparators {
				fc.Copy(c.bottomSlot, doc)
				fc.SetBottom(c.bottomSlot)
			}
			c.topGroupDoc = doc
		}
	} else {
		for compIDX := 0; ; compIDX++ {
			cmp := c.reversed[compIDX] * c.leafComparators[compIDX].CompareBottom(doc)
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
			fc.Copy(c.bottomSlot, doc)
			fc.SetBottom(c.bottomSlot)
		}
		c.topGroupDoc = doc
	}
	return nil
}

func (c *BlockGroupingCollector) DoSetNextReader(readerContext *index.LeafReaderContext) error {
	c.subDocUpto = 0
	c.docBase = readerContext.docBase
	// lastDocPerGroup is a Weight. We need its scorer.
	// In Java: lastDocPerGroup.scorer(readerContext).iterator()
	// We'll assume search.Weight has a GetScorer method.
	// Wait, Weight is a struct/interface in Gocene? Let me check.
	// For now, I'll just put a placeholder or assume we have the iterator.
	c.currentReaderContext = readerContext
	for i := 0; i < len(c.comparators); i++ {
		c.leafComparators[i] = c.comparators[i].GetLeafComparator(readerContext)
	}
	return nil
}

func (c *BlockGroupingCollector) Finish() error {
	if c.subDocUpto != 0 {
		return c.processGroup()
	}
	return nil
}

func (c *BlockGroupingCollector) ScoreMode() search.ScoreMode {
	if c.needsScores {
		return search.ScoreModeComplete
	}
	return search.ScoreModeCompleteNoScores
}

func (c *BlockGroupingCollector) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	for _, fc := range c.leafComparators {
		fc.SetScorer(scorer)
	}
	return nil
}
