package search

// AfterEffect acts as the base for the implementations of the first normalization
// of the informative content in the DFR framework. This component is also called the
// after effect and is defined by the formula Inf2 = 1 - Prob2, where Prob2
// measures the information gain.
type AfterEffect interface {
	// ScoreTimes1pTfn returns the product of the after effect with 1+tfn.
	// This may not depend on the value of tfn.
	ScoreTimes1pTfn(stats *LuceneBasicStats) float64

	// Explain returns an explanation for the score.
	Explain(stats *LuceneBasicStats, tfn float64) Explanation

	// String returns the code of the after effect formula.
	String() string
}

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

// AfterEffectB is a model of the information gain based on the ratio of two Bernoulli processes.
type AfterEffectB struct{}

// NewAfterEffectB creates a new AfterEffectB.
func NewAfterEffectB() *AfterEffectB {
	return &AfterEffectB{}
}

// ScoreTimes1pTfn returns the product of the after effect with 1+tfn.
func (e *AfterEffectB) ScoreTimes1pTfn(stats *LuceneBasicStats) float64 {
	f := float64(stats.TotalTermFreq()) + 1.0
	n := float64(stats.DocFreq()) + 1.0
	return (f + 1.0) / n
}

// Explain returns an explanation for the score.
func (e *AfterEffectB) Explain(stats *LuceneBasicStats, tfn float64) Explanation {
	scoreTimes1pTfn := e.ScoreTimes1pTfn(stats)
	score := scoreTimes1pTfn / (1.0 + tfn)
	return MatchExplanationWithDetails(
		float32(score),
		"AfterEffectB, computed as (F + 1) / (n * (tfn + 1)) from:",
		MatchExplanation(float32(tfn), "tfn, normalized term frequency"),
		MatchExplanation(float32(stats.TotalTermFreq()), "F, total number of occurrences of term across all documents + 1"),
		MatchExplanation(float32(stats.DocFreq()), "n, number of documents containing term + 1"),
		MatchExplanation(float32(tfn), "tfn, normalized term frequency"),
	)
}

// String returns the code of the after effect formula.
func (e *AfterEffectB) String() string {
	return "B"
}
