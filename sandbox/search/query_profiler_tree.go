// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.QueryProfilerTree.
package search

import (
	"fmt"
	"time"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// QueryProfilerTree tracks the dependency tree for queries (scoring and
// rewriting) and generates a QueryProfilerBreakdown for each node.
//
// Mirrors org.apache.lucene.sandbox.search.QueryProfilerTree (package-private
// in Java).
type QueryProfilerTree struct {
	// breakdowns holds the per-node breakdown, indexed by token.
	breakdowns []*QueryProfilerBreakdown

	// tree holds, for each token, the list of child tokens.
	tree [][]int

	// queries holds the original query per token.
	queries []search.Query

	// roots holds the token IDs of top-level (root) query nodes.
	roots []int

	// stack is the current depth-first traversal stack.
	stack []int

	currentToken int

	// rewriteTime accumulates total rewrite time in nanoseconds.
	rewriteTime int64

	// rewriteScratch is the start time of the current rewrite window.
	rewriteScratch int64
}

// NewQueryProfilerTree constructs a QueryProfilerTree with default capacity.
func NewQueryProfilerTree() *QueryProfilerTree {
	return &QueryProfilerTree{
		breakdowns: make([]*QueryProfilerBreakdown, 0, 10),
		tree:       make([][]int, 0, 10),
		queries:    make([]search.Query, 0, 10),
		roots:      make([]int, 0, 10),
		stack:      make([]int, 0, 10),
	}
}

// GetProfileBreakdown returns a QueryProfilerBreakdown for a scoring query.
// Scoring queries follow a recursive progression tracked via the stack.
//
// The query parameter is any search.Query; its string form is stored for
// later result assembly.
func (t *QueryProfilerTree) GetProfileBreakdown(query search.Query) *QueryProfilerBreakdown {
	token := t.currentToken

	stackEmpty := len(t.stack) == 0

	if stackEmpty {
		// New root node.
		t.roots = append(t.roots, token)
		t.currentToken++
		t.stack = append(t.stack, token)
		return t.addDependencyNode(query, token)
	}

	t.updateParent(token)
	t.currentToken++
	t.stack = append(t.stack, token)
	return t.addDependencyNode(query, token)
}

// PollLast removes the most recent token from the stack.
func (t *QueryProfilerTree) PollLast() {
	if len(t.stack) == 0 {
		return
	}
	t.stack = t.stack[:len(t.stack)-1]
}

// GetTree returns the hierarchical profiled results for all root nodes.
func (t *QueryProfilerTree) GetTree() []QueryProfilerResult {
	results := make([]QueryProfilerResult, 0, len(t.roots))
	for _, root := range t.roots {
		results = append(results, t.doGetTree(root))
	}
	return results
}

// StartRewriteTime begins timing a rewrite phase.
func (t *QueryProfilerTree) StartRewriteTime() {
	t.rewriteScratch = time.Now().UnixNano()
}

// StopAndAddRewriteTime stops the current rewrite timer and accumulates the
// elapsed nanoseconds. Returns the elapsed time (minimum 1 ns).
func (t *QueryProfilerTree) StopAndAddRewriteTime() int64 {
	elapsed := time.Now().UnixNano() - t.rewriteScratch
	if elapsed < 1 {
		elapsed = 1
	}
	t.rewriteTime += elapsed
	t.rewriteScratch = 0
	return elapsed
}

// GetRewriteTime returns the total accumulated rewrite time in nanoseconds.
func (t *QueryProfilerTree) GetRewriteTime() int64 {
	return t.rewriteTime
}

// addDependencyNode inserts a new leaf node at the given token position.
func (t *QueryProfilerTree) addDependencyNode(query search.Query, token int) *QueryProfilerBreakdown {
	// Slot for children list.
	t.tree = append(t.tree, make([]int, 0, 5))

	// Store the query for later description assembly.
	t.queries = append(t.queries, query)

	bd := newQueryProfilerBreakdown()
	// Ensure breakdowns slice is large enough for token index.
	for len(t.breakdowns) <= token {
		t.breakdowns = append(t.breakdowns, nil)
	}
	t.breakdowns[token] = bd
	return bd
}

// doGetTree recursively builds the result tree for the given token.
func (t *QueryProfilerTree) doGetTree(token int) QueryProfilerResult {
	query := t.queries[token]
	bd := t.breakdowns[token]
	children := t.tree[token]

	var childResults []QueryProfilerResult
	if len(children) > 0 {
		childResults = make([]QueryProfilerResult, 0, len(children))
		for _, child := range children {
			childResults = append(childResults, t.doGetTree(child))
		}
	}

	// Mirrors Java: queryName = class.getSimpleName(), description = query.toString().
	queryName := fmt.Sprintf("%T", query)
	description := fmt.Sprintf("%v", query)
	return bd.GetQueryProfilerResult(queryName, description, childResults)
}

// updateParent records childToken as a child of the current stack top.
func (t *QueryProfilerTree) updateParent(childToken int) {
	parent := t.stack[len(t.stack)-1]
	t.tree[parent] = append(t.tree[parent], childToken)
}
