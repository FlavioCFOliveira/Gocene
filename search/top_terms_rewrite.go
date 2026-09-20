// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

// Ported from Apache Lucene 10.5.0:
//
//	lucene/core/src/java/org/apache/lucene/search/TopTermsRewrite.java

import (
	"container/heap"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TopTermsRewriteDelegate carries the abstract members that a TopTermsRewrite
// subclass must supply: getMaxSize() declared by TopTermsRewrite itself, and
// getTopLevelBuilder() / build(B) / addClause(B, Term, int, float, TermStates)
// inherited from TermCollectingRewrite<B>.
//
// Go has no abstract methods, so the concrete rewrite installs itself as the
// delegate in its constructor — the same indirection [ScoringRewriteDelegate]
// already uses for ScoringRewrite.
type TopTermsRewriteDelegate[B any] interface {
	// GetMaxSize returns the maximum size of the priority queue (for boolean
	// rewrites this is IndexSearcher.getMaxClauseCount()).
	//
	// Mirrors {@code protected abstract int getMaxSize()}.
	GetMaxSize() int

	// GetTopLevelBuilder returns a suitable builder for the top-level Query
	// holding all expanded terms.
	//
	// Mirrors {@code protected abstract B getTopLevelBuilder() throws IOException}.
	GetTopLevelBuilder() (B, error)

	// Build finalizes the creation of the query from the builder.
	//
	// Mirrors {@code protected abstract Query build(B builder)}. Java's
	// signature declares no checked exception; the Go error return carries
	// the unchecked IllegalArgumentException a builder may raise (for
	// example SpanOrQuery's "Clauses must have same field").
	Build(builder B) (Query, error)

	// AddClause adds a MultiTermQuery term to the top-level query builder.
	//
	// Mirrors {@code protected abstract void addClause(B topLevel, Term term,
	// int docCount, float boost, TermStates states) throws IOException}.
	AddClause(topLevel B, term *index.Term, docCount int, boost float32, states *index.TermStates) error
}

// TopTermsRewrite is the base rewrite method for collecting only the top terms
// via a priority queue.
//
// Mirrors org.apache.lucene.search.TopTermsRewrite<B>, declared as
// {@code abstract class TopTermsRewrite<B> extends TermCollectingRewrite<B>}
// (Lucene 10.5.0). Java's type parameter B is carried over directly.
type TopTermsRewrite[B any] struct {
	size     int
	delegate TopTermsRewriteDelegate[B]
}

// NewTopTermsRewrite creates a TopTermsRewrite for at most size terms, backed
// by the supplied delegate.
//
// Mirrors {@code public TopTermsRewrite(int size)}. The delegate parameter is
// the Go stand-in for the subclass body that Java reaches through {@code this}.
// Lucene declares the class package-private but its constructor public, and
// org.apache.lucene.queries.spans.SpanMultiTermQueryWrapper subclasses it from
// another package (TopTermsSpanBooleanQueryRewrite), so the Go constructor is
// exported to keep that subclass expressible.
func NewTopTermsRewrite[B any](size int, delegate TopTermsRewriteDelegate[B]) *TopTermsRewrite[B] {
	return &TopTermsRewrite[B]{size: size, delegate: delegate}
}

// GetSize returns the maximum priority queue size.
//
// Mirrors {@code public int getSize()}.
func (r *TopTermsRewrite[B]) GetSize() int {
	return r.size
}

// Rewrite collects the top-scoring terms of query across every leaf of the
// searcher's reader and assembles them into the builder supplied by the
// delegate.
//
// Mirrors {@code public final Query rewrite(IndexSearcher indexSearcher,
// final MultiTermQuery query)} of Apache Lucene 10.5.0, statement for
// statement. The Java body's `assert` machinery (compareToLastTerm, the
// state != null and docFreq() == 0 checks) is omitted: Java asserts are
// disabled unless the JVM runs with -ea, so they are not observable behaviour.
func (r *TopTermsRewrite[B]) Rewrite(searcher *IndexSearcher, query *MultiTermQuery) (Query, error) {
	maxSize := r.size
	if m := r.delegate.GetMaxSize(); m < maxSize {
		maxSize = m
	}
	stQueue := &scoreTermQueue{}
	heap.Init(stQueue)

	collector := &topTermsCollector{
		maxSize:      maxSize,
		stQueue:      stQueue,
		visitedTerms: make(map[string]*scoreTerm),
	}
	// private final MaxNonCompetitiveBoostAttribute maxBoostAtt =
	//     attributes.addAttribute(MaxNonCompetitiveBoostAttribute.class);
	maxBoostAtt, ok := collector.Attributes().AddAttribute(MaxNonCompetitiveBoostAttributeType).(MaxNonCompetitiveBoostAttribute)
	if !ok {
		return nil, errMaxNonCompetitiveBoostAttribute
	}
	collector.maxBoostAtt = maxBoostAtt

	if err := CollectTerms(r, searcher.GetIndexReader(), query, collector); err != nil {
		return nil, err
	}

	b, err := r.delegate.GetTopLevelBuilder()
	if err != nil {
		return nil, err
	}
	// final ScoreTerm[] scoreTerms = stQueue.toArray(ScoreTerm[]::new);
	// ArrayUtil.timSort(scoreTerms, (st1, st2) -> st1.bytes.get().compareTo(st2.bytes.get()));
	scoreTerms := make([]*scoreTerm, len(*stQueue))
	copy(scoreTerms, *stQueue)
	util.TimSort(scoreTerms, func(st1, st2 *scoreTerm) int {
		return util.BytesRefCompare(st1.bytes.Get(), st2.bytes.Get())
	})

	for _, st := range scoreTerms {
		term := index.NewTermFromBytesRef(query.field, st.bytes.ToBytesRef())
		// We allow negative term scores (fuzzy query does this, for example)
		// while collecting the terms, but truncate such boosts to 0.0f when
		// building the query:
		boost := st.boost
		if boost < 0.0 {
			boost = 0.0
		}
		if err := r.delegate.AddClause(b, term, st.termState.DocFreq(), boost, st.termState); err != nil {
			return nil, err
		}
	}
	return r.delegate.Build(b)
}

// HashCode reproduces {@code public int hashCode() { return 31 * size; }}.
func (r *TopTermsRewrite[B]) HashCode() int {
	return 31 * r.size
}

// Equals reproduces
//
//	if (this == obj) return true;
//	if (obj == null) return false;
//	if (getClass() != obj.getClass()) return false;
//	final TopTermsRewrite<?> other = (TopTermsRewrite<?>) obj;
//	if (size != other.size) return false;
//	return true;
//
// getClass() is the concrete subclass, which the installed delegate carries.
func (r *TopTermsRewrite[B]) Equals(other RewriteMethod) bool {
	if other == nil {
		return false
	}
	if reflect.TypeOf(r.delegate) != reflect.TypeOf(other) {
		return false
	}
	sized, ok := other.(interface{ GetSize() int })
	if !ok {
		return false
	}
	return r.size == sized.GetSize()
}

// ─── ScoreTerm ──────────────────────────────────────────────────────────────

// scoreTerm mirrors the package-private static final nested class
// TopTermsRewrite.ScoreTerm, which implements Comparable<ScoreTerm>.
type scoreTerm struct {
	bytes     *util.BytesRefBuilder
	boost     float32
	termState *index.TermStates
}

// newScoreTerm mirrors {@code public ScoreTerm(TermStates termState)}; the
// BytesRefBuilder is a field initialiser in Java.
func newScoreTerm(termState *index.TermStates) *scoreTerm {
	return &scoreTerm{
		bytes:     util.NewBytesRefBuilder(),
		termState: termState,
	}
}

// compareTo reproduces
//
//	if (this.boost == other.boost) return other.bytes.get().compareTo(this.bytes.get());
//	else return Float.compare(this.boost, other.boost);
func (s *scoreTerm) compareTo(other *scoreTerm) int {
	if s.boost == other.boost {
		return util.BytesRefCompare(other.bytes.Get(), s.bytes.Get())
	}
	return floatCompare(s.boost, other.boost)
}

// floatCompare reproduces java.lang.Float#compare(float, float) for the
// non-NaN, non-negative-zero values a boost takes here.
func floatCompare(a, b float32) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// scoreTermQueue is the Go rendering of the java.util.PriorityQueue<ScoreTerm>
// TopTermsRewrite.rewrite allocates: a binary min-heap ordered by
// ScoreTerm.compareTo, so the head is the least competitive entry.
type scoreTermQueue []*scoreTerm

func (q scoreTermQueue) Len() int           { return len(q) }
func (q scoreTermQueue) Less(i, j int) bool { return q[i].compareTo(q[j]) < 0 }
func (q scoreTermQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *scoreTermQueue) Push(x any)        { *q = append(*q, x.(*scoreTerm)) }
func (q *scoreTermQueue) Pop() any {
	old := *q
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*q = old[:n-1]
	return item
}

// peek reproduces {@code PriorityQueue#peek()}: the head, or null when empty.
func (q *scoreTermQueue) peek() *scoreTerm {
	if len(*q) == 0 {
		return nil
	}
	return (*q)[0]
}

// offer reproduces {@code PriorityQueue#offer(E)}.
func (q *scoreTermQueue) offer(st *scoreTerm) { heap.Push(q, st) }

// poll reproduces {@code PriorityQueue#poll()}: removes and returns the head.
func (q *scoreTermQueue) poll() *scoreTerm {
	if len(*q) == 0 {
		return nil
	}
	return heap.Pop(q).(*scoreTerm)
}

// ─── TermCollector ──────────────────────────────────────────────────────────

// topTermsCollector is the anonymous TermCollector that
// TopTermsRewrite.rewrite instantiates inline.
type topTermsCollector struct {
	BaseTermCollector

	maxSize      int
	stQueue      *scoreTermQueue
	visitedTerms map[string]*scoreTerm
	maxBoostAtt  MaxNonCompetitiveBoostAttribute

	termsEnum index.TermsEnum
	boostAtt  BoostAttribute
	st        *scoreTerm
}

// SetNextEnum reproduces
//
//	this.termsEnum = termsEnum;
//	if (st == null) st = new ScoreTerm(new TermStates(topReaderContext));
//	boostAtt = termsEnum.attributes().addAttribute(BoostAttribute.class);
func (c *topTermsCollector) SetNextEnum(termsEnum index.TermsEnum) error {
	c.termsEnum = termsEnum

	// lazy init the initial ScoreTerm because comparator is not known on ctor:
	if c.st == nil {
		states, err := index.NewTermStatesForContext(c.TopReaderContext)
		if err != nil {
			return err
		}
		c.st = newScoreTerm(states)
	}
	boostAtt, ok := termsEnum.Attributes().AddAttribute(BoostAttributeType).(BoostAttribute)
	if !ok {
		return errBoostAttribute
	}
	c.boostAtt = boostAtt
	return nil
}

// Collect reproduces {@code public boolean collect(BytesRef bytes)} of the
// anonymous TermCollector in TopTermsRewrite.rewrite.
func (c *topTermsCollector) Collect(term *index.Term) (bool, error) {
	boost := c.boostAtt.GetBoost()
	bytes := term.Bytes

	// ignore uncompetitive hits
	if c.stQueue.Len() == c.maxSize {
		t := c.stQueue.peek()
		if boost < t.boost {
			return true, nil
		}
		if boost == t.boost && util.BytesRefCompare(bytes, t.bytes.Get()) > 0 {
			return true, nil
		}
	}
	key := string(bytes.ValidBytes())
	t := c.visitedTerms[key]
	state, err := index.TermStateDelegated(c.termsEnum)
	if err != nil {
		return false, err
	}
	docFreq, err := c.termsEnum.DocFreq()
	if err != nil {
		return false, err
	}
	totalTermFreq, err := c.termsEnum.TotalTermFreq()
	if err != nil {
		return false, err
	}
	if t != nil {
		// if the term is already in the PQ, only update docFreq of term in PQ
		t.termState.Register(c.ReaderContext.Ord, state, docFreq, totalTermFreq)
		return true, nil
	}

	// add new entry in PQ, we must clone the term, else it may get overwritten!
	c.st.bytes.CopyBytesRef(bytes)
	c.st.boost = boost
	c.visitedTerms[string(c.st.bytes.Get().ValidBytes())] = c.st
	c.st.termState.Register(c.ReaderContext.Ord, state, docFreq, totalTermFreq)
	c.stQueue.offer(c.st)
	// possibly drop entries from queue
	if c.stQueue.Len() > c.maxSize {
		c.st = c.stQueue.poll()
		delete(c.visitedTerms, string(c.st.bytes.Get().ValidBytes()))
		c.st.termState.Clear() // reset the termstate!
	} else {
		states, err := index.NewTermStatesForContext(c.TopReaderContext)
		if err != nil {
			return false, err
		}
		c.st = newScoreTerm(states)
	}
	// set maxBoostAtt with values to help FuzzyTermsEnum to optimize
	if c.stQueue.Len() == c.maxSize {
		top := c.stQueue.peek()
		c.maxBoostAtt.SetMaxNonCompetitiveBoost(top.boost)
		c.maxBoostAtt.SetCompetitiveTerm(top.bytes.Get().ValidBytes())
	}

	return true, nil
}

// ─── instanceof TopTermsRewrite ─────────────────────────────────────────────

// topTermsRewriteMarker is the method set a value carries when, in Java, it
// *is* a TopTermsRewrite: the public getSize() accessor plus a discriminator
// only [TopTermsRewrite] can contribute, since the marker method is unexported
// and therefore unimplementable outside this package.
type topTermsRewriteMarker interface {
	GetSize() int
	topTermsRewrite()
}

// topTermsRewrite is the discriminator that makes [topTermsRewriteMarker]
// identify a TopTermsRewrite and nothing else. Embedding a *TopTermsRewrite[B]
// promotes it, exactly as extending the class does in Java.
func (r *TopTermsRewrite[B]) topTermsRewrite() {}

// AsTopTermsRewrite renders Java's
//
//	if (method instanceof TopTermsRewrite) {
//	  final int pqsize = ((TopTermsRewrite<?>) method).getSize();
//
// which Go cannot spell directly: TopTermsRewrite is generic, and a type
// assertion cannot bind its type parameter the way Java's wildcard does. The
// test is therefore performed against the class's own method set. It reports
// whether method is a TopTermsRewrite and, when it is, its priority-queue
// size.
//
// Only Apache Lucene 10.5.0's SpanMultiTermQueryWrapper.selectRewriteMethod
// performs this test.
func AsTopTermsRewrite(method RewriteMethod) (size int, ok bool) {
	m, ok := method.(topTermsRewriteMarker)
	if !ok {
		return 0, false
	}
	return m.GetSize(), true
}
