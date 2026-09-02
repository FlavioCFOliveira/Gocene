// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// TokenStreamWithReset is an interface for TokenStreams that support Reset.
type TokenStreamWithReset interface {
	TokenStream
	Reset() error
}

// TokenStreamWithNext is an interface for TokenStreams that support Next.
type TokenStreamWithNext interface {
	TokenStream
	Next() (Token, bool)
}

// ResetTokenStream safely resets a TokenStream if it supports Reset.
func ResetTokenStream(ts TokenStream) error {
	if resetter, ok := ts.(TokenStreamWithReset); ok {
		return resetter.Reset()
	}
	// If the underlying type has a Reset method accessible via the interface chain, try it
	if resetter, ok := ts.(interface{ Reset() error }); ok {
		return resetter.Reset()
	}
	return nil
}

// NextFromTokenStream safely calls Next on a TokenStream if it supports it.
func NextFromTokenStream(ts TokenStream) (Token, bool) {
	if nextor, ok := ts.(TokenStreamWithNext); ok {
		return nextor.Next()
	}
	// If the underlying type has a Next method accessible via the interface chain, try it
	if nextor, ok := ts.(interface{ Next() (Token, bool) }); ok {
		return nextor.Next()
	}
	// Fallback: try IncrementToken
	if ok, _ := ts.IncrementToken(); ok {
		return Token{}, true
	}
	return Token{}, false
}
