// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/util/TestTimSorterWorstCase.java
// Purpose: Verifies that the TimSorter merge-stack never overflows when fed an
// adversarial sequence of run lengths derived from the abstools java-timsort-bug
// repository.
//
// Lucene marks the test @Nightly because the adversarial array must contain
// 140M..400M entries to exercise the failure mode. The port mirrors that policy
// by gating execution on the GOCENE_RUN_MONSTERS=1 environment variable, in line
// with other monster-class tests in this package (see stress_ram_usage_estimator_test.go).
//
// Storage mirrors Lucene's PackedInts.getMutable(length, 1, 0) (one bit per value)
// to keep peak heap below ~50 MiB at length=400M; the runs only ever encode 0/1.
// We use a local bitmap rather than util/packed.Mutable to avoid an import cycle
// between util and util/packed.

package util

import (
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"
)

// timSorterWorstCaseEnv is the opt-in flag used by monster-class tests in this
// package; matches the convention established in stress_ram_usage_estimator_test.go.
const timSorterWorstCaseEnv = "GOCENE_RUN_MONSTERS"

// bitArray is a tightly-packed 1-bit-per-value mutable array; it stands in for
// Lucene's PackedInts.getMutable(length, 1, 0) without crossing the util ->
// util/packed package boundary.
type bitArray struct {
	bits   []uint64
	length int
}

func newBitArray(length int) *bitArray {
	return &bitArray{
		bits:   make([]uint64, (length+63)>>6),
		length: length,
	}
}

func (b *bitArray) get(i int) int64 {
	return int64((b.bits[i>>6] >> uint(i&63)) & 1)
}

func (b *bitArray) set(i int, v int64) {
	mask := uint64(1) << uint(i&63)
	if v&1 == 1 {
		b.bits[i>>6] |= mask
	} else {
		b.bits[i>>6] &^= mask
	}
}

// timSorterMinRun mirrors the static TimSorter.minRun(length) helper from Lucene.
// The instance receiver in tim_sorter.go uses the identical arithmetic; we
// duplicate it here to keep the test free of side effects on TimSorter state and
// to match the static call shape of the reference test.
func timSorterMinRun(length int) int {
	if length <= Threshold {
		return length
	}
	n := length
	r := 0
	for n >= 64 {
		r |= n & 1
		n >>= 1
	}
	return n + r
}

// bitArrayTimSorter adapts bitArray to TimSorterInterface. The save/restore/
// compareSaved hooks are unreachable for the worst-case input (every merge fits
// into the in-place path because maxTempSlots == 0) and panic to match Lucene's
// UnsupportedOperationException semantics.
type bitArrayTimSorter struct {
	Sorter
	arr *bitArray
}

func (s *bitArrayTimSorter) Compare(i, j int) int {
	a, b := s.arr.get(i), s.arr.get(j)
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func (s *bitArrayTimSorter) Swap(i, j int) {
	tmp := s.arr.get(i)
	s.arr.set(i, s.arr.get(j))
	s.arr.set(j, tmp)
}

func (s *bitArrayTimSorter) Copy(src, dest int) { s.arr.set(dest, s.arr.get(src)) }

func (s *bitArrayTimSorter) Save(_, _ int)    { panic("Save: unsupported on worst-case fixture") }
func (s *bitArrayTimSorter) Restore(_, _ int) { panic("Restore: unsupported on worst-case fixture") }
func (s *bitArrayTimSorter) CompareSaved(_, _ int) int {
	panic("CompareSaved: unsupported on worst-case fixture")
}

// Sort is required by SorterInterface but never invoked on the impl directly
// (TimSorter is the orchestrator); the empty body matches the convention used
// by entryTimSorter in sorters_test.go.
func (s *bitArrayTimSorter) Sort(_, _ int) {}

// generateWrongElem appends a sequence x_1, ..., x_n of run lengths to runs such
// that they satisfy the conditions of generateWrongElem in the upstream test:
// each x_j >= minRun, x_1 + ... + x_{j-2} < x_j < x_1 + ... + x_{j-1}, and the
// total equals X. The merges of these runs (one-by-one against the
// second-to-last entry) are what stress mergeCollapse / ensureInvariants.
func generateWrongElem(X, minRun int, runs *[]int) {
	for X >= 2*minRun+1 {
		newTotal := X/2 + 1

		switch {
		case 3*minRun+3 <= X && X <= 4*minRun+1:
			newTotal = 2*minRun + 1
		case 5*minRun+5 <= X && X <= 6*minRun+5:
			newTotal = 3*minRun + 3
		case 8*minRun+9 <= X && X <= 10*minRun+9:
			newTotal = 5*minRun + 5
		case 13*minRun+15 <= X && X <= 16*minRun+17:
			newTotal = 8*minRun + 9
		}

		*runs = append([]int{X - newTotal}, *runs...)
		X = newTotal
	}
	*runs = append([]int{X}, *runs...)
}

// runsWorstCase constructs the adversarial run-length sequence described in the
// java-timsort-bug repository, producing Y_i / x_{i,j} runs that survive
// mergeCollapse's invariant checks but eventually merge into runs that violate
// the invariant. See the upstream TestTimSorterWorstCase for the derivation.
func runsWorstCase(length, minRun int) []int {
	runs := make([]int, 0, 256)

	runningTotal := 0
	Y := minRun + 4
	X := minRun

	for int64(runningTotal)+int64(Y)+int64(X) <= int64(length) {
		runningTotal += X + Y
		generateWrongElem(X, minRun, &runs)
		runs = append([]int{Y}, runs...)

		// X_{i+1} = Y_i + x_{i,1} + 1, since runs[1] = x_{i,1}
		X = Y + runs[1] + 1

		// Y_{i+1} = X_{i+1} + Y_i + 1
		Y += X + 1
	}

	if int64(runningTotal)+int64(X) <= int64(length) {
		runningTotal += X
		generateWrongElem(X, minRun, &runs)
	}

	runs = append(runs, length-runningTotal)
	return runs
}

// createWorstCaseArray builds a bitArray of the requested length whose 1-bits
// mark the end of each run from runs. Inside a run all values are equal (0),
// then the run terminator (1) breaks monotonicity for the next run; the final
// position is reset to 0 so the trailing run remains a non-descending sequence,
// matching the upstream construction.
func createWorstCaseArray(length int, runs []int) *bitArray {
	arr := newBitArray(length)
	endRun := -1
	for _, runLen := range runs {
		endRun += runLen
		arr.set(endRun, 1)
	}
	arr.set(length-1, 0)
	return arr
}

func generateWorstCaseArray(length int) *bitArray {
	minRun := timSorterMinRun(length)
	return createWorstCaseArray(length, runsWorstCase(length, minRun))
}

// TestTimSorterWorstCase exercises the merge-stack growth path that originally
// caused stack overflows in the JDK 7 TimSort port. The test passes if Sort
// returns without panicking; the upstream assertion was equivalent (the JVM
// version would throw ArrayIndexOutOfBoundsException when the stack overflowed).
//
// Gated on GOCENE_RUN_MONSTERS=1 because the adversarial array needs 140M+
// entries to trigger the failure mode (matches Lucene's @Nightly annotation).
func TestTimSorterWorstCase(t *testing.T) {
	if v, _ := strconv.ParseBool(os.Getenv(timSorterWorstCaseEnv)); !v {
		t.Skipf("monster test (allocates a ~25 MiB bitmap and sorts 140M+ entries); set %s=1 to run", timSorterWorstCaseEnv)
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// Mirror the non-nightly bounds from the upstream test; @Nightly raises the
	// ceiling to 400M but the failure mode reproduces from 140M onward.
	const lo, hi = 140_000_000, 200_000_000
	length := lo + r.Intn(hi-lo+1)

	arr := generateWorstCaseArray(length)
	sorter := &bitArrayTimSorter{arr: arr}
	NewTimSorter(sorter, 0).Sort(0, length)
}
