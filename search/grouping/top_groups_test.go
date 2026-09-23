// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// Port of
// lucene/grouping/src/test/org/apache/lucene/search/grouping/TestTopGroups.java
// (Apache Lucene 10.5.0).

func TestTopGroupsAllGroupsEmptyInSecondPass(t *testing.T) {
	narrativeMergeTestImplementation(t, false, false, false, false)
}

func TestTopGroupsSomeGroupsEmptyInSecondPass(t *testing.T) {
	narrativeMergeTestImplementation(t, false, false, false, true)
	narrativeMergeTestImplementation(t, false, false, true, false)
	narrativeMergeTestImplementation(t, false, false, true, true)

	narrativeMergeTestImplementation(t, false, true, false, false)
	narrativeMergeTestImplementation(t, false, true, false, true)
	narrativeMergeTestImplementation(t, false, true, true, false)
	narrativeMergeTestImplementation(t, false, true, true, true)

	narrativeMergeTestImplementation(t, true, false, false, false)
	narrativeMergeTestImplementation(t, true, false, false, true)
	narrativeMergeTestImplementation(t, true, false, true, false)
	narrativeMergeTestImplementation(t, true, false, true, true)

	narrativeMergeTestImplementation(t, true, true, false, false)
	narrativeMergeTestImplementation(t, true, true, false, true)
	narrativeMergeTestImplementation(t, true, true, true, false)
}

func TestTopGroupsNoGroupsEmptyInSecondPass(t *testing.T) {
	narrativeMergeTestImplementation(t, true, true, true, true)
}

// narrativeMergeTestImplementation implements tests for the TopGroup.merge
// method using a narrative approach. Use of a creative narrative may seem
// unusual or even silly but the idea behind it is to make it hopefully easier
// to reason about the documents and groups and scores in the test whilst
// testing several scenario permutations.
//
// Imagine:
//
// Each document represents (say) a picture book of an animal. We are
// searching for two books and wish to draw a picture of our own, inspired by
// the books. We think that large animals are easier to draw and therefore
// order the books by the featured animal's size. We think that different
// colors would make for a good drawing and therefore group the books by the
// featured animal's color.
//
// Index content:
//
// The documents are in 2 groups ("blue" and "red") and there are 4 documents
// across 2 shards: shard 1 (blue whale, red ant) and shard 2 (blue dragonfly,
// red squirrel).
//
// If all documents are present the "blue whale" and the "red squirrel"
// documents would be returned for our drawing since they are the largest
// animals in their respective groups.
//
// Test permutations (haveBlueWhale, haveRedAnt, haveBlueDragonfly,
// haveRedSquirrel) arise because in the first pass of the search all
// documents can be present, but in the second pass of the search some
// documents could be missing if they have been deleted 'just so' between the
// two phases.
//
// Additionally a haveAnimal == false condition also represents scenarios
// where a given group has documents on some but not all shards in the
// collection.
func narrativeMergeTestImplementation(t *testing.T, haveBlueWhale, haveRedAnt, haveBlueDragonfly, haveRedSquirrel bool) {
	t.Helper()
	const blueGroupValue = "blue"
	const redGroupValue = "red"

	// Java boxes the sizes as Integer.
	redAntSize := int32(1)
	blueDragonflySize := int32(10)
	redSquirrelSize := int32(100)
	blueWhaleSize := int32(1000)

	redAntScore := float32(redAntSize)
	blueDragonflyScore := float32(blueDragonflySize)
	redSquirrelScore := float32(redSquirrelSize)
	blueWhaleScore := float32(blueWhaleSize)

	sort := search.RELEVANCE
	nan := float32(math.NaN())

	var shard1TopGroups *TopGroups[string]
	{
		var group1, group2 *GroupDocs[string]
		if haveBlueWhale {
			group1 = createSingletonGroupDocs(blueGroupValue, []any{blueWhaleSize}, 1 /* docId */, blueWhaleScore, 0 /* shardIndex */)
		} else {
			group1 = createEmptyGroupDocs(blueGroupValue, []any{blueWhaleSize})
		}
		if haveRedAnt {
			group2 = createSingletonGroupDocs(redGroupValue, []any{redAntSize}, 2 /* docId */, redAntScore, 0 /* shardIndex */)
		} else {
			group2 = createEmptyGroupDocs(redGroupValue, []any{redAntSize})
		}

		maxScore := nan
		if haveBlueWhale {
			maxScore = blueWhaleScore
		} else if haveRedAnt {
			maxScore = redAntScore
		}
		shard1TopGroups = NewTopGroups(
			sort.GetSort(), /* groupSort */
			sort.GetSort(), /* withinGroupSort */
			len(group1.ScoreDocs)+len(group2.ScoreDocs), /* totalHitCount */
			len(group1.ScoreDocs)+len(group2.ScoreDocs), /* totalGroupedHitCount */
			combineGroupDocs(group1, group2),            /* groups */
			maxScore /* maxScore */)
	}

	var shard2TopGroups *TopGroups[string]
	{
		var group1, group2 *GroupDocs[string]
		if haveBlueDragonfly {
			group1 = createSingletonGroupDocs(blueGroupValue, []any{blueDragonflySize}, 3 /* docId */, blueDragonflyScore, 1 /* shardIndex */)
		} else {
			group1 = createEmptyGroupDocs(blueGroupValue, []any{blueDragonflySize})
		}
		if haveRedSquirrel {
			group2 = createSingletonGroupDocs(redGroupValue, []any{redSquirrelSize}, 4 /* docId */, redSquirrelScore, 1 /* shardIndex */)
		} else {
			group2 = createEmptyGroupDocs(redGroupValue, []any{redSquirrelSize})
		}

		maxScore := nan
		if haveRedSquirrel {
			maxScore = redSquirrelScore
		} else if haveBlueDragonfly {
			maxScore = blueDragonflyScore
		}
		shard2TopGroups = NewTopGroups(
			sort.GetSort(), /* groupSort */
			sort.GetSort(), /* withinGroupSort */
			len(group1.ScoreDocs)+len(group2.ScoreDocs), /* totalHitCount */
			len(group1.ScoreDocs)+len(group2.ScoreDocs), /* totalGroupedHitCount */
			combineGroupDocs(group1, group2),            /* groups */
			maxScore /* maxScore */)
	}

	mergedTopGroups, err := MergeTopGroups(
		combineTopGroups(shard1TopGroups, shard2TopGroups),
		sort, /* groupSort */
		sort, /* docSort */
		0,    /* docOffset */
		2,    /* docTopN */
		ScoreMergeModeNone)
	if err != nil {
		t.Fatalf("TopGroups.merge: %v", err)
	}
	if mergedTopGroups == nil {
		t.Fatal("mergedTopGroups is null")
	}

	expectedCount := 0
	for _, have := range []bool{haveBlueWhale, haveRedAnt, haveBlueDragonfly, haveRedSquirrel} {
		if have {
			expectedCount++
		}
	}

	if mergedTopGroups.TotalHitCount != expectedCount {
		t.Fatalf("totalHitCount: expected %d, got %d", expectedCount, mergedTopGroups.TotalHitCount)
	}
	if mergedTopGroups.TotalGroupedHitCount != expectedCount {
		t.Fatalf("totalGroupedHitCount: expected %d, got %d", expectedCount, mergedTopGroups.TotalGroupedHitCount)
	}

	if len(mergedTopGroups.Groups) != 2 {
		t.Fatalf("groups: expected 2, got %d", len(mergedTopGroups.Groups))
	}
	{
		if mergedTopGroups.Groups[0].GroupValue != blueGroupValue {
			t.Fatalf("groups[0].groupValue: expected %q, got %q", blueGroupValue, mergedTopGroups.Groups[0].GroupValue)
		}
		expectedBlueMaxScore := nan
		if haveBlueWhale {
			expectedBlueMaxScore = blueWhaleScore
		} else if haveBlueDragonfly {
			expectedBlueMaxScore = blueDragonflyScore
		}
		checkMaxScore(t, expectedBlueMaxScore, mergedTopGroups.Groups[0].MaxScore)
	}
	{
		if mergedTopGroups.Groups[1].GroupValue != redGroupValue {
			t.Fatalf("groups[1].groupValue: expected %q, got %q", redGroupValue, mergedTopGroups.Groups[1].GroupValue)
		}
		expectedRedMaxScore := nan
		if haveRedSquirrel {
			expectedRedMaxScore = redSquirrelScore
		} else if haveRedAnt {
			expectedRedMaxScore = redAntScore
		}
		checkMaxScore(t, expectedRedMaxScore, mergedTopGroups.Groups[1].MaxScore)
	}

	expectedMaxScore := nan
	switch {
	case haveBlueWhale:
		expectedMaxScore = blueWhaleScore
	case haveRedSquirrel:
		expectedMaxScore = redSquirrelScore
	case haveBlueDragonfly:
		expectedMaxScore = blueDragonflyScore
	case haveRedAnt:
		expectedMaxScore = redAntScore
	}
	checkMaxScore(t, expectedMaxScore, mergedTopGroups.MaxScore)
}

func checkMaxScore(t *testing.T, expected, actual float32) {
	t.Helper()
	if expected != expected {
		if actual == actual {
			t.Fatalf("maxScore: expected NaN, got %v", actual)
		}
	} else if expected != actual {
		t.Fatalf("maxScore: expected %v, got %v", expected, actual)
	}
}

// helper methods

func createEmptyGroupDocs(groupValue string, groupSortValues []any) *GroupDocs[string] {
	return NewGroupDocs(
		float32(math.NaN()), /* score */
		float32(math.NaN()), /* maxScore */
		search.NewTotalHits(0, search.EQUAL_TO),
		[]*search.ScoreDoc{},
		groupValue,
		groupSortValues)
}

func createSingletonGroupDocs(groupValue string, groupSortValues []any, docID int, docScore float32, shardIndex int) *GroupDocs[string] {
	return NewGroupDocs(
		float32(math.NaN()), /* score */
		docScore,            /* maxScore */
		search.NewTotalHits(1, search.EQUAL_TO),
		[]*search.ScoreDoc{search.NewScoreDoc(docID, docScore, shardIndex)},
		groupValue,
		groupSortValues)
}

func combineGroupDocs(group0, group1 *GroupDocs[string]) []*GroupDocs[string] {
	return []*GroupDocs[string]{group0, group1}
}

func combineTopGroups(group0, group1 *TopGroups[string]) []*TopGroups[string] {
	return []*TopGroups[string]{group0, group1}
}
