// Package misc implements org.apache.lucene.misc: assorted utilities.
package misc

// TermStats captures the (field, term, docFreq, totalTermFreq) tuple emitted
// by HighFreqTerms. Mirrors org.apache.lucene.misc.TermStats.
type TermStats struct {
	Field         string
	Term          string
	DocFreq       int
	TotalTermFreq int64
}

// NewTermStats builds a TermStats.
func NewTermStats(field, term string, docFreq int, totalTermFreq int64) *TermStats {
	return &TermStats{Field: field, Term: term, DocFreq: docFreq, TotalTermFreq: totalTermFreq}
}
