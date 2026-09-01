package search

import (
	"time"
)

type Operator int

const (
	OpAND Operator = iota
	OpOR
)

// QueryParserConfig holds configurations for query parser.
type QueryParserConfig struct {
	UseClassicParser                    bool
	EnablePositionIncrements             bool
	AllowLeadingWildcard                bool
	DateResolution                     string
	DefaultOperator                    Operator
	FuzzyMinSim                         float32
	FuzzyPrefixLength                   int
	Locale                              string
	TimeZone                            *time.Location
	PhraseSlop                          int
	AutoGenerateMultiTermSynonymsPhraseQuery bool
	AutoGeneratePhraseQueries           bool
	SplitOnWhitespace                  bool
	TypeMap                             map[string]string
}

func NewQueryParserConfig() *QueryParserConfig {
	return &QueryParserConfig{
		UseClassicParser:        true,
		EnablePositionIncrements: true,
		AllowLeadingWildcard:    false,
		DateResolution:          "MILLISECOND",
		DefaultOperator:        OpOR,
		FuzzyMinSim:             2.0,
		FuzzyPrefixLength:       0,
		Locale:                  "en",
		PhraseSlop:              0,
		AutoGenerateMultiTermSynonymsPhraseQuery: false,
		AutoGeneratePhraseQueries:           false,
		SplitOnWhitespace:                  false,
		TypeMap:                             make(map[string]string),
	}
}
