package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupReducer defines what to collect for individual groups during the second-pass of a grouping search.
type GroupReducer[T any] struct {
	groups map[any]*groupCollector
}

type groupCollector struct {
	collector     search.Collector
	leafCollector search.LeafCollector
}

func NewGroupReducer[T any]() *GroupReducer[T] {
	return &GroupReducer[T]{
		groups: make(map[any]*groupCollector),
	}
}

// SetGroups defines which groups should be reduced.
func (r *GroupReducer[T]) SetGroups(groups []SearchGroup[T]) {
	for _, g := range groups {
		collector := r.newCollector()
		r.groups[any(g.GroupValue)] = &groupCollector{
			collector: collector,
		}
	}
}

// newCollector must be implemented by the concrete reducer to create a collector for each group.
func (r *GroupReducer[T]) newCollector() search.Collector {
	panic("newCollector must be implemented by the concrete reducer")
}

func (r *GroupReducer[T]) GetCollector(value T) search.Collector {
	if gc, ok := r.groups[any(value)]; ok {
		return gc.collector
	}
	return nil
}

func (r *GroupReducer[T]) Collect(value T, doc int) error {
	gc, ok := r.groups[any(value)]
	if !ok {
		return nil
	}
	if gc.leafCollector == nil {
		// This should have been set by setNextReader, but as a safety...
		return nil
	}
	return gc.leafCollector.Collect(doc)
}

func (r *GroupReducer[T]) SetScorer(scorer search.Scorable) error {
	for _, gc := range r.groups {
		if gc.leafCollector != nil {
			if err := gc.leafCollector.SetScorer(scorer); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *GroupReducer[T]) SetNextReader(ctx *index.LeafReaderContext) error {
	for _, gc := range r.groups {
		lc, err := gc.collector.GetLeafCollector(ctx)
		if err != nil {
			return err
		}
		gc.leafCollector = lc
	}
	return nil
}

// NeedsScores returns whether or not this reducer requires collected documents to be scored.
func (r *GroupReducer[T]) NeedsScores() bool {
	return true
}
