package vectorhighlight

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// maxMTQTerms is the maximum number of different matching terms accumulated
// from any one MultiTermQuery.
//
// Mirrors FieldQuery.MAX_MTQ_TERMS of Apache Lucene 10.5.0.
const maxMTQTerms = 1024

// QueryPhraseMap is the internal structure of a query for highlighting: it
// represents a nested query structure.
//
// Mirrors the nested class FieldQuery.QueryPhraseMap of Apache Lucene 10.5.0.
type QueryPhraseMap struct {
	Terminal bool
	// Slop is valid if Terminal == true and phraseHighlight == true.
	Slop int
	// Boost is valid if Terminal == true.
	Boost float32
	// TermOrPhraseNumber is valid if Terminal == true.
	TermOrPhraseNumber int
	fieldQuery         *FieldQuery
	SubMap             map[string]*QueryPhraseMap
}

// NewQueryPhraseMap creates a QueryPhraseMap owned by fq.
func NewQueryPhraseMap(fq *FieldQuery) *QueryPhraseMap {
	return &QueryPhraseMap{
		fieldQuery: fq,
		SubMap:     make(map[string]*QueryPhraseMap),
	}
}

func (qpm *QueryPhraseMap) addTerm(term *index.Term, boost float32) {
	m := qpm.getOrNewMap(qpm.SubMap, term.Text())
	m.markTerminalBoost(boost)
}

func (qpm *QueryPhraseMap) getOrNewMap(subMap map[string]*QueryPhraseMap, term string) *QueryPhraseMap {
	if mapVal, ok := subMap[term]; ok {
		return mapVal
	}
	newMap := NewQueryPhraseMap(qpm.fieldQuery)
	subMap[term] = newMap
	return newMap
}

// Add breaks query down and records it in this map.
//
// Mirrors QueryPhraseMap.add(Query, IndexReader) of Apache Lucene 10.5.0.
func (qpm *QueryPhraseMap) Add(query search.Query, reader index.IndexReader) {
	boost := float32(1.0)
	for {
		bq, ok := query.(*search.BoostQuery)
		if !ok {
			break
		}
		query = bq.Query()
		boost = bq.Boost()
	}

	switch q := query.(type) {
	case *search.TermQuery:
		qpm.addTerm(q.GetTerm(), boost)
	case *search.PhraseQuery:
		terms := q.GetTerms()
		subMap := qpm.SubMap
		var qpmCurrent *QueryPhraseMap
		for _, term := range terms {
			qpmCurrent = qpm.getOrNewMap(subMap, term.Text())
			subMap = qpmCurrent.SubMap
		}
		qpmCurrent.markTerminal(q.GetSlop(), boost)
	default:
		panic(fmt.Sprintf("query %q must be flatten first.", query))
	}
}

// GetTermMap returns the sub-map recorded under term.
func (qpm *QueryPhraseMap) GetTermMap(term string) *QueryPhraseMap {
	return qpm.SubMap[term]
}

func (qpm *QueryPhraseMap) markTerminalBoost(boost float32) {
	qpm.markTerminal(0, boost)
}

func (qpm *QueryPhraseMap) markTerminal(slop int, boost float32) {
	qpm.Terminal = true
	qpm.Slop = slop
	qpm.Boost = boost
	qpm.TermOrPhraseNumber = qpm.fieldQuery.nextTermOrPhraseNumber()
}

// IsTerminal reports whether this node terminates a term or a phrase.
func (qpm *QueryPhraseMap) IsTerminal() bool { return qpm.Terminal }

// GetSlop returns the slop recorded for a terminal node.
func (qpm *QueryPhraseMap) GetSlop() int { return qpm.Slop }

// GetBoost returns the boost recorded for a terminal node.
func (qpm *QueryPhraseMap) GetBoost() float32 { return qpm.Boost }

// GetTermOrPhraseNumber returns the colour-tag number of a terminal node.
func (qpm *QueryPhraseMap) GetTermOrPhraseNumber() int { return qpm.TermOrPhraseNumber }

// SearchPhrase walks phraseCandidate through the map and returns the terminal
// node it reaches, or nil.
func (qpm *QueryPhraseMap) SearchPhrase(phraseCandidate []*TermInfo) *QueryPhraseMap {
	currMap := qpm
	for _, ti := range phraseCandidate {
		currMap = currMap.SubMap[ti.Text]
		if currMap == nil {
			return nil
		}
	}
	if currMap.IsValidTermOrPhrase(phraseCandidate) {
		return currMap
	}
	return nil
}

// IsValidTermOrPhrase reports whether phraseCandidate is a valid term or a
// valid phrase against this terminal node.
func (qpm *QueryPhraseMap) IsValidTermOrPhrase(phraseCandidate []*TermInfo) bool {
	// check terminal
	if !qpm.Terminal {
		return false
	}

	// if the candidate is a term, it is valid
	if len(phraseCandidate) == 1 {
		return true
	}

	// else check whether the candidate is valid phrase
	// compare position-gaps between terms to slop
	pos := phraseCandidate[0].Position
	for i := 1; i < len(phraseCandidate); i++ {
		nextPos := phraseCandidate[i].Position
		diff := nextPos - pos - 1
		if diff < 0 {
			diff = -diff
		}
		if diff > qpm.Slop {
			return false
		}
		pos = nextPos
	}
	return true
}

// FieldQuery breaks down query object into terms/phrases and keeps them in a
// QueryPhraseMap structure.
//
// Mirrors org.apache.lucene.search.vectorhighlight.FieldQuery of Apache Lucene
// 10.5.0.
type FieldQuery struct {
	FieldMatch bool

	// RootMaps is Map<fieldName,QueryPhraseMap> when FieldMatch == true, and
	// Map<null,QueryPhraseMap> when FieldMatch == false. The Java null key is
	// rendered as the empty string.
	RootMaps map[string]*QueryPhraseMap

	// TermSetMap is Map<fieldName,setOfTermsInQueries> when FieldMatch ==
	// true, and Map<null,setOfTermsInQueries> when FieldMatch == false.
	TermSetMap map[string]map[string]struct{}

	// termOrPhraseNumber is used for coloured tag support.
	termOrPhraseNumber int
}

// NewFieldQuery creates a FieldQuery from query.
//
// Mirrors FieldQuery(Query, IndexReader, boolean, boolean) of Apache Lucene
// 10.5.0.
func NewFieldQuery(query search.Query, reader index.IndexReader, phraseHighlight bool, fieldMatch bool) (*FieldQuery, error) {
	fq := &FieldQuery{
		FieldMatch: fieldMatch,
		RootMaps:   make(map[string]*QueryPhraseMap),
		TermSetMap: make(map[string]map[string]struct{}),
	}

	var searcher *search.IndexSearcher
	if reader != nil {
		searcher = search.NewIndexSearcher(reader)
	}

	flatQueries := make([]search.Query, 0)
	if err := fq.flatten(query, searcher, &flatQueries, 1.0); err != nil {
		return nil, err
	}
	if err := fq.saveTerms(flatQueries, searcher); err != nil {
		return nil, err
	}
	expandQueries := fq.expand(flatQueries)

	for _, flatQuery := range expandQueries {
		rootMap := fq.getRootMap(flatQuery)
		rootMap.Add(flatQuery, reader)
		boost := float32(1.0)
		for {
			bq, ok := flatQuery.(*search.BoostQuery)
			if !ok {
				break
			}
			flatQuery = bq.Query()
			boost *= bq.Boost()
		}
		if !phraseHighlight {
			if pq, ok := flatQuery.(*search.PhraseQuery); ok {
				if len(pq.GetTerms()) > 1 {
					for _, term := range pq.GetTerms() {
						rootMap.addTerm(term, boost)
					}
				}
			}
		}
	}

	return fq, nil
}

// NewFieldQueryWithoutReader initializes a FieldQuery without an IndexReader,
// which is only required to support MultiTermQuery.
//
// Mirrors the package-private FieldQuery(Query, boolean, boolean) of Apache
// Lucene 10.5.0.
func NewFieldQueryWithoutReader(query search.Query, phraseHighlight bool, fieldMatch bool) (*FieldQuery, error) {
	return NewFieldQuery(query, nil, phraseHighlight, fieldMatch)
}

// containsQuery reproduces Set.contains for the LinkedHashSet<Query>
// collections Apache Lucene 10.5.0 uses in flatten/expand.
func containsQuery(queries []search.Query, query search.Query) bool {
	for _, q := range queries {
		if q.Equals(query) {
			return true
		}
	}
	return false
}

func (fq *FieldQuery) flatten(sourceQuery search.Query, searcher *search.IndexSearcher, flatQueries *[]search.Query, boost float32) error {
	for {
		bq, ok := sourceQuery.(*search.BoostQuery)
		if !ok {
			break
		}
		sourceQuery = bq.Query()
		boost *= bq.Boost()
	}

	switch q := sourceQuery.(type) {
	case *search.BooleanQuery:
		for _, clause := range q.Clauses() {
			if !clause.IsProhibited() {
				if err := fq.flatten(clause.Query(), searcher, flatQueries, boost); err != nil {
					return err
				}
			}
		}
	case *search.DisjunctionMaxQuery:
		for _, query := range q.Disjuncts() {
			if err := fq.flatten(query, searcher, flatQueries, boost); err != nil {
				return err
			}
		}
	case *search.TermQuery:
		if boost != 1.0 {
			sourceQuery = search.NewBoostQuery(sourceQuery, boost)
		}
		if !containsQuery(*flatQueries, sourceQuery) {
			*flatQueries = append(*flatQueries, sourceQuery)
		}
	case *search.SynonymQuery:
		for _, term := range q.GetTerms() {
			if err := fq.flatten(search.NewTermQuery(term), searcher, flatQueries, boost); err != nil {
				return err
			}
		}
	case *search.PhraseQuery:
		if len(q.GetTerms()) == 1 {
			sourceQuery = search.NewTermQuery(q.GetTerms()[0])
		}
		if boost != 1.0 {
			sourceQuery = search.NewBoostQuery(sourceQuery, boost)
		}
		if !containsQuery(*flatQueries, sourceQuery) {
			*flatQueries = append(*flatQueries, sourceQuery)
		}
	case *search.ConstantScoreQuery:
		if inner := q.GetQuery(); inner != nil {
			if err := fq.flatten(inner, searcher, flatQueries, boost); err != nil {
				return err
			}
		}
	case *function.FunctionScoreQuery:
		if inner := q.GetWrappedQuery(); inner != nil {
			if err := fq.flatten(inner, searcher, flatQueries, boost); err != nil {
				return err
			}
		}
	default:
		if searcher == nil {
			// else discard queries
			return nil
		}
		var rewritten search.Query
		var err error
		if mtq, ok := sourceQuery.(*search.MultiTermQuery); ok {
			rewritten, err = search.NewTopTermsScoringBooleanQueryRewrite(maxMTQTerms).Rewrite(searcher, mtq)
		} else {
			rewritten, err = sourceQuery.Rewrite(searcher)
		}
		if err != nil {
			return err
		}
		if rewritten != sourceQuery {
			// only rewrite once and then flatten again - the rewritten query
			// could have a speacial treatment if this method is overwritten in
			// a subclass.
			if err := fq.flatten(rewritten, searcher, flatQueries, boost); err != nil {
				return err
			}
		}
		// if the query is already rewritten we discard it
	}
	return nil
}

// expand creates expandQueries from flatQueries.
//
//	expandQueries := flatQueries + overlapped phrase queries
//
//	ex1) flatQueries={a,b,c}
//	     => expandQueries={a,b,c}
//	ex2) flatQueries={a,"b c","c d"}
//	     => expandQueries={a,"b c","c d","b c d"}
func (fq *FieldQuery) expand(flatQueries []search.Query) []search.Query {
	expandQueries := make([]search.Query, 0)
	remaining := make([]search.Query, len(flatQueries))
	copy(remaining, flatQueries)

	for len(remaining) > 0 {
		query := remaining[0]
		remaining = remaining[1:]
		if !containsQuery(expandQueries, query) {
			expandQueries = append(expandQueries, query)
		}
		queryBoost := float32(1.0)
		for {
			bq, ok := query.(*search.BoostQuery)
			if !ok {
				break
			}
			queryBoost *= bq.Boost()
			query = bq.Query()
		}
		pqA, ok := query.(*search.PhraseQuery)
		if !ok {
			continue
		}

		for _, qj := range remaining {
			qjBoost := float32(1.0)
			for {
				bq, ok := qj.(*search.BoostQuery)
				if !ok {
					break
				}
				qjBoost *= bq.Boost()
				qj = bq.Query()
			}
			pqB, ok := qj.(*search.PhraseQuery)
			if !ok {
				continue
			}
			fq.checkOverlap(&expandQueries, pqA, queryBoost, pqB, qjBoost)
		}
	}
	return expandQueries
}

// checkOverlap checks if PhraseQuery a and b have an overlapped part.
//
//	ex1) A="a b", B="b c" => overlap; expandQueries={"a b c"}
//	ex2) A="b c", B="a b" => overlap; expandQueries={"a b c"}
//	ex3) A="a b", B="c d" => no overlap; expandQueries={}
func (fq *FieldQuery) checkOverlap(expandQueries *[]search.Query, a *search.PhraseQuery, aBoost float32, b *search.PhraseQuery, bBoost float32) {
	if a.GetSlop() != b.GetSlop() {
		return
	}
	ats := a.GetTerms()
	bts := b.GetTerms()
	if fq.FieldMatch && ats[0].Field != bts[0].Field {
		return
	}
	fq.checkOverlapTerms(expandQueries, ats, bts, a.GetSlop(), aBoost)
	fq.checkOverlapTerms(expandQueries, bts, ats, b.GetSlop(), bBoost)
}

// checkOverlapTerms checks if src and dest have an overlapped part and, if so,
// creates PhraseQueries and adds them to expandQueries.
//
//	ex1) src="a b", dest="c d"       => no overlap
//	ex2) src="a b", dest="a b c"     => no overlap
//	ex3) src="a b", dest="b c"       => overlap; expandQueries={"a b c"}
//	ex4) src="a b c", dest="b c d"   => overlap; expandQueries={"a b c d"}
//	ex5) src="a b c", dest="b c"     => no overlap
//	ex6) src="a b c", dest="b"       => no overlap
//	ex7) src="a a a a", dest="a a a" => overlap;
//	                                    expandQueries={"a a a a a","a a a a a a"}
//	ex8) src="a b c d", dest="b c"   => no overlap
func (fq *FieldQuery) checkOverlapTerms(expandQueries *[]search.Query, src, dest []*index.Term, slop int, boost float32) {
	// beginning from 1 (not 0) is safe because that the PhraseQuery has
	// multiple terms is guaranteed in flatten() method (if PhraseQuery has only
	// one term, flatten() converts PhraseQuery to TermQuery)
	for i := 1; i < len(src); i++ {
		overlap := true
		for j := i; j < len(src); j++ {
			if (j-i) < len(dest) && src[j].Text() != dest[j-i].Text() {
				overlap = false
				break
			}
		}
		if overlap && len(src)-i < len(dest) {
			pqBuilder := search.NewPhraseQueryBuilder()
			for _, srcTerm := range src {
				pqBuilder.Add(srcTerm)
			}
			for k := len(src) - i; k < len(dest); k++ {
				pqBuilder.Add(index.NewTerm(src[0].Field, dest[k].Text()))
			}
			pqBuilder.SetSlop(slop)
			var pq search.Query = pqBuilder.Build()
			if boost != 1.0 {
				pq = search.NewBoostQuery(pq, 1.0)
			}
			if !containsQuery(*expandQueries, pq) {
				*expandQueries = append(*expandQueries, pq)
			}
		}
	}
}

func (fq *FieldQuery) getRootMap(query search.Query) *QueryPhraseMap {
	key := fq.getKey(query)
	if mapVal, ok := fq.RootMaps[key]; ok {
		return mapVal
	}
	newMap := NewQueryPhraseMap(fq)
	fq.RootMaps[key] = newMap
	return newMap
}

// getKey returns the 'key' string, which is the field name of the Query.
// If not FieldMatch, 'key' is the empty string, which renders Java's null key.
func (fq *FieldQuery) getKey(query search.Query) string {
	if !fq.FieldMatch {
		return ""
	}
	for {
		bq, ok := query.(*search.BoostQuery)
		if !ok {
			break
		}
		query = bq.Query()
	}
	switch q := query.(type) {
	case *search.TermQuery:
		return q.GetTerm().Field
	case *search.PhraseQuery:
		return q.GetTerms()[0].Field
	case *search.MultiTermQuery:
		return q.GetField()
	default:
		panic(fmt.Sprintf("query %q must be flatten first.", query))
	}
}

// saveTerms saves the set of terms in the queries to TermSetMap.
//
//	ex1) q=name:john
//	     - FieldMatch==true   TermSetMap=Map<"name",Set<"john">>
//	     - FieldMatch==false  TermSetMap=Map<null,Set<"john">>
//
//	ex2) q=name:john title:manager
//	     - FieldMatch==true   TermSetMap=Map<"name",Set<"john">,
//	                                         "title",Set<"manager">>
//	     - FieldMatch==false  TermSetMap=Map<null,Set<"john","manager">>
//
//	ex3) q=name:"john lennon"
//	     - FieldMatch==true   TermSetMap=Map<"name",Set<"john","lennon">>
//	     - FieldMatch==false  TermSetMap=Map<null,Set<"john","lennon">>
func (fq *FieldQuery) saveTerms(flatQueries []search.Query, searcher *search.IndexSearcher) error {
	for _, query := range flatQueries {
		for {
			bq, ok := query.(*search.BoostQuery)
			if !ok {
				break
			}
			query = bq.Query()
		}
		termSet := fq.getTermSetForQuery(query)
		switch q := query.(type) {
		case *search.TermQuery:
			termSet[q.GetTerm().Text()] = struct{}{}
		case *search.PhraseQuery:
			for _, term := range q.GetTerms() {
				termSet[term.Text()] = struct{}{}
			}
		default:
			mtq, ok := query.(*search.MultiTermQuery)
			if !ok || searcher == nil {
				panic(fmt.Sprintf("query %q must be flatten first.", query))
			}
			rewritten, err := mtq.Rewrite(searcher)
			if err != nil {
				return err
			}
			mtqTerms := rewritten.(*search.BooleanQuery)
			for _, clause := range mtqTerms.Clauses() {
				termSet[clause.Query().(*search.TermQuery).GetTerm().Text()] = struct{}{}
			}
		}
	}
	return nil
}

func (fq *FieldQuery) getTermSetForQuery(query search.Query) map[string]struct{} {
	key := fq.getKey(query)
	if set, ok := fq.TermSetMap[key]; ok {
		return set
	}
	set := make(map[string]struct{})
	fq.TermSetMap[key] = set
	return set
}

// GetTermSet returns the set of terms recorded for field.
func (fq *FieldQuery) GetTermSet(field string) map[string]struct{} {
	key := ""
	if fq.FieldMatch {
		key = field
	}
	return fq.TermSetMap[key]
}

// GetFieldTermMap returns the QueryPhraseMap recorded for term under fieldName.
func (fq *FieldQuery) GetFieldTermMap(fieldName, term string) *QueryPhraseMap {
	rootMap := fq.getRootMapForField(fieldName)
	if rootMap == nil {
		return nil
	}
	return rootMap.SubMap[term]
}

// SearchPhrase returns the QueryPhraseMap that phraseCandidate reaches under
// fieldName, or nil.
func (fq *FieldQuery) SearchPhrase(fieldName string, phraseCandidate []*TermInfo) *QueryPhraseMap {
	root := fq.getRootMapForField(fieldName)
	if root == nil {
		return nil
	}
	return root.SearchPhrase(phraseCandidate)
}

func (fq *FieldQuery) getRootMapForField(fieldName string) *QueryPhraseMap {
	key := ""
	if fq.FieldMatch {
		key = fieldName
	}
	return fq.RootMaps[key]
}

func (fq *FieldQuery) nextTermOrPhraseNumber() int {
	n := fq.termOrPhraseNumber
	fq.termOrPhraseNumber++
	return n
}
