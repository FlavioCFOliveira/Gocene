// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

import "fmt"

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/MultiLeafFieldComparator.java

// multiLeafFieldComparator composes multiple LeafFieldComparator instances into a
// single comparator that applies them in priority order (first non-zero result wins).
// The reverseMul slice carries the sort direction: +1 for ascending, -1 for descending.
type multiLeafFieldComparator struct {
	comparators      []LeafFieldComparator
	reverseMul       []int
	firstComparator  LeafFieldComparator
	firstReverseMul  int
}

// newMultiLeafFieldComparator constructs a multiLeafFieldComparator. Both slices must have
// the same length and at least one element.
func newMultiLeafFieldComparator(comparators []LeafFieldComparator, reverseMul []int) (*multiLeafFieldComparator, error) {
	if len(comparators) != len(reverseMul) {
		return nil, fmt.Errorf("MultiLeafFieldComparator: len(comparators)=%d != len(reverseMul)=%d",
			len(comparators), len(reverseMul))
	}
	return &multiLeafFieldComparator{
		comparators:     comparators,
		reverseMul:      reverseMul,
		firstComparator: comparators[0],
		firstReverseMul: reverseMul[0],
	}, nil
}

// SetBottom forwards to all comparators.
func (m *multiLeafFieldComparator) SetBottom(slot int) error {
	for _, c := range m.comparators {
		if err := c.SetBottom(slot); err != nil {
			return err
		}
	}
	return nil
}

// CompareBottom applies comparators in order; the first non-zero result wins.
func (m *multiLeafFieldComparator) CompareBottom(doc int) (int, error) {
	cmp, err := m.firstComparator.CompareBottom(doc)
	if err != nil {
		return 0, err
	}
	cmp *= m.firstReverseMul
	if cmp != 0 {
		return cmp, nil
	}
	for i := 1; i < len(m.comparators); i++ {
		c, err := m.comparators[i].CompareBottom(doc)
		if err != nil {
			return 0, err
		}
		c *= m.reverseMul[i]
		if c != 0 {
			return c, nil
		}
	}
	return 0, nil
}

// CompareTop applies comparators in order; the first non-zero result wins.
func (m *multiLeafFieldComparator) CompareTop(doc int) (int, error) {
	cmp, err := m.firstComparator.CompareTop(doc)
	if err != nil {
		return 0, err
	}
	cmp *= m.firstReverseMul
	if cmp != 0 {
		return cmp, nil
	}
	for i := 1; i < len(m.comparators); i++ {
		c, err := m.comparators[i].CompareTop(doc)
		if err != nil {
			return 0, err
		}
		c *= m.reverseMul[i]
		if c != 0 {
			return c, nil
		}
	}
	return 0, nil
}

// Copy forwards to all comparators.
func (m *multiLeafFieldComparator) Copy(slot, doc int) error {
	for _, c := range m.comparators {
		if err := c.Copy(slot, doc); err != nil {
			return err
		}
	}
	return nil
}

// SetScorer forwards to all comparators.
func (m *multiLeafFieldComparator) SetScorer(scorer Scorable) error {
	for _, c := range m.comparators {
		if err := c.SetScorer(scorer); err != nil {
			return err
		}
	}
	return nil
}

// SetHitsThresholdReached notifies only the first comparator (skipping is only
// relevant for the primary sort key).
func (m *multiLeafFieldComparator) SetHitsThresholdReached() {
	m.firstComparator.SetHitsThresholdReached()
}

// CompetitiveIterator delegates to the first comparator (skipping is only
// relevant for the primary sort key).
func (m *multiLeafFieldComparator) CompetitiveIterator() (DocIdSetIterator, error) {
	return m.firstComparator.CompetitiveIterator()
}

// Compile-time guarantee.
var _ LeafFieldComparator = (*multiLeafFieldComparator)(nil)
