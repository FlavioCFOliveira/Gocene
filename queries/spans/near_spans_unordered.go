// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/NearSpansUnordered.java

package spans

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// NearSpansUnordered is similar to NearSpansOrdered but for unordered matching.
// It uses a priority queue of sub-spans ordered by start position to find windows
// where all spans fit within allowedSlop extra positions.
//
// Mirrors org.apache.lucene.queries.spans.NearSpansUnordered.
type NearSpansUnordered struct {
	*ConjunctionSpans
	allowedSlop int
	spanWindow  *spanTotalLengthEndPositionWindow
}

// NewNearSpansUnordered constructs a NearSpansUnordered.
func NewNearSpansUnordered(allowedSlop int, subSpans []Spans) (*NearSpansUnordered, error) {
	ns := &NearSpansUnordered{
		allowedSlop: allowedSlop,
	}
	cs, err := NewConjunctionSpans(subSpans, ns.twoPhaseCurrentDocMatches)
	if err != nil {
		return nil, err
	}
	ns.ConjunctionSpans = cs
	window, err := newSpanTotalLengthEndPositionWindow(cs.SubSpans)
	if err != nil {
		return nil, err
	}
	ns.spanWindow = window
	return ns, nil
}

// twoPhaseCurrentDocMatches tries to find an unordered near match in the current doc.
func (ns *NearSpansUnordered) twoPhaseCurrentDocMatches() (bool, error) {
	if err := ns.spanWindow.startDocument(); err != nil {
		return false, err
	}
	for {
		if ns.spanWindow.atMatch(ns.allowedSlop) {
			ns.AtFirstInCurrentDoc = true
			ns.OneExhaustedInCurrentDoc = false
			return true, nil
		}
		ok, err := ns.spanWindow.nextPosition()
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
}

// NextStartPosition returns the next unordered near-span start.
func (ns *NearSpansUnordered) NextStartPosition() (int, error) {
	if ns.AtFirstInCurrentDoc {
		ns.AtFirstInCurrentDoc = false
		return ns.spanWindow.minStart(), nil
	}
	if ns.spanWindow.exhausted() {
		ns.OneExhaustedInCurrentDoc = true
		return NoMorePositions, nil
	}
	for {
		ok, err := ns.spanWindow.nextPosition()
		if err != nil {
			return 0, err
		}
		if !ok {
			ns.OneExhaustedInCurrentDoc = true
			return NoMorePositions, nil
		}
		if ns.spanWindow.atMatch(ns.allowedSlop) {
			return ns.spanWindow.minStart(), nil
		}
	}
}

// StartPosition returns the current start position.
func (ns *NearSpansUnordered) StartPosition() int {
	if ns.AtFirstInCurrentDoc {
		return -1
	}
	if ns.OneExhaustedInCurrentDoc {
		return NoMorePositions
	}
	return ns.spanWindow.minStart()
}

// EndPosition returns the maximum end position in the window.
func (ns *NearSpansUnordered) EndPosition() int {
	if ns.AtFirstInCurrentDoc {
		return -1
	}
	if ns.OneExhaustedInCurrentDoc {
		return NoMorePositions
	}
	return ns.spanWindow.maxEndPosition
}

// Width returns the window width.
func (ns *NearSpansUnordered) Width() int {
	return ns.spanWindow.maxEndPosition - ns.spanWindow.minStart()
}

// Collect invokes the collector for all sub-spans.
func (ns *NearSpansUnordered) Collect(collector SpanCollector) error {
	for _, s := range ns.SubSpans {
		if err := s.Collect(collector); err != nil {
			return err
		}
	}
	return nil
}

// spanTotalLengthEndPositionWindow is a priority queue of Spans ordered by
// start position. It tracks totalSpanLength and maxEndPosition to determine
// whether the current window is a valid near-match.
type spanTotalLengthEndPositionWindow struct {
	pq              *util.PriorityQueue[Spans]
	subSpans        []Spans
	totalSpanLength int
	maxEndPosition  int
	done            bool
}

func newSpanTotalLengthEndPositionWindow(subSpans []Spans) (*spanTotalLengthEndPositionWindow, error) {
	pq, err := util.NewPriorityQueue[Spans](len(subSpans), positionsOrdered)
	if err != nil {
		return nil, err
	}
	return &spanTotalLengthEndPositionWindow{
		pq:       pq,
		subSpans: subSpans,
	}, nil
}

// positionsOrdered returns true iff spans1 should be ordered before spans2
// (same comparator as Java: by start position, then by end position).
func positionsOrdered(spans1, spans2 Spans) bool {
	start1 := spans1.StartPosition()
	start2 := spans2.StartPosition()
	if start1 == start2 {
		return spans1.EndPosition() < spans2.EndPosition()
	}
	return start1 < start2
}

// startDocument initialises the window for a new document.
// All sub-spans must be positioned at -1 (before first position call).
func (w *spanTotalLengthEndPositionWindow) startDocument() error {
	w.pq.Clear()
	w.totalSpanLength = 0
	w.maxEndPosition = -1
	w.done = false

	for _, s := range w.subSpans {
		pos, err := s.NextStartPosition()
		if err != nil {
			return err
		}
		if pos == NoMorePositions {
			w.done = true
			return nil
		}
		w.pq.Add(s)
		end := s.EndPosition()
		if end > w.maxEndPosition {
			w.maxEndPosition = end
		}
		w.totalSpanLength += end - s.StartPosition()
	}
	return nil
}

// nextPosition advances the minimum-start span to its next position.
// Returns false when any span is exhausted.
func (w *spanTotalLengthEndPositionWindow) nextPosition() (bool, error) {
	top := w.pq.Top()
	if top == nil {
		return false, nil
	}
	spanLength := top.EndPosition() - top.StartPosition()
	next, err := top.NextStartPosition()
	if err != nil {
		return false, err
	}
	if next == NoMorePositions {
		w.done = true
		return false, nil
	}
	w.totalSpanLength -= spanLength
	newLen := top.EndPosition() - top.StartPosition()
	w.totalSpanLength += newLen
	if top.EndPosition() > w.maxEndPosition {
		w.maxEndPosition = top.EndPosition()
	}
	w.pq.UpdateTop()
	return true, nil
}

// atMatch reports whether the current window satisfies the slop constraint.
func (w *spanTotalLengthEndPositionWindow) atMatch(allowedSlop int) bool {
	top := w.pq.Top()
	if top == nil {
		return false
	}
	return (w.maxEndPosition - top.StartPosition() - w.totalSpanLength) <= allowedSlop
}

// minStart returns the start position of the earliest span in the window.
func (w *spanTotalLengthEndPositionWindow) minStart() int {
	top := w.pq.Top()
	if top == nil {
		return NoMorePositions
	}
	return top.StartPosition()
}

// exhausted reports whether any span was exhausted.
func (w *spanTotalLengthEndPositionWindow) exhausted() bool { return w.done }

var _ Spans = (*NearSpansUnordered)(nil)
