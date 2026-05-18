package surround

// Token mirrors org.apache.lucene.queryparser.surround.parser.Token: a single
// lexical token produced by the surround parser's token manager.
type Token struct {
	Kind        int
	BeginLine   int
	BeginColumn int
	EndLine     int
	EndColumn   int
	Image       string

	Next         *Token
	SpecialToken *Token
}

// NewToken builds an empty token of the given kind.
func NewToken(kind int) *Token {
	return &Token{Kind: kind}
}

// NewTokenWithImage builds a token with an image string.
func NewTokenWithImage(kind int, image string) *Token {
	return &Token{Kind: kind, Image: image}
}

// String returns the token image, matching Java's Token.toString().
func (t *Token) String() string { return t.Image }

// NewTokenOfKind is the JavaCC factory used by the generated lexer. It is kept
// for API parity even though Go users will typically call NewToken/NewTokenWithImage.
func NewTokenOfKind(kind int, image string) *Token { return NewTokenWithImage(kind, image) }
