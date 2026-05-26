// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/NearSpansOrdered.java

package spans

// NearSpansOrdered iterates over spans where the sub-spans appear in order
// with a total gap ≤ allowedSlop between them.
//
// Mirrors org.apache.lucene.queries.spans.NearSpansOrdered.
//
// Only minimum-slop matches are returned. Successive matches are formed from
// successive Spans of the SpanNearQuery.
type NearSpansOrdered struct {
	*ConjunctionSpans
	allowedSlop int
	matchStart  int
	matchEnd    int
	matchWidth  int
}

// NewNearSpansOrdered constructs a NearSpansOrdered from subSpans.
// All subSpans must be from the same field.
func NewNearSpansOrdered(allowedSlop int, subSpans []Spans) (*NearSpansOrdered, error) {
	ns := &NearSpansOrdered{
		allowedSlop: allowedSlop,
		matchStart:  -1,
		matchEnd:    -1,
		matchWidth:  -1,
	}
	cs, err := NewConjunctionSpans(subSpans, ns.twoPhaseCurrentDocMatches)
	if err != nil {
		return nil, err
	}
	cs.AtFirstInCurrentDoc = true
	ns.ConjunctionSpans = cs
	return ns, nil
}

// twoPhaseCurrentDocMatches tries to find a valid ordered match in the current doc.
func (ns *NearSpansOrdered) twoPhaseCurrentDocMatches() (bool, error) {
	ns.OneExhaustedInCurrentDoc = false
	// Advance the first sub-span to position it.
	first := ns.SubSpans[0]
	if _, err := first.NextStartPosition(); err != nil {
		return false, err
	}
	if first.StartPosition() == NoMorePositions {
		return false, nil
	}
	for first.StartPosition() != NoMorePositions && !ns.OneExhaustedInCurrentDoc {
		ok, err := ns.stretchToOrder()
		if err != nil {
			return false, err
		}
		if ok && ns.matchWidth <= ns.allowedSlop {
			ns.AtFirstInCurrentDoc = true
			return true, nil
		}
		// Advance first span past current position.
		if _, err := first.NextStartPosition(); err != nil {
			return false, err
		}
	}
	return false, nil
}

// NextStartPosition returns the next ordered near-span start position.
func (ns *NearSpansOrdered) NextStartPosition() (int, error) {
	if ns.AtFirstInCurrentDoc {
		ns.AtFirstInCurrentDoc = false
		return ns.matchStart, nil
	}
	ns.OneExhaustedInCurrentDoc = false
	first := ns.SubSpans[0]
	if _, err := first.NextStartPosition(); err != nil {
		return 0, err
	}
	for first.StartPosition() != NoMorePositions && !ns.OneExhaustedInCurrentDoc {
		ok, err := ns.stretchToOrder()
		if err != nil {
			return 0, err
		}
		if ok && ns.matchWidth <= ns.allowedSlop {
			return ns.matchStart, nil
		}
		if _, err := first.NextStartPosition(); err != nil {
			return 0, err
		}
	}
	ns.matchStart = NoMorePositions
	ns.matchEnd = NoMorePositions
	return NoMorePositions, nil
}

// stretchToOrder advances sub-spans[1..n] to build a non-overlapping ordered
// match starting at sub-spans[0].StartPosition(). Returns true when all sub-spans
// could be ordered without any being exhausted.
func (ns *NearSpansOrdered) stretchToOrder() (bool, error) {
	subSpans := ns.SubSpans
	prev := subSpans[0]
	ns.matchStart = prev.StartPosition()
	ns.matchWidth = 0

	for i := 1; i < len(subSpans); i++ {
		cur := subSpans[i]
		newStart, err := advancePosition(cur, prev.EndPosition())
		if err != nil {
			return false, err
		}
		if newStart == NoMorePositions {
			ns.OneExhaustedInCurrentDoc = true
			return false, nil
		}
		ns.matchWidth += cur.StartPosition() - prev.EndPosition()
		prev = cur
	}
	ns.matchEnd = subSpans[len(subSpans)-1].EndPosition()
	return true, nil
}

// advancePosition advances spans to the first start position >= minPos.
// If spans is a GapSpans, it uses skipToPosition for efficiency.
func advancePosition(sp Spans, minPos int) (int, error) {
	if gs, ok := sp.(*GapSpans); ok {
		return gs.SkipToPosition(minPos)
	}
	for sp.StartPosition() < minPos {
		pos, err := sp.NextStartPosition()
		if err != nil {
			return 0, err
		}
		if pos == NoMorePositions {
			return NoMorePositions, nil
		}
	}
	return sp.StartPosition(), nil
}

// StartPosition returns the current match start, or -1 if at-first-in-doc.
func (ns *NearSpansOrdered) StartPosition() int {
	if ns.AtFirstInCurrentDoc {
		return -1
	}
	return ns.matchStart
}

// EndPosition returns the current match end, or -1 if at-first-in-doc.
func (ns *NearSpansOrdered) EndPosition() int {
	if ns.AtFirstInCurrentDoc {
		return -1
	}
	return ns.matchEnd
}

// Width returns the total gap width of the current match.
func (ns *NearSpansOrdered) Width() int { return ns.matchWidth }

// Collect invokes the collector for all sub-spans.
func (ns *NearSpansOrdered) Collect(collector SpanCollector) error {
	for _, s := range ns.SubSpans {
		if err := s.Collect(collector); err != nil {
			return err
		}
	}
	return nil
}

var _ Spans = (*NearSpansOrdered)(nil)
