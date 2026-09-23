// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DrillDownQuery is a Query for drill-down over facet categories. You should
// call Add for every group of categories you want to drill-down over.
//
// NOTE: if you choose to create your own Query by calling DrillDownQueryTerm,
// it is recommended to wrap it in a BoostQuery with a boost of 0.0, so that it
// does not affect the scores of the documents.
//
// Port of org.apache.lucene.facet.DrillDownQuery
// (lucene/facet/src/java/org/apache/lucene/facet/DrillDownQuery.java, Apache
// Lucene 10.5.0).
//
// @lucene.experimental
type DrillDownQuery struct {
	config             *FacetsConfig
	baseQuery          search.Query
	dimQueries         []*search.BooleanQueryBuilder
	drillDownDims      map[string]int // LinkedHashMap: insertion order is the value order
	drillDownDimsOrder []string
	builtDimQueries    []search.Query
	// dirtyDimQueryIndex renders the lucene.internal hppc IntHashSet of dirty
	// dimension indexes; only its membership is observable.
	dirtyDimQueryIndex map[int]struct{}
}

// DrillDownQueryTerm creates a drill-down term.
//
// Renders the static Term term(String field, String dim, String... path).
func DrillDownQueryTerm(field, dim string, path ...string) *index.Term {
	return index.NewTerm(field, PathToString(dim, path))
}

// newDrillDownQueryFrom renders the package-private constructor
// DrillDownQuery(FacetsConfig, Query baseQuery, List<BooleanQuery.Builder>
// dimQueries, Map<String, Integer> drillDownDims), used by clone() and
// DrillSideways.
func newDrillDownQueryFrom(config *FacetsConfig, baseQuery search.Query, dimQueries []*search.BooleanQueryBuilder,
	drillDownDims map[string]int, drillDownDimsOrder []string) *DrillDownQuery {
	q := &DrillDownQuery{
		baseQuery:          baseQuery,
		dimQueries:         append([]*search.BooleanQueryBuilder(nil), dimQueries...),
		drillDownDims:      make(map[string]int, len(drillDownDims)),
		drillDownDimsOrder: append([]string(nil), drillDownDimsOrder...),
		dirtyDimQueryIndex: map[int]struct{}{},
		config:             config,
	}
	for k, v := range drillDownDims {
		q.drillDownDims[k] = v
	}
	for i := 0; i < len(q.dimQueries); i++ {
		q.builtDimQueries = append(q.builtDimQueries, nil)
		q.dirtyDimQueryIndex[i] = struct{}{}
	}
	return q
}

// newDrillDownQueryWithFilter renders the package-private constructor
// DrillDownQuery(FacetsConfig config, Query filter, DrillDownQuery other),
// used by DrillSideways.
func newDrillDownQueryWithFilter(config *FacetsConfig, filter search.Query, other *DrillDownQuery) *DrillDownQuery {
	var base search.Query = search.Instance
	if other.baseQuery != nil {
		base = other.baseQuery
	}
	q := newDrillDownQueryFrom(config, nil, other.dimQueries, other.drillDownDims, other.drillDownDimsOrder)
	q.baseQuery = search.NewBooleanQueryBuilder().
		Add(base, search.MUST).
		Add(filter, search.FILTER).
		Build()
	return q
}

// NewDrillDownQuery creates a new DrillDownQuery without a base query, to
// perform a pure browsing query (equivalent to using MatchAllDocsQuery as
// base).
//
// Renders DrillDownQuery(FacetsConfig config).
func NewDrillDownQuery(config *FacetsConfig) *DrillDownQuery {
	return NewDrillDownQueryWithBaseQuery(config, nil)
}

// NewDrillDownQueryWithBaseQuery creates a new DrillDownQuery over the given
// base query. Can be nil, in which case the result Query from Rewrite will be
// a pure browsing query, filtering on the added categories only.
//
// Renders DrillDownQuery(FacetsConfig config, Query baseQuery).
func NewDrillDownQueryWithBaseQuery(config *FacetsConfig, baseQuery search.Query) *DrillDownQuery {
	return &DrillDownQuery{
		baseQuery:          baseQuery,
		config:             config,
		drillDownDims:      map[string]int{},
		dirtyDimQueryIndex: map[int]struct{}{},
	}
}

// Add adds one dimension of drill downs; if you pass the same dimension more
// than once it is OR'd with the previous constraints on that dimension, and
// all dimensions are AND'd against each other and the base query.
//
// Renders add(String dim, String... path).
func (q *DrillDownQuery) Add(dim string, path ...string) {
	indexedField := q.config.GetIndexFieldName(dim) // config.getDimConfig(dim).indexFieldName
	q.AddQuery(dim, search.NewTermQuery(DrillDownQueryTerm(indexedField, dim, path...)))
}

// AddQuery is the expert method to add a custom drill-down subQuery. Use this
// when you have a separate way to drill-down on the dimension than the
// indexed facet ordinals.
//
// Renders add(String dim, Query subQuery).
func (q *DrillDownQuery) AddQuery(dim string, subQuery search.Query) {
	if util.AssertsEnabled() && len(q.dimQueries) != len(q.builtDimQueries) {
		panic(util.NewAssertionError(""))
	}
	if util.AssertsEnabled() && len(q.drillDownDims) != len(q.dimQueries) {
		panic(util.NewAssertionError(""))
	}
	if _, ok := q.drillDownDims[dim]; !ok {
		q.drillDownDims[dim] = len(q.drillDownDims)
		q.drillDownDimsOrder = append(q.drillDownDimsOrder, dim)
		builder := search.NewBooleanQueryBuilder()
		q.dimQueries = append(q.dimQueries, builder)
		q.builtDimQueries = append(q.builtDimQueries, nil)
	}
	index := q.drillDownDims[dim]
	q.dimQueries[index].Add(subQuery, search.SHOULD)
	q.dirtyDimQueryIndex[index] = struct{}{}
}

// Clone renders clone().
func (q *DrillDownQuery) Clone() *DrillDownQuery {
	return newDrillDownQueryFrom(q.config, q.baseQuery, q.dimQueries, q.drillDownDims, q.drillDownDimsOrder)
}

// HashCode renders hashCode(): classHash() + Objects.hash(baseQuery,
// dimQueries). BooleanQuery.Builder has no hashCode override in Java, so the
// dim queries contribute their identity; Go renders that identity as the
// position-independent builder count, the only part of it that is stable.
func (q *DrillDownQuery) HashCode() int {
	h := int32(1)
	if q.baseQuery != nil {
		h = 31*h + int32(q.baseQuery.HashCode())
	} else {
		h = 31 * h
	}
	h = 31*h + int32(len(q.dimQueries))
	return int(javaStringHashCode(reflect.TypeOf(q).String()) + h)
}

// Equals renders equals(Object): sameClassAs(other) &&
// Objects.equals(baseQuery, other.baseQuery) &&
// dimQueries.equals(other.dimQueries). BooleanQuery.Builder does not override
// equals, so the dim query lists are equal when they hold the same builder
// instances in the same order.
func (q *DrillDownQuery) Equals(other spi.Query) bool {
	o, ok := other.(*DrillDownQuery)
	if !ok {
		return false
	}
	return q.equalsTo(o)
}

func (q *DrillDownQuery) equalsTo(other *DrillDownQuery) bool {
	if (q.baseQuery == nil) != (other.baseQuery == nil) {
		return false
	}
	if q.baseQuery != nil && !q.baseQuery.Equals(other.baseQuery) {
		return false
	}
	if len(q.dimQueries) != len(other.dimQueries) {
		return false
	}
	for i := range q.dimQueries {
		if q.dimQueries[i] != other.dimQueries[i] {
			return false
		}
	}
	return true
}

// Rewrite renders rewrite(IndexSearcher).
func (q *DrillDownQuery) Rewrite(indexSearcher *search.IndexSearcher) (search.Query, error) {
	rewritten := q.getBooleanQuery()
	if len(rewritten.Clauses()) == 0 {
		return search.Instance, nil
	}
	return rewritten, nil
}

// ToString renders toString(String field).
func (q *DrillDownQuery) ToString(field string) string {
	return q.getBooleanQuery().ToString(field)
}

// String renders toString().
func (q *DrillDownQuery) String() string {
	return q.ToString("")
}

// Visit renders visit(QueryVisitor): visitor.visitLeaf(this).
func (q *DrillDownQuery) Visit(visitor search.QueryVisitor) {
	visitor.VisitLeaf(q)
}

// CreateWeight renders the inherited Query.createWeight, which DrillDownQuery
// does not override: a DrillDownQuery always rewrites first.
func (q *DrillDownQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	return nil, fmt.Errorf("Query %s does not implement createWeight", q.String())
}

func (q *DrillDownQuery) getBooleanQuery() *search.BooleanQuery {
	bq := search.NewBooleanQueryBuilder()
	if q.baseQuery != nil {
		bq.Add(q.baseQuery, search.MUST)
	}
	for _, query := range q.GetDrillDownQueries() {
		bq.Add(query, search.FILTER)
	}

	return bq.Build()
}

// GetBaseQuery returns the internal baseQuery of the DrillDownQuery, the
// baseQuery used on initialization of DrillDownQuery.
func (q *DrillDownQuery) GetBaseQuery() search.Query {
	return q.baseQuery
}

// GetDrillDownQueries returns the dimension queries added either via AddQuery
// or Add.
func (q *DrillDownQuery) GetDrillDownQueries() []search.Query {
	dirty := make([]int, 0, len(q.dirtyDimQueryIndex))
	for i := range q.dirtyDimQueryIndex {
		dirty = append(dirty, i)
	}
	sort.Ints(dirty) // the rebuild order is not observable; sorting keeps it deterministic
	for _, dirtyDimIndex := range dirty {
		q.builtDimQueries[dirtyDimIndex] = q.dimQueries[dirtyDimIndex].Build()
	}
	clear(q.dirtyDimQueryIndex)

	return append([]search.Query(nil), q.builtDimQueries...)
}

// getDims renders the package-private Map<String, Integer> getDims().
func (q *DrillDownQuery) getDims() map[string]int {
	return q.drillDownDims
}

var _ search.Query = (*DrillDownQuery)(nil)

// javaStringHashCode renders java.lang.String#hashCode() over UTF-16 units;
// it stands in for Query.classHash() (getClass().getName().hashCode()).
func javaStringHashCode(s string) int32 {
	var h int32
	for _, r := range s {
		if r <= 0xFFFF {
			h = 31*h + int32(r)
			continue
		}
		r -= 0x10000
		h = 31*h + int32(0xD800+(r>>10))
		h = 31*h + int32(0xDC00+(r&0x3FF))
	}
	return h
}
