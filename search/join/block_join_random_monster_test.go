// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

package join

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
)

// TestBlockJoin.testRandom (Apache Lucene 10.5.0) is annotated @Nightly
// ("TODO: incredibly slow"); it is built only with the gocene_monsters tag.

// testUtilCloneDocumentBlocker names TestUtil.cloneDocument(Document).
const testUtilCloneDocumentBlocker = "requires org.apache.lucene.tests.util.TestUtil.cloneDocument(Document) (not ported)"

// intPointNewSetQueryBlocker names IntPoint.newSetQuery(String, Collection<Integer>).
const intPointNewSetQueryBlocker = "requires org.apache.lucene.document.IntPoint.newSetQuery(String, Collection<Integer>) (not ported)"

func getRandomFields(maxUniqueValues int) [][]string {
	fields := make([][]string, nextInt(2, 4))
	for fieldID := range fields {
		var valueCount int
		if fieldID == 0 {
			valueCount = 2
		} else {
			valueCount = nextInt(1, maxUniqueValues)
		}

		values := make([]string, valueCount)
		fields[fieldID] = values
		for i := range values {
			values[i] = randomRealisticUnicodeString(random())
			// values[i] = TestUtil.randomSimpleString(random());
		}
	}

	return fields
}

func randomParentTerm(values []string) *index.Term {
	return index.NewTerm("parent0", values[random().Intn(len(values))])
}

func randomChildTerm(values []string) *index.Term {
	return index.NewTerm("child0", values[random().Intn(len(values))])
}

func getRandomBlockJoinSort(prefix string, numFields int) *search.Sort {
	sortFields := make([]*search.SortField, 0)
	// TODO: sometimes sort by score; problem is scores are
	// not comparable across the two indices
	// sortFields.add(SortField.FIELD_SCORE);
	if random().Intn(2) == 0 {
		sortFields = append(sortFields, search.NewSortFieldWithReverse(
			prefix+strconv.Itoa(random().Intn(numFields)), spi.SortFieldTypeString, random().Intn(2) == 0))
	} else if random().Intn(2) == 0 {
		sortFields = append(sortFields, search.NewSortFieldWithReverse(
			prefix+strconv.Itoa(random().Intn(numFields)), spi.SortFieldTypeString, random().Intn(2) == 0))
		sortFields = append(sortFields, search.NewSortFieldWithReverse(
			prefix+strconv.Itoa(random().Intn(numFields)), spi.SortFieldTypeString, random().Intn(2) == 0))
	}
	// Break ties:
	sortFields = append(sortFields, search.NewSortField(prefix+"ID", spi.SortFieldTypeInt))
	return search.NewSort(sortFields...)
}

// randomTermQuery renders new TermQuery(new Term(prefix + fieldID,
// fields[fieldID][random().nextInt(fields[fieldID].length)])).
func randomTermQuery(prefix string, fields [][]string, fieldID int) search.Query {
	return search.NewTermQuery(index.NewTerm(prefix+strconv.Itoa(fieldID), fields[fieldID][random().Intn(len(fields[fieldID]))]))
}

// randomBooleanChildOrParentQuery renders the three-way random query builder
// testRandom uses for both the child query and the parent query 2.
func randomBooleanChildOrParentQuery(prefix string, fields [][]string, randomTerm func([]string) *index.Term) search.Query {
	if random().Intn(3) == 2 {
		return randomTermQuery(prefix, fields, random().Intn(len(fields)))
	} else if random().Intn(3) == 2 {
		bq := search.NewBooleanQueryBuilder()
		numClauses := nextInt(2, 4)
		didMust := false
		for clauseIDX := 0; clauseIDX < numClauses; clauseIDX++ {
			var clause search.Query
			var occur search.Occur
			if !didMust && random().Intn(2) == 0 {
				occur = search.MUST_NOT
				if random().Intn(2) == 0 {
					occur = search.MUST
				}
				clause = search.NewTermQuery(randomTerm(fields[0]))
				didMust = true
			} else {
				occur = search.SHOULD
				clause = randomTermQuery(prefix, fields, nextInt(1, len(fields)-1))
			}
			bq.Add(clause, occur)
		}
		return bq.Build()
	}
	bq := search.NewBooleanQueryBuilder()

	bq.Add(search.NewTermQuery(randomTerm(fields[0])), search.MUST)
	occur := search.MUST_NOT
	if random().Intn(2) == 0 {
		occur = search.MUST
	}
	bq.Add(randomTermQuery(prefix, fields, nextInt(1, len(fields)-1)), occur)
	return bq.Build()
}

// TODO: incredibly slow
func TestBlockJoinRandom(t *testing.T) {
	// We build two indices at once: one normalized (which
	// ToParentBlockJoinQuery/Collector,
	// ToChildBlockJoinQuery can query) and the other w/
	// the same docs, just fully denormalized:
	dir := newDirectory()
	joinDir := newDirectory()

	maxNumChildrenPerParent := 20
	numParentDocs := nextInt(10, 30) // RANDOM_MULTIPLIER = 1
	// final int numParentDocs = 30;

	// Values for parent fields:
	parentFields := getRandomFields(numParentDocs / 2)
	// Values for child fields:
	childFields := getRandomFields(numParentDocs)

	doDeletes := random().Intn(2) == 0
	toDelete := make([]int, 0)

	// TODO: parallel star join, nested join cases too!
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	joinIwc := newIndexWriterConfig()
	joinIwc.SetMergePolicy(newMergePolicyNoMock(t))
	joinW := newRandomIndexWriterWithConfig(t, joinDir, joinIwc)
	for parentDocID := 0; parentDocID < numParentDocs; parentDocID++ {
		parentDoc := document.NewDocument()
		parentJoinDoc := document.NewDocument()
		id := mustStoredIntField(t, "parentID", parentDocID)
		parentDoc.Add(id)
		parentJoinDoc.Add(id)
		parentJoinDoc.Add(newStringField(t, "isParent", "x", false))
		idDV := mustNumericDVField(t, "parentID", int64(parentDocID))
		parentDoc.Add(idDV)
		parentJoinDoc.Add(idDV)
		parentJoinDoc.Add(newStringField(t, "isParent", "x", false))
		for field := range parentFields {
			if random().Float64() < 0.9 {
				s := parentFields[field][random().Intn(len(parentFields[field]))]
				f := newStringField(t, "parent"+strconv.Itoa(field), s, false)
				parentDoc.Add(f)
				parentJoinDoc.Add(f)

				dv := mustSortedDVField(t, "parent"+strconv.Itoa(field), s)
				parentDoc.Add(dv)
				parentJoinDoc.Add(dv)
			}
		}

		if doDeletes {
			parentDoc.Add(document.NewIntPoint("blockID", int32(parentDocID)))
			parentJoinDoc.Add(document.NewIntPoint("blockID", int32(parentDocID)))
		}

		joinDocs := make([]*document.Document, 0)

		numChildDocs := nextInt(1, maxNumChildrenPerParent)
		for childDocID := 0; childDocID < numChildDocs; childDocID++ {
			// Denormalize: copy all parent fields into child doc:
			t.Fatal(testUtilCloneDocumentBlocker)
			var childDoc *document.Document
			joinChildDoc := document.NewDocument()
			joinDocs = append(joinDocs, joinChildDoc)

			childID := mustStoredIntField(t, "childID", childDocID)
			childDoc.Add(childID)
			joinChildDoc.Add(childID)
			childIDDV := mustNumericDVField(t, "childID", int64(childDocID))
			childDoc.Add(childIDDV)
			joinChildDoc.Add(childIDDV)

			for childFieldID := range childFields {
				if random().Float64() < 0.9 {
					s := childFields[childFieldID][random().Intn(len(childFields[childFieldID]))]
					f := newStringField(t, "child"+strconv.Itoa(childFieldID), s, false)
					childDoc.Add(f)
					joinChildDoc.Add(f)

					dv := mustSortedDVField(t, "child"+strconv.Itoa(childFieldID), s)
					childDoc.Add(dv)
					joinChildDoc.Add(dv)
				}
			}

			if doDeletes {
				joinChildDoc.Add(document.NewIntPoint("blockID", int32(parentDocID)))
			}

			mustAddDocument(t, w, childDoc)
		}

		// Parent last:
		joinDocs = append(joinDocs, parentJoinDoc)
		mustAddDocuments(t, joinW, joinDocs...)

		if doDeletes && random().Intn(30) == 7 {
			toDelete = append(toDelete, parentDocID)
		}
	}

	if len(toDelete) != 0 {
		// Query query = IntPoint.newSetQuery("blockID", toDelete);
		t.Fatal(intPointNewSetQueryBlocker)
	}

	r := mustGetReader(t, w)
	mustClose(t, w)
	joinR := mustGetReader(t, joinW)
	mustClose(t, joinW)

	s := newSearcherMaybeWrap(t, r, false)

	joinS := newSearcher(t, joinR)

	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("isParent", "x")))
	mustCheckJoinIndex(t, joinS.GetIndexReader(), parentsFilter)

	iters := 200 // * RANDOM_MULTIPLIER

	explanationPattern := regexp.MustCompile(`^Score based on ([0-9]+) child docs in range from ([0-9]+) to ([0-9]+), using score mode (None|Avg|Min|Max|Total)$`)

	for iter := 0; iter < iters; iter++ {
		if verbose {
			fmt.Printf("TEST: iter=%d of %d\n", 1+iter, iters)
		}

		childQuery := randomBooleanChildOrParentQuery("child", childFields, randomChildTerm)
		if random().Intn(2) == 0 {
			childQuery = testsearch.NewRandomApproximationQuery(childQuery, random())
		}

		agg := joinScoreModes[random().Intn(len(joinScoreModes))]
		childJoinQuery := NewToParentBlockJoinQuery(childQuery, parentsFilter, agg)

		// To run against the block-join index:
		var parentJoinQuery search.Query

		// Same query as parentJoinQuery, but to run against
		// the fully denormalized index (so we can compare
		// results):
		var parentQuery search.Query

		if random().Intn(2) == 0 {
			parentQuery = childQuery
			parentJoinQuery = childJoinQuery
		} else {
			// AND parent field w/ child field
			bq := search.NewBooleanQueryBuilder()
			parentTerm := randomParentTerm(parentFields[0])
			if random().Intn(2) == 0 {
				bq.Add(childJoinQuery, search.MUST)
				bq.Add(search.NewTermQuery(parentTerm), search.MUST)
			} else {
				bq.Add(search.NewTermQuery(parentTerm), search.MUST)
				bq.Add(childJoinQuery, search.MUST)
			}

			bq2 := search.NewBooleanQueryBuilder()
			if random().Intn(2) == 0 {
				bq2.Add(childQuery, search.MUST)
				bq2.Add(search.NewTermQuery(parentTerm), search.MUST)
			} else {
				bq2.Add(search.NewTermQuery(parentTerm), search.MUST)
				bq2.Add(childQuery, search.MUST)
			}
			parentJoinQuery = bq.Build()
			parentQuery = bq2.Build()
		}

		parentSort := getRandomBlockJoinSort("parent", len(parentFields))
		childSort := getRandomBlockJoinSort("child", len(childFields))

		// Merge both sorts:
		sortFields := append(append([]*search.SortField{}, parentSort.GetSort()...), childSort.GetSort()...)
		parentAndChildSort := search.NewSort(sortFields...)

		results := mustSearchSort(t, s, parentQuery, r.NumDocs(), parentAndChildSort)

		joinedResults := mustSearch(t, joinS, parentJoinQuery, numParentDocs)
		joinResults := map[int]*search.TopFieldDocs{}
		for _, parentHit := range joinedResults.ScoreDocs {
			childrenQuery := NewParentChildrenBlockJoinQuery(parentsFilter, childQuery, parentHit.Doc)
			childTopDocs := mustSearchSort(t, joinS, childrenQuery, maxNumChildrenPerParent, childSort)
			parentID, err := strconv.Atoi(storedGet(t, joinS, parentHit.Doc, "parentID"))
			if err != nil {
				t.Fatal(err)
			}
			joinResults[parentID] = childTopDocs
		}

		if results.TotalHits.Value == 0 {
			assertIntEquals(t, 0, len(joinResults))
		} else {
			compareHits(t, r, joinR, results, joinResults)
			b := mustSearch(t, joinS, childJoinQuery, 10)
			for _, hit := range b.ScoreDocs {
				explanation := mustExplain(t, joinS, childJoinQuery, hit.Doc)
				childID, err := strconv.Atoi(storedGet(t, joinS, hit.Doc-1, "childID"))
				if err != nil {
					t.Fatal(err)
				}
				if !explanation.IsMatch() {
					t.Fatal("explanation must match")
				}
				if hit.Score != explanation.GetValue() {
					t.Fatalf("explanation value: expected %v, got %v", hit.Score, explanation.GetValue())
				}
				m := explanationPattern.FindStringSubmatch(explanation.GetDescription())
				if m == nil {
					t.Fatalf("Block Join description not matches: %q", explanation.GetDescription())
				}
				matched, _ := strconv.Atoi(m[1])
				from, _ := strconv.Atoi(m[2])
				to, _ := strconv.Atoi(m[3])
				if !(matched > 0) {
					t.Fatal("Matched children not positive")
				}
				if hit.Doc-1-childID != from {
					t.Fatalf("Wrong child range start: expected %d, got %d", hit.Doc-1-childID, from)
				}
				if hit.Doc-1 != to {
					t.Fatalf("Wrong child range end: expected %d, got %d", hit.Doc-1, to)
				}
				childWeightExplanation := explanation.GetDetails()[0]
				if childWeightExplanation.GetDescription() == "sum of:" {
					childWeightExplanation = childWeightExplanation.GetDetails()[0]
				}
				if agg == None {
					if !startsWith(childWeightExplanation.GetDescription(), "ConstantScore(") {
						t.Fatalf("Wrong child weight description: %q", childWeightExplanation.GetDescription())
					}
				} else if !startsWith(childWeightExplanation.GetDescription(), "weight(child") {
					t.Fatalf("Wrong child weight description: %q", childWeightExplanation.GetDescription())
				}
			}
		}

		// Test joining in the opposite direction (parent to
		// child):

		// Get random query against parent documents:
		parentQuery2 := randomBooleanChildOrParentQuery("parent", parentFields, randomParentTerm)

		// Maps parent query to child docs:
		parentJoinQuery2 := NewToChildBlockJoinQuery(parentQuery2, parentsFilter)

		// To run against the block-join index:
		var childJoinQuery2 search.Query

		// Same query as parentJoinQuery, but to run against
		// the fully denormalized index (so we can compare
		// results):
		var childQuery2 search.Query

		if random().Intn(2) == 0 {
			childQuery2 = parentQuery2
			childJoinQuery2 = parentJoinQuery2
		} else {
			childTerm := randomChildTerm(childFields[0])
			if random().Intn(2) == 0 { // filtered case
				childJoinQuery2 = parentJoinQuery2
				childJoinQuery2 = search.NewBooleanQueryBuilder().
					Add(childJoinQuery2, search.MUST).
					Add(search.NewTermQuery(childTerm), search.FILTER).
					Build()
			} else {
				// AND child field w/ parent query:
				bq := search.NewBooleanQueryBuilder()
				if random().Intn(2) == 0 {
					bq.Add(parentJoinQuery2, search.MUST)
					bq.Add(search.NewTermQuery(childTerm), search.MUST)
				} else {
					bq.Add(search.NewTermQuery(childTerm), search.MUST)
					bq.Add(parentJoinQuery2, search.MUST)
				}
				childJoinQuery2 = bq.Build()
			}

			if random().Intn(2) == 0 { // filtered case
				childQuery2 = parentQuery2
				childQuery2 = search.NewBooleanQueryBuilder().
					Add(childQuery2, search.MUST).
					Add(search.NewTermQuery(childTerm), search.FILTER).
					Build()
			} else {
				bq2 := search.NewBooleanQueryBuilder()
				if random().Intn(2) == 0 {
					bq2.Add(parentQuery2, search.MUST)
					bq2.Add(search.NewTermQuery(childTerm), search.MUST)
				} else {
					bq2.Add(search.NewTermQuery(childTerm), search.MUST)
					bq2.Add(parentQuery2, search.MUST)
				}
				childQuery2 = bq2.Build()
			}
		}

		childSort2 := getRandomBlockJoinSort("child", len(childFields))

		// Search denormalized index:
		results2 := mustSearchSort(t, s, childQuery2, r.NumDocs(), childSort2)

		// Search join index:
		joinResults2 := mustSearchSort(t, joinS, childJoinQuery2, joinR.NumDocs(), childSort2)

		compareChildHits(t, r, joinR, results2, joinResults2)
	}

	mustClose(t, r, joinR, dir, joinDir)
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func mustSearchSort(t testing.TB, s *search.IndexSearcher, q search.Query, n int, sort *search.Sort) *search.TopFieldDocs {
	t.Helper()
	td, err := s.SearchWithSortNoScores(q, n, sort)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return td
}

func compareChildHits(t testing.TB, r, joinR index.IndexReaderInterface, results, joinResults *search.TopFieldDocs) {
	t.Helper()
	assertInt64Equals(t, results.TotalHits.Value, joinResults.TotalHits.Value)
	assertIntEquals(t, len(results.ScoreDocs), len(joinResults.ScoreDocs))
	for hitCount := range results.ScoreDocs {
		hit := results.ScoreDocs[hitCount]
		joinHit := joinResults.ScoreDocs[hitCount]
		if a, b := storedGet(t, r, hit.Doc, "childID"), storedGet(t, joinR, joinHit.Doc, "childID"); a != b {
			t.Fatalf("hit %d differs: %q != %q", hitCount, a, b)
		}
		// don't compare scores -- they are expected to differ

		hit0 := results.FieldDocs[hitCount]
		joinHit0 := joinResults.FieldDocs[hitCount]
		if len(hit0.Fields) != len(joinHit0.Fields) {
			t.Fatalf("hit %d sort values differ: %v != %v", hitCount, hit0.Fields, joinHit0.Fields)
		}
		for i := range hit0.Fields {
			if fmt.Sprint(hit0.Fields[i]) != fmt.Sprint(joinHit0.Fields[i]) {
				t.Fatalf("hit %d sort values differ: %v != %v", hitCount, hit0.Fields, joinHit0.Fields)
			}
		}
	}
}

func compareHits(t testing.TB, r, joinR index.IndexReaderInterface, controlHits *search.TopFieldDocs, joinResults map[int]*search.TopFieldDocs) {
	t.Helper()
	currentParentID := -1
	childHitSlot := 0
	childHits := search.NewTopDocs(search.NewTotalHits(0, search.EQUAL_TO), []*search.ScoreDoc{})
	for _, controlHit := range controlHits.ScoreDocs {
		parentID, err := strconv.Atoi(storedGet(t, r, controlHit.Doc, "parentID"))
		if err != nil {
			t.Fatal(err)
		}
		if parentID != currentParentID {
			assertIntEquals(t, childHitSlot, len(childHits.ScoreDocs))
			currentParentID = parentID
			childHitSlot = 0
			childHits = joinResults[parentID].TopDocs
		}

		controlChildID := storedGet(t, r, controlHit.Doc, "childID")
		childID := storedGet(t, joinR, childHits.ScoreDocs[childHitSlot].Doc, "childID")
		childHitSlot++
		assertStringEquals(t, controlChildID, childID)
	}
}
