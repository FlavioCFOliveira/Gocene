package api

// TokenFilter is a TokenStream that wraps another TokenStream.
type TokenFilter interface {
	TokenStream
	GetInput() TokenStream
	Unwrap() TokenStream
}

// TokenFilterFactory is the interface for factories that create TokenFilter instances.
type TokenFilterFactory interface {
	Create(input TokenStream) TokenFilter
	Normalize(input TokenStream) TokenStream
}
