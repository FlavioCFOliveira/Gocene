package search

// AfterEffectL is a model of the information gain based on Laplace's law of succession.
type AfterEffectL struct{}

// NewAfterEffectL creates a new AfterEffectL.
func NewAfterEffectL() *AfterEffectL {
	return &AfterEffectL{}
}

// ScoreTimes1pTfn returns the product of the after effect with 1+tfn.
func (e *AfterEffectL) ScoreTimes1pTfn(stats *LuceneBasicStats) float64 {
	return 1.0
}

// Explain returns an explanation for the score.
func (e *AfterEffectL) Explain(stats *LuceneBasicStats, tfn float64) Explanation {
	score := e.ScoreTimes1pTfn(stats) / (1.0 + tfn)
	return MatchExplanationWithDetails(
		float32(score),
		"AfterEffectL, computed as 1 / (tfn + 1) from:",
		MatchExplanation(float32(tfn), "tfn, normalized term frequency"),
	)
}

// String returns the code of the after effect formula.
func (e *AfterEffectL) String() string {
	return "L"
}
