// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser

// CharStream is an interface for character streams used by the query parser.
// This is the Go port of Lucene's org.apache.lucene.queryparser.CharStream.
type CharStream interface {
	// ReadChar reads the next character from the stream.
	// Returns the character and any error encountered.
	ReadChar() (rune, error)

	// GetBeginLine returns the line number of the beginning of the current token.
	GetBeginLine() int

	// GetBeginColumn returns the column number of the beginning of the current token.
	GetBeginColumn() int

	// GetEndLine returns the line number of the end of the current token.
	GetEndLine() int

	// GetEndColumn returns the column number of the end of the current token.
	GetEndColumn() int

	// GetImage returns the string image of the current token.
	GetImage() string

	// GetBeginToken returns the character in the input stream at the beginning of the current token.
	GetBeginToken() int

	// GetEndToken returns the character in the input stream at the end of the current token.
	GetEndToken() int

	// Backup backs up the stream by the specified number of characters.
	Backup(amount int)

	// BeginToken marks the beginning of a token.
	BeginToken() rune

	// Done indicates that the stream is done being used.
	Done()

	// GetColumn returns the current column number.
	GetColumn() int

	// GetLine returns the current line number.
	GetLine() int

	// SetTabSize sets the tab size for column counting.
	SetTabSize(size int)

	// GetTabSize returns the current tab size.
	GetTabSize() int

	// SetTrackLineColumn sets whether to track line and column numbers.
	SetTrackLineColumn(track bool)

	// GetTrackLineColumn returns whether line and column tracking is enabled.
	GetTrackLineColumn() bool
}
