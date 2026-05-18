package surround

// DistanceSubQuery is the marker interface implemented by SrndQuery nodes that
// may appear as sub-clauses of a DistanceQuery. The Java original is an empty
// interface; in Go we surface a single method so concrete implementations can
// be matched via type assertion and contribute their SpanQuery clauses to the
// surrounding SpanNearClauseFactory.
type DistanceSubQuery interface {
	SrndQuery

	// AddSpanQueries contributes one or more SpanQuery clauses to the supplied
	// factory. The implementation is expected to expand any wildcard/prefix
	// terms against the underlying IndexReader before adding clauses.
	AddSpanQueries(factory *SpanNearClauseFactory) error

	// DistanceSubQueryNotAllowed returns the reason why this node cannot
	// participate in a DistanceQuery, or the empty string when the node is
	// acceptable.
	DistanceSubQueryNotAllowed() string
}
