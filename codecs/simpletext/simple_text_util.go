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

// SimpleTextChecksumPrefix is the literal that opens the footer line of every
// SimpleText file. Port of the package-private constant
// org.apache.lucene.codecs.simpletext.SimpleTextUtil#CHECKSUM
// (SimpleTextUtil.java:33): {@code static final BytesRef CHECKSUM = new
// BytesRef("checksum ")}.
var SimpleTextChecksumPrefix = []byte("checksum ")

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
//
// Port of SimpleTextUtil.readLine(DataInput, BytesRefBuilder)
// (SimpleTextUtil.java:54). The parameter is a DataInput in Java, so it is
// store.DataInput here: the callers hand it a *store.ChecksumIndexInput, which
// is a DataInput but not an IndexInput.
func ReadLine(in store.DataInput) ([]byte, error) {
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
	// Java calls the abstract IndexOutput.getChecksum() (SimpleTextUtil.java:76).
	// Gocene's store.IndexOutput does not declare it, so the checksum is reached
	// through the same narrow assertion codecs.WriteCRC uses
	// (codecs/codec_util.go:27).
	cw, ok := out.(checksumWriter)
	if !ok {
		return fmt.Errorf("SimpleTextUtil.WriteChecksum: output %T does not support checksums", out)
	}
	checksum := fmt.Sprintf("%020d", cw.GetChecksum())
	if err := out.WriteBytes(SimpleTextChecksumPrefix, 0, len(SimpleTextChecksumPrefix)); err != nil {
		return err
	}
	if err := out.WriteBytes([]byte(checksum), 0, len(checksum)); err != nil {
		return err
	}
	return WriteNewline(out)
}

// checksumWriter is the subset of store.IndexOutput that exposes the running
// checksum. It renders the abstract member
// org.apache.lucene.store.IndexOutput#getChecksum(), which Gocene's
// store.IndexOutput interface does not declare; codecs/codec_util.go carries
// the same assertion for CodecUtil.writeCRC.
type checksumWriter interface {
	store.IndexOutput
	GetChecksum() uint32
}

// CheckFooter verifies the checksum at the end of the file.
//
// Port of SimpleTextUtil.checkFooter(ChecksumIndexInput)
// (SimpleTextUtil.java:82).
func CheckFooter(in *store.ChecksumIndexInput) error {
	line, err := ReadLine(in)
	if err != nil {
		return err
	}

	if len(line) < len(SimpleTextChecksumPrefix) {
		return fmt.Errorf("SimpleText failure: expected checksum line")
	}

	if string(line[:len(SimpleTextChecksumPrefix)]) != string(SimpleTextChecksumPrefix) {
		return fmt.Errorf("SimpleText failure: expected checksum line")
	}

	actualChecksum := string(line[len(SimpleTextChecksumPrefix):])
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
//
// Port of SimpleTextUtil.fromBytesRefString(String) (SimpleTextUtil.java:105).
// Java throws IllegalArgumentException on a malformed string; Gocene returns
// the error so the caller's read path can surface it as a corrupt index.
func FromBytesRefString(s string) ([]byte, error) {
	if len(s) < 2 {
		return nil, fmt.Errorf("string %s was not created from BytesRef.toString?", s)
	}
	if s[0] != '[' || s[len(s)-1] != ']' {
		return nil, fmt.Errorf("string %s was not created from BytesRef.toString?", s)
	}
	if len(s) == 2 {
		return []byte{}, nil
	}
	parts := strings.Split(s[1:len(s)-1], " ")
	res := make([]byte, len(parts))
	for i, p := range parts {
		val, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid hex byte in BytesRef string %s: %w", s, err)
		}
		res[i] = byte(val)
	}
	return res, nil
}

// BytesRefString renders b the way org.apache.lucene.util.BytesRef#toString()
// does — "[" plus the unpadded lower-case hex of each byte, space separated,
// plus "]" — which is the exact inverse of [FromBytesRefString] and the form
// SimpleTextSegmentInfoFormat.write puts on disk for the segment id and for the
// serialised sort fields (SimpleTextSegmentInfoFormat.java:316,342).
//
// It is a free function rather than a method because Gocene's
// util.BytesRef.String() diverges from Java: it returns the UTF-8 text of the
// bytes, not this hex form. Correcting that method would change every %v of a
// BytesRef in the tree, so the SimpleText on-disk contract is served here.
func BytesRefString(b []byte) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, bx := range b {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(strconv.FormatUint(uint64(bx), 16))
	}
	sb.WriteByte(']')
	return sb.String()
}
