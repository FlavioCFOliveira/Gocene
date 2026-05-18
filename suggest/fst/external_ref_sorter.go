package fst

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"sort"
)

// ExternalRefSorter spills accumulated byte-slices to a temporary file and
// streams them back sorted. Mirrors
// org.apache.lucene.search.suggest.fst.ExternalRefSorter.
type ExternalRefSorter struct {
	tmp *os.File
	w   *bufio.Writer
	cnt int
}

// NewExternalRefSorter opens a fresh temporary file.
func NewExternalRefSorter() (*ExternalRefSorter, error) {
	tmp, err := os.CreateTemp("", "gocene-extsort-*")
	if err != nil {
		return nil, err
	}
	return &ExternalRefSorter{tmp: tmp, w: bufio.NewWriter(tmp)}, nil
}

// Add appends item to the spill file.
func (s *ExternalRefSorter) Add(item []byte) error {
	var lenBuf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(lenBuf[:], uint64(len(item)))
	if _, err := s.w.Write(lenBuf[:n]); err != nil {
		return err
	}
	if _, err := s.w.Write(item); err != nil {
		return err
	}
	s.cnt++
	return nil
}

// Iterate flushes the spill file, reads every record back, sorts them, and
// returns the sorted slice.
func (s *ExternalRefSorter) Iterate() ([][]byte, error) {
	if err := s.w.Flush(); err != nil {
		return nil, err
	}
	if _, err := s.tmp.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	r := bufio.NewReader(s.tmp)
	out := make([][]byte, 0, s.cnt)
	for {
		length, err := binary.ReadUvarint(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		b := make([]byte, length)
		if _, err := io.ReadFull(r, b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	sort.SliceStable(out, func(i, j int) bool { return bytes.Compare(out[i], out[j]) < 0 })
	return out, nil
}

// Close releases the temp file.
func (s *ExternalRefSorter) Close() error {
	if err := s.tmp.Close(); err != nil {
		return err
	}
	return os.Remove(s.tmp.Name())
}

var _ BytesRefSorter = (*ExternalRefSorter)(nil)
