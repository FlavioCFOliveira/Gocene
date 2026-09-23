// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/join/src/test/org/apache/lucene/search/join/TestBlockJoinSelector.java
// (Apache Lucene 10.5.0).

package join

import (
	"math"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// bjsRandom renders LuceneTestCase.random().
func bjsRandom() *rand.Rand {
	return rand.New(rand.NewSource(rand.Int63()))
}

func bjsFixedBitSet(t *testing.T, numBits int, bits ...int) *util.FixedBitSet {
	t.Helper()
	bs, err := util.NewFixedBitSet(numBits)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bits {
		bs.Set(b)
	}
	return bs
}

func assertBJSEquals[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected != actual {
		t.Fatalf("expected:<%v> but was:<%v>", expected, actual)
	}
}

func TestBlockJoinSelectorDocsWithValue(t *testing.T) {
	parents := bjsFixedBitSet(t, 20, 0, 5, 6, 10, 15, 19)
	children := bjsFixedBitSet(t, 20, 2, 3, 4, 12, 17)
	childDocsWithValue := bjsFixedBitSet(t, 20, 2, 3, 4, 8, 16)

	docsWithValue := WrapBits(childDocsWithValue, parents, children)
	assertBJSEquals(t, false, docsWithValue.Get(0))
	assertBJSEquals(t, true, docsWithValue.Get(5))
	assertBJSEquals(t, false, docsWithValue.Get(6))
	assertBJSEquals(t, false, docsWithValue.Get(10))
	assertBJSEquals(t, false, docsWithValue.Get(15))
	assertBJSEquals(t, false, docsWithValue.Get(19))
}

// bjsAssertNoMoreDoc renders the static assertNoMoreDoc(DocIdSetIterator, int).
func bjsAssertNoMoreDoc(t *testing.T, sdv index.DocValuesIterator, maxDoc int) {
	t.Helper()
	r := bjsRandom()
	if r.Intn(2) == 0 {
		assertBJSEquals(t, search.NO_MORE_DOCS, bjsMust(t)(sdv.NextDoc()))
	} else {
		if r.Intn(2) == 0 {
			assertBJSEquals(t, search.NO_MORE_DOCS, bjsMust(t)(sdv.Advance(sdv.DocID()+bjsRandom().Intn(maxDoc-sdv.DocID()))))
		} else {
			noMatchDoc := sdv.DocID() + bjsRandom().Intn(maxDoc-sdv.DocID()-1) + 1
			assertBJSEquals(t, false, bjsAdvanceExact(t, sdv, noMatchDoc))
			assertBJSEquals(t, noMatchDoc, sdv.DocID())
			if r.Intn(2) == 0 {
				assertBJSEquals(t, search.NO_MORE_DOCS, bjsMust(t)(sdv.NextDoc()))
			}
		}
	}
}

// bjsNextDoc renders the static nextDoc(DocIdSetIterator, int).
func bjsNextDoc(t *testing.T, sdv index.DocValuesIterator, docID int) int {
	t.Helper()
	r := bjsRandom()
	if r.Intn(2) == 0 {
		return bjsMust(t)(sdv.NextDoc())
	}
	if r.Intn(2) == 0 {
		return bjsMust(t)(sdv.Advance(sdv.DocID() + bjsRandom().Intn(docID-sdv.DocID()-1) + 1))
	}
	if r.Intn(2) == 0 {
		noMatchDoc := sdv.DocID() + bjsRandom().Intn(docID-sdv.DocID()-1) + 1
		assertBJSEquals(t, false, bjsAdvanceExact(t, sdv, noMatchDoc))
		assertBJSEquals(t, noMatchDoc, sdv.DocID())
	}
	assertBJSEquals(t, true, bjsAdvanceExact(t, sdv, docID))
	return sdv.DocID()
}

// bjsAdvanceExact renders the private static advanceExact(DocIdSetIterator, int):
// the iterator is a SortedDocValues or a NumericDocValues.
func bjsAdvanceExact(t *testing.T, sdv index.DocValuesIterator, target int) bool {
	t.Helper()
	ok, err := sdv.AdvanceExact(target)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

// bjsMust unwraps an (int, error) iterator result.
func bjsMust(t *testing.T) func(int, error) int {
	return func(v int, err error) int {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
}

func bjsOrdValue(t *testing.T, dv index.SortedDocValues) int {
	t.Helper()
	ord, err := dv.OrdValue()
	if err != nil {
		t.Fatal(err)
	}
	return ord
}

func bjsLongValue(t *testing.T, dv index.NumericDocValues) int64 {
	t.Helper()
	v, err := dv.LongValue()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func bjsOrds(set map[int]int) []int {
	ords := make([]int, 20)
	for i := range ords {
		ords[i] = -1
	}
	for doc, ord := range set {
		ords[doc] = ord
	}
	return ords
}

func TestBlockJoinSelectorSortedSelector(t *testing.T) {
	parents := bjsFixedBitSet(t, 20, 0, 5, 6, 10, 15, 18)
	children := bjsFixedBitSet(t, 20, 2, 3, 4, 12, 16, 17)

	ords := bjsOrds(map[int]int{2: 5, 3: 7, 4: 3, 12: 10, 16: 9, 17: 10, 18: 11, 19: 12})

	mins := WrapSortedSet(index.SingletonSortedSet(newCannedSortedDocValues(ords)),
		BlockJoinSelectorMin, parents, toIter(children), true)
	assertBJSEquals(t, 5, bjsNextDoc(t, mins, 5))
	assertBJSEquals(t, 3, bjsOrdValue(t, mins))
	assertBJSEquals(t, 15, bjsNextDoc(t, mins, 15))
	assertBJSEquals(t, 10, bjsOrdValue(t, mins))
	assertBJSEquals(t, 18, bjsNextDoc(t, mins, 18))
	bjsAssertNoMoreDoc(t, mins, 20)

	maxs := WrapSortedSet(index.SingletonSortedSet(newCannedSortedDocValues(ords)),
		BlockJoinSelectorMax, parents, toIter(children), false)
	assertBJSEquals(t, 5, bjsNextDoc(t, maxs, 5))
	assertBJSEquals(t, 7, bjsOrdValue(t, maxs))
	assertBJSEquals(t, 15, bjsNextDoc(t, maxs, 15))
	assertBJSEquals(t, 10, bjsOrdValue(t, maxs))
	assertBJSEquals(t, 18, bjsNextDoc(t, maxs, 18))
	bjsAssertNoMoreDoc(t, maxs, 20)

	withMissingValues := WrapSortedSet(index.SingletonSortedSet(newCannedSortedDocValues(ords)),
		BlockJoinSelectorMin, parents, toIter(children), false)
	assertBJSEquals(t, 5, bjsNextDoc(t, withMissingValues, 5))
	assertBJSEquals(t, -1, bjsOrdValue(t, withMissingValues))
	assertBJSEquals(t, 15, bjsNextDoc(t, withMissingValues, 15))
	assertBJSEquals(t, -1, bjsOrdValue(t, withMissingValues))
	assertBJSEquals(t, 18, bjsNextDoc(t, withMissingValues, 18))
	assertBJSEquals(t, 9, bjsOrdValue(t, withMissingValues))
	bjsAssertNoMoreDoc(t, withMissingValues, 20)

	withMissingValues = WrapSortedSet(index.SingletonSortedSet(newCannedSortedDocValues(ords)),
		BlockJoinSelectorMin, parents, toIter(children), true)
	assertBJSEquals(t, 5, bjsNextDoc(t, withMissingValues, 5))
	assertBJSEquals(t, 3, bjsOrdValue(t, withMissingValues))
	assertBJSEquals(t, 15, bjsNextDoc(t, withMissingValues, 15))
	assertBJSEquals(t, 10, bjsOrdValue(t, withMissingValues))
	assertBJSEquals(t, 18, bjsNextDoc(t, withMissingValues, 18))
	assertBJSEquals(t, 9, bjsOrdValue(t, withMissingValues))
	bjsAssertNoMoreDoc(t, withMissingValues, 20)

	withMissingValues = WrapSortedSet(index.SingletonSortedSet(newCannedSortedDocValues(ords)),
		BlockJoinSelectorMax, parents, toIter(children), false)
	assertBJSEquals(t, 5, bjsNextDoc(t, withMissingValues, 5))
	assertBJSEquals(t, 7, bjsOrdValue(t, withMissingValues))
	assertBJSEquals(t, 15, bjsNextDoc(t, withMissingValues, 15))
	assertBJSEquals(t, 10, bjsOrdValue(t, withMissingValues))
	assertBJSEquals(t, 18, bjsNextDoc(t, withMissingValues, 18))
	assertBJSEquals(t, 10, bjsOrdValue(t, withMissingValues))
	bjsAssertNoMoreDoc(t, withMissingValues, 20)

	withMissingValues = WrapSortedSet(index.SingletonSortedSet(newCannedSortedDocValues(ords)),
		BlockJoinSelectorMax, parents, toIter(children), true)
	assertBJSEquals(t, 5, bjsNextDoc(t, withMissingValues, 5))
	assertBJSEquals(t, math.MaxInt32, bjsOrdValue(t, withMissingValues))
	assertBJSEquals(t, 15, bjsNextDoc(t, withMissingValues, 15))
	assertBJSEquals(t, math.MaxInt32, bjsOrdValue(t, withMissingValues))
	assertBJSEquals(t, 18, bjsNextDoc(t, withMissingValues, 18))
	assertBJSEquals(t, 10, bjsOrdValue(t, withMissingValues))
	bjsAssertNoMoreDoc(t, withMissingValues, 20)
}

func TestBlockJoinSelectorNextDocWithSkippedParents(t *testing.T) {
	parents := bjsFixedBitSet(t, 20, 0, 3, 5, 10)
	children := bjsFixedBitSet(t, 20, 1, 2, 4, 6, 7, 8, 9)

	ords := bjsOrds(map[int]int{1: 5, 4: 7, 5: 3, 8: 10})

	naturalOrder := WrapSortedSet(index.SingletonSortedSet(newCannedSortedDocValues(ords)),
		BlockJoinSelectorMin, parents, toIter(children), false)
	assertBJSEquals(t, 3, bjsMust(t)(naturalOrder.NextDoc()))
	assertBJSEquals(t, -1, bjsOrdValue(t, naturalOrder))
	assertBJSEquals(t, 5, bjsMust(t)(naturalOrder.NextDoc()))
	assertBJSEquals(t, 7, bjsOrdValue(t, naturalOrder))
	assertBJSEquals(t, 10, bjsMust(t)(naturalOrder.NextDoc()))
	assertBJSEquals(t, -1, bjsOrdValue(t, naturalOrder))
	assertBJSEquals(t, search.NO_MORE_DOCS, bjsMust(t)(naturalOrder.NextDoc()))

	reverseOrder := WrapSortedSet(index.SingletonSortedSet(newCannedSortedDocValues(ords)),
		BlockJoinSelectorMax, parents, toIter(children), true)
	assertBJSEquals(t, 3, bjsMust(t)(reverseOrder.NextDoc()))
	assertBJSEquals(t, math.MaxInt32, bjsOrdValue(t, reverseOrder))
	assertBJSEquals(t, 5, bjsMust(t)(reverseOrder.NextDoc()))
	assertBJSEquals(t, 7, bjsOrdValue(t, reverseOrder))
	assertBJSEquals(t, 10, bjsMust(t)(reverseOrder.NextDoc()))
	assertBJSEquals(t, math.MaxInt32, bjsOrdValue(t, reverseOrder))
	assertBJSEquals(t, search.NO_MORE_DOCS, bjsMust(t)(reverseOrder.NextDoc()))
}

// cannedSortedDocValues renders the private static CannedSortedDocValues.
type cannedSortedDocValues struct {
	ords  []int
	docID int
}

func newCannedSortedDocValues(ords []int) *cannedSortedDocValues {
	return &cannedSortedDocValues{ords: ords, docID: -1}
}

func (c *cannedSortedDocValues) DocID() int { return c.docID }

func (c *cannedSortedDocValues) NextDoc() (int, error) {
	for {
		c.docID++
		if c.docID == len(c.ords) {
			c.docID = search.NO_MORE_DOCS
			break
		}
		if c.ords[c.docID] != -1 {
			break
		}
	}
	return c.docID, nil
}

func (c *cannedSortedDocValues) Advance(target int) (int, error) {
	if target >= len(c.ords) {
		c.docID = search.NO_MORE_DOCS
	} else {
		c.docID = target
		if c.ords[c.docID] == -1 {
			if _, err := c.NextDoc(); err != nil {
				return 0, err
			}
		}
	}
	return c.docID, nil
}

func (c *cannedSortedDocValues) AdvanceExact(target int) (bool, error) {
	c.docID = target
	return c.ords[c.docID] != -1, nil
}

func (c *cannedSortedDocValues) OrdValue() (int, error) {
	if util.AssertsEnabled() && c.ords[c.docID] == -1 {
		panic(util.NewAssertionError(""))
	}
	return c.ords[c.docID], nil
}

func (c *cannedSortedDocValues) Cost() int64 { return 5 }

func (c *cannedSortedDocValues) LookupOrd(ord int) ([]byte, error) {
	panic("UnsupportedOperationException")
}

func (c *cannedSortedDocValues) GetValueCount() int { return 11 }

// LongValue and BinaryValue are the members Gocene's SortedDocValues
// inherits from its NumericDocValues contract; the Java SortedDocValues has
// neither, so the test double never reaches them.
func (c *cannedSortedDocValues) LongValue() (int64, error) {
	panic("UnsupportedOperationException")
}

func (c *cannedSortedDocValues) BinaryValue() ([]byte, error) {
	panic("UnsupportedOperationException")
}

func (c *cannedSortedDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(c, upTo, bitSet, offset)
}

func (c *cannedSortedDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(c)
}

func TestBlockJoinSelectorNumericSelector(t *testing.T) {
	parents := bjsFixedBitSet(t, 20, 0, 5, 6, 10, 15, 19)
	children := bjsFixedBitSet(t, 20, 2, 3, 4, 12, 17)

	longs := make([]int64, 20)
	docsWithValue := bjsFixedBitSet(t, 20)
	docsWithValue.Set(2)
	longs[2] = 5
	docsWithValue.Set(3)
	longs[3] = 7
	docsWithValue.Set(4)
	longs[4] = 3
	docsWithValue.Set(12)
	longs[12] = 10
	docsWithValue.Set(18)
	longs[18] = 10

	mins := WrapSortedNumeric(index.Singleton(newCannedNumericDocValues(longs, docsWithValue)),
		BlockJoinSelectorMin, parents, toIter(children), nil)
	assertBJSEquals(t, 5, bjsNextDoc(t, mins, 5))
	assertBJSEquals(t, int64(3), bjsLongValue(t, mins))
	assertBJSEquals(t, 15, bjsNextDoc(t, mins, 15))
	assertBJSEquals(t, int64(10), bjsLongValue(t, mins))
	bjsAssertNoMoreDoc(t, mins, 20)

	maxs := WrapSortedNumeric(index.Singleton(newCannedNumericDocValues(longs, docsWithValue)),
		BlockJoinSelectorMax, parents, toIter(children), nil)
	assertBJSEquals(t, 5, bjsNextDoc(t, maxs, 5))
	assertBJSEquals(t, int64(7), bjsLongValue(t, maxs))
	assertBJSEquals(t, 15, bjsNextDoc(t, maxs, 15))
	assertBJSEquals(t, int64(10), bjsLongValue(t, maxs))
	bjsAssertNoMoreDoc(t, maxs, 20)
}

// cannedNumericDocValues renders the private static CannedNumericDocValues.
type cannedNumericDocValues struct {
	docsWithValue util.Bits
	values        []int64
	docID         int
}

func newCannedNumericDocValues(values []int64, docsWithValue util.Bits) *cannedNumericDocValues {
	return &cannedNumericDocValues{values: values, docsWithValue: docsWithValue, docID: -1}
}

func (c *cannedNumericDocValues) DocID() int { return c.docID }

func (c *cannedNumericDocValues) NextDoc() (int, error) {
	for {
		c.docID++
		if c.docID == len(c.values) {
			c.docID = search.NO_MORE_DOCS
			break
		}
		if c.docsWithValue.Get(c.docID) {
			break
		}
	}
	return c.docID, nil
}

func (c *cannedNumericDocValues) Advance(target int) (int, error) {
	if target >= len(c.values) {
		c.docID = search.NO_MORE_DOCS
		return c.docID, nil
	}
	c.docID = target - 1
	return c.NextDoc()
}

func (c *cannedNumericDocValues) AdvanceExact(target int) (bool, error) {
	c.docID = target
	return c.docsWithValue.Get(c.docID), nil
}

func (c *cannedNumericDocValues) LongValue() (int64, error) { return c.values[c.docID], nil }

func (c *cannedNumericDocValues) Cost() int64 { return 5 }

func (c *cannedNumericDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(c, upTo, bitSet, offset)
}

func (c *cannedNumericDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(c)
}
