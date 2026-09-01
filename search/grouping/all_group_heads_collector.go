package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupHead represents a group head. A group head is the most relevant document for a particular group.
type GroupHead[T any] interface {
	GetDoc() int
	GetGroupValue() T
	SetNextReader(ctx *index.LeafReaderContext) error
	SetScorer(scorer search.Scorable) error
	Compare(compIDX int, doc int) (int, error)
	UpdateDocHead(doc int) error
	GetSortValues() []any
}

// AllGroupHeadsCollector collects the most relevant document (group head) for each group.
type AllGroupHeadsCollector[T any] struct {
	search.BaseSimpleCollector
	groupSelector GroupSelector[T]
	sort          *search.Sort
	reversed     []int
	compIDXEnd    int
	heads         map[any]GroupHead[T]
	context       *index.LeafReaderContext
	scorer        search.Scorable
}

func NewAllGroupHeadsCollector[T any](selector GroupSelector[T], sort *search.Sort) *AllGroupHeadsCollector[T] {
	reversed := make([]int, len(sort.Fields))
	for i, f := range sort.Fields {
		if f.Reverse {
			reversed[i] = -1
		} else {
			reversed[i] = 1
		}
	}

	return &AllGroupHeadsCollector[T]{
		groupSelector: selector,
		sort:          sort,
		reversed:      reversed,
		compIDXEnd:    len(sort.Fields) - 1,
		heads:         make(map[any]GroupHead[T]),
	}
}

func (c *AllGroupHeadsCollector[T]) DoSetNextReader(context *index.LeafReaderContext) error {
	if err := c.groupSelector.SetNextReader(context); err != nil {
		return err
	}
	c.context = context
	for _, head := range c.heads {
		if err := head.SetNextReader(context); err != nil {
			return err
		}
	}
	return nil
}

func (c *AllGroupHeadsCollector[T]) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	for _, head := range c.heads {
		if err := head.SetScorer(scorer); err != nil {
			return err
		}
	}
	return nil
}

func (c *AllGroupHeadsCollector[T]) Collect(doc int) error {
	state, err := c.groupSelector.AdvanceTo(doc)
	if err != nil {
		return err
	}
	if state == StateSkip {
		return nil
	}

	val, err := c.groupSelector.CurrentValue()
	if err != nil {
		return err
	}

	key := any(val)
	head, ok := c.heads[key]
	if !ok {
		copyVal, err := c.groupSelector.CopyValue()
		if err != nil {
			return err
		}
		newHead := c.newGroupHead(doc, copyVal)
		c.heads[key] = newHead
		return nil
	}

	for compIDX := 0; ; compIDX++ {
		cmp, err := head.Compare(compIDX, doc)
		if err != nil {
			return err
		}
		res := c.reversed[compIDX] * cmp
		if res < 0 {
			return nil
		} else if res > 0 {
			break
		} else if compIDX == c.compIDXEnd {
			return nil
		}
	}

	if err := head.UpdateDocHead(doc); err != nil {
		return err
	}

	return nil
}

func (c *AllGroupHeadsCollector[T]) Finish() error {
	return nil
}

func (c *AllGroupHeadsCollector[T]) ScoreMode() search.ScoreMode {
	if c.sort.NeedsScores() {
		return search.ScoreModeComplete
	}
	return search.ScoreModeCompleteNoScores
}

func (c *AllGroupHeadsCollector[T]) GetGroupHeads() []GroupHead[T] {
	res := make([]GroupHead[T], 0, len(c.heads))
	for _, h := range c.heads {
		res = append(res, h)
	}
	return res
}

func (c *AllGroupHeadsCollector[T]) GetGroupHeadsDocs() []int {
	res := make([]int, 0, len(c.heads))
	for _, h := range c.heads {
		res = append(res, h.GetDoc())
	}
	return res
}

func (c *AllGroupHeadsCollector[T]) newGroupHead(doc int, value T) GroupHead[T] {
	if c.sort.IsRelevance() {
		return &scoringGroupHead[T]{
			groupValue: value,
			doc:        doc + c.context.DocBase,
			scorer:     c.scorer,
			topScore:   c.scorer.Score(),
			sortValues: []any{c.scorer.Score()},
		}
	}
	return &sortingGroupHead[T]{
		groupValue:     value,
		doc:            doc + c.context.DocBase,
		sort:           c.sort,
		context:        c.context,
		scorer:         c.scorer,
		leafComparators: make([]search.LeafFieldComparator, len(c.sort.Fields)),
		sortValues:      make([]any, len(c.sort.Fields)),
	}
}

type scoringGroupHead[T any] struct {
	groupValue T
	doc        int
	scorer     search.Scorable
	topScore   float32
	sortValues []any
}

func (h *scoringGroupHead[T]) GetDoc() int { return h.doc }
func (h *scoringGroupHead[T]) GetGroupValue() T { return h.groupValue }
func (h *scoringGroupHead[T]) SetNextReader(ctx *index.LeafReaderContext) error { return nil }
func (h *scoringGroupHead[T]) SetScorer(scorer search.Scorable) error {
	h.scorer = scorer
	return nil
}
func (h *scoringGroupHead[T]) Compare(compIDX int, doc int) (int, error) {
	score := h.scorer.Score()
	cmp := compareFloat32(score, h.topScore)
	if cmp > 0 {
		h.topScore = score
	}
	return cmp, nil
}
func (h *scoringGroupHead[T]) UpdateDocHead(doc int) error {
	h.doc = doc // This is a bit simplified; should probably handle docBase
	h.sortValues[0] = h.topScore
	return nil
}
func (h *scoringGroupHead[T]) GetSortValues() []any { return h.sortValues }

type sortingGroupHead[T any] struct {
	groupValue      T
	doc             int
	sort            *search.Sort
	context         *index.LeafReaderContext
	scorer          search.Scorable
	leafComparators []search.LeafFieldComparator
	sortValues      []any
}

func (h *sortingGroupHead[T]) GetDoc() int { return h.doc }
func (h *sortingGroupHead[T]) GetGroupValue() T { return h.groupValue }
func (h *sortingGroupHead[T]) SetNextReader(ctx *index.LeafReaderContext) error {
	h.context = ctx
	for i := 0; i < len(h.leafComparators); i++ {
		comp := h.sort.Fields[i].GetComparator()
		h.leafComparators[i] = comp.GetLeafComparator(ctx)
	}
	return nil
}
func (h *sortingGroupHead[T]) SetScorer(scorer search.Scorable) error {
	h.scorer = scorer
	for _, lc := range h.leafComparators {
		lc.SetScorer(scorer)
	}
	return nil
}
func (h *sortingGroupHead[T]) Compare(compIDX int, doc int) (int, error) {
	return h.leafComparators[compIDX].CompareBottom(doc), nil
}
func (h *sortingGroupHead[T]) UpdateDocHead(doc int) error {
	for i := 0; i < len(h.leafComparators); i++ {
		h.leafComparators[i].Copy(0, doc)
		h.leafComparators[i].SetBottom(0)
		h.sortValues[i] = h.sort.Fields[i].GetComparator().Value(0)
	}
	h.doc = doc + h.context.DocBase
	return nil
}
func (h *sortingGroupHead[T]) GetSortValues() []any { return h.sortValues }

func compareFloat32(a, b float32) int {
	if a < b { return -1 }
	if a > b { return 1 }
	return 0
}
