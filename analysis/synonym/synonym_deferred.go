// Package synonym hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.synonym.
package synonym

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// SolrSynonymParser mirrors org.apache.lucene.analysis.synonym.SolrSynonymParser.
type SolrSynonymParser struct{}

// NewSolrSynonymParser builds a SolrSynonymParser.
func NewSolrSynonymParser() *SolrSynonymParser { return &SolrSynonymParser{} }

// WordnetSynonymParser mirrors org.apache.lucene.analysis.synonym.WordnetSynonymParser.
type WordnetSynonymParser struct{}

// NewWordnetSynonymParser builds a WordnetSynonymParser.
func NewWordnetSynonymParser() *WordnetSynonymParser { return &WordnetSynonymParser{} }
