package join

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ToParentBlockJoinQuery requires that you index children and parent docs as a single block,
// using the IndexWriter.AddDocuments or IndexWriter.UpdateDocuments API.
// In each block, the child documents must appear first, ending with the parent document.
// At search time you provide a Filter identifying the parents, however this Filter
// must provide a BitSet per sub-reader.
type ToParentBlockJoinQuery struct {
	search.BaseQuery
	parentsFilter BitSetProducer
	childQuery    search.Query
	scoreMode     ScoreMode
}

// NewToParentBlockJoinQuery creates a ToParentBlockJoinQuery.
func NewToParentBlockJoinQuery(childQuery search.Query, parentsFilter BitSetProducer, scoreMode ScoreMode) *ToParentBlockJoinQuery {
	return &ToParentBlockJoinQuery{
		parentsFilter: parentsFilter,
		childQuery:    childQuery,
		scoreMode:     scoreMode,
	}
}

func (q *ToParentBlockJoinQuery) Visit(visitor search.QueryVisitor) {
	visitor.VisitLeaf(q)
}

func (q *ToParentBlockJoinQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	childScoreMode := ScoreMode(0) // Default None
	if needsScores {
		childScoreMode = q.scoreMode
	}

	var childWeight search.Weight
	var err error
	if childScoreMode == None {
		// Wrap under a constant score query
		csQuery := search.NewConstantScoreQuery(q.childQuery)
		childWeight, err = searcher.Rewrite(csQuery).CreateWeight(searcher, needsScores, 0)
		if err != nil {
			return nil, err
		}
	} else {
		mode := needsScores && childScoreMode != Max
		// Note: in Lucene, they use COMPLETE if not Max and needsScores is true.
		// In Gocene, we only have needsScores bool.
		childWeight, err = q.childQuery.CreateWeight(searcher, mode, boost)
		if err != nil {
			return nil, err
		}
	}
	return newBlockJoinWeight(q, childWeight, q.parentsFilter, childScoreMode), nil
}

func (q *ToParentBlockJoinQuery) GetChildQuery() search.Query {
	return q.childQuery
}

type blockJoinWeight struct {
	search.FilterWeight
	parentsFilter BitSetProducer
	scoreMode     ScoreMode
}

func newBlockJoinWeight(joinQuery *ToParentBlockJoinQuery, childWeight search.Weight, parentsFilter BitSetProducer, scoreMode ScoreMode) *blockJoinWeight {
	return &blockJoinWeight{
		FilterWeight:  *search.NewFilterWeightWithQuery(joinQuery, childWeight),
		parentsFilter: parentsFilter,
		scoreMode:     scoreMode,
	}
}

func (w *blockJoinWeight) ScorerSupplier(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
	childScorerSupplier, err := w.in.ScorerSupplier(context)
	if err != nil {
		return nil, err
	}
	if childScorerSupplier == nil {
		return nil, nil
	}

	parents, err := w.parentsFilter.GetBitSet(context)
	if err != nil {
		return nil, err
	}
	if parents == nil {
		return nil, nil
	}

	return &blockJoinScorerSupplier{
		childScorerSupplier: childScorerSupplier,
		parents:             parents,
		scoreMode:           w.scoreMode,
	}, nil
}

type blockJoinScorerSupplier struct {
	childScorerSupplier search.ScorerSupplier
	parents             util.BitSet
	scoreMode           ScoreMode
}

func (s *blockJoinScorerSupplier) Get(leadCost int64) (search.Scorer, error) {
	childScorer, err := s.childScorerSupplier.Get(leadCost)
	if err != nil {
		return nil, err
	}
	return newBlockJoinScorer(childScorer, s.parents, s.scoreMode), nil
}

func (s *blockJoinScorerSupplier) BulkScorer() (search.BulkScorer, error) {
	if s.scoreMode == None {
		// Optimized case
		return search.NewDefaultBulkScorer(s.Get(0)) // This is a bit simplified
	}
	childBulkScorer, err := s.childScorerSupplier.BulkScorer()
	if err != nil {
		return nil, err
	}
	return &blockJoinBulkScorer{
		childBulkScorer: childBulkScorer,
		parents:         s.parents,
		scoreMode:       s.scoreMode,
	}, nil
}

func (s *blockJoinScorerSupplier) Cost() int64 {
	return s.childScorerSupplier.Cost()
}

func (s *blockJoinScorerSupplier) SetTopLevelScoringClause() error {
	if s.scoreMode == Max {
		return s.childScorerSupplier.SetTopLevelScoringClause()
	}
	return nil
}

func (w *blockJoinWeight) Explain(context *index.LeafReaderContext, doc int) (search.Explanation, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	bjScorer := scorer.(*blockJoinScorer)
	if bjScorer.iterator().DocID() != doc {
		// Need to advance
		if _, err := bjScorer.iterator().Advance(doc); err != nil {
			return nil, err
		}
		if bjScorer.iterator().DocID() != doc {
			return search.ExplanationNoMatch("Not a match"), nil
		}
	}
	return bjScorer.explain(context, w.in, w.scoreMode), nil
}

func (w *blockJoinWeight) Matches(context *index.LeafReaderContext, doc int) (search.Matches, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	bjScorer := scorer.(*blockJoinScorer)
	it := bjScorer.iterator()
	if it.DocID() != doc {
		if _, err := it.Advance(doc); err != nil {
			return nil, err
		}
		if it.DocID() != doc {
			return nil, nil
		}
	}
	return search.MatchWithNoTerms, nil
}

type parentApproximation struct {
	childApproximation util.DocIdSetIterator
	parentBits          util.BitSet
	doc                 int
}

func newParentApproximation(childApproximation util.DocIdSetIterator, parentBits util.BitSet) *parentApproximation {
	return &parentApproximation{
		childApproximation: childApproximation,
		parentBits:          parentBits,
		doc:                 -1,
	}
}

func (pa *parentApproximation) DocID() int {
	return pa.doc
}

func (pa *parentApproximation) NextDoc() (int, error) {
	return pa.Advance(pa.doc + 1)
}

func (pa *parentApproximation) Advance(target int) (int, error) {
	if target >= pa.parentBits.Length() {
		pa.doc = search.NO_MORE_DOCS
		return pa.doc, nil
	}
	firstChildTarget := 0
	if target != 0 {
		firstChildTarget = pa.parentBits.PrevSetBit(target - 1) + 1
	}
	childDoc, err := pa.childApproximation.Advance(firstChildTarget)
	if err != nil {
		return 0, err
	}
	if childDoc >= pa.parentBits.Length()-1 {
		pa.doc = search.NO_MORE_DOCS
		return pa.doc, nil
	}
	pa.doc = pa.parentBits.NextSetBit(childDoc + 1)
	return pa.doc, nil
}

func (pa *parentApproximation) Cost() int64 {
	return pa.childApproximation.Cost()
}

type parentTwoPhase struct {
	search.TwoPhaseIterator
	parentApproximation *parentApproximation
	childApproximation  util.DocIdSetIterator
	childTwoPhase       search.TwoPhaseIterator
}

func newParentTwoPhase(parentApproximation *parentApproximation, childTwoPhase search.TwoPhaseIterator) *parentTwoPhase {
	return &parentTwoPhase{
		parentApproximation: parentApproximation,
		childApproximation:  childTwoPhase.Approximation(),
		childTwoPhase:       childTwoPhase,
	}
}

func (ptp *parentTwoPhase) Matches() (bool, error) {
	for {
		if ptp.childApproximation.DocID() >= ptp.parentApproximation.DocID() {
			return false, nil
		}
		match, err := ptp.childTwoPhase.Matches()
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
		nextChild, err := ptp.childApproximation.NextDoc()
		if err != nil {
			return false, err
		}
		if nextChild >= ptp.parentApproximation.DocID() {
			return false, nil
		}
		_ = nextChild
	}
}

func (ptp *parentTwoPhase) MatchCost() float32 {
	return ptp.childTwoPhase.MatchCost() + 10
}

type joinScore struct {
	scoreMode ScoreMode
	score     float64
	freq      int
}

func (s *joinScore) reset(firstChildScorer search.Scorable) error {
	s.freq = 1
	if s.scoreMode == None {
		s.score = 0
	} else {
		val, err := firstChildScorer.Score()
		if err != nil {
			return err
		}
		s.score = float64(val)
	}
	return nil
}

func (s *joinScore) addChildScore(childScorer search.Scorable) error {
	var childScore float32
	if s.scoreMode != None {
		val, err := childScorer.Score()
		if err != nil {
			return err
		}
		childScore = val
	}
	s.freq++
	switch s.scoreMode {
	case Total, Avg:
		s.score += float64(childScore)
	case Min:
		if float64(childScore) < s.score {
			s.score = float64(childScore)
		}
	case Max:
		if float64(childScore) > s.score {
			s.score = float64(childScore)
		}
	case None:
		// do nothing
	}
	return nil
}

func (s *joinScore) score() float32 {
	if s.freq == 0 {
		return 0
	}
	res := s.score
	if s.scoreMode == Avg {
		res /= float64(s.freq)
	}
	return float32(res)
}

type blockJoinScorer struct {
	search.Scorer
	childScorer      search.Scorer
	parentBits       util.BitSet
	scoreMode        ScoreMode
	childApproximation util.DocIdSetIterator
	childTwoPhase     search.TwoPhaseIterator
	parentApproximation *parentApproximation
	parentTwoPhase     *parentTwoPhase
	parentScore       *joinScore
}

func newBlockJoinScorer(childScorer search.Scorer, parentBits util.BitSet, scoreMode ScoreMode) *blockJoinScorer {
	parentScore := &joinScore{scoreMode: scoreMode}
	childTwoPhase := childScorer.TwoPhaseIterator()
	var childApproximation util.DocIdSetIterator
	var parentApproximation *parentApproximation
	var parentTwoPhase *parentTwoPhase

	if childTwoPhase == nil {
		childApproximation = childScorer.Iterator()
		parentApproximation = newParentApproximation(childApproximation, parentBits)
		parentTwoPhase = nil
	} else {
		childApproximation = childTwoPhase.Approximation()
		parentApproximation = newParentApproximation(childTwoPhase.Approximation(), parentBits)
		parentTwoPhase = newParentTwoPhase(parentApproximation, childTwoPhase)
	}

	return &blockJoinScorer{
		childScorer:        childScorer,
		parentBits:         parentBits,
		scoreMode:          scoreMode,
		childApproximation: childApproximation,
		childTwoPhase:      childTwoPhase,
		parentApproximation: parentApproximation,
		parentTwoPhase:      parentTwoPhase,
		parentScore:        parentScore,
	}
}

func (s *blockJoinScorer) DocID() int {
	return s.parentApproximation.DocID()
}

func (s *blockJoinScorer) Iterator() util.DocIdSetIterator {
	if s.parentTwoPhase == nil {
		return s.parentApproximation
	}
	// Use a wrapper that converts TwoPhaseIterator to DocIdSetIterator
	return &twoPhaseToDocIdSetIterator{twoPhase: s.parentTwoPhase}
}

func (s *blockJoinScorer) TwoPhaseIterator() search.TwoPhaseIterator {
	return s.parentTwoPhase
}

func (s *blockJoinScorer) Score() (float32, error) {
	return s.scoreChildDocs()
}

func (s *blockJoinScorer) GetMaxScore(upTo int) (float32, error) {
	if s.scoreMode == None {
		return s.childScorer.GetMaxScore(upTo)
	}
	return float32(fmt.Sprintf("%f", 1e38)), nil // Rough infinity
}

func (s *blockJoinScorer) SetMinCompetitiveScore(minScore float32) error {
	if s.scoreMode == None || s.scoreMode == Max {
		return s.childScorer.SetMinCompetitiveScore(minScore)
	}
	return nil
}

func (s *blockJoinScorer) scoreChildDocs() (float32, error) {
	if s.childApproximation.DocID() >= s.parentApproximation.DocID() {
		return s.parentScore.score(), nil
	}

	if s.scoreMode != None {
		if err := s.parentScore.reset(s.childScorer); err != nil {
			return 0, err
		}
		for {
			nextChild, err := s.childApproximation.NextDoc()
			if err != nil {
				return 0, err
			}
			if nextChild >= s.parentApproximation.DocID() {
				break
			}
			if s.childTwoPhase == nil {
				if err := s.parentScore.addChildScore(s.childScorer); err != nil {
					return 0, err
				}
			} else {
				match, err := s.childTwoPhase.Matches()
				if err != nil {
					return 0, err
				}
				if match {
					if err := s.parentScore.addChildScore(s.childScorer); err != nil {
						return 0, err
					}
				}
			}
		}
	}

	if s.childApproximation.DocID() == s.parentApproximation.DocID() {
		if s.childTwoPhase == nil || func() bool {
			m, _ := s.childTwoPhase.Matches()
			return m
		}() {
			return 0, fmt.Errorf("Child query must not match same docs with parent filter. docId=%d", s.parentApproximation.DocID())
		}
	}

	return s.parentScore.score(), nil
}

func (s *blockJoinScorer) explain(context *index.LeafReaderContext, childWeight search.Weight, scoreMode ScoreMode) search.Explanation {
	prevParentDoc := s.parentBits.PrevSetBit(s.parentApproximation.DocID() - 1)
	start := context.DocBase + prevParentDoc + 1
	end := context.DocBase + s.parentApproximation.DocID() - 1

	var bestChild search.Explanation
	var worstChild search.Explanation

	matches := 0
	for childDoc := start; childDoc <= end; childDoc++ {
		child, _ := childWeight.Explain(context, childDoc-context.DocBase)
		if child != nil {
			matches++
			if bestChild == nil || child.Value() > bestChild.Value() {
				bestChild = child
			}
			if worstChild == nil || child.Value() < worstChild.Value() {
				worstChild = child
			}
		}
	}
	if matches == 0 {
		return search.ExplanationNoMatch("No matches")
	}
	subExplain := bestChild
	if scoreMode == Min {
		subExplain = worstChild
	}
	return search.ExplanationMatch(
		s.Score(),
		fmt.Sprintf("Score based on %d child docs in range from %d to %d, using score mode %v", matches, start, end, scoreMode),
		subExplain,
	)
}

type blockJoinBulkScorer struct {
	search.BulkScorer
	childBulkScorer search.BulkScorer
	parents         util.BitSet
	scoreMode       ScoreMode
}

func (bs *blockJoinBulkScorer) Score(collector search.LeafCollector, acceptDocs util.Bits, min, max int) (int, error) {
	if min == max {
		return bs.scoringCompleteCheck(max, max), nil
	}

	lastParent := bs.parents.PrevSetBit(min(bs.parents.Length(), max) - 1)
	prevParent := -1
	if min != 0 {
		prevParent = bs.parents.PrevSetBit(min - 1)
	}
	if lastParent == prevParent {
		return bs.scoringCompleteCheck(max, max), nil
	}

	wrappedCollector := bs.wrapCollector(collector)
	_, err := bs.childBulkScorer.Score(wrappedCollector, acceptDocs, prevParent+1, lastParent+1)
	if err != nil {
		return 0, err
	}
	wrappedCollector.endBatch()

	return bs.scoringCompleteCheck(lastParent+1, max), nil
}

func (bs *blockJoinBulkScorer) scoringCompleteCheck(innerMax, returnedMax int) int {
	if innerMax >= bs.parents.Length() {
		return search.NO_MORE_DOCS
	}
	return returnedMax
}

func (bs *blockJoinBulkScorer) wrapCollector(collector search.LeafCollector) *batchAwareLeafCollector {
	return &batchAwareLeafCollector{
		FilterLeafCollector: search.FilterLeafCollector{
			LeafCollector: collector,
		},
		currentParentScore: &joinScore{scoreMode: bs.scoreMode},
		currentParent:       -1,
		scorer:              nil,
		bulkScorer:          bs,
	}
}

type batchAwareLeafCollector struct {
	search.FilterLeafCollector
	currentParentScore *joinScore
	currentParent      int
	scorer             search.Scorable
	bulkScorer         *blockJoinBulkScorer
}

func (c *batchAwareLeafCollector) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	return c.LeafCollector.SetScorer(func() search.Scorable {
		return &scoreWrapper{
			scoreFunc: func() (float32, error) {
				return c.currentParentScore.score(), nil
			},
			minCompetitiveScoreFunc: func(minScore float32) error {
				if c.bulkScorer.scoreMode == None || c.bulkScorer.scoreMode == Max {
					return c.scorer.SetMinCompetitiveScore(minScore)
				}
				return nil
			},
		}
	})
}

type scoreWrapper struct {
	scoreFunc               func() (float32, error)
	minCompetitiveScoreFunc func(float32) error
}

func (sw *scoreWrapper) Score() (float32, error) { return sw.scoreFunc() }
func (sw *scoreWrapper) SmoothingScore(docID int) (float32, error) { return 0, nil }
func (sw *scoreWrapper) SetMinCompetitiveScore(minScore float32) error { return sw.minCompetitiveScoreFunc(minScore) }
func (sw *scoreWrapper) GetChildren() ([]search.ChildScorable, error) { return nil, nil }

func (c *batchAwareLeafCollector) Collect(doc int) error {
	if doc > c.currentParent {
		if c.currentParent >= 0 {
			if err := c.LeafCollector.Collect(c.currentParent); err != nil {
				return err
			}
		}
		c.currentParent = c.bulkScorer.parents.NextSetBit(doc)
		if err := c.currentParentScore.reset(c.scorer); err != nil {
			return err
		}
	} else if doc == c.currentParent {
		return fmt.Errorf("Child query must not match same docs with parent filter. docId=%d", doc)
	} else {
		if err := c.currentParentScore.addChildScore(c.scorer); err != nil {
			return err
		}
	}
	return nil
}

func (c *batchAwareLeafCollector) endBatch() {
	if c.currentParent >= 0 {
		_ = c.LeafCollector.Collect(c.currentParent)
	}
}

type twoPhaseToDocIdSetIterator struct {
	twoPhase search.TwoPhaseIterator
}

func (it *twoPhaseToDocIdSetIterator) DocID() int {
	return it.twoPhase.Approximation().DocID()
}

func (it *twoPhaseToDocIdSetIterator) NextDoc() (int, error) {
	return it.twoPhase.Approximation().NextDoc()
}

func (it *twoPhaseToDocIdSetIterator) Advance(target int) (int, error) {
	return it.twoPhase.Approximation().Advance(target)
}

func (it *twoPhaseToDocIdSetIterator) Cost() int64 {
	return it.twoPhase.Approximation().Cost()
}
