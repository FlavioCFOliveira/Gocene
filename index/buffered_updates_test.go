// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestBufferedUpdates.java
// (Apache Lucene 10.5.0). The Java class lives in org.apache.lucene.index and
// uses search.TermQuery; the port lives in the external index_test package to
// avoid the index -> search import cycle and reaches the package-private
// members through export_test.go.

package index_test

import (
	"bytes"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// bufferedUpdatesAtLeast renders LuceneTestCase.atLeast(int) with the default
// multipliers (RANDOM_MULTIPLIER = 1, not nightly).
func bufferedUpdatesAtLeast(i int) int {
	return i + rand.Intn(i/2+1)
}

func TestBufferedUpdatesRamBytesUsed(t *testing.T) {
	bu := index.NewBufferedUpdates("seg1")
	if bu.RamBytesUsed() != 0 {
		t.Fatalf("expected 0, got %d", bu.RamBytesUsed())
	}
	if bu.Any() {
		t.Fatal("any")
	}
	queries := bufferedUpdatesAtLeast(1)
	for i := 0; i < queries; i++ {
		docIDUpto := rand.Intn(100000)
		if rand.Intn(2) == 0 {
			docIDUpto = math.MaxInt32
		}
		term := index.NewTerm("id", strconv.Itoa(rand.Intn(100)))
		bu.AddQuery(search.NewTermQuery(term), docIDUpto)
	}

	terms := bufferedUpdatesAtLeast(1)
	for i := 0; i < terms; i++ {
		docIDUpto := rand.Intn(100000)
		if rand.Intn(2) == 0 {
			docIDUpto = math.MaxInt32
		}
		term := index.NewTerm("id", strconv.Itoa(rand.Intn(100)))
		bu.AddTerm(*term, docIDUpto)
	}
	if !bu.Any() {
		t.Fatal("we have added tons of docIds, terms and queries")
	}

	totalUsed := bu.RamBytesUsed()
	if !(totalUsed > 0) {
		t.Fatalf("totalUsed=%d", totalUsed)
	}

	bu.ClearDeleteTerms()
	if !bu.Any() {
		t.Fatal("only terms and docIds are cleaned, the queries are still in memory")
	}
	if !(totalUsed > bu.RamBytesUsed()) {
		t.Fatal("terms are cleaned, ram in used should decrease")
	}

	bu.Clear()
	if bu.Any() {
		t.Fatal("any")
	}
	if bu.RamBytesUsed() != 0 {
		t.Fatalf("expected 0, got %d", bu.RamBytesUsed())
	}
}

type bufferedUpdatesEntry struct {
	field string
	bytes []byte
	docID int
}

func TestBufferedUpdatesDeletedTerms(t *testing.T) {
	iters := bufferedUpdatesAtLeast(10)
	fields := []string{"a", "b", "c"}
	actual := index.NewDeletedTerms()
	for iter := 0; iter < iters; iter++ {
		expected := make(map[string]bufferedUpdatesEntry)
		if !actual.IsEmpty() {
			t.Fatal("isEmpty")
		}

		termCount := bufferedUpdatesAtLeast(5000)
		maxBytesNum := rand.Intn(3) + 1
		for i := 0; i < termCount; i++ {
			byteNum := rand.Intn(maxBytesNum) + 1
			b := make([]byte, byteNum)
			rand.Read(b)
			term := index.NewTermFromBytesRef(fields[rand.Intn(len(fields))], util.NewBytesRef(b))
			value := rand.Intn(10000000)
			expected[term.Field+"\x00"+string(b)] = bufferedUpdatesEntry{field: term.Field, bytes: b, docID: value}
			actual.Put(*term, value)
		}

		if len(expected) != actual.Size() {
			t.Fatalf("expected %d, got %d", len(expected), actual.Size())
		}

		for _, entry := range expected {
			got := actual.Get(*index.NewTermFromBytesRef(entry.field, util.NewBytesRef(entry.bytes)))
			if entry.docID != got {
				t.Fatalf("expected %d, got %d", entry.docID, got)
			}
		}

		// Map.Entry.comparingByKey(): Term.compareTo orders by field, then bytes.
		expectedSorted := make([]bufferedUpdatesEntry, 0, len(expected))
		for _, e := range expected {
			expectedSorted = append(expectedSorted, e)
		}
		sort.Slice(expectedSorted, func(i, j int) bool {
			if expectedSorted[i].field != expectedSorted[j].field {
				return expectedSorted[i].field < expectedSorted[j].field
			}
			return bytes.Compare(expectedSorted[i].bytes, expectedSorted[j].bytes) < 0
		})
		actualSorted := actual.ForEachOrdered()

		if len(expectedSorted) != len(actualSorted) {
			t.Fatalf("expected %d entries, got %d", len(expectedSorted), len(actualSorted))
		}
		for i := range expectedSorted {
			e, a := expectedSorted[i], actualSorted[i]
			if e.field != a.Field || !bytes.Equal(e.bytes, a.Bytes) || e.docID != a.Value {
				t.Fatalf("entry %d: expected %v, got %v", i, e, a)
			}
		}

		actual.Clear()
		if actual.Size() != 0 {
			t.Fatalf("expected 0, got %d", actual.Size())
		}
		if actual.RamBytesUsed() != 0 {
			t.Fatalf("expected 0, got %d", actual.RamBytesUsed())
		}
		if actual.PoolBuffer() != nil {
			t.Fatal("actual.getPool().buffer must be null")
		}
	}
}
