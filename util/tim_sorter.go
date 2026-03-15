// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// TimSorter implements the TimSort algorithm. It sorts small arrays with a binary sort.
//
// This algorithm is stable. It's especially good at sorting partially-sorted arrays.
//
// NOTE: There are a few differences with the original implementation:
//   - The extra amount of memory to perform merges is configurable. This allows small merges
//     to be very fast while large merges will be performed in-place (slightly slower).
//   - Only the fast merge routine can gallop (the one that doesn't run in-place) and it only
//     gallops on the longest slice.
//
// This is a port of Apache Lucene's TimSorter class.
type TimSorter struct {
	Sorter
	impl         TimSorterInterface
	maxTempSlots int
	minRun       int
	to           int
	stackSize    int
	runEnds      []int
}

// TimSorterInterface extends SorterInterface with methods specific to TimSorter.
type TimSorterInterface interface {
	SorterInterface
	// Copy copies data from slot src to slot dest.
	Copy(src, dest int)
	// Save saves all elements between slots i and i+len into temporary storage.
	Save(i, length int)
	// Restore restores element j from temporary storage into slot i.
	Restore(i, j int)
	// CompareSaved compares element i from temporary storage with element j.
	CompareSaved(i, j int) int
}

// Constants for TimSort
const (
	MinRun    = 32
	Threshold = 64
	StackLen  = 49 // depends on MINRUN
	MinGallop = 7
)

// NewTimSorter creates a new TimSorter with the given implementation and maxTempSlots.
// maxTempSlots is the maximum amount of extra memory to run merges.
func NewTimSorter(impl TimSorterInterface, maxTempSlots int) *TimSorter {
	return &TimSorter{
		impl:         impl,
		maxTempSlots: maxTempSlots,
		runEnds:      make([]int, 1+StackLen),
	}
}

// Sort sorts the range [from, to).
func (ts *TimSorter) Sort(from, to int) {
	ts.CheckRange(from, to)
	if to-from <= 1 {
		return
	}
	ts.reset(from, to)
	for {
		ts.ensureInvariants()
		ts.pushRunLen(ts.nextRun())
		if ts.runEnd(0) >= to {
			break
		}
	}
	ts.exhaustStack()
}

// reset initializes the sorter state for a new sort.
func (ts *TimSorter) reset(from, to int) {
	ts.stackSize = 0
	for i := range ts.runEnds {
		ts.runEnds[i] = 0
	}
	ts.runEnds[0] = from
	ts.to = to
	length := to - from
	if length <= Threshold {
		ts.minRun = length
	} else {
		ts.minRun = ts.minRunCalc(length)
	}
}

// minRunCalc calculates the minimum run length for an array of given length.
func (ts *TimSorter) minRunCalc(length int) int {
	n := length
	r := 0
	for n >= 64 {
		r |= n & 1
		n >>= 1
	}
	return n + r
}

// runLen returns the length of run i.
func (ts *TimSorter) runLen(i int) int {
	off := ts.stackSize - i
	return ts.runEnds[off] - ts.runEnds[off-1]
}

// runBase returns the base index of run i.
func (ts *TimSorter) runBase(i int) int {
	return ts.runEnds[ts.stackSize-i-1]
}

// runEnd returns the end index of run i.
func (ts *TimSorter) runEnd(i int) int {
	return ts.runEnds[ts.stackSize-i]
}

// setRunEnd sets the end index of run i.
func (ts *TimSorter) setRunEnd(i, runEnd int) {
	ts.runEnds[ts.stackSize-i] = runEnd
}

// pushRunLen pushes a new run of given length.
func (ts *TimSorter) pushRunLen(length int) {
	ts.runEnds[ts.stackSize+1] = ts.runEnds[ts.stackSize] + length
	ts.stackSize++
}

// nextRun computes the length of the next run, makes the run sorted and returns its length.
func (ts *TimSorter) nextRun() int {
	runBase := ts.runEnd(0)
	if runBase >= ts.to-1 {
		return 1
	}
	o := runBase + 2
	if ts.impl.Compare(runBase, runBase+1) > 0 {
		// run must be strictly descending
		for o < ts.to && ts.impl.Compare(o-1, o) > 0 {
			o++
		}
		ts.Sorter.Reverse(runBase, o, ts.impl)
	} else {
		// run must be non-descending
		for o < ts.to && ts.impl.Compare(o-1, o) <= 0 {
			o++
		}
	}
	runHi := o
	if runBase+ts.minRun < ts.to {
		if runHi < runBase+ts.minRun {
			runHi = runBase + ts.minRun
		}
	}
	if runHi > ts.to {
		runHi = ts.to
	}
	ts.BinarySortWithStart(runBase, runHi, o, ts.impl)
	return runHi - runBase
}

// ensureInvariants maintains the TimSort invariants by merging runs as needed.
func (ts *TimSorter) ensureInvariants() {
	for ts.stackSize > 1 {
		runLen0 := ts.runLen(0)
		runLen1 := ts.runLen(1)

		if ts.stackSize > 2 {
			runLen2 := ts.runLen(2)

			if runLen2 <= runLen1+runLen0 {
				// merge the smaller of 0 and 2 with 1
				if runLen2 < runLen0 {
					ts.mergeAt(1)
				} else {
					ts.mergeAt(0)
				}
				continue
			}
		}

		if runLen1 <= runLen0 {
			ts.mergeAt(0)
			continue
		}

		break
	}
}

// exhaustStack merges all remaining runs.
func (ts *TimSorter) exhaustStack() {
	for ts.stackSize > 1 {
		ts.mergeAt(0)
	}
}

// mergeAt merges runs at position n and n+1.
func (ts *TimSorter) mergeAt(n int) {
	lo := ts.runBase(n + 1)
	mid := ts.runBase(n)
	hi := ts.runEnd(n)
	ts.merge(lo, mid, hi)
	for j := n + 1; j > 0; j-- {
		ts.setRunEnd(j, ts.runEnd(j-1))
	}
	ts.stackSize--
}

// merge merges two sorted ranges [lo, mid) and [mid, hi).
func (ts *TimSorter) merge(lo, mid, hi int) {
	if ts.impl.Compare(mid-1, mid) <= 0 {
		return
	}
	lo = ts.Upper2(lo, mid, mid, ts.impl)
	hi = ts.Lower2(mid, hi, mid-1, ts.impl)

	if hi-mid <= mid-lo && hi-mid <= ts.maxTempSlots {
		ts.mergeHi(lo, mid, hi)
	} else if mid-lo <= ts.maxTempSlots {
		ts.mergeLo(lo, mid, hi)
	} else {
		ts.Sorter.MergeInPlace(lo, mid, hi, ts.impl)
	}
}

// mergeLo merges the left run into the right using temporary storage.
func (ts *TimSorter) mergeLo(lo, mid, hi int) {
	len1 := mid - lo
	ts.impl.Save(lo, len1)
	ts.impl.Copy(mid, lo)
	i, j, dest := 0, mid+1, lo+1

outer:
	for {
		for count := 0; count < MinGallop; {
			if i >= len1 || j >= hi {
				break outer
			}
			if ts.impl.CompareSaved(i, j) <= 0 {
				ts.impl.Restore(i, dest)
				i++
				dest++
				count = 0
			} else {
				ts.impl.Copy(j, dest)
				j++
				dest++
				count++
			}
		}
		// galloping...
		next := ts.lowerSaved3(j, hi, i)
		for ; j < next; dest++ {
			ts.impl.Copy(j, dest)
			j++
		}
		ts.impl.Restore(i, dest)
		i++
		dest++
	}
	for ; i < len1; dest++ {
		ts.impl.Restore(i, dest)
		i++
	}
}

// mergeHi merges the right run into the left using temporary storage.
func (ts *TimSorter) mergeHi(lo, mid, hi int) {
	len2 := hi - mid
	ts.impl.Save(mid, len2)
	ts.impl.Copy(mid-1, hi-1)
	i, j, dest := mid-2, len2-1, hi-2

outer:
	for {
		for count := 0; count < MinGallop; {
			if i < lo || j < 0 {
				break outer
			}
			if ts.impl.CompareSaved(j, i) >= 0 {
				ts.impl.Restore(j, dest)
				j--
				dest--
				count = 0
			} else {
				ts.impl.Copy(i, dest)
				i--
				dest--
				count++
			}
		}
		// galloping
		next := ts.upperSaved3(lo, i+1, j)
		for i >= next {
			ts.impl.Copy(i, dest)
			i--
			dest--
		}
		ts.impl.Restore(j, dest)
		j--
		dest--
	}
	for ; j >= 0; dest-- {
		ts.impl.Restore(j, dest)
		j--
	}
}

// lowerSaved finds the first position in [from, to) where val could be inserted.
func (ts *TimSorter) lowerSaved(from, to, val int) int {
	length := to - from
	for length > 0 {
		half := length >> 1
		mid := from + half
		if ts.impl.CompareSaved(val, mid) > 0 {
			from = mid + 1
			length = length - half - 1
		} else {
			length = half
		}
	}
	return from
}

// upperSaved finds the first position in [from, to) where val is less than the element.
func (ts *TimSorter) upperSaved(from, to, val int) int {
	length := to - from
	for length > 0 {
		half := length >> 1
		mid := from + half
		if ts.impl.CompareSaved(val, mid) < 0 {
			length = half
		} else {
			from = mid + 1
			length = length - half - 1
		}
	}
	return from
}

// lowerSaved3 is faster than lowerSaved when val is at the beginning of [from, to).
func (ts *TimSorter) lowerSaved3(from, to, val int) int {
	f, t := from, from+1
	for t < to {
		if ts.impl.CompareSaved(val, t) <= 0 {
			return ts.lowerSaved(f, t, val)
		}
		delta := t - f
		f = t
		t += delta << 1
	}
	return ts.lowerSaved(f, to, val)
}

// upperSaved3 is faster than upperSaved when val is at the end of [from, to).
func (ts *TimSorter) upperSaved3(from, to, val int) int {
	f, t := to-1, to
	for f > from {
		if ts.impl.CompareSaved(val, f) >= 0 {
			return ts.upperSaved(f, t, val)
		}
		delta := t - f
		t = f
		f -= delta << 1
	}
	return ts.upperSaved(from, t, val)
}
