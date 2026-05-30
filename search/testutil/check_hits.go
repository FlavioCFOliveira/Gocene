// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package testutil hosts search-side test helpers ported from Apache
// Lucene 10.4.0's lucene-test-framework. It provides [CheckHits], the
// canonical utility for asserting that a query matches an expected set
// of documents, that two result lists are equal, and that per-document
// score explanations are self-consistent.
//
// Lucene reference:
//
//	lucene/test-framework/src/java/org/apache/lucene/tests/search/CheckHits.java
//
// Scope note. The Lucene CheckHits also exposes checkHitCollector,
// checkMatches, and checkTopScores. Those depend on infrastructure not
// yet ported to Gocene — a docBase-aware Collector contract
// (search.Collector.GetLeafCollector currently takes the minimal
// search.IndexReader and exposes no docBase, tracked by roadmap #10),
// Weight.Matches, and the block-max Scorer surface
// (advanceShallow/getMaxScore/setMinCompetitiveScore/ScorerSupplier).
// Porting those helpers is tracked as roadmap #117. The result-set and
// explanation validators in this file rely only on the stable
// IndexSearcher.Search / IndexSearcher.Explain API and are complete.
package testutil

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// scoreTolerance mirrors CheckHits.checkEqual's 1.0e-6f tolerance for
// comparing the scores of two hit lists.
const scoreTolerance = 1.0e-6

// TB is the subset of testing.TB used by the CheckHits helpers.
// *testing.T and *testing.B satisfy it. Defining a local interface,
// rather than depending on testing.TB directly, keeps the assertion
// surface explicit and lets the package's own tests supply a recording
// stub to exercise both the pass and fail paths. Following Lucene's
// JUnit-throwing semantics, every helper returns immediately after a
// Fatalf so a stub that does not abort the goroutine still stops.
type TB interface {
	Helper()
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

// QueryString renders q for assertion messages, mirroring Lucene's
// Query.toString(field). A query may opt in to field-aware rendering
// by implementing ToString(field string) string; otherwise a
// fmt.Stringer is used, falling back to %v.
func QueryString(q search.Query, field string) string {
	switch v := q.(type) {
	case interface{ ToString(string) string }:
		return v.ToString(field)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", q)
	}
}

// CheckHits tests that query matches exactly the expected set of
// document ids using the top-docs API. Following Lucene, it requests
// max(10, len(results)*2) hits and compares the returned doc ids as a
// set (order-independent).
func CheckHits(t TB, query search.Query, defaultField string, searcher *search.IndexSearcher, results []int) {
	t.Helper()

	n := len(results) * 2
	if n < 10 {
		n = 10
	}
	top, err := searcher.Search(query, n)
	if err != nil {
		t.Fatalf("CheckHits: search failed for [[%s]]: %v", QueryString(query, defaultField), err)
		return
	}

	correct := intSet(results)
	actual := make(map[int]struct{}, len(top.ScoreDocs))
	for _, hit := range top.ScoreDocs {
		actual[hit.Doc] = struct{}{}
	}

	if !setsEqual(correct, actual) {
		t.Errorf("%s\n  expected docs: %v\n  actual docs:   %v",
			QueryString(query, defaultField), sortedKeys(correct), sortedKeys(actual))
	}
}

// CheckDocIds tests that hits has exactly the expected doc ids in the
// given order.
func CheckDocIds(t TB, mes string, results []int, hits []*search.ScoreDoc) {
	t.Helper()
	if len(hits) != len(results) {
		t.Errorf("%s nr of hits: expected %d, got %d", mes, len(results), len(hits))
		return
	}
	for i := range results {
		if hits[i].Doc != results[i] {
			t.Errorf("%s doc nrs for hit %d: expected %d, got %d", mes, i, results[i], hits[i].Doc)
		}
	}
}

// CheckHitsQuery tests that two queries produce the expected document
// order and that the two hit lists are equal (doc ids and scores).
func CheckHitsQuery(t TB, query search.Query, hits1, hits2 []*search.ScoreDoc, results []int) {
	t.Helper()
	CheckDocIds(t, "hits1", results, hits1)
	CheckDocIds(t, "hits2", results, hits2)
	CheckEqual(t, query, hits1, hits2)
}

// CheckEqual asserts that two hit lists are equal in length, doc ids,
// and scores (within scoreTolerance).
func CheckEqual(t TB, query search.Query, hits1, hits2 []*search.ScoreDoc) {
	t.Helper()
	if len(hits1) != len(hits2) {
		t.Fatalf("Unequal lengths: hits1=%d,hits2=%d", len(hits1), len(hits2))
		return
	}
	for i := range hits1 {
		if hits1[i].Doc != hits2[i].Doc {
			t.Fatalf("Hit %d docnumbers don't match\n%sfor query:%s",
				i, Hits2Str(hits1, hits2, 0, 0), QueryString(query, ""))
			return
		}
		if math.Abs(float64(hits1[i].Score-hits2[i].Score)) > scoreTolerance {
			t.Fatalf("Hit %d, doc nrs %d and %d\nunequal       : %v\n           and: %v\nfor query:%s",
				i, hits1[i].Doc, hits2[i].Doc, hits1[i].Score, hits2[i].Score, QueryString(query, ""))
			return
		}
	}
}

// Hits2Str formats two hit lists for diagnostic messages, mirroring
// CheckHits.hits2str. end <= 0 means "to the longer of the two".
func Hits2Str(hits1, hits2 []*search.ScoreDoc, start, end int) string {
	var sb strings.Builder
	len1, len2 := len(hits1), len(hits2)
	if end <= 0 {
		end = len1
		if len2 > end {
			end = len2
		}
	}
	fmt.Fprintf(&sb, "Hits length1=%d\tlength2=%d\n", len1, len2)
	for i := start; i < end; i++ {
		fmt.Fprintf(&sb, "hit=%d:", i)
		if i < len1 {
			fmt.Fprintf(&sb, " doc%d=%v shardIndex=%d", hits1[i].Doc, hits1[i].Score, hits1[i].ShardIndex)
		} else {
			sb.WriteString("               ")
		}
		sb.WriteString(",\t")
		if i < len2 {
			fmt.Fprintf(&sb, " doc%d=%v shardIndex=%d", hits2[i].Doc, hits2[i].Score, hits2[i].ShardIndex)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// TopDocsString formats a TopDocs for diagnostic messages, mirroring
// CheckHits.topdocsString. end <= 0 means "to the end of scoreDocs".
func TopDocsString(docs *search.TopDocs, start, end int) string {
	var sb strings.Builder
	total := int64(0)
	if docs.TotalHits != nil {
		total = docs.TotalHits.Value
	}
	fmt.Fprintf(&sb, "TopDocs totalHits=%d top=%d\n", total, len(docs.ScoreDocs))
	if end <= 0 {
		end = len(docs.ScoreDocs)
	} else if end > len(docs.ScoreDocs) {
		end = len(docs.ScoreDocs)
	}
	for i := start; i < end; i++ {
		fmt.Fprintf(&sb, "\t%d) doc=%d\tscore=%v\n", i, docs.ScoreDocs[i].Doc, docs.ScoreDocs[i].Score)
	}
	return sb.String()
}

// CheckNoMatchExplanations tests that every document up to maxDoc which
// is *not* in the expected result set has an explanation that indicates
// a non-match.
func CheckNoMatchExplanations(t TB, q search.Query, defaultField string, searcher *search.IndexSearcher, results []int) {
	t.Helper()
	d := QueryString(q, defaultField)
	ignore := intSet(results)
	maxDoc := searcher.GetIndexReader().MaxDoc()
	for doc := 0; doc < maxDoc; doc++ {
		if _, skip := ignore[doc]; skip {
			continue
		}
		exp, err := searcher.Explain(q, doc)
		if err != nil {
			t.Errorf("Explanation of [[%s]] for #%d errored: %v", d, doc, err)
			continue
		}
		if exp == nil {
			t.Errorf("Explanation of [[%s]] for #%d is null", d, doc)
			continue
		}
		if exp.IsMatch() {
			t.Errorf("Explanation of [[%s]] for #%d doesn't indicate non-match: %s",
				d, doc, ExplanationString(exp))
		}
	}
}

// CheckExplanations asserts that the explanation value for every
// document matching a query corresponds with the true score. When deep
// is true, the sub-detail combine rule (product/sum/max/...) is also
// verified. Unlike the Lucene reference, which drives a collector, this
// port enumerates matches via the top-docs API (requesting every doc),
// which yields the same per-document score under COMPLETE scoring.
func CheckExplanations(t TB, query search.Query, defaultField string, searcher *search.IndexSearcher, deep bool) {
	t.Helper()
	d := QueryString(query, defaultField)
	maxDoc := searcher.GetIndexReader().MaxDoc()
	n := maxDoc
	if n < 1 {
		n = 1
	}
	top, err := searcher.Search(query, n)
	if err != nil {
		t.Fatalf("CheckExplanations: search failed for [[%s]]: %v", d, err)
		return
	}
	for _, hit := range top.ScoreDocs {
		exp, err := searcher.Explain(query, hit.Doc)
		if err != nil {
			t.Errorf("exception in explanation of [[%s]] for #%d: %v", d, hit.Doc, err)
			continue
		}
		if exp == nil {
			t.Errorf("Explanation of [[%s]] for #%d is null", d, hit.Doc)
			continue
		}
		VerifyExplanation(t, d, hit.Doc, hit.Score, deep, exp)
		if !exp.IsMatch() {
			t.Errorf("Explanation of [[%s]] for #%d does not indicate match: %s",
				d, hit.Doc, ExplanationString(exp))
		}
	}
}

// VerifyExplanation asserts that an explanation has the expected score
// and, optionally, that its sub-detail max/sum/product/factor combine
// to that score. This is a faithful port of CheckHits.verifyExplanation.
func VerifyExplanation(t TB, q string, doc int, score float32, deep bool, expl search.Explanation) {
	t.Helper()
	value := expl.GetValue()
	if value != score {
		t.Errorf("%s: score(doc=%d)=%v != explanationScore=%v Explanation: %s",
			q, doc, score, value, ExplanationString(expl))
	}

	if !deep {
		return
	}

	detail := expl.GetDetails()
	if strings.HasSuffix(expl.GetDescription(), "computed from:") {
		return // something more complicated.
	}
	descr := strings.ToLower(expl.GetDescription())
	if strings.HasPrefix(descr, "score based on ") && strings.Contains(descr, "child docs in range") {
		if len(detail) == 0 {
			t.Errorf("Child doc explanations are missing")
		}
	}
	if len(detail) > 0 && expl.IsMatch() {
		if len(detail) == 1 && !computedFromPattern(descr) {
			// Simple containment, unless it's a "freq of:" (which lets a
			// query explain how the freq is calculated); just verify the
			// contained explanation has the same score.
			if !strings.HasSuffix(expl.GetDescription(), "with freq of:") &&
				(score >= 0 || !strings.HasSuffix(expl.GetDescription(), "times others of:")) {
				VerifyExplanation(t, q, doc, score, deep, detail[0])
			}
		} else {
			// The explanation must either end with one of "product of:",
			// "sum of:", "max of:", be "computed as x from:", or read
			// "max plus <x> times others of:".
			var x float32
			productOf := strings.HasSuffix(descr, "product of:")
			sumOf := strings.HasSuffix(descr, "sum of:")
			maxOf := strings.HasSuffix(descr, "max of:")
			computedOf := strings.Index(descr, "computed as") > 0 && computedFromPattern(descr)
			maxTimesOthers := false
			if !(productOf || sumOf || maxOf || computedOf) {
				k1 := strings.Index(descr, "max plus ")
				if k1 >= 0 {
					k1 += len("max plus ")
					k2 := strings.IndexByte(descr[k1:], ' ')
					if k2 >= 0 {
						k2 += k1
						if f, err := strconv.ParseFloat(strings.TrimSpace(descr[k1:k2]), 32); err == nil {
							x = float32(f)
							if strings.TrimSpace(descr[k2:]) == "times others of:" {
								maxTimesOthers = true
							}
						}
					}
				}
			}
			if !(productOf || sumOf || maxOf || computedOf || maxTimesOthers) {
				t.Errorf("%s: multi valued explanation description=%q must be 'max of plus x times others', "+
					"'computed as x from:' or end with 'product of' or 'sum of:' or 'max of:' - %s",
					q, descr, ExplanationString(expl))
			}
			var sum float64
			var product float32 = 1
			max := float32(math.Inf(-1))
			var maxError float64
			for i := range detail {
				dval := detail[i].GetValue()
				VerifyExplanation(t, q, doc, dval, deep, detail[i])
				product *= dval
				sum += float64(dval)
				if dval > max {
					max = dval
				}
				if sumOf {
					// "sum of" is used by BooleanQuery; intermediate float
					// casts in ReqOptSumScorer require some leniency.
					maxError += float64(ulp32(dval)) * 2
				}
			}
			var combined float32
			switch {
			case productOf:
				combined = product
			case sumOf:
				combined = float32(sum)
			case maxOf:
				combined = max
			case maxTimesOthers:
				combined = float32(float64(max) + float64(x)*(sum-float64(max)))
			default:
				if !computedOf {
					t.Errorf("should never get here!")
				}
				combined = value
			}
			if math.Abs(float64(combined-value)) > maxError {
				t.Errorf("%s: actual subDetails combined==%v != value=%v Explanation: %s",
					q, combined, value, ExplanationString(expl))
			}
		}
	}
}

// --- Internal -------------------------------------------------------

// ExplanationString renders an Explanation tree for diagnostics,
// mirroring the layout of Lucene's Explanation.toString.
func ExplanationString(e search.Explanation) string {
	var sb strings.Builder
	explanationToString(&sb, e, 0)
	return sb.String()
}

func explanationToString(sb *strings.Builder, e search.Explanation, depth int) {
	for i := 0; i < depth; i++ {
		sb.WriteString("  ")
	}
	if e.IsMatch() {
		fmt.Fprintf(sb, "%v = %s\n", e.GetValue(), e.GetDescription())
	} else {
		fmt.Fprintf(sb, "%v = (NON-MATCH) %s\n", e.GetValue(), e.GetDescription())
	}
	for _, d := range e.GetDetails() {
		explanationToString(sb, d, depth+1)
	}
}

// computedFromPattern reports whether descr matches the Lucene
// ".*, computed as .* from:" anchored pattern. Java's Pattern.matches
// is full-string; we replicate it with substring checks ordered so the
// "computed as" precedes " from:" at the tail.
func computedFromPattern(descr string) bool {
	if !strings.HasSuffix(descr, " from:") {
		return false
	}
	ca := strings.Index(descr, ", computed as ")
	return ca >= 0 && ca < len(descr)-len(" from:")
}

// ulp32 returns the unit in the last place of x as a float32, matching
// Java's Math.ulp(float).
func ulp32(x float32) float32 {
	if x != x { // NaN
		return x
	}
	if math.IsInf(float64(x), 0) {
		return float32(math.Inf(1))
	}
	if x == 0 {
		return math.Float32frombits(1)
	}
	bits := math.Float32bits(x)
	// Step to the next representable magnitude and take the gap.
	next := math.Float32frombits(bits + 1)
	d := next - x
	if d < 0 {
		d = -d
	}
	return d
}

func intSet(vals []int) map[int]struct{} {
	s := make(map[int]struct{}, len(vals))
	for _, v := range vals {
		s[v] = struct{}{}
	}
	return s
}

func setsEqual(a, b map[int]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func sortedKeys(s map[int]struct{}) []int {
	out := make([]int, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}
