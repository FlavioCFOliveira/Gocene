package join

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DefaultChildLimitPerParent is the default maximum number of child documents
// to match per parent document.
//
// Mirrors ParentsChildrenBlockJoinQuery.DEFAULT_CHILD_LIMIT_PER_PARENT.
const DefaultChildLimitPerParent = math.MaxInt32

// ParentsChildrenBlockJoinQuery is a query that returns the matching child
// documents for matching parent documents indexed together in the same block.
// The provided parentQuery determines the parent documents of the returned
// children documents. The provided childQuery determines which matching
// children documents are being returned. childLimitPerParent is the maximum
// number of child documents to match per parent document.
//
// Port of org.apache.lucene.search.join.ParentsChildrenBlockJoinQuery (Apache
// Lucene 10.5.0).
//
// lucene.experimental
type ParentsChildrenBlockJoinQuery struct {
	parentFilter        BitSetProducer
	parentQuery         search.Query
	childQuery          search.Query
	childLimitPerParent int
	scoreCombiner       func(float32, float32) float32
}

// NewParentsChildrenBlockJoinQuery creates a ParentsChildrenBlockJoinQuery.
//
//   - parentFilter: filter identifying the parent documents.
//   - parentQuery: query that matches parent documents.
//   - childQuery: query that matches child documents.
//   - childLimitPerParent: the maximum number of child documents to match per parent.
func NewParentsChildrenBlockJoinQuery(parentFilter BitSetProducer, parentQuery, childQuery search.Query, childLimitPerParent int) *ParentsChildrenBlockJoinQuery {
	return NewParentsChildrenBlockJoinQueryWithCombiner(
		parentFilter, parentQuery, childQuery, childLimitPerParent, floatSum)
}

// NewParentsChildrenBlockJoinQueryDefault creates a
// ParentsChildrenBlockJoinQuery with DefaultChildLimitPerParent.
func NewParentsChildrenBlockJoinQueryDefault(parentFilter BitSetProducer, parentQuery, childQuery search.Query) *ParentsChildrenBlockJoinQuery {
	return NewParentsChildrenBlockJoinQuery(parentFilter, parentQuery, childQuery, DefaultChildLimitPerParent)
}

// NewParentsChildrenBlockJoinQueryWithCombiner creates a
// ParentsChildrenBlockJoinQuery with a custom score combiner.
func NewParentsChildrenBlockJoinQueryWithCombiner(parentFilter BitSetProducer, parentQuery, childQuery search.Query, childLimitPerParent int, scoreCombiner func(float32, float32) float32) *ParentsChildrenBlockJoinQuery {
	if childLimitPerParent <= 0 {
		panic(fmt.Sprintf("childLimitPerParent must be > 0, got %d", childLimitPerParent))
	}
	return &ParentsChildrenBlockJoinQuery{
		parentFilter:        parentFilter,
		parentQuery:         parentQuery,
		childQuery:          childQuery,
		childLimitPerParent: childLimitPerParent,
		scoreCombiner:       scoreCombiner,
	}
}

// floatSum renders the Float::sum method reference Apache Lucene 10.5.0 uses as
// the default score combiner.
func floatSum(a, b float32) float32 { return a + b }

// Visit reports this query to the visitor as a leaf.
func (q *ParentsChildrenBlockJoinQuery) Visit(visitor search.QueryVisitor) {
	visitor.VisitLeaf(q)
}

// GetParentQuery returns the parent query.
func (q *ParentsChildrenBlockJoinQuery) GetParentQuery() search.Query { return q.parentQuery }

// GetChildQuery returns the child query.
func (q *ParentsChildrenBlockJoinQuery) GetChildQuery() search.Query { return q.childQuery }

// CreateWeight creates the Weight for this query.
func (q *ParentsChildrenBlockJoinQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	parentWeight, err := q.parentQuery.CreateWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	childWeight, err := q.childQuery.CreateWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	return newParentsChildrenBlockJoinWeight(
		q, q.parentFilter, parentWeight, childWeight, q.childLimitPerParent,
		scoreMode.NeedsScores(), q.scoreCombiner), nil
}

// Rewrite rewrites the parent and child queries and wraps the result.
//
// NOTE: as in Apache Lucene 10.5.0, the rewritten query is built through the
// four-argument constructor, so a custom score combiner is not carried over.
func (q *ParentsChildrenBlockJoinQuery) Rewrite(indexSearcher *search.IndexSearcher) (search.Query, error) {
	parentRewrite, err := q.parentQuery.Rewrite(indexSearcher)
	if err != nil {
		return nil, err
	}
	childRewrite, err := q.childQuery.Rewrite(indexSearcher)
	if err != nil {
		return nil, err
	}
	if parentRewrite != q.parentQuery || childRewrite != q.childQuery {
		return NewParentsChildrenBlockJoinQuery(
			q.parentFilter, parentRewrite, childRewrite, q.childLimitPerParent), nil
	}
	// super.rewrite(indexSearcher) returns this.
	return q, nil
}

// ToString renders toString(String field).
func (q *ParentsChildrenBlockJoinQuery) ToString(field string) string {
	return "ParentsChildrenBlockJoinQuery(parentQuery=" +
		joinQueryToString(q.parentQuery, field) + ", childQuery=" +
		joinQueryToString(q.childQuery, field) + ")"
}

// Equals reports whether other is an equal ParentsChildrenBlockJoinQuery.
func (q *ParentsChildrenBlockJoinQuery) Equals(other spi.Query) bool {
	o, ok := other.(*ParentsChildrenBlockJoinQuery)
	if !ok {
		return false
	}
	return q.parentFilter == o.parentFilter &&
		q.parentQuery.Equals(o.parentQuery) &&
		q.childQuery.Equals(o.childQuery) &&
		q.childLimitPerParent == o.childLimitPerParent
}

// HashCode renders the Java hashCode().
//
// PORT NOTE: see the note on ParentChildrenBlockJoinQuery.HashCode for the
// seed. parentFilter is a BitSetProducer interface and scoreCombiner is a Go
// func, neither of which carries a hash, so both contribute a constant.
func (q *ParentsChildrenBlockJoinQuery) HashCode() int {
	const prime = 31
	hash := 31
	hash = prime*hash + 17
	hash = prime*hash + q.parentQuery.HashCode()
	hash = prime*hash + q.childQuery.HashCode()
	hash = prime*hash + q.childLimitPerParent
	hash = prime*hash + 17
	return hash
}

// parentsChildrenBlockJoinWeight renders the nested class
// ParentsChildrenBlockJoinQuery.ParentsChildrenBlockJoinWeight.
type parentsChildrenBlockJoinWeight struct {
	query               search.Query
	parentFilter        BitSetProducer
	parentWeight        search.Weight
	childWeight         search.Weight
	childLimitPerParent int
	doScores            bool
	scoreCombiner       func(float32, float32) float32
	seenContexts        map[*index.LeafReaderContext]struct{}
}

func newParentsChildrenBlockJoinWeight(query search.Query, parentFilter BitSetProducer, parentWeight, childWeight search.Weight, childLimitPerParent int, doScores bool, scoreCombiner func(float32, float32) float32) *parentsChildrenBlockJoinWeight {
	return &parentsChildrenBlockJoinWeight{
		query:               query,
		parentFilter:        parentFilter,
		parentWeight:        parentWeight,
		childWeight:         childWeight,
		childLimitPerParent: childLimitPerParent,
		doScores:            doScores,
		scoreCombiner:       scoreCombiner,
		seenContexts:        make(map[*index.LeafReaderContext]struct{}),
	}
}

func (w *parentsChildrenBlockJoinWeight) GetQuery() search.Query { return w.query }

func (w *parentsChildrenBlockJoinWeight) Explain(context *index.LeafReaderContext, doc int) (search.Explanation, error) {
	s, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	scorer, _ := s.(*parentsChildrenBlockJoinScorer)
	if scorer != nil {
		advanced, err := scorer.Iterator().Advance(doc)
		if err != nil {
			return nil, err
		}
		if advanced == doc {
			parentDoc := scorer.GetParentDoc()
			childDoc := scorer.DocID()
			score, err := scorer.Score()
			if err != nil {
				return nil, err
			}
			parentExpl, err := w.parentWeight.Explain(context, parentDoc)
			if err != nil {
				return nil, err
			}
			childExpl, err := w.childWeight.Explain(context, childDoc)
			if err != nil {
				return nil, err
			}
			return search.MatchExplanationWithDetails(
				score,
				fmt.Sprintf("Score based on parent document %d and child document %d ",
					parentDoc+context.DocBase, childDoc+context.DocBase),
				parentExpl, childExpl), nil
		}
	}
	return search.NoMatchExplanation("Not a match"), nil
}

func (w *parentsChildrenBlockJoinWeight) ScorerSupplier(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
	if _, seen := w.seenContexts[context]; seen {
		return nil, fmt.Errorf(
			"ParentsChildrenBlockJoinQuery does not support intraSegment concurrency. Context %v was already seen.",
			context)
	}
	w.seenContexts[context] = struct{}{}

	parentBits, err := w.parentFilter.GetBitSet(context)
	if err != nil {
		return nil, err
	}
	if parentBits == nil {
		return nil, nil
	}
	parentScorerSupplier, err := w.parentWeight.ScorerSupplier(context)
	if err != nil {
		return nil, err
	}
	childScorerSupplier, err := w.childWeight.ScorerSupplier(context)
	if err != nil {
		return nil, err
	}
	if parentScorerSupplier == nil || childScorerSupplier == nil {
		return nil, nil
	}

	return &parentsChildrenScorerSupplier{
		cost:                 -1,
		parentBits:           parentBits,
		parentScorerSupplier: parentScorerSupplier,
		childScorerSupplier:  childScorerSupplier,
		childLimitPerParent:  w.childLimitPerParent,
		doScores:             w.doScores,
		scoreCombiner:        w.scoreCombiner,
	}, nil
}

func (w *parentsChildrenBlockJoinWeight) Scorer(context *index.LeafReaderContext) (search.Scorer, error) {
	supplier, err := w.ScorerSupplier(context)
	if err != nil || supplier == nil {
		return nil, err
	}
	return supplier.Get(math.MaxInt64)
}

func (w *parentsChildrenBlockJoinWeight) BulkScorer(context *index.LeafReaderContext) (search.BulkScorer, error) {
	scorer, err := w.Scorer(context)
	if err != nil || scorer == nil {
		return nil, err
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

func (w *parentsChildrenBlockJoinWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

func (w *parentsChildrenBlockJoinWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.parentWeight.IsCacheable(ctx) && w.childWeight.IsCacheable(ctx)
}

func (w *parentsChildrenBlockJoinWeight) Matches(context *index.LeafReaderContext, doc int) (search.Matches, error) {
	parentMatch, err := w.parentWeight.Matches(context, doc)
	if err != nil {
		return nil, err
	}
	childMatch, err := w.childWeight.Matches(context, doc)
	if err != nil {
		return nil, err
	}
	if parentMatch == nil && childMatch == nil {
		// Neither matches
		return nil, nil
	}
	// Combine non-null matches
	subMatches := make([]search.Matches, 0, 2)
	if parentMatch != nil {
		subMatches = append(subMatches, parentMatch)
	}
	if childMatch != nil {
		subMatches = append(subMatches, childMatch)
	}
	return search.MatchesUtils.FromSubMatches(subMatches), nil
}

// parentsChildrenScorerSupplier renders the anonymous ScorerSupplier created by
// ParentsChildrenBlockJoinWeight.scorerSupplier.
type parentsChildrenScorerSupplier struct {
	search.BaseScorerSupplier
	cost                 int64
	parentBits           util.BitSet
	parentScorerSupplier search.ScorerSupplier
	childScorerSupplier  search.ScorerSupplier
	childLimitPerParent  int
	doScores             bool
	scoreCombiner        func(float32, float32) float32
}

func (s *parentsChildrenScorerSupplier) Get(leadCost int64) (search.Scorer, error) {
	parentScorer, err := s.parentScorerSupplier.Get(leadCost)
	if err != nil {
		return nil, err
	}
	childScorer, err := s.childScorerSupplier.Get(leadCost)
	if err != nil {
		return nil, err
	}
	return newParentsChildrenBlockJoinScorer(
		s.parentBits, parentScorer, childScorer, s.childLimitPerParent, s.doScores, s.scoreCombiner), nil
}

func (s *parentsChildrenScorerSupplier) Cost() int64 {
	if s.cost == -1 {
		// Calculate cost based on parent and child costs
		// The cost should reflect the number of documents that will be visited
		parentCost := s.parentScorerSupplier.Cost()
		childCost := s.childScorerSupplier.Cost()
		// The actual cost depends on how many children per parent we'll visit
		s.cost = min(parentCost*int64(s.childLimitPerParent), childCost)
	}
	return s.cost
}

func (s *parentsChildrenScorerSupplier) SetTopLevelScoringClause() error {
	// Propagate to both parent and child scorers
	if err := s.parentScorerSupplier.SetTopLevelScoringClause(); err != nil {
		return err
	}
	return s.childScorerSupplier.SetTopLevelScoringClause()
}

func (s *parentsChildrenScorerSupplier) BulkScorer() (search.BulkScorer, error) {
	return search.DefaultScorerSupplierBulkScorer(s)
}

// parentsChildrenBlockJoinScorer renders the nested class
// ParentsChildrenBlockJoinQuery.ParentsChildrenBlockJoinScorer.
type parentsChildrenBlockJoinScorer struct {
	search.BaseScorer
	parentBits          util.BitSet
	parentScorer        search.Scorer
	parentIt            util.DocIdSetIterator
	childScorer         search.Scorer
	childIt             util.DocIdSetIterator
	childLimitPerParent int
	doScores            bool
	scoreCombiner       func(float32, float32) float32
	parentScore         float32
	childScore          float32

	parentDoc     int
	childDoc      int
	childDocCount int
}

func newParentsChildrenBlockJoinScorer(parentBits util.BitSet, parentScorer, childScorer search.Scorer, childLimitPerParent int, doScores bool, scoreCombiner func(float32, float32) float32) *parentsChildrenBlockJoinScorer {
	return &parentsChildrenBlockJoinScorer{
		parentBits:          parentBits,
		parentScorer:        parentScorer,
		parentIt:            parentScorer.Iterator(),
		childScorer:         childScorer,
		childIt:             childScorer.Iterator(),
		childLimitPerParent: childLimitPerParent,
		doScores:            doScores,
		scoreCombiner:       scoreCombiner,
		parentDoc:           0,
		childDoc:            -1,
	}
}

func (s *parentsChildrenBlockJoinScorer) GetChildren() ([]search.ChildScorable, error) {
	return []search.ChildScorable{
		{Child: s.parentScorer, Relationship: "BLOCK_JOIN"},
		{Child: s.childScorer, Relationship: "BLOCK_JOIN"},
	}, nil
}

func (s *parentsChildrenBlockJoinScorer) Iterator() util.DocIdSetIterator {
	return &parentsChildrenIterator{scorer: s}
}

// validateParentDoc detects mis-use, where the provided parent query in fact
// sometimes returns child documents.
func (s *parentsChildrenBlockJoinScorer) validateParentDoc() error {
	if s.parentDoc != util.NO_MORE_DOCS && !s.parentBits.Get(s.parentDoc) {
		return fmt.Errorf("%s%d", invalidQueryMessage, s.parentDoc)
	}
	return nil
}

func (s *parentsChildrenBlockJoinScorer) DocID() int { return s.childDoc }

func (s *parentsChildrenBlockJoinScorer) Score() (float32, error) {
	if s.doScores {
		return s.scoreCombiner(s.parentScore, s.childScore), nil
	}
	return 1.0, nil
}

func (s *parentsChildrenBlockJoinScorer) GetMaxScore(upTo int) (float32, error) {
	return float32(math.Inf(1)), nil
}

// GetParentDoc returns the parent doc the scorer is currently positioned in.
func (s *parentsChildrenBlockJoinScorer) GetParentDoc() int { return s.parentDoc }

// NextDocsAndScores carries the concrete body of Scorer.nextDocsAndScores.
func (s *parentsChildrenBlockJoinScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// parentsChildrenIterator renders the anonymous DocIdSetIterator returned by
// ParentsChildrenBlockJoinScorer.iterator().
type parentsChildrenIterator struct {
	scorer *parentsChildrenBlockJoinScorer
}

func (it *parentsChildrenIterator) DocID() int { return it.scorer.childDoc }

func (it *parentsChildrenIterator) exhausted() bool {
	return it.scorer.childIt.DocID() == util.NO_MORE_DOCS ||
		it.scorer.parentIt.DocID() == util.NO_MORE_DOCS
}

func (it *parentsChildrenIterator) NextDoc() (int, error) {
	s := it.scorer
	if s.childDocCount < s.childLimitPerParent {
		next, err := s.childIt.NextDoc()
		if err != nil {
			return 0, err
		}
		s.childDoc = next
	}

	// Need to move to the next parent if we have exhausted the current parent
	// or child is out of the current parent block
	if s.childDocCount >= s.childLimitPerParent || s.childDoc >= s.parentDoc {
		s.childDocCount = 0
		nextParent, err := s.parentIt.NextDoc()
		if err != nil {
			return 0, err
		}
		s.parentDoc = nextParent
		if s.parentDoc == 0 {
			// first parent doc has no children
			nextParent, err = s.parentIt.NextDoc()
			if err != nil {
				return 0, err
			}
			s.parentDoc = nextParent
		}
		if err := s.validateParentDoc(); err != nil {
			return 0, err
		}
	}

	// Adjust the parentIt and childIt so that they are in the same block
	if err := it.alignParentAndChildIterator(); err != nil {
		return 0, err
	}

	if it.exhausted() {
		s.childDoc = util.NO_MORE_DOCS
		s.parentDoc = util.NO_MORE_DOCS
		return s.childDoc, nil
	}

	if s.doScores {
		childScore, err := s.childScorer.Score()
		if err != nil {
			return 0, err
		}
		s.childScore = childScore
		parentScore, err := s.parentScorer.Score()
		if err != nil {
			return 0, err
		}
		s.parentScore = parentScore
	}

	s.childDocCount++
	return s.childDoc, nil
}

func (it *parentsChildrenIterator) Advance(target int) (int, error) {
	s := it.scorer
	if target <= s.childDoc {
		return s.childDoc, nil
	}

	advanced, err := s.childIt.Advance(target)
	if err != nil {
		return 0, err
	}
	s.childDoc = advanced
	if s.childDocCount >= s.childLimitPerParent || s.childDoc >= s.parentDoc {
		// need to move to the next parent block
		s.childDocCount = 0
		if s.childDoc <= s.parentDoc {
			nextParent, err := s.parentIt.NextDoc()
			if err != nil {
				return 0, err
			}
			s.parentDoc = nextParent
		} else {
			nextParent, err := s.parentIt.Advance(s.childDoc)
			if err != nil {
				return 0, err
			}
			s.parentDoc = nextParent
		}
		if err := s.validateParentDoc(); err != nil {
			return 0, err
		}

		// Adjust the parentIt and childIt so that they are in the same block
		if err := it.alignParentAndChildIterator(); err != nil {
			return 0, err
		}
	}

	if it.exhausted() {
		s.childDoc = util.NO_MORE_DOCS
		s.parentDoc = util.NO_MORE_DOCS
		return s.childDoc, nil
	}

	if s.doScores {
		childScore, err := s.childScorer.Score()
		if err != nil {
			return 0, err
		}
		s.childScore = childScore
		parentScore, err := s.parentScorer.Score()
		if err != nil {
			return 0, err
		}
		s.parentScore = parentScore
	}

	s.childDocCount++
	return s.childDoc, nil
}

func (it *parentsChildrenIterator) alignParentAndChildIterator() error {
	s := it.scorer
	for !it.exhausted() {
		firstChild := s.parentBits.PrevSetBit(s.parentDoc-1) + 1
		if s.childDoc >= firstChild && s.childDoc < s.parentDoc {
			// order is correct, childDoc is within a valid parent block
			break
		} else if s.childDoc < firstChild {
			// childDoc is before the current parent block, advance the child
			// iterator
			advanced, err := s.childIt.Advance(firstChild)
			if err != nil {
				return err
			}
			s.childDoc = advanced
		} else {
			// childDoc is after the current parent block, advance the parent
			// iterator; when childDoc equals to parentDoc we skip to the next
			// parent as well
			if s.childDoc == s.parentDoc {
				nextParent, err := s.parentIt.NextDoc()
				if err != nil {
					return err
				}
				s.parentDoc = nextParent
			} else {
				nextParent, err := s.parentIt.Advance(s.childDoc)
				if err != nil {
					return err
				}
				s.parentDoc = nextParent
			}
			if err := s.validateParentDoc(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (it *parentsChildrenIterator) Cost() int64 {
	s := it.scorer
	if s.childLimitPerParent == DefaultChildLimitPerParent {
		// When there's no limit, we'll visit all child documents for each
		// parent
		return s.childIt.Cost()
	}
	// When there's a limit, we'll visit at most childLimitPerParent child
	// documents for each parent that matches the parent query
	return min(s.childIt.Cost(), s.parentIt.Cost()*int64(s.childLimitPerParent))
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int).
func (it *parentsChildrenIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd().
func (it *parentsChildrenIterator) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(it)
}

var _ search.Query = (*ParentsChildrenBlockJoinQuery)(nil)
var _ search.Weight = (*parentsChildrenBlockJoinWeight)(nil)
var _ search.Scorer = (*parentsChildrenBlockJoinScorer)(nil)
var _ util.DocIdSetIterator = (*parentsChildrenIterator)(nil)
var _ search.ScorerSupplier = (*parentsChildrenScorerSupplier)(nil)
