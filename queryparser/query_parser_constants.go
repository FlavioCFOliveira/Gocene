package queryparser

// QueryParserConstants defines the token constants used by the QueryParser.
// This is the Go equivalent of Lucene's JavaCC-generated QueryParserConstants interface.
// These constants represent token types and lexical states used during query parsing.
const (
	// EOF represents end of file token.
	EOF = 0

	// Character regex token types (internal use).
	NumChar       = 1
	EscapedChar   = 2
	TermStartChar = 3
	TermChar      = 4
	Whitespace    = 5
	QuotedChar    = 6

	// Boolean operators.
	TokenAND = 8
	TokenOR  = 9
	TokenNOT = 10

	// Modifiers and prefixes.
	TokenPLUS     = 11
	TokenMINUS    = 12
	TokenBAREOPER = 13

	// Structural tokens.
	TokenLPAREN = 14
	TokenRPAREN = 15
	TokenCOLON  = 16
	TokenSTAR   = 17
	TokenCARAT  = 18

	// Term types.
	TokenQUOTED     = 19
	TokenTERM       = 20
	TokenFUZZY_SLOP = 21
	TokenPREFIXTERM = 22
	TokenWILDTERM   = 23
	TokenREGEXPTERM = 24
	TokenNUMBER     = 25

	// Range query tokens.
	TokenRANGEIN_START = 26
	TokenRANGEIN_END   = 27
	TokenRANGEEX_START = 28
	TokenRANGEEX_END   = 29
	TokenRANGE_TO      = 30
	TokenRANGE_QUOTED  = 31
	TokenRANGE_GOOP    = 32

	// Lexical states.
	DefaultState = 0
	BoostState   = 1
	RangeState   = 2
)

// TokenImage provides string representations of token types.
// The index corresponds to the token constant values.
var TokenImage = []string{
	"<EOF>",
	"<NUM_CHAR>",
	"<ESCAPED_CHAR>",
	"<TERM_START_CHAR>",
	"<TERM_CHAR>",
	"<WHITESPACE>",
	"<QUOTED_CHAR>",
	"<token of kind 7>",
	"\"AND\"",
	"\"OR\"",
	"\"NOT\"",
	"\"+\"",
	"\"-\"",
	"<BAREOPER>",
	"\"(\"",
	"\")\"",
	"\":\"",
	"\"*\"",
	"\"^\"",
	"<QUOTED>",
	"<TERM>",
	"<FUZZY_SLOP>",
	"<PREFIXTERM>",
	"<WILDTERM>",
	"<REGEXPTERM>",
	"<NUMBER>",
	"\"[\"",
	"\"]\"",
	"\"{\"",
	"\"}\"",
	"\"TO\"",
	"<RANGE_QUOTED>",
	"<RANGE_GOOP>",
}

// Note: Legacy constants AND, OR conflict with BooleanOperator type in standard_query_parser.go
// Use TokenAND, TokenOR, etc. from this package to avoid conflicts

// TokenNames maps token constants to their human-readable names.
// Useful for debugging and error messages.
var TokenNames = map[int]string{
	EOF:                "EOF",
	NumChar:            "NUM_CHAR",
	EscapedChar:        "ESCAPED_CHAR",
	TermStartChar:      "TERM_START_CHAR",
	TermChar:           "TERM_CHAR",
	Whitespace:         "WHITESPACE",
	QuotedChar:         "QUOTED_CHAR",
	TokenAND:           "AND",
	TokenOR:            "OR",
	TokenNOT:           "NOT",
	TokenPLUS:          "PLUS",
	TokenMINUS:         "MINUS",
	TokenBAREOPER:      "BAREOPER",
	TokenLPAREN:        "LPAREN",
	TokenRPAREN:        "RPAREN",
	TokenCOLON:         "COLON",
	TokenSTAR:          "STAR",
	TokenCARAT:         "CARAT",
	TokenQUOTED:        "QUOTED",
	TokenTERM:          "TERM",
	TokenFUZZY_SLOP:    "FUZZY_SLOP",
	TokenPREFIXTERM:    "PREFIXTERM",
	TokenWILDTERM:      "WILDTERM",
	TokenREGEXPTERM:    "REGEXPTERM",
	TokenNUMBER:        "NUMBER",
	TokenRANGEIN_START: "RANGEIN_START",
	TokenRANGEIN_END:   "RANGEIN_END",
	TokenRANGEEX_START: "RANGEEX_START",
	TokenRANGEEX_END:   "RANGEEX_END",
	TokenRANGE_TO:      "RANGE_TO",
	TokenRANGE_QUOTED:  "RANGE_QUOTED",
	TokenRANGE_GOOP:    "RANGE_GOOP",
}

// GetTokenName returns the human-readable name for a token type.
func GetTokenName(tokenType int) string {
	if name, ok := TokenNames[tokenType]; ok {
		return name
	}
	return "<UNKNOWN>"
}

// GetTokenImage returns the string representation for a token type.
func GetTokenImage(tokenType int) string {
	if tokenType >= 0 && tokenType < len(TokenImage) {
		return TokenImage[tokenType]
	}
	return "<UNKNOWN>"
}

// IsBooleanOperator returns true if the token is a boolean operator (AND, OR, NOT).
func IsBooleanOperator(tokenType int) bool {
	return tokenType == TokenAND || tokenType == TokenOR || tokenType == TokenNOT
}

// IsModifier returns true if the token is a modifier (+, -, BAREOPER).
func IsModifier(tokenType int) bool {
	return tokenType == TokenPLUS || tokenType == TokenMINUS || tokenType == TokenBAREOPER
}

// IsRangeToken returns true if the token is related to range queries.
func IsRangeToken(tokenType int) bool {
	return tokenType >= TokenRANGEIN_START && tokenType <= TokenRANGE_GOOP
}

// IsTermToken returns true if the token represents a term value.
func IsTermToken(tokenType int) bool {
	return tokenType == TokenTERM || tokenType == TokenQUOTED || tokenType == TokenPREFIXTERM ||
		tokenType == TokenWILDTERM || tokenType == TokenREGEXPTERM || tokenType == TokenNUMBER
}
