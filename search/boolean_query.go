// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BooleanQuery matches documents matching boolean combinations of other queries.
//
// Port of org.apache.lucene.search.BooleanQuery (Apache Lucene 10.5.0).
type BooleanQuery struct {
	minimumNumberShouldMatch int
	clauses                  []*BooleanClause // used for toString() and getClauses()
	// clauseSets renders the Java EnumMap<Occur, Collection<Query>> used for
	// equals/hashCode: SHOULD and MUST are Multisets (duplicates matter),
	// FILTER and MUST_NOT are HashSets (duplicates do not). It is indexed by
	// the Occur ordinal, so iterating it follows the EnumMap key order.
	// WARNING: Do not let clauseSets escape from this type as it breaks
	// immutability.
	clauseSets [MUST_NOT + 1]*queryCollection
	// cached hash code is ok since boolean queries are immutable
	hashCode int
}

// BooleanQueryBuilder is a builder for boolean queries
// (org.apache.lucene.search.BooleanQuery.Builder).
type BooleanQueryBuilder struct {
	minimumNumberShouldMatch int
	clauses                  []*BooleanClause
}

// NewBooleanQueryBuilder creates a new BooleanQueryBuilder.
func NewBooleanQueryBuilder() *BooleanQueryBuilder {
	return &BooleanQueryBuilder{}
}

// SetMinimumNumberShouldMatch specifies a minimum number of the optional
// BooleanClauses which must be satisfied.
//
// By default no optional clauses are necessary for a match (unless there are
// no required clauses). If this method is used, then the specified number of
// clauses is required.
//
// Use of this method is totally independent of specifying that any specific
// clauses are required (or prohibited). This number will only be compared
// against the number of matching optional clauses.
func (b *BooleanQueryBuilder) SetMinimumNumberShouldMatch(min int) *BooleanQueryBuilder {
	b.minimumNumberShouldMatch = min
	return b
}

// AddClause renders Builder.add(BooleanClause): it adds a new clause to this
// Builder. Note that the order in which clauses are added does not have any
// impact on matching documents or query performance. It panics with
// *TooManyClauses if the new number of clauses exceeds the maximum clause
// number.
func (b *BooleanQueryBuilder) AddClause(clause *BooleanClause) *BooleanQueryBuilder {
	// We do the final deep check for max clauses count limit during
	// IndexSearcher.rewrite but do this check to short circuit in case a
	// single query holds more than numClauses
	//
	// NOTE: this is not just an early check for optimization -- it's
	// neccessary to prevent run-away 'rewriting' of bad queries from
	// creating BQ objects that might eat up all the Heap.
	if len(b.clauses) >= GetMaxClauseCount() {
		panic(NewTooManyClauses())
	}
	b.clauses = append(b.clauses, clause)
	return b
}

// AddCollection renders Builder.add(Collection<BooleanClause>): it adds a
// collection of BooleanClauses to this Builder. It panics with
// *TooManyClauses if the new number of clauses exceeds the maximum clause
// number.
func (b *BooleanQueryBuilder) AddCollection(collection []*BooleanClause) *BooleanQueryBuilder {
	// see #addClause(BooleanClause)
	if len(b.clauses)+len(collection) > GetMaxClauseCount() {
		panic(NewTooManyClauses())
	}
	b.clauses = append(b.clauses, collection...)
	return b
}

// Add renders Builder.add(Query, Occur): it adds a new clause to this
// Builder. It panics with *TooManyClauses if the new number of clauses
// exceeds the maximum clause number.
func (b *BooleanQueryBuilder) Add(query Query, occur Occur) *BooleanQueryBuilder {
	return b.AddClause(NewBooleanClause(query, occur))
}

// Build creates a new BooleanQuery based on the parameters that have been set
// on this builder.
func (b *BooleanQueryBuilder) Build() *BooleanQuery {
	return newBooleanQuery(b.minimumNumberShouldMatch, append([]*BooleanClause(nil), b.clauses...))
}

// newBooleanQuery renders the private BooleanQuery(int, BooleanClause[])
// constructor.
func newBooleanQuery(minimumNumberShouldMatch int, clauses []*BooleanClause) *BooleanQuery {
	bq := &BooleanQuery{
		minimumNumberShouldMatch: minimumNumberShouldMatch,
		clauses:                  clauses,
	}
	// duplicates matter for SHOULD and MUST
	bq.clauseSets[SHOULD] = newQueryMultiset()
	bq.clauseSets[MUST] = newQueryMultiset()
	// but not for FILTER and MUST_NOT
	bq.clauseSets[FILTER] = newQueryHashSet()
	bq.clauseSets[MUST_NOT] = newQueryHashSet()
	for _, clause := range clauses {
		bq.clauseSets[clause.Occur()].add(clause.Query())
	}
	return bq
}

// GetMinimumNumberShouldMatch gets the minimum number of the optional
// BooleanClauses which must be satisfied.
func (q *BooleanQuery) GetMinimumNumberShouldMatch() int {
	return q.minimumNumberShouldMatch
}

// Clauses returns a list of the clauses of this BooleanQuery. Java returns an
// unmodifiable list; the returned slice has no spare capacity, so appending
// to it never writes into this query.
func (q *BooleanQuery) Clauses() []*BooleanClause {
	return q.clauses[:len(q.clauses):len(q.clauses)]
}

// GetClauses returns the collection of queries for the given Occur. Java
// returns an unmodifiable view; the returned slice is a copy, so it never
// exposes clauseSets.
func (q *BooleanQuery) GetClauses(occur Occur) []Query {
	// turn this immutable here, because we need to preserve the correct
	// collection types for equals/hashCode!
	return q.clauseSets[occur].toSlice()
}

// IsPureDisjunction returns whether this query is a pure disjunction.
func (q *BooleanQuery) IsPureDisjunction() bool {
	return len(q.clauses) == q.clauseSets[SHOULD].size() && q.minimumNumberShouldMatch <= 1
}

// IsTwoClausePureDisjunctionWithTerms returns whether this query is a two clause disjunction with two term query clauses.
func (q *BooleanQuery) IsTwoClausePureDisjunctionWithTerms() bool {
	return len(q.clauses) == 2 &&
		q.IsPureDisjunction() &&
		isTermQuery(q.clauses[0].Query()) &&
		isTermQuery(q.clauses[1].Query())
}

func isTermQuery(q Query) bool {
	_, ok := q.(*TermQuery)
	return ok
}

// RewriteTwoClauseDisjunctionWithTermsForCount rewrites a single two clause disjunction query with terms to two term queries and a conjunction query.
func (q *BooleanQuery) RewriteTwoClauseDisjunctionWithTermsForCount(searcher *IndexSearcher) ([]Query, error) {
	builder := NewBooleanQueryBuilder()
	queries := make([]Query, 3)
	for i := 0; i < len(q.clauses); i++ {
		tq := q.clauses[i].Query().(*TermQuery)
		// Optimization will count term query several times so use cache to avoid multiple terms dictionary lookups
		if tq.GetTermStates() == nil {
			termStates, err := index.BuildTermStates(searcher, tq.GetTerm(), false)
			if err != nil {
				return nil, err
			}
			tq = NewTermQueryWithStates(tq.GetTerm(), termStates)
		}
		builder.Add(tq, MUST)
		queries[i] = tq
	}
	queries[2] = builder.Build()
	return queries, nil
}

// RewriteNoScoring is a utility method for rewriting BooleanQuery when scores
// are not needed. This is called from ConstantScoreQuery#rewrite.
func (q *BooleanQuery) RewriteNoScoring() *BooleanQuery {
	actuallyRewritten := false
	newQuery := NewBooleanQueryBuilder().SetMinimumNumberShouldMatch(q.GetMinimumNumberShouldMatch())

	keepShould := q.GetMinimumNumberShouldMatch() > 0 ||
		(q.clauseSets[MUST].size()+q.clauseSets[FILTER].size() == 0)

	for _, clause := range q.clauses {
		query := clause.Query()
		// NOTE: rewritingNoScoring() should not call rewrite(), otherwise this
		// method could run in exponential time with the depth of the query as
		// every new level would rewrite 2x more than its parent level.
		rewritten := query
		if bq, ok := rewritten.(*BoostQuery); ok {
			rewritten = bq.Query()
		}
		if csq, ok := rewritten.(*ConstantScoreQuery); ok {
			rewritten = csq.GetQuery()
		}
		if bq, ok := rewritten.(*BooleanQuery); ok {
			rewritten = bq.RewriteNoScoring()
		}
		occur := clause.Occur()
		if occur == SHOULD && !keepShould {
			// ignore clause
			actuallyRewritten = true
		} else if occur == MUST {
			// replace MUST clauses with FILTER clauses
			newQuery.Add(rewritten, FILTER)
			actuallyRewritten = true
		} else if query != rewritten {
			newQuery.Add(rewritten, occur)
			actuallyRewritten = true
		} else {
			newQuery.AddClause(clause)
		}
	}

	if !actuallyRewritten {
		return q
	}

	return newQuery.Build()
}

// CreateWeight builds the Weight for the given query.
func (q *BooleanQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return NewBooleanWeight(q, searcher, scoreMode, boost)
}

// Rewrite rewrites the query into primitive queries.
func (q *BooleanQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	if len(q.clauses) == 0 {
		return NewMatchNoDocsQuery("empty BooleanQuery"), nil
	}

	// Queries with no positive clauses have no matches
	if len(q.clauses) == q.clauseSets[MUST_NOT].size() {
		return NewMatchNoDocsQuery("pure negative BooleanQuery"), nil
	}

	// optimize 1-clause queries
	if len(q.clauses) == 1 {
		c := q.clauses[0]
		query := c.Query()
		if q.minimumNumberShouldMatch == 1 && c.Occur() == SHOULD {
			return query, nil
		} else if q.minimumNumberShouldMatch == 0 {
			switch c.Occur() {
			case SHOULD, MUST:
				return query, nil
			case FILTER:
				// no scoring clauses, so return a score of 0
				return NewBoostQuery(NewConstantScoreQuery(query), 0), nil
			default: // MUST_NOT
				panic(util.NewAssertionError(""))
			}
		}
	}

	// recursively rewrite
	{
		builder := NewBooleanQueryBuilder()
		builder.SetMinimumNumberShouldMatch(q.GetMinimumNumberShouldMatch())
		actuallyRewritten := false
		for _, clause := range q.clauses {
			query := clause.Query()
			occur := clause.Occur()
			var rewritten Query
			var err error
			if occur == FILTER || occur == MUST_NOT {
				// Clauses that are not involved in scoring can get some extra simplifications
				rewritten, err = NewConstantScoreQuery(query).Rewrite(searcher)
				if err != nil {
					return nil, err
				}
				if csq, ok := rewritten.(*ConstantScoreQuery); ok {
					rewritten = csq.GetQuery()
				}
			} else {
				rewritten, err = query.Rewrite(searcher)
				if err != nil {
					return nil, err
				}
			}
			if rewritten != query || isMatchNoDocs(query) {
				// rewrite clause
				actuallyRewritten = true
				if isMatchNoDocs(rewritten) {
					switch occur {
					case SHOULD, MUST_NOT:
						// the clause can be safely ignored
					case MUST, FILTER:
						return rewritten, nil
					}
				} else {
					builder.Add(rewritten, occur)
				}
			} else {
				// leave as-is
				builder.AddClause(clause)
			}
		}
		if actuallyRewritten {
			return builder.Build(), nil
		}
	}

	// remove duplicate FILTER and MUST_NOT clauses
	{
		clauseCount := 0
		for _, queries := range q.clauseSets {
			clauseCount += queries.size()
		}
		if clauseCount != len(q.clauses) {
			// since clauseSets implicitly deduplicates FILTER and MUST_NOT
			// clauses, this means there were duplicates
			rewritten := NewBooleanQueryBuilder()
			rewritten.SetMinimumNumberShouldMatch(q.minimumNumberShouldMatch)
			for occur, queries := range q.clauseSets {
				queries.forEach(func(query Query) {
					rewritten.Add(query, Occur(occur))
				})
			}
			return rewritten.Build(), nil
		}
	}

	// Check whether some clauses are both required and excluded
	mustNotClauses := q.clauseSets[MUST_NOT]
	if !mustNotClauses.isEmpty() {
		anyMatch := false
		mustNotClauses.forEach(func(query Query) {
			if q.clauseSets[MUST].contains(query) || q.clauseSets[FILTER].contains(query) {
				anyMatch = true
			}
		})
		if anyMatch {
			return NewMatchNoDocsQuery("FILTER or MUST clause also in MUST_NOT"), nil
		}
		if mustNotClauses.contains(Instance) {
			return NewMatchNoDocsQuery("MUST_NOT clause is MatchAllDocsQuery"), nil
		}
	}

	// remove FILTER clauses that are also MUST clauses or that match all documents
	if q.clauseSets[FILTER].size() > 0 {
		filters := newQueryHashSetFrom(q.clauseSets[FILTER])
		modified := false
		if filters.size() > 1 || !q.clauseSets[MUST].isEmpty() {
			modified = filters.remove(Instance)
		}
		modified = filters.removeAll(q.clauseSets[MUST]) || modified
		if modified {
			builder := NewBooleanQueryBuilder()
			builder.SetMinimumNumberShouldMatch(q.GetMinimumNumberShouldMatch())
			for _, clause := range q.clauses {
				if clause.Occur() != FILTER {
					builder.AddClause(clause)
				}
			}
			filters.forEach(func(filter Query) {
				builder.Add(filter, FILTER)
			})
			return builder.Build(), nil
		}
	}

	// convert FILTER clauses that are also SHOULD clauses to MUST clauses
	if q.clauseSets[SHOULD].size() > 0 && q.clauseSets[FILTER].size() > 0 {
		filters := q.clauseSets[FILTER]
		shoulds := q.clauseSets[SHOULD]

		intersection := newQueryHashSetFrom(filters)
		intersection.retainAll(shoulds)

		if !intersection.isEmpty() {
			builder := NewBooleanQueryBuilder()
			minShouldMatch := q.GetMinimumNumberShouldMatch()

			for _, clause := range q.clauses {
				if intersection.contains(clause.Query()) {
					if clause.Occur() == SHOULD {
						builder.AddClause(NewBooleanClause(clause.Query(), MUST))
						minShouldMatch--
					}
				} else {
					builder.AddClause(clause)
				}
			}

			builder.SetMinimumNumberShouldMatch(max(0, minShouldMatch))
			return builder.Build(), nil
		}
	}

	// Deduplicate SHOULD clauses by summing up their boosts
	if q.clauseSets[SHOULD].size() > 0 && q.minimumNumberShouldMatch <= 1 {
		shouldClauses := newQueryBoostMap()
		q.clauseSets[SHOULD].forEach(func(query Query) {
			boost := 1.0
			for {
				bq, ok := query.(*BoostQuery)
				if !ok {
					break
				}
				boost *= float64(bq.Boost())
				query = bq.Query()
			}
			shouldClauses.put(query, shouldClauses.getOrDefault(query, 0)+boost)
		})
		if shouldClauses.size() != q.clauseSets[SHOULD].size() {
			builder := NewBooleanQueryBuilder().SetMinimumNumberShouldMatch(q.minimumNumberShouldMatch)
			for i, query := range shouldClauses.keys {
				boost := float32(shouldClauses.values[i])
				if boost != 1 {
					query = NewBoostQuery(query, boost)
				}
				builder.Add(query, SHOULD)
			}
			for _, clause := range q.clauses {
				if clause.Occur() != SHOULD {
					builder.AddClause(clause)
				}
			}
			return builder.Build(), nil
		}
	}

	// Deduplicate MUST clauses by summing up their boosts
	if q.clauseSets[MUST].size() > 0 {
		mustClauses := newQueryBoostMap()
		q.clauseSets[MUST].forEach(func(query Query) {
			boost := 1.0
			for {
				bq, ok := query.(*BoostQuery)
				if !ok {
					break
				}
				boost *= float64(bq.Boost())
				query = bq.Query()
			}
			mustClauses.put(query, mustClauses.getOrDefault(query, 0)+boost)
		})
		if mustClauses.size() != q.clauseSets[MUST].size() {
			builder := NewBooleanQueryBuilder().SetMinimumNumberShouldMatch(q.minimumNumberShouldMatch)
			for i, query := range mustClauses.keys {
				boost := float32(mustClauses.values[i])
				if boost != 1 {
					query = NewBoostQuery(query, boost)
				}
				builder.Add(query, MUST)
			}
			for _, clause := range q.clauses {
				if clause.Occur() != MUST {
					builder.AddClause(clause)
				}
			}
			return builder.Build(), nil
		}
	}

	// Rewrite queries whose single scoring clause is a MUST clause on a
	// MatchAllDocsQuery to a ConstantScoreQuery
	{
		musts := q.clauseSets[MUST]
		filters := q.clauseSets[FILTER]
		if musts.size() == 1 && filters.size() > 0 {
			must := musts.first()
			boost := float32(1)
			if boostQuery, ok := must.(*BoostQuery); ok {
				must = boostQuery.Query()
				boost = boostQuery.Boost()
			}
			if _, ok := must.(*MatchAllDocsQuery); ok {
				// our single scoring clause matches everything: rewrite to a CSQ on the filter
				// ignore SHOULD clause for now
				builder := NewBooleanQueryBuilder()
				for _, clause := range q.clauses {
					switch clause.Occur() {
					case FILTER, MUST_NOT:
						builder.AddClause(clause)
					default:
						// ignore
					}
				}
				var rewritten Query = builder.Build()
				rewritten = NewConstantScoreQuery(rewritten)
				if boost != 1 {
					rewritten = NewBoostQuery(rewritten, boost)
				}

				// now add back the SHOULD clauses
				builder = NewBooleanQueryBuilder().
					SetMinimumNumberShouldMatch(q.GetMinimumNumberShouldMatch()).
					Add(rewritten, MUST)
				q.clauseSets[SHOULD].forEach(func(query Query) {
					builder.Add(query, SHOULD)
				})
				return builder.Build(), nil
			}
		}
	}

	// Flatten nested disjunctions, this is important for block-max WAND to perform well
	if q.minimumNumberShouldMatch <= 1 {
		builder := NewBooleanQueryBuilder()
		builder.SetMinimumNumberShouldMatch(q.minimumNumberShouldMatch)
		actuallyRewritten := false
		for _, clause := range q.clauses {
			if innerQuery, ok := clause.Query().(*BooleanQuery); ok && clause.Occur() == SHOULD {
				if innerQuery.IsPureDisjunction() {
					actuallyRewritten = true
					for _, innerClause := range innerQuery.Clauses() {
						builder.AddClause(innerClause)
					}
				} else {
					builder.AddClause(clause)
				}
			} else {
				builder.AddClause(clause)
			}
		}
		if actuallyRewritten {
			return builder.Build(), nil
		}
	}

	// Inline required / prohibited clauses. This helps run filtered conjunctive
	// queries more efficiently by providing all clauses to the block-max AND
	// scorer.
	{
		builder := NewBooleanQueryBuilder()
		builder.SetMinimumNumberShouldMatch(q.minimumNumberShouldMatch)
		actuallyRewritten := false
		for _, outerClause := range q.clauses {
			if innerQuery, ok := outerClause.Query().(*BooleanQuery); ok && outerClause.IsRequired() {
				// Inlining prohibited clauses is not legal if the query is a pure
				// negation, since pure negations have no matches. It works because
				// the inner BooleanQuery would have first rewritten to a
				// MatchNoDocsQuery if it only had prohibited clauses.
				if util.AssertsEnabled() && innerQuery.clauseSets[MUST_NOT].size() == len(innerQuery.clauses) {
					panic(util.NewAssertionError(""))
				}
				if innerQuery.GetMinimumNumberShouldMatch() == 0 && innerQuery.clauseSets[SHOULD].isEmpty() {
					actuallyRewritten = true
					for _, innerClause := range innerQuery.clauses {
						innerOccur := innerClause.Occur()
						if innerOccur == FILTER || innerOccur == MUST_NOT || outerClause.Occur() == MUST {
							builder.AddClause(innerClause)
						} else {
							if util.AssertsEnabled() && !(outerClause.Occur() == FILTER && innerOccur == MUST) {
								panic(util.NewAssertionError(""))
							}
							// In this case we need to change the occur of the inner query from MUST to FILTER.
							builder.Add(innerClause.Query(), FILTER)
						}
					}
				} else {
					builder.AddClause(outerClause)
				}
			} else {
				builder.AddClause(outerClause)
			}
		}
		if actuallyRewritten {
			return builder.Build(), nil
		}
	}

	// SHOULD clause count less than or equal to minimumNumberShouldMatch
	// Important(this can only be processed after nested clauses have been flattened)
	{
		shoulds := q.clauseSets[SHOULD]
		if shoulds.size() < q.minimumNumberShouldMatch {
			return NewMatchNoDocsQuery("SHOULD clause count less than minimumNumberShouldMatch"), nil
		}
		if shoulds.size() > 0 && shoulds.size() == q.minimumNumberShouldMatch {
			builder := NewBooleanQueryBuilder()
			for _, clause := range q.clauses {
				if clause.Occur() == SHOULD {
					builder.Add(clause.Query(), MUST)
				} else {
					builder.AddClause(clause)
				}
			}
			return builder.Build(), nil
		}
	}

	// Inline SHOULD clauses from the only MUST clause
	{
		if q.clauseSets[SHOULD].isEmpty() && q.clauseSets[MUST].size() == 1 {
			if inner, ok := q.clauseSets[MUST].first().(*BooleanQuery); ok &&
				len(inner.clauses) == inner.clauseSets[SHOULD].size() {
				rewritten := NewBooleanQueryBuilder()
				for _, clause := range q.clauses {
					if clause.Occur() != MUST {
						rewritten.AddClause(clause)
					}
				}
				for _, innerClause := range inner.Clauses() {
					rewritten.AddClause(innerClause)
				}
				rewritten.SetMinimumNumberShouldMatch(max(1, inner.GetMinimumNumberShouldMatch()))
				return rewritten.Build(), nil
			}
		}
	}

	// super.rewrite(indexSearcher): Query.rewrite returns this.
	return q, nil
}

func isMatchNoDocs(q Query) bool {
	_, ok := q.(*MatchNoDocsQuery)
	return ok
}

// contains renders java.util.Collection.contains(Object) over a clause list:
// membership is decided by Query.equals, not by object identity. The identity
// test is kept as the fast path, exactly as Java's AbstractCollection.contains
// reaches equals only after the reference comparison inside it.
func contains(slice []Query, q Query) bool {
	for _, item := range slice {
		if item == q || item.Equals(q) {
			return true
		}
	}
	return false
}

// Visit implements the Query visitor pattern.
func (q *BooleanQuery) Visit(visitor QueryVisitor) {
	sub := visitor.GetSubVisitor(MUST, q)
	// clauseSets.keySet() of the EnumMap: the Occur declaration order.
	for occur, queries := range q.clauseSets {
		if queries.size() > 0 {
			if Occur(occur) == MUST {
				queries.forEach(func(query Query) {
					query.Visit(sub)
				})
			} else {
				v := sub.GetSubVisitor(Occur(occur), q)
				queries.forEach(func(query Query) {
					query.Visit(v)
				})
			}
		}
	}
}

// ToString prints a user-readable version of this query.
func (q *BooleanQuery) ToString(field string) string {
	var buffer strings.Builder
	needParens := q.GetMinimumNumberShouldMatch() > 0
	if needParens {
		buffer.WriteString("(")
	}

	for i, c := range q.clauses {
		buffer.WriteString(c.Occur().String())

		subQuery := c.Query()
		if _, ok := subQuery.(*BooleanQuery); ok { // wrap sub-bools in parens
			buffer.WriteString("(")
			buffer.WriteString(queryToString(subQuery, field))
			buffer.WriteString(")")
		} else {
			buffer.WriteString(queryToString(subQuery, field))
		}

		if i != len(q.clauses)-1 {
			buffer.WriteString(" ")
		}
	}

	if needParens {
		buffer.WriteString(")")
	}

	if q.GetMinimumNumberShouldMatch() > 0 {
		buffer.WriteByte('~')
		buffer.WriteString(fmt.Sprintf("%d", q.GetMinimumNumberShouldMatch()))
	}

	return buffer.String()
}

// Equals compares the specified object with this boolean query for equality.
// Returns true if and only if the provided object
//   - is also a BooleanQuery,
//   - has the same value of GetMinimumNumberShouldMatch()
//   - has the same SHOULD clauses, regardless of the order
//   - has the same MUST clauses, regardless of the order
//   - has the same set of FILTER clauses, regardless of the order and
//     regardless of duplicates
//   - has the same set of MUST_NOT clauses, regardless of the order and
//     regardless of duplicates
func (q *BooleanQuery) Equals(other spi.Query) bool {
	o, ok := other.(*BooleanQuery)
	return ok && q.equalsTo(o)
}

func (q *BooleanQuery) equalsTo(other *BooleanQuery) bool {
	if q.GetMinimumNumberShouldMatch() != other.GetMinimumNumberShouldMatch() {
		return false
	}
	for occur := range q.clauseSets {
		if !q.clauseSets[occur].equals(other.clauseSets[occur]) {
			return false
		}
	}
	return true
}

// computeHashCode renders Objects.hash(minimumNumberShouldMatch, clauseSets).
// EnumMap.hashCode() sums key.hashCode() ^ value.hashCode(); Java's
// Enum.hashCode() is the JVM identity hash, which has no Go counterpart, so the
// key hash is rendered as the Occur ordinal. The result is order-independent
// and equal for equal queries, which is the contract Lucene relies on.
func (q *BooleanQuery) computeHashCode() int {
	clauseSetsHash := int32(0)
	for occur, queries := range q.clauseSets {
		clauseSetsHash += int32(occur) ^ queries.hashCode()
	}
	hashCode := 31*(31*1+int32(q.minimumNumberShouldMatch)) + clauseSetsHash
	if hashCode == 0 {
		hashCode = 1
	}
	return int(hashCode)
}

// HashCode returns a hash code for this query.
func (q *BooleanQuery) HashCode() int {
	// no need for synchronization, in the worst case we would just compute the hash several times.
	if q.hashCode == 0 {
		q.hashCode = q.computeHashCode()
		if util.AssertsEnabled() && q.hashCode == 0 {
			panic(util.NewAssertionError(""))
		}
	}
	if util.AssertsEnabled() && q.hashCode != q.computeHashCode() {
		panic(util.NewAssertionError(""))
	}
	return q.hashCode
}

// queryCollection renders the two Collection<Query> kinds of clauseSets:
// org.apache.lucene.search.Multiset<Query> (duplicates kept) and
// java.util.HashSet<Query> (duplicates dropped). Membership is decided by
// Query.equals/hashCode, as in Java. Elements are kept in insertion order:
// Java iterates both in hash order, which depends on JVM identity hashes of
// Query classes (Query.classHash) and is therefore not reproducible.
type queryCollection struct {
	multiset bool
	elements []Query // distinct elements, insertion order
	counts   []int   // occurrences of elements[i] (always 1 for a set)
	total    int
}

func newQueryMultiset() *queryCollection { return &queryCollection{multiset: true} }

func newQueryHashSet() *queryCollection { return &queryCollection{} }

// newQueryHashSetFrom renders new HashSet<>(collection).
func newQueryHashSetFrom(c *queryCollection) *queryCollection {
	s := newQueryHashSet()
	c.forEach(func(query Query) { s.add(query) })
	return s
}

func (c *queryCollection) indexOf(query Query) int {
	for i, e := range c.elements {
		if e == query || e.Equals(query) {
			return i
		}
	}
	return -1
}

func (c *queryCollection) add(query Query) bool {
	if i := c.indexOf(query); i >= 0 {
		if !c.multiset {
			return false
		}
		c.counts[i]++
		c.total++
		return true
	}
	c.elements = append(c.elements, query)
	c.counts = append(c.counts, 1)
	c.total++
	return true
}

// remove renders Collection.remove(Object) (one occurrence for a Multiset).
func (c *queryCollection) remove(query Query) bool {
	i := c.indexOf(query)
	if i < 0 {
		return false
	}
	c.total--
	if c.counts[i] > 1 {
		c.counts[i]--
		return true
	}
	c.elements = append(c.elements[:i], c.elements[i+1:]...)
	c.counts = append(c.counts[:i], c.counts[i+1:]...)
	return true
}

// removeAll renders HashSet.removeAll(Collection).
func (c *queryCollection) removeAll(other *queryCollection) bool {
	modified := false
	other.forEach(func(query Query) {
		for c.indexOf(query) >= 0 {
			c.remove(query)
			modified = true
		}
	})
	return modified
}

// retainAll renders HashSet.retainAll(Collection).
func (c *queryCollection) retainAll(other *queryCollection) bool {
	modified := false
	for i := 0; i < len(c.elements); {
		if other.contains(c.elements[i]) {
			i++
			continue
		}
		c.total -= c.counts[i]
		c.elements = append(c.elements[:i], c.elements[i+1:]...)
		c.counts = append(c.counts[:i], c.counts[i+1:]...)
		modified = true
	}
	return modified
}

func (c *queryCollection) contains(query Query) bool { return c.indexOf(query) >= 0 }

func (c *queryCollection) size() int { return c.total }

func (c *queryCollection) isEmpty() bool { return c.total == 0 }

// first renders collection.iterator().next().
func (c *queryCollection) first() Query { return c.elements[0] }

// forEach visits every element, each Multiset element as many times as it
// occurs.
func (c *queryCollection) forEach(fn func(Query)) {
	for i, e := range c.elements {
		for j := 0; j < c.counts[i]; j++ {
			fn(e)
		}
	}
}

func (c *queryCollection) toSlice() []Query {
	out := make([]Query, 0, c.total)
	c.forEach(func(query Query) { out = append(out, query) })
	return out
}

// equals renders Multiset.equals (same element counts) and HashSet.equals
// (same elements).
func (c *queryCollection) equals(other *queryCollection) bool {
	if c.multiset != other.multiset || c.total != other.total || len(c.elements) != len(other.elements) {
		return false
	}
	for i, e := range c.elements {
		j := other.indexOf(e)
		if j < 0 || other.counts[j] != c.counts[i] {
			return false
		}
	}
	return true
}

// hashCode renders Multiset.hashCode() (31 * classHash + map.hashCode(),
// map.hashCode() summing key.hashCode() ^ count) and HashSet.hashCode() (the
// sum of the element hash codes). The Multiset class hash is the JVM identity
// hash in Java; it is rendered as a constant.
func (c *queryCollection) hashCode() int32 {
	h := int32(0)
	for i, e := range c.elements {
		if c.multiset {
			h += int32(e.HashCode()) ^ int32(c.counts[i])
		} else {
			h += int32(e.HashCode())
		}
	}
	if c.multiset {
		classHash := queryMultisetClassHash
		h = 31*classHash + h
	}
	return h
}

// queryMultisetClassHash stands in for Multiset.class.hashCode().
const queryMultisetClassHash = int32(0x4d756c74) // "Mult"

// queryBoostMap renders the HashMap<Query, Double> used to deduplicate SHOULD
// and MUST clauses: keys are compared with Query.equals and kept in insertion
// order.
type queryBoostMap struct {
	keys   []Query
	values []float64
}

func newQueryBoostMap() *queryBoostMap { return &queryBoostMap{} }

func (m *queryBoostMap) indexOf(query Query) int {
	for i, k := range m.keys {
		if k == query || k.Equals(query) {
			return i
		}
	}
	return -1
}

func (m *queryBoostMap) getOrDefault(query Query, def float64) float64 {
	if i := m.indexOf(query); i >= 0 {
		return m.values[i]
	}
	return def
}

func (m *queryBoostMap) put(query Query, value float64) {
	if i := m.indexOf(query); i >= 0 {
		m.values[i] = value
		return
	}
	m.keys = append(m.keys, query)
	m.values = append(m.values, value)
}

func (m *queryBoostMap) size() int { return len(m.keys) }
