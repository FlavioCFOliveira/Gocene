package search

// ScoreMode defines different modes of search.
type ScoreMode int

const (
	COMPLETE = ScoreModeComplete
	COMPLETE_NO_SCORES = ScoreModeCompleteNoScores
	TOP_SCORES = ScoreModeTopScores
	TOP_DOCS = ScoreModeTopDocs
	TOP_DOCS_WITH_SCORES = ScoreModeTopDocsWithScores

	// ScoreModeComplete produced scorers will allow visiting all matches and get their score.
	ScoreModeComplete ScoreMode = iota
	// ScoreModeCompleteNoScores produced scorers will allow visiting all matches but scores won't be available.
	ScoreModeCompleteNoScores
	// ScoreModeTopScores produced scorers will optionally allow skipping over non-competitive hits.
	ScoreModeTopScores
	// ScoreModeTopDocs score mode for top field collectors that can provide their own iterators.
	ScoreModeTopDocs
	// ScoreModeTopDocsWithScores score mode for top field collectors that can provide their own iterators, used when there is a secondary sort by _score.
	ScoreModeTopDocsWithScores
)

func (s ScoreMode) NeedsScores() bool {
	switch s {
	case ScoreModeComplete, ScoreModeTopScores, ScoreModeTopDocsWithScores:
		return true
	default:
		return false
	}
}

func (s ScoreMode) IsExhaustive() bool {
	switch s {
	case ScoreModeComplete, ScoreModeCompleteNoScores:
		return true
	default:
		return false
	}
}
