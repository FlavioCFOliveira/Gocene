// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Shared helpers for the ported ParentBlockJoinKnnVectorQueryTestCase tests.
// They mirror the Java test-case helper methods (assertScorerResults,
// assertIdMatches, createFamily, randomVector, CountingQueryTimeout) on top of
// the Gocene IndexWriter / IndexSearcher surface.

package join

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// pbjItoa renders an int as its plain decimal string. The package-level itoa
// in block_join_test_helpers_test.go is a 4-digit zero-padded year formatter,
// unsuitable for the unpadded ids these ports assert against.
func pbjItoa(i int) string { return strconv.Itoa(i) }

// min3 returns the minimum of three ints.
func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// pbjVector returns a constant dim-vector with every component set to v.
func pbjVector(dim int, v float32) []float32 {
	out := make([]float32, dim)
	for i := range out {
		out[i] = v
	}
	return out
}

// pbjScores formats the scores of a TopDocs for diagnostics.
func pbjScores(td *search.TopDocs) []float32 {
	out := make([]float32, len(td.ScoreDocs))
	for i, sd := range td.ScoreDocs {
		out[i] = sd.Score
	}
	return out
}

// pbjIDs resolves the stored "id" of each hit for diagnostics.
func pbjIDs(t *testing.T, s *search.IndexSearcher, td *search.TopDocs) []string {
	t.Helper()
	out := make([]string, len(td.ScoreDocs))
	for i, sd := range td.ScoreDocs {
		doc, err := s.Doc(sd.Doc)
		if err != nil {
			out[i] = fmt.Sprintf("<err:%v>", err)
			continue
		}
		out[i] = storedString(doc, "id")
	}
	return out
}

// pbjAssertIDMatches asserts the stored "id" of docID equals want.
// Mirrors ParentBlockJoinKnnVectorQueryTestCase.assertIdMatches.
func pbjAssertIDMatches(t *testing.T, s *search.IndexSearcher, want string, docID int) {
	t.Helper()
	doc, err := s.Doc(docID)
	if err != nil {
		t.Fatalf("Doc(%d): %v", docID, err)
	}
	if got := storedString(doc, "id"); got != want {
		t.Errorf("id of doc %d = %q, want %q", docID, got, want)
	}
}

// pbjAssertScorerResults drives the per-leaf Scorer of query and asserts the
// first count hits have ids/scores drawn from idToScore. Mirrors
// ParentBlockJoinKnnVectorQueryTestCase.assertScorerResults.
func pbjAssertScorerResults(
	t *testing.T,
	s *search.IndexSearcher,
	r *index.DirectoryReader,
	query search.Query,
	idToScore map[string]float32,
	count int,
) {
	t.Helper()
	scorer := firstLeafScorer(t, s, r, query)
	if scorer == nil {
		t.Fatalf("nil scorer for query %v", query)
	}
	// Prior to advancing, the scorer doc is undefined (-1).
	if scorer.DocID() != -1 {
		t.Errorf("initial scorer.DocID() = %d, want -1", scorer.DocID())
	}
	for i := 0; i < count; i++ {
		docID, err := scorer.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if docID == search.NO_MORE_DOCS {
			t.Fatalf("NextDoc[%d] = NO_MORE_DOCS, want a hit", i)
		}
		doc, err := s.Doc(docID)
		if err != nil {
			t.Fatalf("Doc(%d): %v", docID, err)
		}
		id := storedString(doc, "id")
		want, ok := idToScore[id]
		if !ok {
			t.Fatalf("hit id %q not in expected set %v", id, idToScore)
		}
		sc := scorer.Score()
		if diff := sc - want; diff > 1e-4 || diff < -1e-4 {
			t.Errorf("score for id %q = %v, want %v", id, sc, want)
		}
	}
}

// pbjAddFamily adds one parent/child family of the given size and dimension,
// each child carrying a random vector and a stored "parentId". The parent
// itself is the trailing docType=_parent document with no stored parentId.
// Mirrors ParentBlockJoinKnnVectorQueryTestCase.createFamily, where the
// StoredField("parentId") is added to the child documents (the result docs).
func pbjAddFamily(t *testing.T, w *index.IndexWriter, parentID string, size, dim int) {
	t.Helper()
	rng := newPBJRand(int64(hashString(parentID)))
	var family []index.Document
	for i := 0; i < size; i++ {
		d := document.NewDocument()
		vf, err := document.NewKnnFloatVectorFieldEuclidean("field", pbjRandomVector(rng, dim))
		if err != nil {
			t.Fatalf("NewKnnFloatVectorField: %v", err)
		}
		d.Add(vf)
		d.Add(mustStringField(t, "parentId", parentID, true))
		family = append(family, d)
	}
	family = append(family, pbjMakeParent(t, ""))
	addBlock(t, w, family...)
}

// pbjAllDocsIterator returns a DocIdSetIterator over all docs in ctx, used to
// drive ExactSearch directly in the timeout test. The query parameter is
// accepted for symmetry with the search path but unused.
func pbjAllDocsIterator(_ *search.IndexSearcher, ctx *index.LeafReaderContext, _ search.Query) (search.DocIdSetIterator, error) {
	maxDoc := ctx.LeafReader().MaxDoc()
	accept := search.AcceptDocsFromLiveDocs(nil, maxDoc)
	return accept.Iterator()
}

// pbjCountingTimeout exits after the first `remaining` ShouldExit calls return
// false. Mirrors ParentBlockJoinKnnVectorQueryTestCase.CountingQueryTimeout.
type pbjCountingTimeout struct{ remaining int }

func (c *pbjCountingTimeout) ShouldExit() bool {
	if c.remaining > 0 {
		c.remaining--
		return false
	}
	return true
}

// --- deterministic RNG (java.util.Random LCG) -----------------------------
//
// A faithful java.util.Random clone keeps the random-vector tests
// deterministic without depending on Go's PRNG implementation details.

type pbjRand struct{ seed int64 }

func newPBJRand(seed int64) *pbjRand {
	return &pbjRand{seed: (seed ^ 0x5DEECE66D) & ((1 << 48) - 1)}
}

func (r *pbjRand) next(bits int) int32 {
	r.seed = (r.seed*0x5DEECE66D + 0xB) & ((1 << 48) - 1)
	return int32(int64(uint64(r.seed) >> uint(48-bits)))
}

// intn returns a pseudo-random int in [0, n).
func (r *pbjRand) intn(n int) int {
	if n <= 0 {
		return 0
	}
	if n&(n-1) == 0 { // power of two
		return int((int64(n) * int64(r.next(31))) >> 31)
	}
	for {
		bits := int(r.next(31))
		val := bits % n
		if bits-val+(n-1) >= 0 {
			return val
		}
	}
}

// float32 returns a pseudo-random float32 in [0, 1).
func (r *pbjRand) float32() float32 {
	return float32(r.next(24)) / float32(1<<24)
}

// pbjRandomVector returns a dim-vector of pseudo-random components in [0, 1).
// Mirrors the abstract randomVector(dim).
func pbjRandomVector(r *pbjRand, dim int) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = r.float32()
	}
	return v
}

// hashString gives a stable seed from a string (FNV-1a-ish, sufficient for
// deterministic test vectors).
func hashString(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}
