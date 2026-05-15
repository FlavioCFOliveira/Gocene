// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.LimitedFiniteStringsIterator from
// Apache Lucene 10.4.0 (Apache License 2.0).

package automaton

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// LimitedFiniteStringsIterator caps the number of strings returned by a
// FiniteStringsIterator. limit == -1 means unlimited; limit <= 0 (other than
// -1) is rejected.
type LimitedFiniteStringsIterator struct {
	inner *FiniteStringsIterator
	limit int
	count int
}

// NewLimitedFiniteStringsIterator constructs an iterator with the given limit.
func NewLimitedFiniteStringsIterator(a *Automaton, limit int) (*LimitedFiniteStringsIterator, error) {
	if limit != -1 && limit <= 0 {
		return nil, fmt.Errorf("automaton: limit must be -1 or > 0; got %d", limit)
	}
	effective := limit
	if effective <= 0 {
		effective = int(^uint(0) >> 1)
	}
	return &LimitedFiniteStringsIterator{
		inner: NewFiniteStringsIterator(a),
		limit: effective,
	}, nil
}

// Next returns the next accepted string up to the configured limit.
func (l *LimitedFiniteStringsIterator) Next() (*util.IntsRef, error) {
	if l.count >= l.limit {
		return nil, nil
	}
	out, err := l.inner.Next()
	if err != nil {
		return nil, err
	}
	if out != nil {
		l.count++
	}
	return out, nil
}

// Size returns the number of strings yielded so far.
func (l *LimitedFiniteStringsIterator) Size() int { return l.count }
