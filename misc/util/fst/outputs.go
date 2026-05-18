// Package fst implements org.apache.lucene.misc.util.fst.
package fst

import "encoding/binary"

// ListOfOutputs is the Outputs implementation that stores a slice of outputs
// per arc. Mirrors org.apache.lucene.misc.util.fst.ListOfOutputs.
type ListOfOutputs struct{}

// Add merges two lists into one (the second list extends the first).
func (ListOfOutputs) Add(a, b [][]byte) [][]byte {
	if len(a) == 0 {
		return append([][]byte(nil), b...)
	}
	if len(b) == 0 {
		return append([][]byte(nil), a...)
	}
	out := make([][]byte, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out
}

// UpToTwoPositiveIntOutputs is the Outputs that stores either one or two
// positive int64 values per arc. Mirrors
// org.apache.lucene.misc.util.fst.UpToTwoPositiveIntOutputs.
type UpToTwoPositiveIntOutputs struct{}

// Encode packs the supplied values into a varint stream; at most two values
// are supported.
func (UpToTwoPositiveIntOutputs) Encode(values []int64) []byte {
	if len(values) > 2 {
		values = values[:2]
	}
	var tmp [binary.MaxVarintLen64]byte
	out := make([]byte, 0, len(values)*binary.MaxVarintLen64)
	for _, v := range values {
		if v < 0 {
			v = 0
		}
		n := binary.PutUvarint(tmp[:], uint64(v))
		out = append(out, tmp[:n]...)
	}
	return out
}

// Decode reads back the values written by Encode.
func (UpToTwoPositiveIntOutputs) Decode(buf []byte) []int64 {
	var out []int64
	for len(out) < 2 && len(buf) > 0 {
		v, n := binary.Uvarint(buf)
		if n <= 0 {
			break
		}
		out = append(out, int64(v))
		buf = buf[n:]
	}
	return out
}
