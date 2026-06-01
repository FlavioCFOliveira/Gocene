// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestInetAddressRangeQueries.java
//   (concrete instantiation of BaseRangeFieldQueryTestCase)
//
// These tests build random single-dimension IP ranges (InetAddressRange,
// 16 bytes/dim), run every supported query type (INTERSECTS / WITHIN / CONTAINS
// / CROSSES) through the live BKD points path, and assert each doc's hit/miss
// against the brute-force oracle ported from BaseRangeFieldQueryTestCase.verify
// / expectedBBoxQueryResult and the IpRange relate() predicates. testMultiValued
// additionally indexes some docs with two ranges. Seeds are fixed for
// determinism (the Lucene original uses LuceneTestCase randomness).

package search_test

import (
	"bytes"
	"math/rand"
	"net"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const ipRangeFieldName = "ipRangeField"

// ipRange is the Go port of TestInetAddressRangeQueries.IpRange: a single-dim
// IP range holding the 16-byte encoded min/max. The relate predicates mirror
// the Java IpRange exactly (unsigned byte comparison).
type ipRange struct {
	min []byte // encoded (16 bytes)
	max []byte
}

func newIPRange(a, b net.IP) ipRange {
	ea := document.EncodeInetAddress(a)
	eb := document.EncodeInetAddress(b)
	if bytes.Compare(ea, eb) > 0 {
		ea, eb = eb, ea
	}
	return ipRange{min: ea, max: eb}
}

func (r ipRange) isEqual(o ipRange) bool {
	return bytes.Equal(r.min, o.min) && bytes.Equal(r.max, o.max)
}
func (r ipRange) isDisjoint(o ipRange) bool {
	return bytes.Compare(r.min, o.max) > 0 || bytes.Compare(r.max, o.min) < 0
}
func (r ipRange) isWithin(o ipRange) bool {
	return bytes.Compare(r.min, o.min) >= 0 && bytes.Compare(r.max, o.max) <= 0
}
func (r ipRange) contains(o ipRange) bool {
	return bytes.Compare(r.min, o.min) <= 0 && bytes.Compare(r.max, o.max) >= 0
}

// relate mirrors BaseRangeFieldQueryTestCase.Range.relate: returns one of
// "within"/"contains"/"crosses", or "" for disjoint.
func (r ipRange) relate(o ipRange) string {
	if r.isDisjoint(o) {
		return ""
	}
	if r.isWithin(o) {
		return "within"
	}
	if r.contains(o) {
		return "contains"
	}
	return "crosses"
}

// expectedBBoxQueryResult ports BaseRangeFieldQueryTestCase.expectedBBoxQueryResult.
func expectedBBoxQueryResult(queryRange ipRange, doc ipRange, queryType string) bool {
	if queryRange.isEqual(doc) && queryType != "crosses" {
		return true
	}
	rel := doc.relate(queryRange)
	switch queryType {
	case "intersects":
		return rel != ""
	case "crosses":
		// Ranges that CONTAIN the query are also considered to cross.
		return rel == "crosses" || rel == "contains"
	default: // within / contains
		return rel == queryType
	}
}

// expectedResult ports the multi-valued expectedResult: a doc matches if any of
// its ranges matches.
func expectedResult(queryRange ipRange, docRanges []ipRange, queryType string) bool {
	for _, r := range docRanges {
		if expectedBBoxQueryResult(queryRange, r, queryType) {
			return true
		}
	}
	return false
}

// randomInetAddress returns a random IPv4 or IPv6 address with the same
// distribution Lucene's nextInetaddress uses (all-zero / all-ff / all-0x2a /
// random).
func randomInetAddress(rng *rand.Rand) net.IP {
	width := 4
	if rng.Intn(2) == 0 {
		width = 16
	}
	b := make([]byte, width)
	switch rng.Intn(5) {
	case 0:
		// all zero
	case 1:
		for i := range b {
			b[i] = 0xff
		}
	case 2:
		for i := range b {
			b[i] = 42
		}
	default:
		rng.Read(b)
	}
	return net.IP(b)
}

func randomIPRange(rng *rand.Rand) ipRange {
	return newIPRange(randomInetAddress(rng), randomInetAddress(rng))
}

// ipQuery builds the BKD-backed RangeFieldQuery for the given query type.
func ipQuery(t *testing.T, qr ipRange, queryType string) search.Query {
	t.Helper()
	var qt search.RangeFieldQueryType
	switch queryType {
	case "intersects":
		qt = search.RangeFieldQueryTypeIntersects
	case "within":
		qt = search.RangeFieldQueryTypeWithin
	case "contains":
		qt = search.RangeFieldQueryTypeContains
	case "crosses":
		qt = search.RangeFieldQueryTypeCrosses
	}
	q, err := search.NewRangeFieldQueryFull(ipRangeFieldName, qr.min, qr.max, 1, document.InetAddressPointBytes, qt)
	if err != nil {
		t.Fatalf("NewRangeFieldQueryFull: %v", err)
	}
	return q
}

// verifyIPRanges ports BaseRangeFieldQueryTestCase.verify for single/multi-valued
// IP ranges (no deletions, deterministic seed). docRanges[i] is the list of
// ranges for doc i (empty == missing).
func verifyIPRanges(t *testing.T, seed int64, docRanges [][]ipRange) {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))

	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for _, ranges := range docRanges {
		doc := document.NewDocument()
		for _, r := range ranges {
			f, err := document.NewInetAddressRange(ipRangeFieldName, net.IP(r.min), net.IP(r.max))
			if err != nil {
				t.Fatalf("NewInetAddressRange: %v", err)
			}
			doc.Add(f)
		}
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	defer func() { _ = w.Close() }()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = reader.Close() }()
	searcher := search.NewIndexSearcher(reader)

	queryTypes := []string{"intersects", "within", "contains", "crosses"}
	const iters = 25
	for iter := 0; iter < iters; iter++ {
		qr := randomIPRange(rng)
		queryType := queryTypes[rng.Intn(len(queryTypes))]

		top, err := searcher.Search(ipQuery(t, qr, queryType), len(docRanges)+1)
		if err != nil {
			t.Fatalf("iter %d Search(%s): %v", iter, queryType, err)
		}
		hitSet := make(map[int]bool, len(top.ScoreDocs))
		for _, sd := range top.ScoreDocs {
			hitSet[sd.Doc] = true
		}

		for id, ranges := range docRanges {
			expected := len(ranges) > 0 && expectedResult(qr, ranges, queryType)
			if hitSet[id] != expected {
				t.Fatalf("iter %d queryType=%s id=%d: hit=%v expected=%v (query=[%x..%x])",
					iter, queryType, id, hitSet[id], expected, qr.min, qr.max)
			}
		}
	}
}

// makeSingleValuedDocs builds n single-valued docs, ~5% missing.
func makeSingleValuedDocs(rng *rand.Rand, n int) [][]ipRange {
	docs := make([][]ipRange, n)
	for i := range docs {
		if rng.Intn(20) == 17 { // ~5% missing
			docs[i] = nil
			continue
		}
		docs[i] = []ipRange{randomIPRange(rng)}
	}
	return docs
}

// makeMultiValuedDocs builds n docs, some with up to 2 ranges, ~5% missing.
func makeMultiValuedDocs(rng *rand.Rand, n int) [][]ipRange {
	docs := make([][]ipRange, n)
	for i := range docs {
		if rng.Intn(20) == 17 {
			docs[i] = nil
			continue
		}
		count := 1
		if rng.Intn(2) == 0 {
			count = 1 + rng.Intn(2) // 1 or 2 values
		}
		ranges := make([]ipRange, count)
		for k := range ranges {
			ranges[k] = randomIPRange(rng)
		}
		docs[i] = ranges
	}
	return docs
}

// TestInetAddressRangeQueries_RandomTiny ports testRandomTiny: small single-leaf
// indexes, repeated.
func TestInetAddressRangeQueries_RandomTiny(t *testing.T) {
	gen := rand.New(rand.NewSource(0x71117))
	for i := 0; i < 10; i++ {
		verifyIPRanges(t, gen.Int63(), makeSingleValuedDocs(gen, 10))
	}
}

// TestInetAddressRangeQueries_RandomMedium ports testRandomMedium.
func TestInetAddressRangeQueries_RandomMedium(t *testing.T) {
	gen := rand.New(rand.NewSource(0x4ED10))
	verifyIPRanges(t, gen.Int63(), makeSingleValuedDocs(gen, 1000))
}

// TestInetAddressRangeQueries_RandomBig ports testRandomBig (scaled down from
// the @Nightly 200k to keep the unit run fast while exercising many segments).
func TestInetAddressRangeQueries_RandomBig(t *testing.T) {
	gen := rand.New(rand.NewSource(0xB16))
	verifyIPRanges(t, gen.Int63(), makeSingleValuedDocs(gen, 5000))
}

// TestInetAddressRangeQueries_MultiValued ports testMultiValued: docs with up to
// two ranges each.
func TestInetAddressRangeQueries_MultiValued(t *testing.T) {
	gen := rand.New(rand.NewSource(0x3C0FF))
	verifyIPRanges(t, gen.Int63(), makeMultiValuedDocs(gen, 1000))
}
