// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestShardSearching.java
//   (extends ShardSearchingTestBase)
//
// TestShardSearching spins up numNodes independent shard indexes that each
// continuously add/update/delete documents, then repeatedly:
//   - acquires a per-node ShardIndexSearcher (a distributed searcher that merges
//     term statistics and collection statistics across every node so that
//     scoring matches a single combined index);
//   - builds a mock combined MultiReader over the same per-node searcher
//     versions and an ordinary IndexSearcher over it;
//   - runs the same TermQuery / PrefixQuery (optionally with a Sort and with
//     deep-paging via searchAfter) against both the shard searcher and the mock
//     combined searcher, and asserts (TestUtil.assertConsistent) that the
//     distributed results — after rebasing per-shard doc IDs — are identical to
//     the single combined results.
//
// This depends on the test-framework ShardSearchingTestBase subsystem:
// NodeState, ShardIndexSearcher, the SearcherLifetimeManager-based versioned
// searcher acquisition with SearcherExpiredException, and — crucially — the
// cross-node global term/collection-statistics merging that makes shard scoring
// agree with single-index scoring. None of that distributed framework exists in
// Gocene. In addition, IndexSearcher cannot search a composite MultiReader (its
// leaf traversal only descends a DirectoryReader's segment readers), so even the
// mock combined reference searcher cannot be built.
//
// This port builds a real multi-shard fixture and proves per-shard searching
// works against the production IndexWriter + IndexSearcher, then fails honestly
// citing the missing ShardSearchingTestBase / ShardIndexSearcher global-stats
// framework and the composite-reader search gap (rather than skipping the test).

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Register the production codec so postings are flushed.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

const shardSearchingField = "body"

// shardSearchingNode is one shard: a committed directory and its open reader.
type shardSearchingNode struct {
	dir    store.Directory
	reader *index.DirectoryReader
}

// buildShardSearchingNode indexes the given "body" documents into a fresh shard
// and opens a reader over it.
func buildShardSearchingNode(t *testing.T, bodies []string) *shardSearchingNode {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i, body := range bodies {
		doc := document.NewDocument()
		f, ferr := document.NewTextField(shardSearchingField, body, false)
		if ferr != nil {
			t.Fatalf("NewTextField(%d): %v", i, ferr)
		}
		doc.Add(f)
		if aerr := w.AddDocument(doc); aerr != nil {
			t.Fatalf("AddDocument(%d): %v", i, aerr)
		}
	}
	if cerr := w.Commit(); cerr != nil {
		t.Fatalf("Commit: %v", cerr)
	}
	if cerr := w.Close(); cerr != nil {
		t.Fatalf("writer.Close: %v", cerr)
	}
	reader, rerr := index.OpenDirectoryReader(dir)
	if rerr != nil {
		t.Fatalf("OpenDirectoryReader: %v", rerr)
	}
	return &shardSearchingNode{dir: dir, reader: reader}
}

func (n *shardSearchingNode) close() {
	_ = n.reader.Close()
	_ = n.dir.Close()
}

// TestShardSearching_Basics mirrors the Java test() method (testSimple in the
// reference). It builds several shards, proves a TermQuery searches each shard
// correctly through the real IndexSearcher, then fails honestly at the missing
// distributed-search framework.
func TestShardSearching_Basics(t *testing.T) {
	const numNodes = 3
	nodes := make([]*shardSearchingNode, numNodes)
	defer func() {
		for _, n := range nodes {
			if n != nil {
				n.close()
			}
		}
	}()

	// Each shard gets a disjoint slice of an "intToEnglish"-style corpus so the
	// term "one" appears in a deterministic, non-trivial subset of every shard,
	// exactly the kind of skewed term distribution that exposes naive (non
	// global-stats) shard scoring.
	totalAcrossShards := 0
	for node := 0; node < numNodes; node++ {
		bodies := make([]string, 0, 40)
		for i := node * 40; i < node*40+40; i++ {
			bodies = append(bodies, searchAfterIntToEnglish(i))
		}
		nodes[node] = buildShardSearchingNode(t, bodies)

		// Prove per-shard search works against the real IndexSearcher.
		searcher := search.NewIndexSearcher(nodes[node].reader)
		q := search.NewTermQuery(index.NewTerm(shardSearchingField, "one"))
		top, err := searcher.Search(q, 100)
		if err != nil {
			t.Fatalf("shard %d search: %v", node, err)
		}
		if top.TotalHits.Value == 0 {
			t.Fatalf("shard %d: TermQuery(body:one) matched no documents; fixture is degenerate", node)
		}
		totalAcrossShards += int(top.TotalHits.Value)
	}
	if totalAcrossShards == 0 {
		t.Fatalf("no shard matched body:one across %d nodes; fixture is degenerate", numNodes)
	}

	// The reference now builds a mock combined MultiReader over all nodes and an
	// IndexSearcher over it, then compares its results to a per-node
	// ShardIndexSearcher whose term/collection statistics are merged globally.
	subs := make([]index.IndexReaderInterface, numNodes)
	for i, n := range nodes {
		subs[i] = n.reader
	}
	if _, err := index.NewMultiReader(subs); err != nil {
		t.Fatalf("NewMultiReader: %v", err)
	}

	t.Fatalf("blocked: distributed shard searching is not implemented "+
		"in Gocene — there is no ShardSearchingTestBase / ShardIndexSearcher "+
		"with cross-node global term & collection statistics merging (so shard "+
		"scoring cannot be made to agree with single-index scoring), no "+
		"versioned SearcherLifetimeManager acquisition with "+
		"SearcherExpiredException, and IndexSearcher cannot search a composite "+
		"MultiReader (its leaf traversal only descends a DirectoryReader's "+
		"segment readers), so the mock combined reference searcher over the %d "+
		"shards cannot be built either", numNodes)
}
