// Package ext hosts the deferred Sprint 28 ports for
// org.tartarus.snowball.ext.
package ext

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// ArmenianStemmer mirrors org.tartarus.snowball.ext.ArmenianStemmer.
type ArmenianStemmer struct{}

// NewArmenianStemmer builds a ArmenianStemmer.
func NewArmenianStemmer() *ArmenianStemmer { return &ArmenianStemmer{} }

// CatalanStemmer mirrors org.tartarus.snowball.ext.CatalanStemmer.
type CatalanStemmer struct{}

// NewCatalanStemmer builds a CatalanStemmer.
func NewCatalanStemmer() *CatalanStemmer { return &CatalanStemmer{} }


// GermanStemmer mirrors org.tartarus.snowball.ext.GermanStemmer.
type GermanStemmer struct{}

// NewGermanStemmer builds a GermanStemmer.
func NewGermanStemmer() *GermanStemmer { return &GermanStemmer{} }

// IndonesianStemmer mirrors org.tartarus.snowball.ext.IndonesianStemmer.
type IndonesianStemmer struct{}

// NewIndonesianStemmer builds a IndonesianStemmer.
func NewIndonesianStemmer() *IndonesianStemmer { return &IndonesianStemmer{} }


// ItalianStemmer mirrors org.tartarus.snowball.ext.ItalianStemmer.
type ItalianStemmer struct{}

// NewItalianStemmer builds a ItalianStemmer.
func NewItalianStemmer() *ItalianStemmer { return &ItalianStemmer{} }

// NepaliStemmer mirrors org.tartarus.snowball.ext.NepaliStemmer.
type NepaliStemmer struct{}

// NewNepaliStemmer builds a NepaliStemmer.
func NewNepaliStemmer() *NepaliStemmer { return &NepaliStemmer{} }


// TurkishStemmer mirrors org.tartarus.snowball.ext.TurkishStemmer.
type TurkishStemmer struct{}

// NewTurkishStemmer builds a TurkishStemmer.
func NewTurkishStemmer() *TurkishStemmer { return &TurkishStemmer{} }


