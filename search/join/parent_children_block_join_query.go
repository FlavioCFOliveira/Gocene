package join

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ParentChildrenBlockJoinQuery is a query that returns all the matching child
// documents for a specific parent document indexed together in the same block.
// The provided child query determines which matching child doc is being
// returned.
//
// Port of org.apache.lucene.search.join.ParentChildrenBlockJoinQuery (Apache
// Lucene 10.5.0).
//
// lucene.experimental
type ParentChildrenBlockJoinQuery struct {
	parentFilter BitSetProducer
	childQuery   search.Query
	parentDocId  int
}

// NewParentChildrenBlockJoinQuery creates a ParentChildrenBlockJoinQuery
// instance.
//
//   - parentFilter: a filter identifying parent documents.
//   - childQuery: a child query that determines which child docs are matching.
//   - parentDocId: the top level doc id of that parent to return children
//     documents for.
func NewParentChildrenBlockJoinQuery(parentFilter BitSetProducer, childQuery search.Query, parentDocId int) *ParentChildrenBlockJoinQuery {
	return &ParentChildrenBlockJoinQuery{
		parentFilter: parentFilter,
		childQuery:   childQuery,
		parentDocId:  parentDocId,
	}
}

// Equals reports whether other is an equal ParentChildrenBlockJoinQuery.
func (q *ParentChildrenBlockJoinQuery) Equals(other spi.Query) bool {
	o, ok := other.(*ParentChildrenBlockJoinQuery)
	if !ok {
		return false
	}
	return q.parentFilter == o.parentFilter &&
		q.childQuery.Equals(o.childQuery) &&
		q.parentDocId == o.parentDocId
}

// HashCode renders the Java hashCode().
//
// PORT NOTE: the seed renders Query.classHash(), which Apache Lucene 10.5.0
// derives from the JVM class identity and which therefore has no reproducible
// Go counterpart. A per-type constant seed is the idiom this package already
// uses (see GlobalOrdinalsQuery.HashCode). parentFilter is a BitSetProducer
// interface whose implementations carry no HashCode, so it contributes a
// constant, exactly as ToParentBlockJoinQuery.HashCode does.
func (q *ParentChildrenBlockJoinQuery) HashCode() int {
	hash := 31
	hash = 31*hash + 17
	hash = 31*hash + q.childQuery.HashCode()
	hash = 31*hash + q.parentDocId
	return hash
}

// ToString renders toString(String field).
func (q *ParentChildrenBlockJoinQuery) ToString(field string) string {
	return "ParentChildrenBlockJoinQuery (" + joinQueryToString(q.childQuery, field) + ")"
}

// joinQueryToString renders Query.toString(String).
//
// PORT NOTE: Gocene's search.Query interface does not declare ToString, so the
// call is made through the method set the concrete query carries, the same
// idiom already used by ConstantScoreWeight (search/constant_score_weight.go).
func joinQueryToString(q search.Query, field string) string {
	if ts, ok := q.(interface{ ToString(string) string }); ok {
		return ts.ToString(field)
	}
	if ts, ok := q.(interface{ String(string) string }); ok {
		return ts.String(field)
	}
	if s, ok := q.(interface{ String() string }); ok {
		return s.String()
	}
	return ""
}

// Visit reports this query to the visitor as a leaf.
func (q *ParentChildrenBlockJoinQuery) Visit(visitor search.QueryVisitor) {
	visitor.VisitLeaf(q)
}

// Rewrite rewrites the child query and wraps the result.
func (q *ParentChildrenBlockJoinQuery) Rewrite(indexSearcher *search.IndexSearcher) (search.Query, error) {
	childRewrite, err := q.childQuery.Rewrite(indexSearcher)
	if err != nil {
		return nil, err
	}
	if childRewrite != q.childQuery {
		return NewParentChildrenBlockJoinQuery(q.parentFilter, childRewrite, q.parentDocId), nil
	}
	// super.rewrite(indexSearcher) returns this.
	return q, nil
}

// CreateWeight creates the Weight for this query.
func (q *ParentChildrenBlockJoinQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	childWeight, err := q.childQuery.CreateWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	leaves, err := searcher.GetIndexReader().Leaves()
	if err != nil {
		return nil, err
	}
	readerIndex := index.ReaderUtilSubIndexLeaves(q.parentDocId, leaves)
	return &parentChildrenBlockJoinWeight{
		query:        q,
		childWeight:  childWeight,
		readerIndex:  readerIndex,
		parentDocId:  q.parentDocId,
		parentFilter: q.parentFilter,
	}, nil
}

// parentChildrenBlockJoinWeight renders the anonymous Weight created by
// ParentChildrenBlockJoinQuery.createWeight.
type parentChildrenBlockJoinWeight struct {
	query        search.Query
	childWeight  search.Weight
	readerIndex  int
	parentDocId  int
	parentFilter BitSetProducer
}

func (w *parentChildrenBlockJoinWeight) GetQuery() search.Query { return w.query }

func (w *parentChildrenBlockJoinWeight) Explain(context *index.LeafReaderContext, doc int) (search.Explanation, error) {
	return search.NoMatchExplanation(
		"Not implemented, use ToParentBlockJoinQuery explain why a document matched"), nil
}

func (w *parentChildrenBlockJoinWeight) ScorerSupplier(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
	// Childs docs only reside in a single segment, so no need to evaluate all
	// segments
	if context.Ord != w.readerIndex {
		return nil, nil
	}

	localParentDocId := w.parentDocId - context.DocBase
	// If parentDocId == 0 then a parent doc doesn't have child docs, because
	// child docs are stored before the parent doc and because parent doc is 0
	// we can safely assume that there are no child docs.
	if localParentDocId == 0 {
		return nil, nil
	}

	parents, err := w.parentFilter.GetBitSet(context)
	if err != nil {
		return nil, err
	}
	firstChildDocId := parents.PrevSetBit(localParentDocId-1) + 1
	// A parent doc doesn't have child docs, so we can early exit here:
	if firstChildDocId == localParentDocId {
		return nil, nil
	}

	childrenScorer, err := w.childWeight.Scorer(context)
	if err != nil {
		return nil, err
	}
	if childrenScorer == nil {
		return nil, nil
	}
	childrenIterator := childrenScorer.Iterator()
	it := &parentChildrenIterator{
		doc:              -1,
		childrenIterator: childrenIterator,
		firstChildDocId:  firstChildDocId,
		localParentDocId: localParentDocId,
	}
	scorer := &parentChildrenScorer{
		it:             it,
		childrenScorer: childrenScorer,
	}
	return search.NewDefaultScorerSupplier(scorer), nil
}

func (w *parentChildrenBlockJoinWeight) Scorer(context *index.LeafReaderContext) (search.Scorer, error) {
	supplier, err := w.ScorerSupplier(context)
	if err != nil || supplier == nil {
		return nil, err
	}
	return supplier.Get(math.MaxInt64)
}

func (w *parentChildrenBlockJoinWeight) BulkScorer(context *index.LeafReaderContext) (search.BulkScorer, error) {
	scorer, err := w.Scorer(context)
	if err != nil || scorer == nil {
		return nil, err
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

func (w *parentChildrenBlockJoinWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

func (w *parentChildrenBlockJoinWeight) Matches(context *index.LeafReaderContext, doc int) (search.Matches, error) {
	return nil, nil
}

func (w *parentChildrenBlockJoinWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return false // TODO delegate to BitSetProducer?
}

// parentChildrenIterator renders the anonymous DocIdSetIterator created by
// ParentChildrenBlockJoinQuery's Weight.scorerSupplier.
type parentChildrenIterator struct {
	doc              int
	childrenIterator util.DocIdSetIterator
	firstChildDocId  int
	localParentDocId int
}

func (it *parentChildrenIterator) DocID() int { return it.doc }

func (it *parentChildrenIterator) NextDoc() (int, error) {
	return it.Advance(it.doc + 1)
}

func (it *parentChildrenIterator) Advance(target int) (int, error) {
	if target < it.firstChildDocId {
		target = it.firstChildDocId
	}
	if target >= it.localParentDocId {
		// We're outside the child nested scope, so it is done
		it.doc = util.NO_MORE_DOCS
		return it.doc, nil
	}
	advanced, err := it.childrenIterator.Advance(target)
	if err != nil {
		return 0, err
	}
	if advanced >= it.localParentDocId {
		// We're outside the child nested scope, so it is done
		it.doc = util.NO_MORE_DOCS
		return it.doc, nil
	}
	it.doc = advanced
	return it.doc, nil
}

func (it *parentChildrenIterator) Cost() int64 {
	return min(it.childrenIterator.Cost(), int64(it.localParentDocId-it.firstChildDocId))
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int).
func (it *parentChildrenIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd().
func (it *parentChildrenIterator) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(it)
}

// parentChildrenScorer renders the anonymous Scorer created by
// ParentChildrenBlockJoinQuery's Weight.scorerSupplier.
type parentChildrenScorer struct {
	search.BaseScorer
	it             *parentChildrenIterator
	childrenScorer search.Scorer
}

func (s *parentChildrenScorer) DocID() int { return s.it.DocID() }

func (s *parentChildrenScorer) Score() (float32, error) { return s.childrenScorer.Score() }

func (s *parentChildrenScorer) GetMaxScore(upTo int) (float32, error) {
	return float32(math.Inf(1)), nil
}

func (s *parentChildrenScorer) Iterator() util.DocIdSetIterator { return s.it }

// NextDocsAndScores carries the concrete body of Scorer.nextDocsAndScores.
func (s *parentChildrenScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

var _ search.Query = (*ParentChildrenBlockJoinQuery)(nil)
var _ search.Weight = (*parentChildrenBlockJoinWeight)(nil)
var _ search.Scorer = (*parentChildrenScorer)(nil)
var _ util.DocIdSetIterator = (*parentChildrenIterator)(nil)
