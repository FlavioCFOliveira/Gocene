package suggest

import (
	"bufio"
	"io"
	"os"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Dictionary is the interface for a dictionary of words.
type Dictionary interface {
	GetEntryIterator() (InputIterator, error)
}

// InputIterator is an iterator over dictionary entries.
type InputIterator interface {
	Next() (*util.BytesRef, error)
}

// PlainTextDictionary represents a dictionary as a text file.
type PlainTextDictionary struct {
	scanner *bufio.Scanner
	file    *os.File
}

func NewPlainTextDictionary(path string) (*PlainTextDictionary, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &PlainTextDictionary{
		scanner: bufio.NewScanner(f),
		file:    f,
	}, nil
}

func NewPlainTextDictionaryReader(r io.Reader) *PlainTextDictionary {
	// We use a dummy file for closure
	return &PlainTextDictionary{
		scanner: bufio.NewScanner(r),
	}
}

func (d *PlainTextDictionary) GetEntryIterator() (InputIterator, error) {
	return &plainTextIterator{
		dict: d,
	}, nil
}

func (d *PlainTextDictionary) Close() error {
	if d.file != nil {
		return d.file.Close()
	}
	return nil
}

type plainTextIterator struct {
	dict *PlainTextDictionary
}

func (it *plainTextIterator) Next() (*util.BytesRef, error) {
	if it.dict.scanner.Scan() {
		line := it.dict.scanner.Text()
		return util.NewBytesRef([]byte(line)), nil
	}
	if err := it.dict.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}
