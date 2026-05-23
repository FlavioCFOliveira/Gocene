// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktreeords_test

import "testing"

// The tests below are ports of
// org.apache.lucene.codecs.blocktreeords.TestOrdsBlockTree (Lucene 10.4.0).
//
// All tests are currently skipped because the BlockTreeOrds postings format
// lacks a write path (BlockTreeOrdsPostingsFormat / OrdsBlockTreeTermsWriter)
// and OrdsSegmentTermsEnum.Next / SeekExact / SeekCeil / Ord are still stubs.
// Once those components land the skip guards should be removed and the
// assertion logic filled in.

// skipReason is the uniform skip message for all tests in this file.
const skipReason = "BlockTreeOrds write path and OrdsSegmentTermsEnum traversal " +
	"are not yet implemented; test body deferred until those components land"

// TestOrdsBlockTree_Basic is a port of TestOrdsBlockTree.testBasic.
//
// Java intent: index a single document with three tokens ("a b c"), iterate
// the TermsEnum, verify term order and ordinals 0/1/2, then seek by term and
// by ordinal and verify round-trip.
func TestOrdsBlockTree_Basic(t *testing.T) {
	t.Skip(skipReason)
}

// TestOrdsBlockTree_TwoBlocks is a port of TestOrdsBlockTree.testTwoBlocks.
//
// Java intent: index 72 single-character and two-character terms across two
// FST blocks, force-merge to one segment, then verify that seekExact by ord
// and seekExact by term agree on every term in random order.
func TestOrdsBlockTree_TwoBlocks(t *testing.T) {
	t.Skip(skipReason)
}

// TestOrdsBlockTree_ThreeBlocks is a port of TestOrdsBlockTree.testThreeBlocks.
//
// Java intent: three levels of FST blocks (single-char, "m"+single-char,
// "mo"+single-char), force-merge, then verify ordinal/term agreement for all
// 108 terms in both forward and random-seek order.
func TestOrdsBlockTree_ThreeBlocks(t *testing.T) {
	t.Skip(skipReason)
}

// TestOrdsBlockTree_FloorBlocks is a port of TestOrdsBlockTree.testFloorBlocks.
//
// Java intent: index 128 documents with single-byte string terms (bytes 0–127)
// to exercise floor blocks, then verify seekExact by term and by ordinal for
// specific entries ("a" → ord 97, "b" → ord 98, "z" → ord 122).
func TestOrdsBlockTree_FloorBlocks(t *testing.T) {
	t.Skip(skipReason)
}

// TestOrdsBlockTree_NonRootFloorBlocks is a port of
// TestOrdsBlockTree.testNonRootFloorBlocks.
//
// Java intent: 36 single-char terms plus 128 "m"+byte terms to create
// floor blocks at a non-root node; verify sequential iteration ordinals
// match, then verify random seeks by ord and by term.
func TestOrdsBlockTree_NonRootFloorBlocks(t *testing.T) {
	t.Skip(skipReason)
}

// TestOrdsBlockTree_SeveralNonRootBlocks is a port of
// TestOrdsBlockTree.testSeveralNonRootBlocks.
//
// Java intent: 30×30 = 900 two-character terms forming a grid of sub-blocks;
// verify sequential iteration yields ordinals 0..899, then random seek by
// ord and by term across all entries.
func TestOrdsBlockTree_SeveralNonRootBlocks(t *testing.T) {
	t.Skip(skipReason)
}

// TestOrdsBlockTree_SeekCeilNotFound is a port of
// TestOrdsBlockTree.testSeekCeilNotFound.
//
// Java intent: index an empty-string term plus 36 single-char and
// "a"+single-char terms, then seekCeil to a byte value that sorts between
// "" and "a" (0x22) and verify the result is SeekStatus.NOT_FOUND with the
// landed term "a" at ordinal 1.
func TestOrdsBlockTree_SeekCeilNotFound(t *testing.T) {
	t.Skip(skipReason)
}
