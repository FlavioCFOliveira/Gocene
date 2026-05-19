// Port of org.apache.lucene.util.SorterBenchmark from Lucene 10.4.0.
//
// The Java reference is a main()-driven benchmark over IntroSorter, TimSorter
// and InPlaceMergeSorter that prints per-strategy timings to stdout. The Go
// port maps the same matrix onto the testing.B harness: one sub-benchmark per
// (strategy, sorter) pair, sized to ARRAY_LENGTH=20000 entries. Each
// b.N iteration mirrors a single LOOPS pass (array copy + sort) so that
// ns/op is directly comparable to the per-loop cost measured by the Java
// driver. Use `go test -bench=BenchmarkSorters -run=^$ ./util/...` to run.

package util

import (
	"fmt"
	"math/rand"
	"testing"
)

// sorterBenchArrayLength matches ARRAY_LENGTH from SorterBenchmark.java.
const sorterBenchArrayLength = 20000

// sorterBenchSeed is fixed so successive runs sort the same input. The Java
// driver picks System.nanoTime(); a fixed seed keeps benchstat comparisons
// stable across invocations.
const sorterBenchSeed int64 = 0x5EED50772E2B0001

// sorterBenchSortFn is a uniform invocation handle over the concrete
// sorters under test. It maps onto Sorter.sort(from, to) in the Java tree.
type sorterBenchSortFn func(from, to int)

// sorterBenchFactory mirrors the SorterFactory enum in the Java reference.
type sorterBenchFactory struct {
	name  string
	build func(arr []entry) sorterBenchSortFn
}

// sorterBenchFactories enumerates the sorters covered by the Java benchmark in
// the same order. Comparator is entry-by-value, matching Entry::compareTo.
var sorterBenchFactories = []sorterBenchFactory{
	{
		name: "IntroSorter",
		build: func(arr []entry) sorterBenchSortFn {
			return NewArrayIntroSorter(arr, compareEntry).Sort
		},
	},
	{
		name: "TimSorter",
		build: func(arr []entry) sorterBenchSortFn {
			return NewArrayTimSorter(arr, compareEntry, len(arr)/64).Sort
		},
	},
	{
		name: "MergeSorter",
		build: func(arr []entry) sorterBenchSortFn {
			return NewArrayInPlaceMergeSorter(arr, compareEntry).Sort
		},
	},
}

// sorterBenchStrategies enumerates the data-generation strategies covered by
// the Java BaseSortTestCase.Strategy enum, with stable names for sub-bench
// reporting.
var sorterBenchStrategies = []struct {
	name     string
	strategy testStrategy
}{
	{"RANDOM", strategyRandom},
	{"RANDOM_LOW_CARDINALITY", strategyRandomLowCardinality},
	{"RANDOM_MEDIUM_CARDINALITY", strategyRandomMediumCardinality},
	{"ASCENDING", strategyAscending},
	{"DESCENDING", strategyDescending},
	{"STRICTLY_DESCENDING", strategyStrictlyDescending},
	{"ASCENDING_SEQUENCES", strategyAscendingSequences},
	{"MOSTLY_ASCENDING", strategyMostlyAscending},
}

// BenchmarkSorters mirrors SorterBenchmark.main: for each strategy and each
// sorter, time a copy+sort loop over a 20k-entry array. Each b.N corresponds
// to one LOOPS iteration in the Java driver, so ns/op is directly comparable.
func BenchmarkSorters(b *testing.B) {
	for _, sc := range sorterBenchStrategies {
		b.Run(sc.name, func(b *testing.B) {
			for _, sf := range sorterBenchFactories {
				b.Run(sf.name, func(b *testing.B) {
					benchSorter(b, sc.strategy, sf)
				})
			}
		})
	}
}

// benchSorter runs the copy+sort hot loop for a single (strategy, factory)
// pair. The original array is generated once from a deterministic seed so the
// workload is reproducible across runs; the sorted scratch buffer is reset
// from the original on every iteration to match the Java arraycopy-then-sort
// pattern.
func benchSorter(b *testing.B, strategy testStrategy, sf sorterBenchFactory) {
	b.Helper()
	r := rand.New(rand.NewSource(sorterBenchSeed))
	original := generateEntries(r, strategy, sorterBenchArrayLength)
	clone := make([]entry, len(original))
	sort := sf.build(clone)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(clone, original)
		sort(0, len(clone))
	}
}

// sorterBenchmarkStrategyName resolves a testStrategy back to the canonical
// Java enum label, useful for ad-hoc reporting outside the b.Run tree.
func sorterBenchmarkStrategyName(s testStrategy) string {
	for _, sc := range sorterBenchStrategies {
		if sc.strategy == s {
			return sc.name
		}
	}
	return fmt.Sprintf("UNKNOWN(%d)", int(s))
}
