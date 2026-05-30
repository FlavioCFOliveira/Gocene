// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"
)

// max_position_test.go ports org.apache.lucene.index.TestMaxPosition
// (LUCENE-6382).
//
// The upstream suite exercises the IndexWriter.MAX_POSITION bound: a token
// stream whose accumulated positions overflow MAX_POSITION must be rejected
// with IllegalArgumentException (testTooBigPosition), while a stream that
// lands exactly on MAX_POSITION must be accepted and read back correctly
// (testMaxPosition).
//
// Both test methods are blocked on Sprint 55 infrastructure gaps:
//
//   - CannedTokenStream is unimplemented, so the two tokens with explicit
//     setPositionIncrement (one of them == IndexWriter.MAX_POSITION) cannot
//     be produced.
//   - DirectoryReader.open(IndexWriter) (the near-real-time open from a
//     writer) has no Gocene equivalent; only OpenDirectoryReader(Directory)
//     exists, and it builds each SegmentReader without core readers, so the
//     leaf-level Terms()/Postings() path used by testMaxPosition returns
//     "core readers are nil".
//   - MultiTerms.getTermPostingsEnum has no Gocene helper.
//
// Each test below documents the verbatim upstream scenario and t.Skip's at
// the first unreachable step, following the established pattern in
// postings_offsets_test.go, payloads_on_vectors_test.go and flex_test.go.
// Unskip once CannedTokenStream, the near-real-time DirectoryReader open and
// the MultiTerms.getTermPostingsEnum read path land.
//
// The Gocene equivalent of IndexWriter.MAX_POSITION is the exported constant
// index.MaxPosition (see index/mapping_multi_postings_enum.go).

// TestMaxPosition_TooBigPosition ports TestMaxPosition.testTooBigPosition.
//
// Upstream indexes one document whose "foo" TextField is a CannedTokenStream
// of two "foo" tokens: t1 at position 1 (positionIncrement 2) and t2 with
// positionIncrement == MAX_POSITION, which overflows the maximum. addDocument
// must throw IllegalArgumentException, and a reader opened on the writer must
// then report numDocs() == 0 (the document is not visible).
func TestMaxPosition_TooBigPosition(t *testing.T) {
	t.Fatal("blocked: CannedTokenStream (explicit positionIncrement tokens) unimplemented, " +
		"and DirectoryReader.open(IndexWriter) for the numDocs()==0 visibility check has no Gocene equivalent")
}

// TestMaxPosition_MaxPosition ports TestMaxPosition.testMaxPosition.
//
// Upstream indexes one document whose "foo" TextField is a CannedTokenStream
// of two "foo" tokens: t1 at position 0 and t2 with positionIncrement ==
// MAX_POSITION, landing exactly on the maximum. addDocument must succeed; a
// reader opened on the writer must report numDocs() == 1, and the PostingsEnum
// for term "foo" must report freq()==2 with positions 0 and MAX_POSITION.
func TestMaxPosition_MaxPosition(t *testing.T) {
	t.Fatal("blocked: CannedTokenStream (explicit positionIncrement tokens) unimplemented, " +
		"and the MultiTerms.getTermPostingsEnum read-back path hits 'core readers are nil' on OpenDirectoryReader")
}
