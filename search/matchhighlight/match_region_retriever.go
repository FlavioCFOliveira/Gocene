package matchhighlight

// FieldValueProvider provides access to field values of the highlighted
// document.
//
// Mirrors the nested interface
// org.apache.lucene.search.matchhighlight.MatchRegionRetriever.FieldValueProvider
// of Apache Lucene 10.5.0.
type FieldValueProvider interface {
	// GetValues returns a list of values for the provided field name or nil if
	// the field is not loaded or does not exist for the field.
	GetValues(field string) []string
}
