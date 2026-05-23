// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SimpleText on-disk constants.
const (
	// SimpleTextNewline is the line terminator used in all SimpleText files.
	SimpleTextNewline byte = 10
	// SimpleTextEscape is the escape prefix for embedded newline or escape
	// bytes within a field value.
	SimpleTextEscape byte = 92
)

// SimpleTextChecksumPrefix is the byte prefix of the trailing checksum line.
var SimpleTextChecksumPrefix = []byte("checksum ")

// SimpleTextUtil exposes the canonical plain-text I/O helpers shared by all
// SimpleText codec components. The inline helpers (stWrite, stReadLine, etc.)
// scattered across the simpletext package delegate to these functions.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextUtil (Lucene 10.4.0).
type SimpleTextUtil struct{}

// Write encodes b into out with escape processing: each NEWLINE (10) and
// ESCAPE (92) byte in b is preceded by an ESCAPE byte.
//
// Port of SimpleTextUtil.write(DataOutput, BytesRef).
func (SimpleTextUtil) Write(out store.DataOutput, b []byte) error {
	for _, bx := range b {
		if bx == SimpleTextNewline || bx == SimpleTextEscape {
			if err := out.WriteByte(SimpleTextEscape); err != nil {
				return err
			}
		}
		if err := out.WriteByte(bx); err != nil {
			return err
		}
	}
	return nil
}

// WriteString converts s to UTF-8 bytes, stores them in scratch, then
// delegates to Write.
//
// Port of SimpleTextUtil.write(DataOutput, String, BytesRefBuilder).
func (SimpleTextUtil) WriteString(out store.DataOutput, s string, scratch *util.BytesRefBuilder) error {
	scratch.CopyChars(s)
	return SimpleTextUtil{}.Write(out, scratch.Bytes()[:scratch.Length()])
}

// WriteNewline writes a single NEWLINE (10) byte.
//
// Port of SimpleTextUtil.writeNewline(DataOutput).
func (SimpleTextUtil) WriteNewline(out store.DataOutput) error {
	return out.WriteByte(SimpleTextNewline)
}

// ReadLine reads one newline-terminated, escape-processed line from in into
// scratch. The NEWLINE terminator is consumed but not stored in scratch.
//
// Port of SimpleTextUtil.readLine(DataInput, BytesRefBuilder).
func (SimpleTextUtil) ReadLine(in store.DataInput, scratch *util.BytesRefBuilder) error {
	upto := 0
	for {
		b, err := in.ReadByte()
		if err != nil {
			return err
		}
		if b == SimpleTextEscape {
			esc, err2 := in.ReadByte()
			if err2 != nil {
				return err2
			}
			scratch.Grow(upto + 1)
			scratch.SetByteAt(upto, esc)
			upto++
		} else if b == SimpleTextNewline {
			break
		} else {
			scratch.Grow(upto + 1)
			scratch.SetByteAt(upto, b)
			upto++
		}
	}
	scratch.SetLength(upto)
	return nil
}

// WriteChecksum writes the trailing "checksum NNNNNNNNNNNNNNNNNNNN\n" line.
// The checksum value is zero-padded to 20 decimal digits so that different
// checksum values always occupy the same number of bytes on disk.
//
// Port of SimpleTextUtil.writeChecksum(IndexOutput, BytesRefBuilder).
func (SimpleTextUtil) WriteChecksum(out *store.ChecksumIndexOutput, scratch *util.BytesRefBuilder) error {
	cs := fmt.Sprintf("%020d", out.GetChecksum())
	u := SimpleTextUtil{}
	if err := u.Write(out, SimpleTextChecksumPrefix); err != nil {
		return err
	}
	if err := u.WriteString(out, cs, scratch); err != nil {
		return err
	}
	return u.WriteNewline(out)
}

// CheckFooter validates the trailing checksum line of a SimpleText file.
// It reads one line from input, verifies the "checksum " prefix, compares
// the encoded value against the running CRC32, and checks that the file
// pointer is at EOF.
//
// Port of SimpleTextUtil.checkFooter(ChecksumIndexInput).
func (SimpleTextUtil) CheckFooter(input *store.ChecksumIndexInput) error {
	scratch := util.NewBytesRefBuilder()
	u := SimpleTextUtil{}
	if err := u.ReadLine(input, scratch); err != nil {
		return fmt.Errorf("SimpleTextUtil.CheckFooter: readLine: %w", err)
	}
	line := scratch.Bytes()[:scratch.Length()]
	if len(line) < len(SimpleTextChecksumPrefix) {
		return fmt.Errorf("SimpleTextUtil.CheckFooter: expected checksum line, got: %s", line)
	}
	for i, c := range SimpleTextChecksumPrefix {
		if line[i] != c {
			return fmt.Errorf("SimpleTextUtil.CheckFooter: expected checksum line, got: %s", line)
		}
	}
	expectedCS := fmt.Sprintf("%020d", input.GetChecksum())
	actualCS := string(line[len(SimpleTextChecksumPrefix):])
	if expectedCS != actualCS {
		return fmt.Errorf("SimpleTextUtil.CheckFooter: checksum mismatch: expected %s, got %s",
			expectedCS, actualCS)
	}
	length := input.Length()
	if length != input.GetFilePointer() {
		return fmt.Errorf(
			"SimpleTextUtil.CheckFooter: unexpected trailing data at position %d of %d",
			input.GetFilePointer(), length)
	}
	return nil
}

// FromBytesRefString parses the hex-encoded BytesRef string produced by
// Java's BytesRef.toString() (e.g. "[0 1f a3]") and returns the decoded
// bytes. Returns an error if the format is invalid.
//
// Port of SimpleTextUtil.fromBytesRefString(String).
func (SimpleTextUtil) FromBytesRefString(s string) ([]byte, error) {
	if len(s) < 2 {
		return nil, fmt.Errorf("SimpleTextUtil.FromBytesRefString: too short: %q", s)
	}
	if s[0] != '[' || s[len(s)-1] != ']' {
		return nil, fmt.Errorf("SimpleTextUtil.FromBytesRefString: not a BytesRef string: %q", s)
	}
	if len(s) == 2 {
		return []byte{}, nil
	}
	parts := strings.Split(s[1:len(s)-1], " ")
	b := make([]byte, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return nil, fmt.Errorf("SimpleTextUtil.FromBytesRefString: parse %q: %w", p, err)
		}
		b[i] = byte(v)
	}
	return b, nil
}
