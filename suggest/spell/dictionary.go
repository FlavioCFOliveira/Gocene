package spell

import (
	"bufio"
	"io"
	"strings"
)

// Dictionary is the contract any spell-check dictionary implements. Mirrors
// org.apache.lucene.search.spell.Dictionary: produces a stream of input
// (term, weight) tuples.
type Dictionary interface {
	// GetEntryIterator returns an iterator over (term, weight) tuples.
	GetEntryIterator() DictionaryEntryIterator
}

// DictionaryEntryIterator walks the dictionary's entries.
type DictionaryEntryIterator interface {
	Next() (term string, weight int64, ok bool, err error)
}

// PlainTextDictionary reads newline-delimited terms from an io.Reader.
// Mirrors org.apache.lucene.search.spell.PlainTextDictionary.
type PlainTextDictionary struct {
	source io.Reader
}

// NewPlainTextDictionary builds the dictionary.
func NewPlainTextDictionary(source io.Reader) *PlainTextDictionary {
	return &PlainTextDictionary{source: source}
}

// GetEntryIterator returns an iterator that streams the wrapped reader.
func (d *PlainTextDictionary) GetEntryIterator() DictionaryEntryIterator {
	return &plainTextIterator{scanner: bufio.NewScanner(d.source)}
}

type plainTextIterator struct {
	scanner *bufio.Scanner
}

func (it *plainTextIterator) Next() (string, int64, bool, error) {
	if !it.scanner.Scan() {
		if err := it.scanner.Err(); err != nil {
			return "", 0, false, err
		}
		return "", 0, false, nil
	}
	line := strings.TrimSpace(it.scanner.Text())
	if line == "" {
		return it.Next()
	}
	return line, 1, true, nil
}

var _ Dictionary = (*PlainTextDictionary)(nil)

// LuceneDictionary streams terms from a per-field Terms snapshot. The Go
// port keeps the source opaque (TermsProvider) so any concrete index or
// memory-index backing works. Mirrors
// org.apache.lucene.search.spell.LuceneDictionary.
type LuceneDictionary struct {
	Field    string
	provider TermsProvider
}

// TermsProvider supplies the (term, frequency) pairs LuceneDictionary needs.
type TermsProvider interface {
	IterateTerms(field string) DictionaryEntryIterator
}

// NewLuceneDictionary builds a LuceneDictionary.
func NewLuceneDictionary(field string, provider TermsProvider) *LuceneDictionary {
	return &LuceneDictionary{Field: field, provider: provider}
}

// GetEntryIterator delegates to the wrapped provider.
func (d *LuceneDictionary) GetEntryIterator() DictionaryEntryIterator {
	if d.provider == nil {
		return &emptyIterator{}
	}
	return d.provider.IterateTerms(d.Field)
}

var _ Dictionary = (*LuceneDictionary)(nil)

// HighFrequencyDictionary filters another dictionary by a minimum frequency.
// Mirrors org.apache.lucene.search.spell.HighFrequencyDictionary.
type HighFrequencyDictionary struct {
	Inner        Dictionary
	MinFrequency int64
}

// NewHighFrequencyDictionary builds the wrapper.
func NewHighFrequencyDictionary(inner Dictionary, minFreq int64) *HighFrequencyDictionary {
	return &HighFrequencyDictionary{Inner: inner, MinFrequency: minFreq}
}

// GetEntryIterator returns an iterator that drops below-threshold entries.
func (d *HighFrequencyDictionary) GetEntryIterator() DictionaryEntryIterator {
	return &highFreqIterator{inner: d.Inner.GetEntryIterator(), min: d.MinFrequency}
}

type highFreqIterator struct {
	inner DictionaryEntryIterator
	min   int64
}

func (it *highFreqIterator) Next() (string, int64, bool, error) {
	for {
		term, w, ok, err := it.inner.Next()
		if err != nil || !ok {
			return term, w, ok, err
		}
		if w >= it.min {
			return term, w, true, nil
		}
	}
}

var _ Dictionary = (*HighFrequencyDictionary)(nil)

type emptyIterator struct{}

func (*emptyIterator) Next() (string, int64, bool, error) { return "", 0, false, nil }
