// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package automaton

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// LimitedFiniteStringsIterator limits the number of iterated accepted strings.
type LimitedFiniteStringsIterator struct {
	*FiniteStringsIterator
	limit int
	count int
}

// NewLimitedFiniteStringsIterator constructs a limited iterator over all accepted strings.
func NewLimitedFiniteStringsIterator(a *Automaton, limit int) *LimitedFiniteStringsIterator {
	if limit != -1 && limit <= 0 {
		panic(fmt.Sprintf("limit must be -1 (no limit), or > 0; got: %d", limit))
	}

	actualLimit := limit
	if limit < 0 {
		actualLimit = 2147483647 // Integer.MAX_VALUE
	}

	return &LimitedFiniteStringsIterator{
		FiniteStringsIterator: NewFiniteStringsIterator(a),
		limit:                 actualLimit,
		count:                 0,
	}
}

// Next returns the next accepted string.
func (it *LimitedFiniteStringsIterator) Next() (*util.IntsRef, error) {
	if it.count >= it.limit {
		return nil, nil
	}

	res, err := it.FiniteStringsIterator.Next()
	if err != nil {
		return nil, err
	}
	if res != nil {
		it.count++
	}
	return res, nil
}

// Size returns the number of iterated finite strings.
func (it *LimitedFiniteStringsIterator) Size() int {
	return it.count
}
