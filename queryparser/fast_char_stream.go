// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser

import (
	"fmt"
	"io"
	"strings"
)

// FastCharStream is a fast character stream implementation for the query parser.
// This is the Go port of Lucene's org.apache.lucene.queryparser.FastCharStream.
type FastCharStream struct {
	input     string
	pos       int
	length    int
	buffer    []rune
	bufPos    int
	bufLength int

	// Token tracking
	beginLine   int
	beginColumn int
	endLine     int
	endColumn   int
	beginToken  int
	endToken    int

	// Current position tracking
	line   int
	column int

	// Configuration
	tabSize          int
	trackLineColumn  bool
}

// NewFastCharStream creates a new FastCharStream from a string.
func NewFastCharStream(input string) *FastCharStream {
	return &FastCharStream{
		input:           input,
		pos:             0,
		length:          len(input),
		buffer:          []rune(input),
		bufLength:       len([]rune(input)),
		line:            1,
		column:          1,
		tabSize:         8,
		trackLineColumn: true,
	}
}

// NewFastCharStreamFromReader creates a new FastCharStream from an io.Reader.
func NewFastCharStreamFromReader(reader io.Reader) (*FastCharStream, error) {
	var sb strings.Builder
	if _, err := io.Copy(&sb, reader); err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	return NewFastCharStream(sb.String()), nil
}

// ReadChar reads the next character from the stream.
func (cs *FastCharStream) ReadChar() (rune, error) {
	if cs.bufPos >= cs.bufLength {
		return 0, io.EOF
	}

	ch := cs.buffer[cs.bufPos]
	cs.bufPos++

	if cs.trackLineColumn {
		if ch == '\n' {
			cs.line++
			cs.column = 1
		} else if ch == '\t' {
			cs.column += cs.tabSize - (cs.column-1)%cs.tabSize
		} else {
			cs.column++
		}
	}

	return ch, nil
}

// GetBeginLine returns the line number of the beginning of the current token.
func (cs *FastCharStream) GetBeginLine() int {
	return cs.beginLine
}

// GetBeginColumn returns the column number of the beginning of the current token.
func (cs *FastCharStream) GetBeginColumn() int {
	return cs.beginColumn
}

// GetEndLine returns the line number of the end of the current token.
func (cs *FastCharStream) GetEndLine() int {
	return cs.endLine
}

// GetEndColumn returns the column number of the end of the current token.
func (cs *FastCharStream) GetEndColumn() int {
	return cs.endColumn
}

// GetImage returns the string image of the current token.
func (cs *FastCharStream) GetImage() string {
	if cs.beginToken >= cs.bufLength || cs.endToken > cs.bufLength {
		return ""
	}
	return string(cs.buffer[cs.beginToken:cs.endToken])
}

// GetBeginToken returns the character position at the beginning of the current token.
func (cs *FastCharStream) GetBeginToken() int {
	return cs.beginToken
}

// GetEndToken returns the character position at the end of the current token.
func (cs *FastCharStream) GetEndToken() int {
	return cs.endToken
}

// Backup backs up the stream by the specified number of characters.
func (cs *FastCharStream) Backup(amount int) {
	for i := 0; i < amount && cs.bufPos > 0; i++ {
		cs.bufPos--
		if cs.trackLineColumn {
			ch := cs.buffer[cs.bufPos]
			if ch == '\n' {
				// Recalculate column - this is simplified
				cs.line--
				cs.column = 1
			} else if ch == '\t' {
				cs.column = cs.column - cs.tabSize
				if cs.column < 1 {
					cs.column = 1
				}
			} else {
				cs.column--
			}
		}
	}
}

// BeginToken marks the beginning of a token.
func (cs *FastCharStream) BeginToken() rune {
	cs.beginToken = cs.bufPos
	cs.beginLine = cs.line
	cs.beginColumn = cs.column

	if cs.bufPos >= cs.bufLength {
		return 0
	}

	ch := cs.buffer[cs.bufPos]
	cs.bufPos++

	if cs.trackLineColumn {
		if ch == '\n' {
			cs.line++
			cs.column = 1
		} else if ch == '\t' {
			cs.column += cs.tabSize - (cs.column-1)%cs.tabSize
		} else {
			cs.column++
		}
	}

	return ch
}

// Done indicates that the stream is done being used.
func (cs *FastCharStream) Done() {
	// Cleanup if needed
	cs.buffer = nil
}

// GetColumn returns the current column number.
func (cs *FastCharStream) GetColumn() int {
	return cs.column
}

// GetLine returns the current line number.
func (cs *FastCharStream) GetLine() int {
	return cs.line
}

// SetTabSize sets the tab size for column counting.
func (cs *FastCharStream) SetTabSize(size int) {
	if size > 0 {
		cs.tabSize = size
	}
}

// GetTabSize returns the current tab size.
func (cs *FastCharStream) GetTabSize() int {
	return cs.tabSize
}

// SetTrackLineColumn sets whether to track line and column numbers.
func (cs *FastCharStream) SetTrackLineColumn(track bool) {
	cs.trackLineColumn = track
}

// GetTrackLineColumn returns whether line and column tracking is enabled.
func (cs *FastCharStream) GetTrackLineColumn() bool {
	return cs.trackLineColumn
}

// EndToken marks the end of the current token.
func (cs *FastCharStream) EndToken() {
	cs.endToken = cs.bufPos
	cs.endLine = cs.line
	cs.endColumn = cs.column
}

// Reset resets the stream to the beginning.
func (cs *FastCharStream) Reset(input string) {
	cs.input = input
	cs.pos = 0
	cs.length = len(input)
	cs.buffer = []rune(input)
	cs.bufPos = 0
	cs.bufLength = len(cs.buffer)
	cs.line = 1
	cs.column = 1
	cs.beginLine = 1
	cs.beginColumn = 1
	cs.endLine = 1
	cs.endColumn = 1
	cs.beginToken = 0
	cs.endToken = 0
}

// GetInput returns the original input string.
func (cs *FastCharStream) GetInput() string {
	return cs.input
}

// GetPosition returns the current position in the buffer.
func (cs *FastCharStream) GetPosition() int {
	return cs.bufPos
}

// SetPosition sets the current position in the buffer.
func (cs *FastCharStream) SetPosition(pos int) {
	if pos >= 0 && pos <= cs.bufLength {
		cs.bufPos = pos
	}
}

// Ensure FastCharStream implements CharStream
var _ CharStream = (*FastCharStream)(nil)
