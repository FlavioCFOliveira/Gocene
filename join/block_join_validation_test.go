// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestBlockJoinValidation.
//
// The validation tests assert that the block-join scorers reject mis-configured
// queries (a child query that matches a parent, or a parent query that matches a
// child). The Lucene reference uses WildcardQuery(parent,"*") as the parents
// filter; Gocene's WildcardQuery is not yet runnable (its ConstantScoreQuery
// weight is a stub, rmp #4760), so the corpus here marks parents with a
// docType=parent field and the parents filter is TermQuery(docType,parent) — an
// equivalent parent selector that preserves exactly what these tests verify.
package join

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

const (
	bjvSegments   = 5
	bjvParentDocs = 10
	bjvChildDocs  = 5
)

// bjvFieldValue mirrors TestBlockJoinValidation.createFieldValue: underscore-
// joined document numbers.
func bjvFieldValue(nums ...int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = itoa(n)
	}
	return strings.Join(parts, "_")
}

// buildValidationIndex builds the AMOUNT_OF_SEGMENTS x AMOUNT_OF_PARENT_DOCS x
// AMOUNT_OF_CHILD_DOCS block corpus used by the validation tests, with a
// docType=parent marker on parents (see the package note).
func buildValidationIndex(t *testing.T) (*index.DirectoryReader, *search.IndexSearcher, BitSetProducer) {
	t.Helper()
	dir, w := newBlockWriter(t)
	for seg := 0; seg < bjvSegments; seg++ {
		for p := 0; p < bjvParentDocs; p++ {
			docs := make([]index.Document, 0, bjvChildDocs+1)
			for c := 0; c < bjvChildDocs; c++ {
				docs = append(docs, newDoc(t, map[string]string{
					"id":           bjvFieldValue(seg*bjvParentDocs+p, c),
					"child":        bjvFieldValue(c),
					"common_field": "1",
				}))
			}
			docs = append(docs, newDoc(t, map[string]string{
				"id":           bjvFieldValue(seg*bjvParentDocs + p),
				"parent":       bjvFieldValue(p),
				"docType":      "parent",
				"common_field": "1",
			}))
			addBlock(t, w, docs...)
		}
		// One commit per segment (matches AMOUNT_OF_SEGMENTS), though Gocene
		// flushes a single segment per commit only loosely; the block contiguity
		// that block joins require is preserved by AddDocuments regardless.
	}
	r, s := commitAndOpen(t, dir, w)
	return r, s, newQueryBitSetParents("docType", "parent")
}

// TestBlockJoinValidation_NextDocValidationForToParentBjq corresponds to
// TestBlockJoinValidation.testNextDocValidationForToParentBjq.
func TestBlockJoinValidation_NextDocValidationForToParentBjq(t *testing.T) {
	t.Skip("the child-matches-parent invariant is raised in Lucene from scoreChildDocs (during scoring); Gocene's Scorer.Score has no error channel: rmp #4765")
}

// TestBlockJoinValidation_NextDocValidationForToChildBjq corresponds to
// TestBlockJoinValidation.testNextDocValidationForToChildBjq: a parent query
// that also matches a child doc must make the ToChild scorer report the
// "parent query must not match child docs" invariant error.
func TestBlockJoinValidation_NextDocValidationForToChildBjq(t *testing.T) {
	r, s, parentsFilter := buildValidationIndex(t)

	// Parent query (parent=value 0) OR a child doc id -> the parent query
	// matches a child, violating the ToChild invariant.
	parentQuery := search.NewBooleanQuery()
	parentQuery.Add(search.NewTermQuery(index.NewTerm("parent", bjvFieldValue(0))), search.SHOULD)
	parentQuery.Add(search.NewTermQuery(index.NewTerm("id", bjvFieldValue(0, 0))), search.SHOULD)

	blockJoinQuery := NewToChildBlockJoinQuery(parentQuery, parentsFilter, None)
	_, err := s.Search(blockJoinQuery, 1)
	if err == nil {
		t.Fatal("expected an invariant error, got nil")
	}
	if !strings.Contains(err.Error(), "Parent query must not match") {
		t.Errorf("error = %q, want it to contain the invalid-query message", err.Error())
	}
	_ = r
}

// TestBlockJoinValidation_AdvanceValidationForToChildBjq corresponds to
// TestBlockJoinValidation.testAdvanceValidationForToChildBjq: advancing the
// ToChild scorer onto a target whose next doc is not a parent must raise the
// invariant error.
func TestBlockJoinValidation_AdvanceValidationForToChildBjq(t *testing.T) {
	r, s, parentsFilter := buildValidationIndex(t)

	// MatchAllDocsQuery as the parent query: it matches children too, so once
	// the scorer is advanced such that the "parent" it lands on is actually a
	// child, validateParentDoc must fire.
	blockJoinQuery := NewToChildBlockJoinQuery(search.NewMatchAllDocsQuery(), parentsFilter, None)

	leaves, err := r.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	ctx := leaves[0]
	rewritten, err := blockJoinQuery.Rewrite(r)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	weight, err := rewritten.CreateWeight(s, true, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	scorer, err := weight.Scorer(ctx)
	if err != nil {
		t.Fatalf("Scorer: %v", err)
	}
	if scorer == nil {
		t.Fatal("expected non-nil scorer")
	}

	parentDocs, err := parentsFilter.GetBitSet(ctx)
	if err != nil {
		t.Fatalf("GetBitSet: %v", err)
	}

	// Find a target whose successor (target+1) is NOT a parent, so advancing the
	// parent iterator to target+1 lands it on a child -> invariant violation.
	maxDoc := ctx.LeafReader().MaxDoc()
	target := -1
	for cand := 0; cand <= maxDoc-2; cand++ {
		if !parentDocs.Get(cand + 1) {
			target = cand
			break
		}
	}
	if target < 0 {
		// The corpus interleaves 5 children before each parent, so a doc whose
		// successor is a child always exists; reaching here means the fixture
		// was changed incorrectly.
		t.Fatal("no suitable non-parent target in this corpus layout")
	}

	if _, err := scorer.Advance(target); err == nil {
		t.Fatalf("Advance(%d) expected an invariant error, got nil", target)
	} else if !strings.Contains(err.Error(), "Parent query must not match") {
		t.Errorf("error = %q, want it to contain the invalid-query message", err.Error())
	}
}

// TestBlockJoinValidation_QueryDescriptors verifies that ToParentBlockJoinQuery
// and ToChildBlockJoinQuery can be constructed and their accessors work,
// mirroring the structural intent of the validation test setup.
func TestBlockJoinValidation_QueryDescriptors(t *testing.T) {
	for _, sm := range []ScoreMode{Avg, Max, Total, Min} {
		tpq := NewToParentBlockJoinQuery(nil, nil, sm)
		if tpq == nil {
			t.Fatalf("expected non-nil ToParentBlockJoinQuery(scoreMode=%v)", sm)
		}
		if tpq.GetScoreMode() != sm {
			t.Errorf("GetScoreMode() = %v, want %v", tpq.GetScoreMode(), sm)
		}
	}

	tcq := NewToChildBlockJoinQuery(nil, nil, Avg)
	if tcq == nil {
		t.Fatal("expected non-nil ToChildBlockJoinQuery")
	}
	if tcq.GetScoreMode() != Avg {
		t.Errorf("GetScoreMode() = %v, want Avg", tcq.GetScoreMode())
	}
}
