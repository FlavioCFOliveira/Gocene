package search

// Scorable allows access to the score of a Query.
type Scorable interface {
	// Score returns the score of the current document matching the query.
	Score() (float32, error)

	// SmoothingScore returns the smoothing score of the current document matching the query.
	SmoothingScore(docID int) (float32, error)

	// SetMinCompetitiveScore tells the scorer that its iterator may safely ignore all documents
	// whose score is less than the given minScore.
	SetMinCompetitiveScore(minScore float32) error

	// GetChildren returns child sub-scorers positioned on the current document.
	GetChildren() ([]ChildScorable, error)
}

// ChildScorable is a child Scorer and its relationship to its parent.
type ChildScorable struct {
	Child        Scorable
	Relationship string
}

// BaseScorable carries the concrete members of the abstract class
// org.apache.lucene.search.Scorable (Lucene 10.5.0): the default bodies of
// smoothingScore(int), setMinCompetitiveScore(float) and getChildren().
//
// Go has no class inheritance, so a type that ports a Scorable subclass embeds
// BaseScorable and overrides only what the Java subclass overrides. Java's
// score() is abstract and is therefore not provided here: the embedder must
// supply it.
type BaseScorable struct{}

// SmoothingScore mirrors Scorable.smoothingScore(int), whose default body in
// Java returns 0f.
func (s *BaseScorable) SmoothingScore(docID int) (float32, error) {
	return 0, nil
}

// SetMinCompetitiveScore mirrors Scorable.setMinCompetitiveScore(float), whose
// default body in Java is empty.
func (s *BaseScorable) SetMinCompetitiveScore(minScore float32) error {
	return nil
}

// GetChildren mirrors Scorable.getChildren(), whose default body in Java
// returns Collections.emptyList().
func (s *BaseScorable) GetChildren() ([]ChildScorable, error) {
	return []ChildScorable{}, nil
}
