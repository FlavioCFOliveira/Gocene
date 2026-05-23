// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest_test

// TestLookupBenchmark mirrors
// org.apache.lucene.search.suggest.TestLookupBenchmark.
//
// The Java original is annotated @Ignore and reads Top50KWiki.utf8 from the
// classpath.  In Go it is expressed as a benchmark (go test -bench=.).
// The Top50KWiki fixture is not vendored, so the benchmark is marked as a
// skip-unless-file-exists.

import (
	"bufio"
	"os"
	"strings"
	"testing"
	"time"

	suggestfst "github.com/FlavioCFOliveira/Gocene/suggest/fst"
	"github.com/FlavioCFOliveira/Gocene/suggest/tst"
)

const top50KWikiFile = "testdata/Top50KWiki.utf8"

// readTop50KWiki reads the Top50KWiki.utf8 file if present.
func readTop50KWiki(t testing.TB) []*Input {
	t.Helper()
	f, err := os.Open(top50KWikiFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("open Top50KWiki: %v", err)
	}
	defer f.Close()

	var inputs []*Input
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.LastIndexByte(line, '|')
		if idx < 0 {
			continue
		}
		key := line[:idx]
		var weight int64
		for _, c := range line[idx+1:] {
			if c >= '0' && c <= '9' {
				weight = weight*10 + int64(c-'0')
			}
		}
		inputs = append(inputs, NewInput(key, weight))
	}
	return inputs
}

// BenchmarkLookupConstruction_TST mirrors
// TestLookupBenchmark.testConstructionTime for TSTLookup.
func BenchmarkLookupConstruction_TST(b *testing.B) {
	inputs := readTop50KWiki(b)
	if inputs == nil {
		b.Skip("Top50KWiki.utf8 fixture not present — skipping benchmark")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := tst.NewTSTLookup()
		if err := l.Build(NewInputArrayIterator(inputs)); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLookupConstruction_WFST mirrors
// TestLookupBenchmark.testConstructionTime for WFSTCompletionLookup.
func BenchmarkLookupConstruction_WFST(b *testing.B) {
	inputs := readTop50KWiki(b)
	if inputs == nil {
		b.Skip("Top50KWiki.utf8 fixture not present — skipping benchmark")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := suggestfst.NewWFSTCompletionLookup()
		if err := l.Build(NewInputArrayIterator(inputs)); err != nil {
			b.Fatal(err)
		}
	}
}

// TestAverage_From verifies AverageFrom using a simple known dataset.
// This is a unit test for the Average helper used by TestLookupBenchmark.
func TestAverage_From(t *testing.T) {
	values := []float64{10, 20, 30}
	avg := AverageFrom(values)
	if avg.Avg != 20 {
		t.Errorf("Avg: got %f, want 20", avg.Avg)
	}
	// stddev of [10,20,30] = sqrt(((10-20)^2+(20-20)^2+(30-20)^2)/3) = sqrt(200/3) ≈ 8.165
	const wantStddev = 8.16496580927726
	diff := avg.Stddev - wantStddev
	if diff < 0 {
		diff = -diff
	}
	if diff > 1e-9 {
		t.Errorf("Stddev: got %f, want %f", avg.Stddev, wantStddev)
	}
}

// TestAverageMeasure is a lightweight test that exercises the timing pattern
// used in TestLookupBenchmark (construct, measure, report).
func TestAverageMeasure(t *testing.T) {
	const rounds = 5
	times := make([]float64, rounds)
	for i := 0; i < rounds; i++ {
		start := time.Now()
		l := tst.NewTSTLookup()
		if err := l.Build(NewInputArrayIterator([]*Input{
			NewInput("alpha", 1),
			NewInput("beta", 2),
			NewInput("gamma", 3),
		})); err != nil {
			t.Fatal(err)
		}
		times[i] = float64(time.Since(start).Nanoseconds()) / 1e6
	}
	avg := AverageFrom(times)
	t.Logf("construction time: %s", avg)
}
