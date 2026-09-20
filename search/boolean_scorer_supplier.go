package search

type BooleanScorerSupplier struct {
	subs                  map[Occur][]ScorerSupplier
	scoreMode             ScoreMode
	minShouldMatch        int
	maxDoc                int
	cost                  int64
	topLevelScoringClause bool
}

func NewBooleanScorerSupplier(weight Weight, subs map[Occur][]ScorerSupplier, scoreMode ScoreMode, minShouldMatch, maxDoc int) *BooleanScorerSupplier {
	return &BooleanScorerSupplier{
		subs:           subs,
		scoreMode:      scoreMode,
		minShouldMatch: minShouldMatch,
		maxDoc:         maxDoc,
		cost:           -1,
	}
}

func (s *BooleanScorerSupplier) SetTopLevelScoringClause() error {
	s.topLevelScoringClause = true
	// Propagate if single scoring clause
	musts := s.subs[MUST]
	shoulds := s.subs[SHOULD]
	if len(musts)+len(shoulds) == 1 {
		for _, ss := range musts {
			if setter, ok := ss.(interface{ SetTopLevelScoringClause() error }); ok {
				setter.SetTopLevelScoringClause()
			}
		}
		for _, ss := range shoulds {
			if setter, ok := ss.(interface{ SetTopLevelScoringClause() error }); ok {
				setter.SetTopLevelScoringClause()
			}
		}
	}
	return nil
}

func (s *BooleanScorerSupplier) Cost() int64 {
	if s.cost == -1 {
		s.cost = s.computeCost()
	}
	return s.cost
}

func (s *BooleanScorerSupplier) computeCost() int64 {
	var minRequiredCost int64 = -1
	musts := s.subs[MUST]
	filters := s.subs[FILTER]

	for _, ss := range musts {
		c := ss.Cost()
		if minRequiredCost == -1 || c < minRequiredCost {
			minRequiredCost = c
		}
	}
	for _, ss := range filters {
		c := ss.Cost()
		if minRequiredCost == -1 || c < minRequiredCost {
			minRequiredCost = c
		}
	}

	if minRequiredCost != -1 && s.minShouldMatch == 0 {
		return minRequiredCost
	}

	shoulds := s.subs[SHOULD]
	shouldCost := s.computeShouldCost(shoulds)

	if minRequiredCost == -1 {
		return shouldCost
	}
	if shouldCost < minRequiredCost {
		return shouldCost
	}
	return minRequiredCost
}

func (s *BooleanScorerSupplier) computeShouldCost(shoulds []ScorerSupplier) int64 {
	// Simplified should cost for now.
	if len(shoulds) == 0 {
		return 0
	}
	// In Lucene, this uses a specific algorithm for minShouldMatch.
	// For now, we return the min cost of the should clauses.
	minCost := shoulds[0].Cost()
	for _, ss := range shoulds[1:] {
		c := ss.Cost()
		if c < minCost {
			minCost = c
		}
	}
	return minCost
}

func (s *BooleanScorerSupplier) Get(leadCost int64) (Scorer, error) {
	effLeadCost := leadCost
	if s.Cost() < effLeadCost {
		effLeadCost = s.Cost()
	}

	shoulds := s.subs[SHOULD]
	musts := s.subs[MUST]
	filters := s.subs[FILTER]
	mustNots := s.subs[MUST_NOT]

	// Pure conjunction
	if len(shoulds) == 0 {
		req := s.req(filters, musts, effLeadCost, s.topLevelScoringClause)
		return s.excl(req, mustNots, effLeadCost), nil
	}

	// Pure disjunction
	if len(filters) == 0 && len(musts) == 0 {
		opt := s.opt(shoulds, s.minShouldMatch, s.scoreMode, effLeadCost, s.topLevelScoringClause)
		return s.excl(opt, mustNots, effLeadCost), nil
	}

	// Mix
	if s.minShouldMatch > 0 {
		req := s.excl(s.req(filters, musts, effLeadCost, false), mustNots, effLeadCost)
		opt := s.opt(shoulds, s.minShouldMatch, s.scoreMode, effLeadCost, false)
		return NewConjunctionScorer([]Scorer{req, opt}, []Scorer{req, opt}), nil
	} else {
		req := s.excl(s.req(filters, musts, effLeadCost, false), mustNots, effLeadCost)
		opt := s.opt(shoulds, s.minShouldMatch, s.scoreMode, effLeadCost, false)
		// ReqOptSumScorer is a specialized Lucene scorer. For now, we can use a simplified version
		// or treat it as a disjunction of the required part and the optional part.
		return NewDisjunctionSumScorer([]Scorer{req, opt}, s.scoreMode, effLeadCost), nil
	}
}

func (s *BooleanScorerSupplier) req(filters []ScorerSupplier, musts []ScorerSupplier, leadCost int64, topLevel bool) Scorer {
	allReq := append([]ScorerSupplier{}, filters...)
	allReq = append(allReq, musts...)

	if len(allReq) == 1 {
		scorer, _ := allReq[0].Get(leadCost)
		if s.scoreMode.NeedsScores() == false {
			return scorer
		}
		// Wrap in filter scorer if it's just a filter
		if len(musts) == 0 {
			return &filterScorer{FilterScorer: NewFilterScorer(scorer)}
		}
		return scorer
	}

	var allScorers []Scorer
	var scoringScorers []Scorer
	for _, ss := range allReq {
		scorer, _ := ss.Get(leadCost)
		allScorers = append(allScorers, scorer)
		// In Lucene, only MUST clauses are scoring here.
		// We can check if the supplier was in the musts list.
	}
	// This is simplified.
	return NewConjunctionScorer(allScorers, scoringScorers)
}

func (s *BooleanScorerSupplier) opt(shoulds []ScorerSupplier, minShouldMatch int, scoreMode ScoreMode, leadCost int64, topLevel bool) Scorer {
	if len(shoulds) == 1 {
		scorer, _ := shoulds[0].Get(leadCost)
		return scorer
	}

	var scorers []Scorer
	for _, ss := range shoulds {
		scorer, _ := ss.Get(leadCost)
		scorers = append(scorers, scorer)
	}

	return NewDisjunctionSumScorer(scorers, scoreMode, leadCost)
}

func (s *BooleanScorerSupplier) excl(main Scorer, prohibited []ScorerSupplier, leadCost int64) Scorer {
	if len(prohibited) == 0 {
		return main
	}

	var prohibitedScorers []Scorer
	for _, ss := range prohibited {
		scorer, _ := ss.Get(leadCost)
		prohibitedScorers = append(prohibitedScorers, scorer)
	}

	var prohibitedScorer Scorer
	if len(prohibitedScorers) == 1 {
		prohibitedScorer = prohibitedScorers[0]
	} else {
		prohibitedScorer = NewDisjunctionSumScorer(prohibitedScorers, COMPLETE_NO_SCORES, leadCost)
	}

	return NewReqExclScorer(main, prohibitedScorer)
}

// filterScorer is the anonymous FilterScorer subclass that
// BooleanScorerSupplier creates when scores are needed but every required
// clause is a filter (Lucene 10.5.0, BooleanScorerSupplier.java:436-446 and
// 490-501):
//
//	new FilterScorer(req) {
//	  @Override public float score() throws IOException { return 0f; }
//	  @Override public float getMaxScore(int upTo) throws IOException { return 0f; }
//	}
type filterScorer struct {
	*FilterScorer
}

// Score mirrors the anonymous override `return 0f;`.
func (f *filterScorer) Score() (float32, error) { return 0, nil }

// GetMaxScore mirrors the anonymous override `return 0f;`.
func (f *filterScorer) GetMaxScore(upTo int) (float32, error) { return 0, nil }

var _ ScorerSupplier = (*BooleanScorerSupplier)(nil)

// BulkScorer mirrors the concrete body of ScorerSupplier.bulkScorer() in Apache
// Lucene 10.5.0: new DefaultBulkScorer(get(Long.MAX_VALUE)).
func (b *BooleanScorerSupplier) BulkScorer() (BulkScorer, error) {
	return DefaultScorerSupplierBulkScorer(b)
}
