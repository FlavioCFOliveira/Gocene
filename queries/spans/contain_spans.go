// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/ContainSpans.java

package spans

// ContainSpans is the abstract base for span iterators that implement containment
// semantics (SpanContainingQuery and SpanWithinQuery).
//
// Mirrors org.apache.lucene.queries.spans.ContainSpans (abstract, package-private).
//
// Deviations from Java:
//   - Java subclasses ContainSpans which subclasses ConjunctionSpans. In Go we
//     embed *ConjunctionSpans and delegate position methods to sourceSpans.
type ContainSpans struct {
	*ConjunctionSpans
	SourceSpans Spans
	BigSpans    Spans
	LittleSpans Spans
}

// NewContainSpans constructs a ContainSpans.
// bigSpans and littleSpans are used for the conjunction; sourceSpans determines
// which of the two is the "result" position source.
// matchFn implements twoPhaseCurrentDocMatches.
func NewContainSpans(bigSpans, littleSpans, sourceSpans Spans, matchFn func() (bool, error)) (*ContainSpans, error) {
	cs, err := NewConjunctionSpans([]Spans{bigSpans, littleSpans}, matchFn)
	if err != nil {
		return nil, err
	}
	return &ContainSpans{
		ConjunctionSpans: cs,
		SourceSpans:      sourceSpans,
		BigSpans:         bigSpans,
		LittleSpans:      littleSpans,
	}, nil
}

// StartPosition returns the start position delegated to sourceSpans.
func (cs *ContainSpans) StartPosition() int {
	if cs.AtFirstInCurrentDoc {
		return -1
	}
	if cs.OneExhaustedInCurrentDoc {
		return NoMorePositions
	}
	return cs.SourceSpans.StartPosition()
}

// EndPosition returns the end position delegated to sourceSpans.
func (cs *ContainSpans) EndPosition() int {
	if cs.AtFirstInCurrentDoc {
		return -1
	}
	if cs.OneExhaustedInCurrentDoc {
		return NoMorePositions
	}
	return cs.SourceSpans.EndPosition()
}

// Width returns the width delegated to sourceSpans.
func (cs *ContainSpans) Width() int {
	return cs.SourceSpans.Width()
}

// Collect collects from both bigSpans and littleSpans.
func (cs *ContainSpans) Collect(collector SpanCollector) error {
	if err := cs.BigSpans.Collect(collector); err != nil {
		return err
	}
	return cs.LittleSpans.Collect(collector)
}
