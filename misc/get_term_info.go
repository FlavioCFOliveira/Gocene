package misc

// GetTermInfo holds the helpers that report per-term statistics. Mirrors
// org.apache.lucene.misc.GetTermInfo.

// GetTermInfoForField runs the supplied lookup against the (field, term)
// pair and packages the result as a TermStats.
type TermInfoLookup func(field, term string) (docFreq int, totalTermFreq int64, err error)

// GetTermInfoForField returns TermStats for the supplied term.
func GetTermInfoForField(field, term string, lookup TermInfoLookup) (*TermStats, error) {
	if lookup == nil {
		return nil, nil
	}
	docFreq, totalTermFreq, err := lookup(field, term)
	if err != nil {
		return nil, err
	}
	return NewTermStats(field, term, docFreq, totalTermFreq), nil
}
