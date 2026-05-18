package surround

// Token kind constants used by the surround parser. The numeric values match
// the JavaCC-generated QueryParserConstants in
// org.apache.lucene.queryparser.surround.parser.QueryParserConstants.
const (
	EOF         = 0
	NumChar     = 1
	EscapedChar = 2
	TermStart   = 3
	TermChar    = 4
	Whitespace  = 5
	StartC      = 6
	CarretC     = 7
	TruncatorC  = 8
	AnyChar     = 9
	Quotedchar  = 10
	OrOp        = 11
	AndOp       = 12
	NotOp       = 13
	NumberToken = 14
	W           = 15
	N           = 16
	Colon       = 17
	Caret       = 18
	Truncated   = 19
	QuotedToken = 20
	Suffixterm  = 21
	Truncterm   = 22
	Term        = 23
	OpenParen   = 24
	CloseParen  = 25
	Comma       = 26
)

// TokenImage provides JavaCC-style printable images for each token kind.
var TokenImage = []string{
	"<EOF>",
	"<NUM_CHAR>",
	"<ESCAPED_CHAR>",
	"<TERM_START_CHAR>",
	"<TERM_CHAR>",
	"<WHITESPACE>",
	"<STARTC>",
	"<CARRETC>",
	"<TRUNCATORC>",
	"<ANYCHAR>",
	"<QUOTEDCHAR>",
	"\"OR\"",
	"\"AND\"",
	"\"NOT\"",
	"<NUMBER>",
	"<W>",
	"<N>",
	"\":\"",
	"\"^\"",
	"<TRUNCATED>",
	"<QUOTED>",
	"<SUFFIXTERM>",
	"<TRUNCTERM>",
	"<TERM>",
	"\"(\"",
	"\")\"",
	"\",\"",
}

// GetTokenImage returns the printable image for a token kind or "<UNKNOWN>".
func GetTokenImage(kind int) string {
	if kind >= 0 && kind < len(TokenImage) {
		return TokenImage[kind]
	}
	return "<UNKNOWN>"
}
