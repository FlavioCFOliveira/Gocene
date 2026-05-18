package vectorhighlight

// FieldTermStack is a stack of term occurrences gathered from a document's
// term vector for a single field. Mirrors
// org.apache.lucene.search.vectorhighlight.FieldTermStack.
type FieldTermStack struct {
	Field      string
	Occurrences []TermOccurrence
}

// TermOccurrence is a single (term, position, start, end) tuple from the
// term vector.
type TermOccurrence struct {
	Term        string
	Position    int
	StartOffset int
	EndOffset   int
}

// NewFieldTermStack builds the stack.
func NewFieldTermStack(field string, occurrences []TermOccurrence) *FieldTermStack {
	clone := make([]TermOccurrence, len(occurrences))
	copy(clone, occurrences)
	return &FieldTermStack{Field: field, Occurrences: clone}
}

// IsEmpty reports whether the stack has any occurrences left.
func (s *FieldTermStack) IsEmpty() bool { return len(s.Occurrences) == 0 }

// Pop removes and returns the smallest-position occurrence.
func (s *FieldTermStack) Pop() (TermOccurrence, bool) {
	if len(s.Occurrences) == 0 {
		return TermOccurrence{}, false
	}
	occ := s.Occurrences[0]
	s.Occurrences = s.Occurrences[1:]
	return occ, true
}
