// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
)

// Port of
// lucene/join/src/test/org/apache/lucene/search/join/TestBlockJoinBulkScorer.java
// (Apache Lucene 10.5.0).

const (
	typeFieldName     = "type"
	valueFieldName    = "value"
	parentFilterValue = "parent"
	childFilterValue  = "child"
)

// matchValue renders the private enum MatchValue.
type matchValue int

const (
	matchA matchValue = iota
	matchB
	matchC
	matchD
)

// matchValues renders MatchValue.VALUES.
var matchValues = []matchValue{matchA, matchB, matchC, matchD}

func (m matchValue) getText() string { return [...]string{"A", "B", "C", "D"}[m] }

func (m matchValue) getScore() int { return [...]int{1, 2, 3, 4}[m] }

func (m matchValue) String() string { return m.getText() }

// randomMatchValue renders MatchValue.random().
func randomMatchValue() matchValue {
	return matchValues[random().Intn(len(matchValues))]
}

// childDocMatch renders the private record ChildDocMatch(int docId,
// List<MatchValue> matches).
type childDocMatch struct {
	docID   int
	matches []matchValue
}

func populateRandomIndex(t testing.TB, writer *testindex.RandomIndexWriter, maxParentDocCount, maxChildDocCount,
	maxChildDocMatches int) map[int][]childDocMatch {
	t.Helper()
	expectedMatches := map[int][]childDocMatch{}

	parentDocCount := 1 + random().Intn(maxParentDocCount) // random().nextInt(1, maxParentDocCount + 1)
	currentDocID := 0
	for i := 0; i < parentDocCount; i++ {
		childDocCount := random().Intn(maxChildDocCount + 1)
		docs := make([]*document.Document, 0, childDocCount)
		childDocMatches := make([]childDocMatch, 0, childDocCount)

		for j := 0; j < childDocCount; j++ {
			// Build a child doc
			childDoc := newTestDocument(newStringField(t, typeFieldName, childFilterValue, false))

			matchCount := random().Intn(maxChildDocMatches + 1)
			values := make([]matchValue, 0, matchCount)
			for k := 0; k < matchCount; k++ {
				// Add a match to the child doc
				mv := randomMatchValue()
				values = append(values, mv)
				childDoc.Add(newStringField(t, valueFieldName, mv.getText(), false))
			}

			docs = append(docs, childDoc)
			childDocMatches = append(childDocMatches, childDocMatch{docID: currentDocID, matches: values})
			currentDocID++
		}

		// Build a parent doc
		parentDoc := newTestDocument(newStringField(t, typeFieldName, parentFilterValue, false))
		docs = append(docs, parentDoc)

		// Don't add parent docs with no children to expectedMatches
		if childDocCount > 0 {
			expectedMatches[currentDocID] = childDocMatches
		}
		currentDocID++

		mustAddDocuments(t, writer, docs...)
	}

	return expectedMatches
}

func populateStaticIndex(t testing.TB, writer *testindex.RandomIndexWriter) {
	t.Helper()
	// Use these vars to improve readability when defining the docs
	a := matchA.getText()
	b := matchB.getText()
	c := matchC.getText()
	d := matchD.getText()

	for _, values := range [][][]string{
		{{a, b}, {a, b, c}},
		{{a}, {b}},
		{{}},
		{{a, b, c}, {a, b, c, d}},
		{{b}},
		{{b, c}, {a, b}, {a, c}},
	} {
		docs := make([]*document.Document, 0)
		for _, value := range values {
			childDoc := newTestDocument(newStringField(t, typeFieldName, childFilterValue, false))
			for _, v := range value {
				childDoc.Add(newStringField(t, valueFieldName, v, false))
			}
			docs = append(docs, childDoc)
		}

		parentDoc := newTestDocument(newStringField(t, typeFieldName, parentFilterValue, false))
		docs = append(docs, parentDoc)

		mustAddDocuments(t, writer, docs...)
	}
}

func computeExpectedScores(expectedMatches map[int][]childDocMatch, joinScoreMode ScoreMode,
	searchScoreMode search.ScoreMode) map[int]float32 {
	expectedScores := map[int]float32{}
	for key, value := range expectedMatches {
		// Filter out child docs with no matches since those will never contribute to the score
		childDocMatches := make([]childDocMatch, 0)
		for _, m := range value {
			if len(m.matches) != 0 {
				childDocMatches = append(childDocMatches, m)
			}
		}
		if len(childDocMatches) == 0 {
			continue
		}

		expectedScore := 0.0
		if searchScoreMode.NeedsScores() {
			firstScore := true
			for _, cdm := range childDocMatches {
				expectedChildDocScore := computeExpectedScore(cdm)
				switch joinScoreMode {
				case Total, Avg:
					expectedScore += float64(expectedChildDocScore)
				case Min:
					if firstScore {
						expectedScore = float64(expectedChildDocScore)
					} else {
						expectedScore = math.Min(expectedScore, float64(expectedChildDocScore))
					}
				case Max:
					expectedScore = math.Max(expectedScore, float64(expectedChildDocScore))
				case None:
				default:
					panic("AssertionError")
				}

				firstScore = false
			}

			if joinScoreMode == Avg {
				expectedScore /= float64(len(childDocMatches))
			}
		}

		expectedScores[key] = float32(expectedScore)
	}

	return expectedScores
}

func computeExpectedScore(cdm childDocMatch) float32 {
	expectedScore := float32(0.0)
	matchValueSet := map[matchValue]struct{}{}
	for _, m := range cdm.matches {
		matchValueSet[m] = struct{}{}
	}
	for mv := range matchValueSet {
		expectedScore += float32(mv.getScore())
	}

	return expectedScore
}

func buildBulkScorerQuery(scoreMode ScoreMode) *ToParentBlockJoinQuery {
	childQueryBuilder := search.NewBooleanQueryBuilder()
	for _, mv := range matchValues {
		childQueryBuilder.Add(
			search.NewBoostQuery(search.NewConstantScoreQuery(search.NewTermQuery(index.NewTerm(valueFieldName, mv.getText()))),
				float32(mv.getScore())),
			search.SHOULD)
	}
	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm(typeFieldName, parentFilterValue)))
	return NewToParentBlockJoinQuery(childQueryBuilder.Build(), parentsFilter, scoreMode)
}

// bulkScorerOf renders ScorerSupplier.bulkScorer().
func bulkScorerOf(t testing.TB, ss search.ScorerSupplier) search.BulkScorer {
	t.Helper()
	var bs search.BulkScorer
	var err error
	if b, ok := ss.(interface {
		BulkScorer() (search.BulkScorer, error)
	}); ok {
		bs, err = b.BulkScorer()
	} else {
		bs, err = search.DefaultScorerSupplierBulkScorer(ss)
	}
	if err != nil {
		t.Fatal(err)
	}
	return bs
}

// scoresCollector is the anonymous LeafCollector of assertScores.
type scoresCollector struct {
	search.BaseLeafCollector
	t            testing.TB
	scoreMode    search.ScoreMode
	minScore     *float32
	scorer       search.Scorable
	actualScores map[int]float32
}

func (c *scoresCollector) SetScorer(scorer search.Scorable) error {
	if scorer == nil {
		return fmt.Errorf("assertNotNull(scorer) failed")
	}
	c.scorer = scorer
	if c.minScore != nil {
		return c.scorer.SetMinCompetitiveScore(*c.minScore)
	}
	return nil
}

func (c *scoresCollector) Collect(doc int) error {
	if c.scorer == nil {
		return fmt.Errorf("assertNotNull(scorer) failed")
	}
	var score float32
	if c.scoreMode.NeedsScores() {
		s, err := c.scorer.Score()
		if err != nil {
			return err
		}
		score = s
	}
	c.actualScores[doc] = score
	return nil
}

func (c *scoresCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

func (c *scoresCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

func assertScores(t testing.TB, bulkScorer search.BulkScorer, scoreMode search.ScoreMode, minScore *float32,
	expectedScoresList ...map[int]float32) {
	t.Helper()
	actualScores := map[int]float32{}
	collector := &scoresCollector{t: t, scoreMode: scoreMode, minScore: minScore, actualScores: actualScores}
	if _, err := bulkScorer.Score(collector, nil, 0, search.NO_MORE_DOCS); err != nil {
		t.Fatalf("score: %v", err)
	}

	if len(expectedScoresList) == 1 {
		if !scoreMapsEqual(expectedScoresList[0], actualScores) {
			t.Fatalf("expected %v, got %v", expectedScoresList[0], actualScores)
		}
	} else {
		assertEqualsToOneOf(t, expectedScoresList, actualScores)
	}
}

func scoreMapsEqual(a, b map[int]float32) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

func assertEqualsToOneOf(t testing.TB, expectedList []map[int]float32, actual map[int]float32) {
	t.Helper()
	foundMatch := false
	for _, expected := range expectedList {
		if scoreMapsEqual(expected, actual) {
			foundMatch = true
			break
		}
	}

	if !foundMatch {
		t.Fatalf("expected one of: %v but was: %v", expectedList, actual)
	}
}
