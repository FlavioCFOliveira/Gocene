// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
)

// Occur specifies how a clause should occur in a BooleanQuery.
type Occur int

const (
	// MUST - the clause must match.
	MUST Occur = iota
	// SHOULD - the clause should match (at least one SHOULD must match).
	SHOULD
	// MUST_NOT - the clause must not match.
	MUST_NOT
	// FILTER - the clause must match (but doesn't affect scoring).
	FILTER
)

func (o Occur) String() string {
	switch o {
	case MUST:
		return "MUST"
	case SHOULD:
		return "SHOULD"
	case MUST_NOT:
		return "MUST_NOT"
	case FILTER:
		return "FILTER"
	default:
		return fmt.Sprintf("Occur(%d)", o)
	}
}

// BooleanClause represents a clause in a BooleanQuery.
type BooleanClause struct {
	Query Query
	Occur Occur
}

// NewBooleanClause creates a new BooleanClause.
func NewBooleanClause(query Query, occur Occur) *BooleanClause {
	return &BooleanClause{Query: query, Occur: occur}
}

// BooleanQuery matches documents matching boolean combinations of clauses.
type BooleanQuery struct {
	*BaseQuery
	clauses        []*BooleanClause
	minShouldMatch int
}

// NewBooleanQuery creates a new BooleanQuery.
func NewBooleanQuery() *BooleanQuery {
	return &BooleanQuery{
		BaseQuery:      &BaseQuery{},
		clauses:        make([]*BooleanClause, 0),
		minShouldMatch: 0,
	}
}

// Add adds a clause to this query.
func (q *BooleanQuery) Add(query Query, occur Occur) {
	q.clauses = append(q.clauses, NewBooleanClause(query, occur))
}

// Clauses returns the clauses in this query.
func (q *BooleanQuery) Clauses() []*BooleanClause {
	return q.clauses
}

// SetMinimumNumberShouldMatch sets the minimum number of SHOULD clauses that must match.
func (q *BooleanQuery) SetMinimumNumberShouldMatch(min int) {
	q.minShouldMatch = min
}

// MinimumNumberShouldMatch returns the minimum number of SHOULD clauses that must match.
func (q *BooleanQuery) MinimumNumberShouldMatch() int {
	return q.minShouldMatch
}

// NewBooleanQueryOrWithQueries creates a BooleanQuery with OR semantics.
func NewBooleanQueryOrWithQueries(queries ...Query) *BooleanQuery {
	bq := NewBooleanQuery()
	for _, q := range queries {
		bq.Add(q, SHOULD)
	}
	return bq
}

// NewBooleanQueryAndWithQueries creates a BooleanQuery with AND semantics.
func NewBooleanQueryAndWithQueries(queries ...Query) *BooleanQuery {
	bq := NewBooleanQuery()
	for _, q := range queries {
		bq.Add(q, MUST)
	}
	return bq
}

// NewBooleanQueryNotWithQuery creates a BooleanQuery with NOT semantics.
func NewBooleanQueryNotWithQuery(query Query) *BooleanQuery {
	bq := NewBooleanQuery()
	bq.Add(query, MUST_NOT)
	return bq
}

// queryKey returns a string key for a Query based on its type name + hash code.
// Used for deduplication maps where we want equality semantics.
func queryKey(q Query) string {
	return fmt.Sprintf("%T:%d", q, q.HashCode())
}

// unwrapNonScoring strips ConstantScoreQuery and BoostQuery wrappers from a query
// that is used in a non-scoring context (FILTER, MUST_NOT). This mirrors the Java
// rewrite logic that wraps FILTER/MUST_NOT clauses in ConstantScoreQuery and then
// extracts the inner query.
func unwrapNonScoring(q Query) Query {
	for {
		switch v := q.(type) {
		case *ConstantScoreQuery:
			q = v.Query()
		case *BoostQuery:
			q = v.Query()
		default:
			return q
		}
	}
}

// countOccur returns the number of clauses with the given occur.
func (q *BooleanQuery) countOccur(o Occur) int {
	n := 0
	for _, c := range q.clauses {
		if c.Occur == o {
			n++
		}
	}
	return n
}

// isMatchNoDocsQuery reports whether q is a *MatchNoDocsQuery instance.
func isMatchNoDocsQuery(query Query) bool {
	_, ok := query.(*MatchNoDocsQuery)
	return ok
}

// isMatchAllDocsQuery reports whether q is a *MatchAllDocsQuery instance.
func isMatchAllDocsQuery(query Query) bool {
	_, ok := query.(*MatchAllDocsQuery)
	return ok
}

// Rewrite rewrites the query to a simpler form, applying all optimisation steps
// iteratively until convergence (no further changes). Mirrors Lucene's
// IndexSearcher.rewrite() loop which calls query.rewrite() until the result
// stabilises.
func (q *BooleanQuery) Rewrite(reader IndexReader) (Query, error) {
	var current Query = q
	for {
		var next Query
		var err error
		if bq, ok := current.(*BooleanQuery); ok {
			next, err = bq.rewriteStep(reader)
		} else {
			next, err = current.Rewrite(reader)
		}
		if err != nil {
			return nil, err
		}
		if next == current {
			return current, nil
		}
		current = next
	}
}

// rewriteStep performs one pass of BooleanQuery rewriting.
// Returns q unchanged (same pointer) if nothing changed.
func (q *BooleanQuery) rewriteStep(reader IndexReader) (Query, error) {
	// 1. Empty query → MatchNoDocsQuery.
	if len(q.clauses) == 0 {
		return NewMatchNoDocsQueryWithReason("empty BooleanQuery"), nil
	}

	// 2. Queries with no positive clauses have no matches.
	mustNotCount := q.countOccur(MUST_NOT)
	if mustNotCount == len(q.clauses) {
		return NewMatchNoDocsQueryWithReason("pure negative BooleanQuery"), nil
	}

	// 3. Optimise single-clause queries.
	if len(q.clauses) == 1 {
		c := q.clauses[0]
		query := c.Query
		if q.minShouldMatch == 1 && c.Occur == SHOULD {
			return query, nil
		}
		if q.minShouldMatch == 0 {
			switch c.Occur {
			case SHOULD, MUST:
				return query, nil
			case FILTER:
				// No scoring clauses: return BoostQuery(CSQ(inner), 0) matching Java.
				return NewBoostQuery(NewConstantScoreQuery(query), 0), nil
			}
		}
	}

	// 4. Recursively rewrite clauses. FILTER/MUST_NOT go through ConstantScoreQuery.rewrite.
	{
		builder := newBQBuilder(q.minShouldMatch)
		actuallyRewritten := false
		for _, clause := range q.clauses {
			query := clause.Query
			occur := clause.Occur
			var (
				rewritten Query
				err       error
			)
			if occur == FILTER || occur == MUST_NOT {
				// Simplify non-scoring clauses: strip ConstantScoreQuery/BoostQuery wrappers.
				rewritten = unwrapNonScoring(query)
				if rewritten != query {
					// We simplified; treat as rewritten.
				} else {
					// Try the normal rewrite path.
					var err2 error
					rewritten, err2 = query.Rewrite(reader)
					if err2 != nil {
						return nil, err2
					}
					// Strip any CSQ wrapper introduced by rewriting.
					rewritten = unwrapNonScoring(rewritten)
				}
				err = nil
			} else {
				rewritten, err = query.Rewrite(reader)
				if err != nil {
					return nil, err
				}
			}
			if rewritten != query || isMatchNoDocsQuery(query) {
				actuallyRewritten = true
				if isMatchNoDocsQuery(rewritten) {
					switch occur {
					case SHOULD, MUST_NOT:
						// ignore clause
					case MUST, FILTER:
						return rewritten, nil
					}
				} else {
					builder.add(rewritten, occur)
				}
			} else {
				builder.addClause(clause)
			}
		}
		if actuallyRewritten {
			return builder.build(), nil
		}
	}

	// 5. Remove duplicate FILTER and MUST_NOT clauses (they are sets in Java).
	{
		nFilter := q.countOccur(FILTER)
		nMustNot := q.countOccur(MUST_NOT)
		// Build sets (string-key dedup for FILTER and MUST_NOT).
		filterSet := map[string]Query{}
		mustNotSet := map[string]Query{}
		for _, c := range q.clauses {
			k := queryKey(c.Query)
			switch c.Occur {
			case FILTER:
				filterSet[k] = c.Query
			case MUST_NOT:
				mustNotSet[k] = c.Query
			}
		}
		if len(filterSet) != nFilter || len(mustNotSet) != nMustNot {
			builder := newBQBuilder(q.minShouldMatch)
			for _, c := range q.clauses {
				switch c.Occur {
				case FILTER, MUST_NOT:
					// skip: we'll re-add from set below
				default:
					builder.addClause(c)
				}
			}
			for _, fq := range filterSet {
				builder.add(fq, FILTER)
			}
			for _, mq := range mustNotSet {
				builder.add(mq, MUST_NOT)
			}
			return builder.build(), nil
		}
	}

	// 6. Check whether some clauses are both required and excluded.
	{
		mustNotSet := map[string]bool{}
		for _, c := range q.clauses {
			if c.Occur == MUST_NOT {
				mustNotSet[queryKey(c.Query)] = true
				if isMatchAllDocsQuery(c.Query) {
					return NewMatchNoDocsQueryWithReason("MUST_NOT clause is MatchAllDocsQuery"), nil
				}
			}
		}
		if len(mustNotSet) > 0 {
			for _, c := range q.clauses {
				if (c.Occur == MUST || c.Occur == FILTER) && mustNotSet[queryKey(c.Query)] {
					return NewMatchNoDocsQueryWithReason("FILTER or MUST clause also in MUST_NOT"), nil
				}
			}
		}
	}

	// 7. Remove FILTER clauses that are also MUST clauses or that match all documents.
	{
		nFilter := q.countOccur(FILTER)
		if nFilter > 0 {
			mustSet := map[string]bool{}
			for _, c := range q.clauses {
				if c.Occur == MUST {
					mustSet[queryKey(c.Query)] = true
				}
			}
			nMust := q.countOccur(MUST)
			filterSet := map[string]Query{}
			for _, c := range q.clauses {
				if c.Occur == FILTER {
					filterSet[queryKey(c.Query)] = c.Query
				}
			}
			modified := false
			// Remove MatchAllDocsQuery from FILTER if there are multiple filters or any MUST.
			if nFilter > 1 || nMust > 0 {
				for k, fq := range filterSet {
					if isMatchAllDocsQuery(fq) {
						delete(filterSet, k)
						modified = true
					}
				}
			}
			// Remove filters that are also MUST clauses.
			for k := range filterSet {
				if mustSet[k] {
					delete(filterSet, k)
					modified = true
				}
			}
			if modified {
				builder := newBQBuilder(q.minShouldMatch)
				for _, c := range q.clauses {
					if c.Occur != FILTER {
						builder.addClause(c)
					}
				}
				for _, fq := range filterSet {
					builder.add(fq, FILTER)
				}
				return builder.build(), nil
			}
		}
	}

	// 8. Convert FILTER clauses that are also SHOULD clauses to MUST clauses.
	{
		if q.countOccur(SHOULD) > 0 && q.countOccur(FILTER) > 0 {
			filterSet := map[string]bool{}
			shouldSet := map[string]bool{}
			for _, c := range q.clauses {
				k := queryKey(c.Query)
				if c.Occur == FILTER {
					filterSet[k] = true
				}
				if c.Occur == SHOULD {
					shouldSet[k] = true
				}
			}
			// intersection: keys in both filter and should
			intersection := map[string]bool{}
			for k := range filterSet {
				if shouldSet[k] {
					intersection[k] = true
				}
			}
			if len(intersection) > 0 {
				builder := newBQBuilder(q.minShouldMatch)
				msm := q.minShouldMatch
				for _, c := range q.clauses {
					if intersection[queryKey(c.Query)] {
						if c.Occur == SHOULD {
							builder.add(c.Query, MUST)
							msm--
						}
						// FILTER occurrence is absorbed — skip adding it
					} else {
						builder.addClause(c)
					}
				}
				if msm < 0 {
					msm = 0
				}
				builder.msm = msm
				return builder.build(), nil
			}
		}
	}

	// 9. Deduplicate SHOULD clauses by summing boosts (only when msm <= 1).
	if q.countOccur(SHOULD) > 0 && q.minShouldMatch <= 1 {
		type boostEntry struct {
			query Query
			boost float64
		}
		shouldByKey := map[string]*boostEntry{}
		order := []string{}
		for _, c := range q.clauses {
			if c.Occur != SHOULD {
				continue
			}
			inner := c.Query
			boost := 1.0
			for {
				if bq, ok := inner.(*BoostQuery); ok {
					boost *= float64(bq.Boost())
					inner = bq.Query()
				} else {
					break
				}
			}
			k := queryKey(inner)
			if e, found := shouldByKey[k]; found {
				e.boost += boost
			} else {
				shouldByKey[k] = &boostEntry{query: inner, boost: boost}
				order = append(order, k)
			}
		}
		if len(shouldByKey) != q.countOccur(SHOULD) {
			builder := newBQBuilder(q.minShouldMatch)
			for _, k := range order {
				e := shouldByKey[k]
				query := e.query
				if e.boost != 1.0 {
					query = NewBoostQuery(query, float32(e.boost))
				}
				builder.add(query, SHOULD)
			}
			for _, c := range q.clauses {
				if c.Occur != SHOULD {
					builder.addClause(c)
				}
			}
			return builder.build(), nil
		}
	}

	// 10. Deduplicate MUST clauses by summing boosts.
	if q.countOccur(MUST) > 0 {
		type boostEntry struct {
			query Query
			boost float64
		}
		mustByKey := map[string]*boostEntry{}
		order := []string{}
		for _, c := range q.clauses {
			if c.Occur != MUST {
				continue
			}
			inner := c.Query
			boost := 1.0
			for {
				if bq, ok := inner.(*BoostQuery); ok {
					boost *= float64(bq.Boost())
					inner = bq.Query()
				} else {
					break
				}
			}
			k := queryKey(inner)
			if e, found := mustByKey[k]; found {
				e.boost += boost
			} else {
				mustByKey[k] = &boostEntry{query: inner, boost: boost}
				order = append(order, k)
			}
		}
		if len(mustByKey) != q.countOccur(MUST) {
			builder := newBQBuilder(q.minShouldMatch)
			for _, k := range order {
				e := mustByKey[k]
				query := e.query
				if e.boost != 1.0 {
					query = NewBoostQuery(query, float32(e.boost))
				}
				builder.add(query, MUST)
			}
			for _, c := range q.clauses {
				if c.Occur != MUST {
					builder.addClause(c)
				}
			}
			return builder.build(), nil
		}
	}

	// 11. Rewrite single MUST(MatchAllDocsQuery) + FILTER clauses → ConstantScoreQuery.
	{
		musts := []*BooleanClause{}
		for _, c := range q.clauses {
			if c.Occur == MUST {
				musts = append(musts, c)
			}
		}
		filters := []*BooleanClause{}
		for _, c := range q.clauses {
			if c.Occur == FILTER {
				filters = append(filters, c)
			}
		}
		if len(musts) == 1 && len(filters) > 0 {
			must := musts[0].Query
			boost := float32(1.0)
			if bq, ok := must.(*BoostQuery); ok {
				must = bq.Query()
				boost = bq.Boost()
			}
			if isMatchAllDocsQuery(must) {
				// Build FILTER+MUST_NOT sub-query.
				fb := newBQBuilder(0)
				for _, c := range q.clauses {
					if c.Occur == FILTER || c.Occur == MUST_NOT {
						fb.addClause(c)
					}
				}
				var rewritten Query = NewConstantScoreQuery(fb.build())
				if boost != 1.0 {
					rewritten = NewBoostQuery(rewritten, boost)
				}
				// Add back SHOULD clauses.
				sb := newBQBuilder(q.minShouldMatch)
				sb.add(rewritten, MUST)
				for _, c := range q.clauses {
					if c.Occur == SHOULD {
						sb.addClause(c)
					}
				}
				return sb.build(), nil
			}
		}
	}

	// 12. Flatten nested pure disjunctions (important for block-max WAND).
	if q.minShouldMatch <= 1 {
		builder := newBQBuilder(q.minShouldMatch)
		actuallyRewritten := false
		for _, clause := range q.clauses {
			if clause.Occur == SHOULD {
				if inner, ok := clause.Query.(*BooleanQuery); ok {
					if inner.isPureDisjunction() {
						actuallyRewritten = true
						for _, ic := range inner.clauses {
							builder.addClause(ic)
						}
						continue
					}
				}
			}
			builder.addClause(clause)
		}
		if actuallyRewritten {
			return builder.build(), nil
		}
	}

	// 13. Inline required/prohibited inner conjunctions.
	{
		builder := newBQBuilder(q.minShouldMatch)
		actuallyRewritten := false
		for _, outer := range q.clauses {
			if outer.isRequired() {
				if inner, ok := outer.Query.(*BooleanQuery); ok {
					if inner.minShouldMatch == 0 && inner.countOccur(SHOULD) == 0 {
						actuallyRewritten = true
						for _, ic := range inner.clauses {
							innerOccur := ic.Occur
							if innerOccur == FILTER || innerOccur == MUST_NOT || outer.Occur == MUST {
								builder.addClause(ic)
							} else {
								// outer is FILTER, inner is MUST → demote to FILTER
								builder.add(ic.Query, FILTER)
							}
						}
						continue
					}
				}
			}
			builder.addClause(outer)
		}
		if actuallyRewritten {
			return builder.build(), nil
		}
	}

	// 14. SHOULD clause count less than minimumNumberShouldMatch.
	{
		nShould := q.countOccur(SHOULD)
		if nShould < q.minShouldMatch {
			return NewMatchNoDocsQueryWithReason("SHOULD clause count less than minimumNumberShouldMatch"), nil
		}
		if nShould > 0 && nShould == q.minShouldMatch {
			builder := newBQBuilder(0)
			for _, c := range q.clauses {
				if c.Occur == SHOULD {
					builder.add(c.Query, MUST)
				} else {
					builder.addClause(c)
				}
			}
			return builder.build(), nil
		}
	}

	// 15. Inline SHOULD clauses from the only MUST clause.
	{
		nShould := q.countOccur(SHOULD)
		nMust := q.countOccur(MUST)
		if nShould == 0 && nMust == 1 {
			var mustClause *BooleanClause
			for _, c := range q.clauses {
				if c.Occur == MUST {
					mustClause = c
					break
				}
			}
			if inner, ok := mustClause.Query.(*BooleanQuery); ok {
				if inner.countOccur(SHOULD) == len(inner.clauses) {
					builder := newBQBuilder(0)
					for _, c := range q.clauses {
						if c.Occur != MUST {
							builder.addClause(c)
						}
					}
					for _, ic := range inner.clauses {
						builder.addClause(ic)
					}
					msm := inner.minShouldMatch
					if msm < 1 {
						msm = 1
					}
					builder.msm = msm
					return builder.build(), nil
				}
			}
		}
	}

	return q, nil
}

// isPureDisjunction reports whether this is a pure disjunction
// (only SHOULD clauses and minShouldMatch <= 1).
func (q *BooleanQuery) isPureDisjunction() bool {
	return q.countOccur(SHOULD) == len(q.clauses) && q.minShouldMatch <= 1
}

// rewriteNoScoring rewrites the query for a non-scoring context (inside a ConstantScoreQuery).
// It strips BoostQuery/ConstantScoreQuery wrappers, converts MUST→FILTER, and removes
// SHOULD clauses when they are not needed (keepShould = minShouldMatch>0 or no MUST+FILTER).
// NOTE: must not call Rewrite() to avoid exponential blowup with nested BooleanQueries.
// Mirrors BooleanQuery.rewriteNoScoring() from Lucene 10.4.0.
func (q *BooleanQuery) rewriteNoScoring() *BooleanQuery {
	nMust := q.countOccur(MUST)
	nFilter := q.countOccur(FILTER)
	keepShould := q.minShouldMatch > 0 || (nMust+nFilter == 0)

	actuallyRewritten := false
	builder := newBQBuilder(q.minShouldMatch)
	for _, clause := range q.clauses {
		query := clause.Query
		occur := clause.Occur

		// Strip BoostQuery/ConstantScoreQuery wrappers without recursing into Rewrite.
		rewritten := query
		if bq, ok := rewritten.(*BoostQuery); ok {
			rewritten = bq.Query()
		} else if csq, ok := rewritten.(*ConstantScoreQuery); ok {
			rewritten = csq.Query()
		}
		if bq2, ok := rewritten.(*BooleanQuery); ok {
			rewritten = bq2.rewriteNoScoring()
		}

		if occur == SHOULD && !keepShould {
			actuallyRewritten = true
			// ignore clause
		} else if occur == MUST {
			builder.add(rewritten, FILTER)
			actuallyRewritten = true
		} else if query != rewritten {
			builder.addClause(&BooleanClause{Query: rewritten, Occur: occur})
			actuallyRewritten = true
		} else {
			builder.addClause(clause)
		}
	}

	if !actuallyRewritten {
		return q
	}
	return builder.build()
}

// isRequired reports whether this clause must match.
func (c *BooleanClause) isRequired() bool {
	return c.Occur == MUST || c.Occur == FILTER
}

// bqBuilder is a lightweight builder for BooleanQuery used during rewriting.
type bqBuilder struct {
	clauses []*BooleanClause
	msm     int
}

func newBQBuilder(msm int) *bqBuilder {
	return &bqBuilder{msm: msm}
}

func (b *bqBuilder) add(query Query, occur Occur) {
	b.clauses = append(b.clauses, &BooleanClause{Query: query, Occur: occur})
}

func (b *bqBuilder) addClause(c *BooleanClause) {
	b.clauses = append(b.clauses, c)
}

func (b *bqBuilder) build() *BooleanQuery {
	bq := &BooleanQuery{
		BaseQuery:      &BaseQuery{},
		clauses:        b.clauses,
		minShouldMatch: b.msm,
	}
	if bq.clauses == nil {
		bq.clauses = make([]*BooleanClause, 0)
	}
	return bq
}

// Clone creates a copy of this query.
func (q *BooleanQuery) Clone() Query {
	clonedClauses := make([]*BooleanClause, len(q.clauses))
	for i, clause := range q.clauses {
		clonedClauses[i] = &BooleanClause{
			Query: clause.Query.Clone(),
			Occur: clause.Occur,
		}
	}
	return &BooleanQuery{
		BaseQuery:      q.BaseQuery,
		clauses:        clonedClauses,
		minShouldMatch: q.minShouldMatch,
	}
}

// Equals checks if this query equals another.
// Mirrors Java's clauseSets semantics:
//   - FILTER and MUST_NOT: set equality (order-independent, deduplicated)
//   - SHOULD and MUST: multiset equality (order-independent, duplicates count)
func (q *BooleanQuery) Equals(other Query) bool {
	o, ok := other.(*BooleanQuery)
	if !ok {
		return false
	}
	if q.minShouldMatch != o.minShouldMatch || len(q.clauses) != len(o.clauses) {
		return false
	}
	// Group clauses by occur for each query.
	type grouped struct {
		must, should, filter, mustNot []Query
	}
	group := func(bq *BooleanQuery) grouped {
		var g grouped
		for _, c := range bq.clauses {
			switch c.Occur {
			case MUST:
				g.must = append(g.must, c.Query)
			case SHOULD:
				g.should = append(g.should, c.Query)
			case FILTER:
				g.filter = append(g.filter, c.Query)
			case MUST_NOT:
				g.mustNot = append(g.mustNot, c.Query)
			}
		}
		return g
	}
	gq := group(q)
	go2 := group(o)
	return matchQueryMultiset(gq.must, go2.must) &&
		matchQueryMultiset(gq.should, go2.should) &&
		matchQueryMultiset(gq.filter, go2.filter) &&
		matchQueryMultiset(gq.mustNot, go2.mustNot)
}

// matchQueryMultiset reports whether two slices of queries are equal as multisets.
// Uses pairwise Equals() matching; O(n²) but n is small for BooleanQuery clauses.
func matchQueryMultiset(a, b []Query) bool {
	if len(a) != len(b) {
		return false
	}
	used := make([]bool, len(b))
	for _, qa := range a {
		found := false
		for j, qb := range b {
			if !used[j] && qa.Equals(qb) {
				used[j] = true
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// HashCode returns a hash code for this query.
// Uses commutative combination so that order of clauses does not affect the result,
// matching the order-independent semantics of Equals.
func (q *BooleanQuery) HashCode() int {
	hash := q.minShouldMatch
	for _, clause := range q.clauses {
		// XOR is commutative and associative → order-independent.
		hash ^= int(clause.Occur)*31 + clause.Query.HashCode()
	}
	return hash
}

func (q *BooleanQuery) String() string {
	buffer := ""
	if q.minShouldMatch > 0 {
		buffer += fmt.Sprintf("minShouldMatch=%d ", q.minShouldMatch)
	}
	for i, clause := range q.clauses {
		if i > 0 {
			buffer += " "
		}
		switch clause.Occur {
		case MUST:
			buffer += "+"
		case MUST_NOT:
			buffer += "-"
		case FILTER:
			buffer += "#"
		}
		buffer += fmt.Sprintf("%v", clause.Query)
	}
	return buffer
}

// CreateWeight creates a Weight for this query.
func (q *BooleanQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewBooleanWeight(q, searcher, needsScores)
}

// Ensure BooleanQuery implements Query
var _ Query = (*BooleanQuery)(nil)
