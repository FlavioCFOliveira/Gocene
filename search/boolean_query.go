// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"math"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// BooleanQuery matches documents matching boolean combinations of other queries.
type BooleanQuery struct {
	minimumNumberShouldMatch int
	clauses                  []*BooleanClause
	// clauseSets is used for equals and hashCode.
	// For SHOULD and MUST, duplicates are preserved. For FILTER and MUST_NOT, they are deduplicated.
	clauseSets map[Occur][]Query
	hashCode   int
}

// BooleanQueryBuilder is a builder for boolean queries.
type BooleanQueryBuilder struct {
	minimumNumberShouldMatch int
	clauses                  []*BooleanClause
}

// NewBooleanQueryBuilder creates a new BooleanQueryBuilder.
func NewBooleanQueryBuilder() *BooleanQueryBuilder {
	return &BooleanQueryBuilder{}
}

// SetMinimumNumberShouldMatch specifies a minimum number of the optional BooleanClauses which must be satisfied.
func (b *BooleanQueryBuilder) SetMinimumNumberShouldMatch(min int) *BooleanQueryBuilder {
	b.minimumNumberShouldMatch = min
	return b
}

// Add adds a new clause to this Builder.
func (b *BooleanQueryBuilder) Add(query Query, occur Occur) *BooleanQueryBuilder {
	return b.AddClause(NewBooleanClause(query, occur))
}

// AddClause adds a new clause to this Builder.
func (b *BooleanQueryBuilder) AddClause(clause *BooleanClause) *BooleanQueryBuilder {
	if len(b.clauses) >= GetMaxClauseCount() {
		panic(NewTooManyClauses())
	}
	b.clauses = append(b.clauses, clause)
	return b
}

// Build creates a new BooleanQuery based on the parameters that have been set on this builder.
func (b *BooleanQueryBuilder) Build() *BooleanQuery {
	bq := &BooleanQuery{
		minimumNumberShouldMatch: b.minimumNumberShouldMatch,
		clauses:                  append([]*BooleanClause(nil), b.clauses...),
		clauseSets:               make(map[Occur][]Query),
	}

	// Deduplicate FILTER and MUST_NOT
	filterSet := make(map[Query]struct{})
	mustNotSet := make(map[Query]struct{})

	for _, c := range b.clauses {
		occur := c.Occur()
		q := c.Query()
		switch occur {
		case SHOULD, MUST:
			bq.clauseSets[occur] = append(bq.clauseSets[occur], q)
		case FILTER:
			if _, exists := filterSet[q]; !exists {
				filterSet[q] = struct{}{}
				bq.clauseSets[occur] = append(bq.clauseSets[occur], q)
			}
		case MUST_NOT:
			if _, exists := mustNotSet[q]; !exists {
				mustNotSet[q] = struct{}{}
				bq.clauseSets[occur] = append(bq.clauseSets[occur], q)
			}
		}
	}

	return bq
}

// GetMinimumNumberShouldMatch returns the minimum number of the optional BooleanClauses which must be satisfied.
func (q *BooleanQuery) GetMinimumNumberShouldMatch() int {
	return q.minimumNumberShouldMatch
}

// Clauses returns a list of the clauses of this BooleanQuery.
func (q *BooleanQuery) Clauses() []*BooleanClause {
	return q.clauses
}

// GetClauses returns the collection of queries for the given Occur.
func (q *BooleanQuery) GetClauses(occur Occur) []Query {
	return q.clauseSets[occur]
}

// IsPureDisjunction returns whether this query is a pure disjunction.
func (q *BooleanQuery) IsPureDisjunction() bool {
	return len(q.clauses) == len(q.GetClauses(SHOULD)) && q.minimumNumberShouldMatch <= 1
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

// RewriteNoScoring is a utility method for rewriting BooleanQuery when scores are not needed.
func (q *BooleanQuery) RewriteNoScoring() *BooleanQuery {
	actuallyRewritten := false
	builder := NewBooleanQueryBuilder().SetMinimumNumberShouldMatch(q.GetMinimumNumberShouldMatch())

	musts := q.GetClauses(MUST)
	filters := q.GetClauses(FILTER)
	keepShould := q.GetMinimumNumberShouldMatch() > 0 || (len(musts)+len(filters) == 0)

	for _, clause := range q.clauses {
		query := clause.Query()
		rewritten := query

		for {
			if bq, ok := rewritten.(*BoostQuery); ok {
				rewritten = bq.Query()
			} else if csq, ok := rewritten.(*ConstantScoreQuery); ok {
				rewritten = csq.GetQuery()
			} else if bq, ok := rewritten.(*BooleanQuery); ok {
				rewritten = bq.RewriteNoScoring()
			} else {
				break
			}
		}

		occur := clause.Occur()
		if occur == SHOULD && !keepShould {
			actuallyRewritten = true
		} else if occur == MUST {
			builder.Add(rewritten, FILTER)
			actuallyRewritten = true
		} else if query != rewritten {
			builder.Add(rewritten, occur)
			actuallyRewritten = true
		} else {
			builder.AddClause(clause)
		}
	}

	if !actuallyRewritten {
		return q
	}

	return builder.Build()
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

	if len(q.clauses) == len(q.GetClauses(MUST_NOT)) {
		return NewMatchNoDocsQuery("pure negative BooleanQuery"), nil
	}

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
				return NewBoostQuery(NewConstantScoreQuery(query), 0), nil
			case MUST_NOT:
				return NewMatchNoDocsQuery("single MUST_NOT clause"), nil
			}
		}
	}

	// recursively rewrite
	{
		builder := NewBooleanQueryBuilder().SetMinimumNumberShouldMatch(q.GetMinimumNumberShouldMatch())
		actuallyRewritten := false
		for _, clause := range q.clauses {
			query := clause.Query()
			occur := clause.Occur()
			var rewritten Query
			var err error

			if occur == FILTER || occur == MUST_NOT {
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

			if rewritten != query || isMatchNoDocs(rewritten) {
				actuallyRewritten = true
				if isMatchNoDocs(rewritten) {
					switch occur {
					case SHOULD, MUST_NOT:
						// ignore
					case MUST, FILTER:
						return rewritten, nil
					}
				} else {
					builder.Add(rewritten, occur)
				}
			} else {
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
			clauseCount += len(queries)
		}
		if clauseCount != len(q.clauses) {
			builder := NewBooleanQueryBuilder().SetMinimumNumberShouldMatch(q.minimumNumberShouldMatch)
			for occur, queries := range q.clauseSets {
				for _, query := range queries {
					builder.Add(query, occur)
				}
			}
			return builder.Build(), nil
		}
	}

	// Check whether some clauses are both required and excluded
	mustNotClauses := q.GetClauses(MUST_NOT)
	if len(mustNotClauses) > 0 {
		musts := q.GetClauses(MUST)
		filters := q.GetClauses(FILTER)
		for _, mNot := range mustNotClauses {
			if contains(musts, mNot) || contains(filters, mNot) {
				return NewMatchNoDocsQuery("FILTER or MUST clause also in MUST_NOT"), nil
			}
		}
		if contains(mustNotClauses, Instance) {
			return NewMatchNoDocsQuery("MUST_NOT clause is MatchAllDocsQuery"), nil
		}
	}

	// remove FILTER clauses that are also MUST clauses or that match all documents
	if len(q.GetClauses(FILTER)) > 0 {
		filters := make(map[Query]struct{})
		for _, f := range q.GetClauses(FILTER) {
			filters[f] = struct{}{}
		}
		modified := false
		if len(filters) > 1 || len(q.GetClauses(MUST)) > 0 {
			modified = removeQueryFromSet(filters, Instance)
		}
		musts := q.GetClauses(MUST)
		for _, m := range musts {
			if _, exists := filters[m]; exists {
				delete(filters, m)
				modified = true
			}
		}
		if modified {
			builder := NewBooleanQueryBuilder().SetMinimumNumberShouldMatch(q.GetMinimumNumberShouldMatch())
			for _, clause := range q.clauses {
				if clause.Occur() != FILTER {
					builder.AddClause(clause)
				}
			}
			for f := range filters {
				builder.Add(f, FILTER)
			}
			return builder.Build(), nil
		}
	}

	// convert FILTER clauses that are also SHOULD clauses to MUST clauses
	if len(q.GetClauses(SHOULD)) > 0 && len(q.GetClauses(FILTER)) > 0 {
		filters := q.GetClauses(FILTER)
		shoulds := q.GetClauses(SHOULD)
		intersection := make(map[Query]struct{})
		for _, f := range filters {
			if contains(shoulds, f) {
				intersection[f] = struct{}{}
			}
		}
		if len(intersection) > 0 {
			builder := NewBooleanQueryBuilder()
			minShouldMatch := q.GetMinimumNumberShouldMatch()
			for _, clause := range q.clauses {
				if _, inInt := intersection[clause.Query()]; inInt {
					if clause.Occur() == SHOULD {
						builder.Add(clause.Query(), MUST)
						minShouldMatch--
					} else {
						builder.AddClause(clause)
					}
				} else {
					builder.AddClause(clause)
				}
			}
			builder.SetMinimumNumberShouldMatch(int(math.Max(0, float64(minShouldMatch))))
			return builder.Build(), nil
		}
	}

	// Deduplicate SHOULD clauses by summing up their boosts
	if len(q.GetClauses(SHOULD)) > 0 && q.minimumNumberShouldMatch <= 1 {
		shouldClauses := make(map[Query]float64)
		for _, query := range q.GetClauses(SHOULD) {
			boost := 1.0
			curr := query
			for {
				if bq, ok := curr.(*BoostQuery); ok {
					boost *= float64(bq.Boost())
					curr = bq.Query()
				} else {
					break
				}
			}
			shouldClauses[curr] += boost
		}
		if len(shouldClauses) != len(q.GetClauses(SHOULD)) {
			builder := NewBooleanQueryBuilder().SetMinimumNumberShouldMatch(q.minimumNumberShouldMatch)
			for query, boost := range shouldClauses {
				var finalQuery Query = query
				if boost != 1.0 {
					finalQuery = NewBoostQuery(query, float32(boost))
				}
				builder.Add(finalQuery, SHOULD)
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
	if len(q.GetClauses(MUST)) > 0 {
		mustClauses := make(map[Query]float64)
		for _, query := range q.GetClauses(MUST) {
			boost := 1.0
			curr := query
			for {
				if bq, ok := curr.(*BoostQuery); ok {
					boost *= float64(bq.Boost())
					curr = bq.Query()
				} else {
					break
				}
			}
			mustClauses[curr] += boost
		}
		if len(mustClauses) != len(q.GetClauses(MUST)) {
			builder := NewBooleanQueryBuilder().SetMinimumNumberShouldMatch(q.minimumNumberShouldMatch)
			for query, boost := range mustClauses {
				var finalQuery Query = query
				if boost != 1.0 {
					finalQuery = NewBoostQuery(query, float32(boost))
				}
				builder.Add(finalQuery, MUST)
			}
			for _, clause := range q.clauses {
				if clause.Occur() != MUST {
					builder.AddClause(clause)
				}
			}
			return builder.Build(), nil
		}
	}

	// Rewrite queries whose single scoring clause is a MUST clause on a MatchAllDocsQuery to a ConstantScoreQuery
	{
		musts := q.GetClauses(MUST)
		filters := q.GetClauses(FILTER)
		if len(musts) == 1 && len(filters) > 0 {
			must := musts[0]
			boost := float32(1.0)
			curr := must
			if bq, ok := curr.(*BoostQuery); ok {
				curr = bq.Query()
				boost = bq.Boost()
			}
			if _, isMatchAllDocs := curr.(*MatchAllDocsQuery); isMatchAllDocs {
				builder := NewBooleanQueryBuilder()
				for _, clause := range q.clauses {
					switch clause.Occur() {
					case FILTER, MUST_NOT:
						builder.AddClause(clause)
					}
				}
				var rewritten Query = builder.Build()
				rewritten = NewConstantScoreQuery(rewritten)
				if boost != 1.0 {
					rewritten = NewBoostQuery(rewritten, boost)
				}

				builder = NewBooleanQueryBuilder().
					SetMinimumNumberShouldMatch(q.GetMinimumNumberShouldMatch()).
					Add(rewritten, MUST)
				for _, query := range q.GetClauses(SHOULD) {
					builder.Add(query, SHOULD)
				}
				return builder.Build(), nil
			}
		}
	}

	// Flatten nested disjunctions
	if q.minimumNumberShouldMatch <= 1 {
		builder := NewBooleanQueryBuilder().SetMinimumNumberShouldMatch(q.minimumNumberShouldMatch)
		actuallyRewritten := false
		for _, clause := range q.clauses {
			if clause.Occur() == SHOULD {
				if inner, ok := clause.Query().(*BooleanQuery); ok && inner.IsPureDisjunction() {
					actuallyRewritten = true
					for _, innerClause := range inner.Clauses() {
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

	// Inline required / prohibited clauses
	{
		builder := NewBooleanQueryBuilder().SetMinimumNumberShouldMatch(q.minimumNumberShouldMatch)
		actuallyRewritten := false
		for _, outerClause := range q.clauses {
			if outerClause.IsRequired() {
				if inner, ok := outerClause.Query().(*BooleanQuery); ok {
					if inner.GetMinimumNumberShouldMatch() == 0 && len(inner.GetClauses(SHOULD)) == 0 {
						actuallyRewritten = true
						for _, innerClause := range inner.Clauses() {
							innerOccur := innerClause.Occur()
							if innerOccur == FILTER || innerOccur == MUST_NOT || outerClause.Occur() == MUST {
								builder.AddClause(innerClause)
							} else {
								builder.Add(innerClause.Query(), FILTER)
							}
						}
					} else {
						builder.AddClause(outerClause)
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
	{
		shoulds := q.GetClauses(SHOULD)
		if len(shoulds) < q.minimumNumberShouldMatch {
			return NewMatchNoDocsQuery("SHOULD clause count less than minimumNumberShouldMatch"), nil
		}
		if len(shoulds) > 0 && len(shoulds) == q.minimumNumberShouldMatch {
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
		if len(q.GetClauses(SHOULD)) == 0 &&
			len(q.GetClauses(MUST)) == 1 &&
			isPureDisjunctionBooleanQuery(q.GetClauses(MUST)[0]) {
			inner := q.GetClauses(MUST)[0].(*BooleanQuery)
			rewritten := NewBooleanQueryBuilder()
			for _, clause := range q.clauses {
				if clause.Occur() != MUST {
					rewritten.AddClause(clause)
				}
			}
			for _, innerClause := range inner.Clauses() {
				rewritten.AddClause(innerClause)
			}
			rewritten.SetMinimumNumberShouldMatch(int(math.Max(1, float64(inner.GetMinimumNumberShouldMatch()))))
			return rewritten.Build(), nil
		}
	}

	return q.RewriteGeneric(searcher)
}

func isPureDisjunctionBooleanQuery(q Query) bool {
	if bq, ok := q.(*BooleanQuery); ok {
		return bq.IsPureDisjunction()
	}
	return false
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

// removeQueryFromSet renders java.util.Set.remove(Object) over a Go set of
// queries: the entry equal to q is removed and true is reported when the set
// changed. Go map lookup compares interface values by identity, so the set is
// scanned with Query.equals instead.
func removeQueryFromSet(set map[Query]struct{}, q Query) bool {
	for item := range set {
		if item == q || item.Equals(q) {
			delete(set, item)
			return true
		}
	}
	return false
}

// Visit implements the Query visitor pattern.
func (q *BooleanQuery) Visit(visitor QueryVisitor) {
	sub := visitor.GetSubVisitor(MUST, q)
	for occur, queries := range q.clauseSets {
		if len(queries) > 0 {
			if occur == MUST {
				for _, query := range queries {
					query.Visit(sub)
				}
			} else {
				v := visitor.GetSubVisitor(occur, q)
				for _, query := range queries {
					query.Visit(v)
				}
			}
		}
	}
}

// ToString returns a user-readable version of this query.
func (q *BooleanQuery) ToString(field string) string {
	var sb strings.Builder
	needParens := q.GetMinimumNumberShouldMatch() > 0
	if needParens {
		sb.WriteString("(")
	}

	for i, c := range q.clauses {
		sb.WriteString(c.Occur().String())

		subQuery := c.Query()
		if bq, ok := subQuery.(*BooleanQuery); ok {
			sb.WriteString("(")
			sb.WriteString(bq.ToString(field))
			sb.WriteString(")")
		} else {
			sb.WriteString(queryToString(subQuery, field))
		}

		if i != len(q.clauses)-1 {
			sb.WriteString(" ")
		}
	}

	if needParens {
		sb.WriteString(")")
	}

	if q.GetMinimumNumberShouldMatch() > 0 {
		sb.WriteByte('~')
		sb.WriteString(fmt.Sprintf("%d", q.GetMinimumNumberShouldMatch()))
	}

	return sb.String()
}

// Equals checks if this query equals another.
func (q *BooleanQuery) Equals(other spi.Query) bool {
	otherQuery, ok := other.(*BooleanQuery)
	if !ok {
		return false
	}
	if q.minimumNumberShouldMatch != otherQuery.minimumNumberShouldMatch {
		return false
	}

	// Compare clause sets
	for occur, queries := range q.clauseSets {
		otherQueries := otherQuery.clauseSets[occur]
		if len(queries) != len(otherQueries) {
			return false
		}
		// For SHOULD and MUST, order doesn't matter but duplicates do.
		// For FILTER and MUST_NOT, they are already deduplicated.
		counts := make(map[Query]int)
		for _, query := range queries {
			counts[query]++
		}
		for _, query := range otherQueries {
			counts[query]--
		}
		for _, count := range counts {
			if count != 0 {
				return false
			}
		}
	}
	return true
}

// HashCode returns a hash code for this query.
func (q *BooleanQuery) HashCode() int {
	if q.hashCode != 0 {
		return q.hashCode
	}

	h := 17
	h = 31*h + q.minimumNumberShouldMatch
	// Hash the clause sets (simplified)
	for occur, queries := range q.clauseSets {
		h = 31*h + int(occur)
		for _, query := range queries {
			h = 31*h + query.HashCode()
		}
	}

	if h == 0 {
		h = 1
	}
	q.hashCode = h
	return q.hashCode
}

// RewriteGeneric is a placeholder for the super.rewrite(indexSearcher) call.
func (q *BooleanQuery) RewriteGeneric(searcher *IndexSearcher) (Query, error) {
	return q, nil
}
