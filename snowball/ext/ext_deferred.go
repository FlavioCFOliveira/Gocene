// Package ext hosts the deferred Sprint 28 ports for
// org.tartarus.snowball.ext.
package ext

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.


// GermanStemmer mirrors org.tartarus.snowball.ext.GermanStemmer.
type GermanStemmer struct{}

// NewGermanStemmer builds a GermanStemmer.
func NewGermanStemmer() *GermanStemmer { return &GermanStemmer{} }

// IndonesianStemmer mirrors org.tartarus.snowball.ext.IndonesianStemmer.
type IndonesianStemmer struct{}

// NewIndonesianStemmer builds a IndonesianStemmer.
func NewIndonesianStemmer() *IndonesianStemmer { return &IndonesianStemmer{} }





