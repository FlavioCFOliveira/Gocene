// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	newline = 10
	escape  = 92
)

var checksumPrefix = []byte("checksum ")

// Write writes a string to the output, escaping newlines and backslashes.
func Write(out store.IndexOutput, s string) error {
	b := []byte(s)
	for _, bx := range b {
		if bx == newline || bx == escape {
			if err := out.WriteByte(escape); err != nil {
				return err
			}
		}
		if err := out.WriteByte(bx); err != nil {
			return err
		}
	}
	return nil
}

// WriteNewline writes a newline character to the output.
func WriteNewline(out store.IndexOutput) error {
	return out.WriteByte(newline)
}

// ReadLine reads a line from the input, handling escapes.
func ReadLine(in store.IndexInput) ([]byte, error) {
	var res []byte
	for {
		b, err := in.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == escape {
			next, err := in.ReadByte()
			if err != nil {
				return nil, err
			}
			res = append(res, next)
		} else if b == newline {
			break
		} else {
			res = append(res, b)
		}
	}
	return res, nil
}

// WriteChecksum writes the checksum of the output to the end of the file.
func WriteChecksum(out store.IndexOutput) error {
	checksum := fmt.Sprintf("%020d", out.GetChecksum())
	if err := out.WriteBytes(checksumPrefix); err != nil {
		return err
	}
	if err := out.WriteBytes([]byte(checksum)); err != nil {
		return err
	}
	return WriteNewline(out)
}

// CheckFooter verifies the checksum at the end of the file.
func CheckFooter(in store.ChecksumIndexInput) error {
	line, err := ReadLine(in)
	if err != nil {
		return err
	}

	if len(line) < len(checksumPrefix) {
		return fmt.Errorf("SimpleText failure: expected checksum line")
	}

	if string(line[:len(checksumPrefix)]) != string(checksumPrefix) {
		return fmt.Errorf("SimpleText failure: expected checksum line")
	}

	actualChecksum := string(line[len(checksumPrefix):])
	expectedChecksum := fmt.Sprintf("%020d", in.GetChecksum())

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("SimpleText checksum failure: %s != %s", actualChecksum, expectedChecksum)
	}

	if in.Length() != in.GetFilePointer() {
		return fmt.Errorf("Unexpected stuff at the end of file")
	}

	return nil
}

// FromBytesRefString converts a string representation of a BytesRef back to bytes.
func FromBytesRefString(s string) []byte {
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		panic("string was not created from BytesRef.toString()")
	}
	if len(s) == 2 {
		return []byte{}
	}
	content := s[1 : len(s)-1]
	parts := strings.Fields(content)
	res := make([]byte, len(parts))
	for i, p := range parts {
		val, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			panic(fmt.Sprintf("invalid hex byte in BytesRef string: %s", p))
		}
		res[i] = byte(val)
	}
	return res
}
