package vectorhighlight

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

const maxMTQTerms = 1024

// QueryPhraseMap represents a nested query structure for highlighting.
type QueryPhraseMap struct {
	Terminal            bool
	Slop                int
	Boost               float32
	TermOrPhraseNumber   int
	fieldQuery          *FieldQuery
	SubMap              map[string]*QueryPhraseMap
}

func NewQueryPhraseMap(fq *FieldQuery) *QueryPhraseMap {
	return &QueryPhraseMap{
		fieldQuery: fq,
		SubMap:     make(map[string]*QueryPhraseMap),
	}
}

func (qpm *QueryPhraseMap) addTerm(term string, boost float32) {
	qpm.markTerminal(0, boost)
}

func (qpm *QueryPhraseMap) getOrNewMap(subMap map[string]*QueryPhraseMap, term string) *QueryPhraseMap {
	if mapVal, ok := subMap[term]; ok {
		return mapVal
	}
	newMap := NewQueryPhraseMap(qpm.fieldQuery)
	subMap[term] = newMap
	return newMap
}

func (qpm *QueryPhraseMap) Add(query search.Query, reader index.IndexReader) {
	boost := float32(1.0)
	for {
		if bq, ok := query.(*search.BoostQuery); ok {
			query = bq.Query()
			boost = bq.Boost()
		} else {
			break
		}
	}

	if tq, ok := query.(*search.TermQuery); ok {
		qpm.addTerm(tq.Term().Text(), boost)
	} else if pq, ok := query.(*search.PhraseQuery); ok {
		terms := pq.Terms()
		subMap := qpm.SubMap
		var qpmCurrent *QueryPhraseMap
		for _, term := range terms {
			qpmCurrent = qpm.getOrNewMap(subMap, term.Text())
			subMap = qpmCurrent.SubMap
		}
		qpmCurrent.markTerminal(pq.Slop(), boost)
	} else {
		panic(fmt.Sprintf("query %v must be flattened first", query))
	}
}

func (qpm *QueryPhraseMap) GetTermMap(term string) *QueryPhraseMap {
	return qpm.SubMap[term]
}

func (qpm *QueryPhraseMap) markTerminal(slop int, boost float32) {
	qpm.Terminal = true
	qpm.Slop = slop
	qpm.Boost = boost
	qpm.TermOrPhraseNumber = qpm.fieldQuery.nextTermOrPhraseNumber()
}

func (qpm *QueryPhraseMap) SearchPhrase(phraseCandidate []*TermInfo) *QueryPhraseMap {
	currMap := qpm
	for _, ti := range phraseCandidate {
		currMap = currMap.SubMap[ti.Text]
		if currMap == nil {
			return nil
		}
	}
	if currMap.isValidTermOrPhrase(phraseCandidate) {
		return currMap
	}
	return nil
}

func (qpm *QueryPhraseMap) isValidTermOrPhrase(phraseCandidate []*TermInfo) bool {
	if !qpm.Terminal {
		return false
	}

	if len(phraseCandidate) == 1 {
		return true
	}

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

// FieldQuery breaks down query object into terms/phrases and keeps them in a QueryPhraseMap structure.
type FieldQuery struct {
	FieldMatch         bool
	RootMaps           map[string]*QueryPhraseMap
	TermSetMap         map[string]map[string]struct{}
	termOrPhraseNumber int
}

func NewFieldQuery(query search.Query, reader index.IndexReader, phraseHighlight bool, fieldMatch bool) (*FieldQuery, error) {
	fq := &FieldQuery{
		FieldMatch: fieldMatch,
		RootMaps:   make(map[string]*QueryPhraseMap),
		TermSetMap: make(map[string]map[string]struct{}),
	}

	var searcher search.IndexSearcher
	if reader != nil {
		searcher = search.NewIndexSearcher(reader)
	}

	flatQueries := make([]search.Query, 0)
	fq.flatten(query, searcher, &flatQueries, 1.0)
	fq.saveTerms(flatQueries, searcher)
	expandQueries := fq.expand(flatQueries)

	for _, flatQuery := range expandQueries {
		rootMap := fq.getRootMap(flatQuery)
		rootMap.Add(flatQuery, reader)
		boost := float32(1.0)
		q := flatQuery
		for {
			if bq, ok := q.(*search.BoostQuery); ok {
				q = bq.Query()
				boost *= bq.Boost()
			} else {
				break
			}
		}
		if !phraseHighlight {
			if pq, ok := q.(*search.PhraseQuery); ok {
				if len(pq.Terms()) > 1 {
					for _, term := range pq.Terms() {
						rootMap.addTerm(term.Text(), boost)
					}
				}
			}
		}
	}

	return fq, nil
}

func (fq *FieldQuery) flatten(sourceQuery search.Query, searcher search.IndexSearcher, flatQueries *[]search.Query, boost float32) {
	q := sourceQuery
	for {
		if bq, ok := q.(*search.BoostQuery); ok {
			q = bq.Query()
			boost *= bq.Boost()
		} else {
			break
		}
	}

	if bq, ok := q.(*search.BooleanQuery); ok {
		for _, clause := range bq.Clauses() {
			if !clause.IsProhibited() {
				fq.flatten(clause.Query(), searcher, flatQueries, boost)
			}
		}
	} else if dmq, ok := q.(*search.DisjunctionMaxQuery); ok {
		for _, query := range dmq.Queries() {
			fq.flatten(query, searcher, flatQueries, boost)
		}
	} else if _, ok := q.(*search.TermQuery); ok {
		if boost != 1.0 {
			q = search.NewBoostQuery(q, boost)
		}
		*flatQueries = append(*flatQueries, q)
	} else if sq, ok := q.(*search.SynonymQuery); ok {
		for _, term := range sq.Terms() {
			fq.flatten(search.NewTermQuery(term), searcher, flatQueries, boost)
		}
	} else if pq, ok := q.(*search.PhraseQuery); ok {
		if len(pq.Terms()) == 1 {
			q = search.NewTermQuery(pq.Terms()[0])
		}
		if boost != 1.0 {
			q = search.NewBoostQuery(q, boost)
		}
		*flatQueries = append(*flatQueries, q)
	} else if csq, ok := q.(*search.ConstantScoreQuery); ok {
		if qInner := csq.Query(); qInner != nil {
			fq.flatten(qInner, searcher, flatQueries, boost)
		}
	} else if fsq, ok := q.(*search.FunctionScoreQuery); ok {
		if qInner := fsq.WrappedQuery(); qInner != nil {
			fq.flatten(qInner, searcher, flatQueries, boost)
		}
	} else if searcher != nil {
		var rewritten search.Query
		if mtq, ok := q.(*search.MultiTermQuery); ok {
			// In Gocene, we need to implement the TopTermsScoringBooleanQueryRewrite logic
			// For now, we'll use the standard rewrite
			rewritten = mtq.Rewrite(searcher)
		} else {
			rewritten = q.Rewrite(searcher)
		}
		if rewritten != q {
			fq.flatten(rewritten, searcher, flatQueries, boost)
		}
	}
}

func (fq *FieldQuery) expand(flatQueries []search.Query) []search.Query {
	expandQueries := make([]search.Query, 0)
	tempFlat := make([]search.Query, len(flatQueries))
	copy(tempFlat, flatQueries)

	for len(tempFlat) > 0 {
		query := tempFlat[0]
		tempFlat = tempFlat[1:]
		expandQueries = append(expandQueries, query)

		q := query
		queryBoost := float32(1.0)
		for {
			if bq, ok := q.(*search.BoostQuery); ok {
				q = bq.Query()
				queryBoost *= bq.Boost()
			} else {
				break
			}
		}
		if _, ok := q.(*search.PhraseQuery); !ok {
			continue
		}
		pqA := q.(*search.PhraseQuery)

		for i := 0; i < len(tempFlat); i++ {
			qj := tempFlat[i]
			qjInner := qj
			qjBoost := float32(1.0)
			for {
				if bq, ok := qjInner.(*search.BoostQuery); ok {
					qjInner = bq.Query()
					qjBoost *= bq.Boost()
				} else {
					break
				}
			}
			if pqB, ok := qjInner.(*search.PhraseQuery); ok {
				fq.checkOverlap(&expandQueries, pqA, queryBoost, pqB, qjBoost)
			}
		}
	}
	return expandQueries
}

func (fq *FieldQuery) checkOverlap(expandQueries *[]search.Query, a *search.PhraseQuery, aBoost float32, b *search.PhraseQuery, bBoost float32) {
	if a.Slop() != b.Slop() {
		return
	}
	ats := a.Terms()
	bts := b.Terms()
	if fq.FieldMatch && ats[0].Field() != bts[0].Field() {
		return
	}
	fq.checkOverlapTerms(expandQueries, ats, bts, a.Slop(), aBoost)
	fq.checkOverlapTerms(expandQueries, bts, ats, b.Slop(), bBoost)
}

func (fq *FieldQuery) checkOverlapTerms(expandQueries *[]search.Query, src, dest []index.Term, slop int, boost float32) {
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
				pqBuilder.Add(index.NewTerm(src[0].Field(), dest[k].Text()))
			}
			pqBuilder.SetSlop(slop)
			pq := pqBuilder.Build()
			if boost != 1.0 {
				pq = search.NewBoostQuery(pq, boost)
			}
			*expandQueries = append(*expandQueries, pq)
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

func (fq *FieldQuery) getKey(query search.Query) string {
	q := query
	for {
		if bq, ok := q.(*search.BoostQuery); ok {
			q = bq.Query()
		} else {
			break
		}
	}
	if tq, ok := q.(*search.TermQuery); ok {
		return tq.Term().Field()
	} else if pq, ok := q.(*search.PhraseQuery); ok {
		return pq.Terms()[0].Field()
	} else if mtq, ok := q.(*search.MultiTermQuery); ok {
		return mtq.Field()
	}
	panic(fmt.Sprintf("query %v must be flattened first", query))
}

func (fq *FieldQuery) saveTerms(flatQueries []search.Query, searcher search.IndexSearcher) {
	for _, query := range flatQueries {
		q := query
		for {
			if bq, ok := q.(*search.BoostQuery); ok {
				q = bq.Query()
			} else {
				break
			}
		}
		termSet := fq.getTermSet(q)
		if tq, ok := q.(*search.TermQuery); ok {
			termSet[tq.Term().Text()] = struct{}{}
		} else if pq, ok := q.(*search.PhraseQuery); ok {
			for _, term := range pq.Terms() {
				termSet[term.Text()] = struct{}{}
			}
		} else if mtq, ok := q.(*search.MultiTermQuery); ok && searcher != nil {
			rewritten := mtq.Rewrite(searcher)
			if bq, ok := rewritten.(*search.BooleanQuery); ok {
				for _, clause := range bq.Clauses() {
					if tq, ok := clause.Query().(*search.TermQuery); ok {
						termSet[tq.Term().Text()] = struct{}{}
					}
				}
			}
		} else {
			panic(fmt.Sprintf("query %v must be flattened first", query))
		}
	}
}

func (fq *FieldQuery) getTermSet(query search.Query) map[string]struct{} {
	key := fq.getKey(query)
	if set, ok := fq.TermSetMap[key]; ok {
		return set
	}
	set := make(map[string]struct{})
	fq.TermSetMap[key] = set
	return set
}

func (fq *FieldQuery) GetTermSet(field string) map[string]struct{} {
	key := ""
	if fq.FieldMatch {
		key = field
	}
	return fq.TermSetMap[key]
}

func (fq *FieldQuery) GetFieldTermMap(fieldName, term string) *QueryPhraseMap {
	key := ""
	if fq.FieldMatch {
		key = fieldName
	}
	rootMap := fq.RootMaps[key]
	if rootMap == nil {
		return nil
	}
	return rootMap.SubMap[term]
}

func (fq *FieldQuery) SearchPhrase(fieldName string, phraseCandidate []*TermInfo) *QueryPhraseMap {
	key := ""
	if fq.FieldMatch {
		key = fieldName
	}
	root := fq.RootMaps[key]
	if root == nil {
		return nil
	}
	return root.SearchPhrase(phraseCandidate)
}

func (fq *FieldQuery) nextTermOrPhraseNumber() int {
	fq.termOrPhraseNumber++
	return fq.termOrPhraseNumber
}
