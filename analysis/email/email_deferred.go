// Package email hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.email.
package email

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// UAX29URLEmailTokenizerImpl mirrors org.apache.lucene.analysis.email.UAX29URLEmailTokenizerImpl.
type UAX29URLEmailTokenizerImpl struct{}

// NewUAX29URLEmailTokenizerImpl builds a UAX29URLEmailTokenizerImpl.
func NewUAX29URLEmailTokenizerImpl() *UAX29URLEmailTokenizerImpl {
	return &UAX29URLEmailTokenizerImpl{}
}

// UAX29URLEmailAnalyzer mirrors org.apache.lucene.analysis.email.UAX29URLEmailAnalyzer.
type UAX29URLEmailAnalyzer struct{}

// NewUAX29URLEmailAnalyzer builds a UAX29URLEmailAnalyzer.
func NewUAX29URLEmailAnalyzer() *UAX29URLEmailAnalyzer { return &UAX29URLEmailAnalyzer{} }
