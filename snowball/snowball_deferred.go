// Package snowball hosts the deferred Sprint 28 ports for
// org.tartarus.snowball.
package snowball

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// Among mirrors org.tartarus.snowball.Among.
type Among struct{}

// NewAmong builds a Among.
func NewAmong() *Among { return &Among{} }

// SnowballProgram mirrors org.tartarus.snowball.SnowballProgram.
type SnowballProgram struct{}

// NewSnowballProgram builds a SnowballProgram.
func NewSnowballProgram() *SnowballProgram { return &SnowballProgram{} }

// SnowballStemmer mirrors org.tartarus.snowball.SnowballStemmer.
type SnowballStemmer struct{}

// NewSnowballStemmer builds a SnowballStemmer.
func NewSnowballStemmer() *SnowballStemmer { return &SnowballStemmer{} }

