package document

// CompletionTerms is the per-field terms view exposed by the suggester. The
// Go port keeps the contract minimal — concrete reading is deferred to the
// codec layer. Mirrors
// org.apache.lucene.search.suggest.document.CompletionTerms.
type CompletionTerms struct {
	Field string
}

// NewCompletionTerms builds a CompletionTerms for field.
func NewCompletionTerms(field string) *CompletionTerms { return &CompletionTerms{Field: field} }
