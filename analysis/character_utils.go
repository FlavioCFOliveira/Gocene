/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package analysis

import (
	"errors"
	"io"
	"unicode"
	"unicode/utf8"
)

// CharacterUtils provides utility functions for working with characters,
// including Unicode code point handling and case conversion.
// This is a port of Lucene's CharacterUtils class.
type CharacterUtils struct{}

// NewCharacterUtils creates a new CharacterUtils instance.
func NewCharacterUtils() *CharacterUtils {
	return &CharacterUtils{}
}

// CharacterBuffer is a simple I/O buffer for use with CharacterUtils.Fill.
// It handles surrogate pairs correctly across buffer boundaries.
type CharacterBuffer struct {
	buffer                    []rune
	offset                    int
	length                    int
	lastTrailingHighSurrogate rune
}

// NewCharacterBuffer creates a new CharacterBuffer with the specified size.
// The bufferSize must be >= 2 to handle surrogate pairs.
func NewCharacterBuffer(bufferSize int) (*CharacterBuffer, error) {
	if bufferSize < 2 {
		return nil, errors.New("buffersize must be >= 2")
	}
	return &CharacterBuffer{
		buffer: make([]rune, bufferSize),
		offset: 0,
		length: 0,
	}, nil
}

// GetBuffer returns the internal buffer.
func (cb *CharacterBuffer) GetBuffer() []rune {
	return cb.buffer
}

// GetOffset returns the data offset in the internal buffer.
func (cb *CharacterBuffer) GetOffset() int {
	return cb.offset
}

// GetLength returns the length of the data in the internal buffer.
func (cb *CharacterBuffer) GetLength() int {
	return cb.length
}

// Reset resets the CharacterBuffer to its default state.
func (cb *CharacterBuffer) Reset() {
	cb.offset = 0
	cb.length = 0
	cb.lastTrailingHighSurrogate = 0
}

// ToLowerCase converts each Unicode code point to lower case in the buffer
// starting at the given offset up to the limit.
// This modifies the buffer in place.
func ToLowerCase(buffer []rune, offset, limit int) {
	if offset < 0 || limit > len(buffer) || offset > limit {
		return
	}
	for i := offset; i < limit; {
		cp := buffer[i]
		lowerCp := unicode.ToLower(cp)
		if lowerCp != cp {
			buffer[i] = lowerCp
		}
		i++
	}
}

// ToUpperCase converts each Unicode code point to upper case in the buffer
// starting at the given offset up to the limit.
// This modifies the buffer in place.
func ToUpperCase(buffer []rune, offset, limit int) {
	if offset < 0 || limit > len(buffer) || offset > limit {
		return
	}
	for i := offset; i < limit; i++ {
		cp := buffer[i]
		upperCp := unicode.ToUpper(cp)
		if upperCp != cp {
			buffer[i] = upperCp
		}
	}
}

// ToCodePoints converts a sequence of runes (Java characters) to a sequence of Unicode code points.
// Returns the number of code points written to the destination buffer.
// In Go, each rune is already a Unicode code point, so this is essentially a copy.
func ToCodePoints(src []rune, srcOff, srcLen int, dest []int, destOff int) (int, error) {
	if srcLen < 0 {
		return 0, errors.New("srcLen must be >= 0")
	}
	codePointCount := 0
	for i := 0; i < srcLen; i++ {
		dest[destOff+codePointCount] = int(src[srcOff+i])
		codePointCount++
	}
	return codePointCount, nil
}

// ToChars converts a sequence of Unicode code points to a sequence of runes.
// Returns the number of runes written to the destination buffer.
func ToChars(src []int, srcOff, srcLen int, dest []rune, destOff int) (int, error) {
	if srcLen < 0 {
		return 0, errors.New("srcLen must be >= 0")
	}
	written := 0
	for i := 0; i < srcLen; i++ {
		dest[destOff+written] = rune(src[srcOff+i])
		written++
	}
	return written, nil
}

// Fill fills the CharacterBuffer with characters read from the given reader.
// This method handles surrogate pairs correctly, ensuring that high and low
// surrogate pairs are always preserved across buffer boundaries.
// Returns false if and only if the reader returned EOF while trying to fill the buffer.
func Fill(buffer *CharacterBuffer, reader io.RuneReader, numChars int) (bool, error) {
	if numChars < 2 || numChars > len(buffer.buffer) {
		return false, errors.New("numChars must be >= 2 and <= the buffer size")
	}

	buffer.offset = 0
	var offset int

	// Install the previously saved ending high surrogate
	if buffer.lastTrailingHighSurrogate != 0 {
		buffer.buffer[0] = buffer.lastTrailingHighSurrogate
		buffer.lastTrailingHighSurrogate = 0
		offset = 1
	} else {
		offset = 0
	}

	// Read characters
	read := 0
	for read < numChars-offset {
		ch, _, err := reader.ReadRune()
		if err != nil {
			if err == io.EOF {
				break
			}
			return false, err
		}
		buffer.buffer[offset+read] = ch
		read++
	}

	buffer.length = offset + read
	result := buffer.length == numChars

	if buffer.length < numChars {
		// We failed to fill the buffer. Even if the last char is a high surrogate,
		// there is nothing we can do
		return result, nil
	}

	// Check if the last character is a high surrogate
	lastChar := buffer.buffer[buffer.length-1]
	if isHighSurrogateRune(lastChar) {
		buffer.lastTrailingHighSurrogate = lastChar
		buffer.length--
	}

	return result, nil
}

// FillBuffer is a convenience method that calls Fill with the full buffer size.
func FillBuffer(buffer *CharacterBuffer, reader io.RuneReader) (bool, error) {
	return Fill(buffer, reader, len(buffer.buffer))
}

// ReadFully reads characters from the reader until the destination is filled or EOF.
func ReadFully(reader io.RuneReader, dest []rune, offset, length int) (int, error) {
	read := 0
	for read < length {
		ch, _, err := reader.ReadRune()
		if err != nil {
			if err == io.EOF {
				break
			}
			return read, err
		}
		dest[offset+read] = ch
		read++
	}
	return read, nil
}

// CodePointCount returns the number of Unicode code points in the given rune slice.
func CodePointCount(chars []rune) int {
	return len(chars)
}

// CodePointAt returns the Unicode code point at the given index in the rune slice.
// It handles surrogate pairs correctly.
func CodePointAt(chars []rune, index int) rune {
	if index < 0 || index >= len(chars) {
		return utf8.RuneError
	}
	return chars[index]
}

// CharCount returns the number of chars needed to represent the given code point.
func CharCount(codePoint rune) int {
	if codePoint >= 0x10000 {
		return 2
	}
	return 1
}

// IsHighSurrogate returns true if the given rune is a high surrogate.
func IsHighSurrogate(ch rune) bool {
	return ch >= 0xD800 && ch <= 0xDBFF
}

// IsLowSurrogate returns true if the given rune is a low surrogate.
func IsLowSurrogate(ch rune) bool {
	return ch >= 0xDC00 && ch <= 0xDFFF
}

// IsSurrogate returns true if the given rune is a surrogate.
func IsSurrogate(ch rune) bool {
	return IsHighSurrogate(ch) || IsLowSurrogate(ch)
}

// isHighSurrogateRune checks if the rune is a high surrogate (for internal use).
func isHighSurrogateRune(ch rune) bool {
	return ch >= 0xD800 && ch <= 0xDBFF
}