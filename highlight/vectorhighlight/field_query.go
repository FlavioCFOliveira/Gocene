package vectorhighlight

// FieldQuery captures the per-field terms a Query is interested in. Mirrors
// org.apache.lucene.search.vectorhighlight.FieldQuery.
type FieldQuery struct {
	terms map[string]map[string]float32 // field -> term -> weight
	phrase bool
}

// NewFieldQuery builds an empty FieldQuery.
func NewFieldQuery(phrase bool) *FieldQuery {
	return &FieldQuery{terms: make(map[string]map[string]float32), phrase: phrase}
}

// AddTerm registers a term with weight under field.
func (q *FieldQuery) AddTerm(field, term string, weight float32) {
	m, ok := q.terms[field]
	if !ok {
		m = make(map[string]float32)
		q.terms[field] = m
	}
	if existing, ok := m[term]; !ok || weight > existing {
		m[term] = weight
	}
}

// Terms returns a snapshot of the terms registered for field.
func (q *FieldQuery) Terms(field string) map[string]float32 {
	m := q.terms[field]
	out := make(map[string]float32, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// IsPhrase reports whether the query came from a phrase context.
func (q *FieldQuery) IsPhrase() bool { return q.phrase }
