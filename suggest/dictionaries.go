package suggest

import (
	"bufio"
	"io"
	"strings"
)

// SuggestDictionary is the contract every dictionary the suggest package
// consumes implements. Mirrors
// org.apache.lucene.search.suggest.Dictionary (the suggest one — distinct
// from the spell.Dictionary contract).
type SuggestDictionary interface {
	GetEntryIterator() (InputIterator, error)
}

// FileDictionary parses newline-delimited records of the form
// "term<TAB>weight<TAB>payload" from an io.Reader. Mirrors
// org.apache.lucene.search.suggest.FileDictionary.
type FileDictionary struct {
	source io.Reader
}

// NewFileDictionary builds a FileDictionary.
func NewFileDictionary(source io.Reader) *FileDictionary { return &FileDictionary{source: source} }

// GetEntryIterator returns the iterator over the file's records.
func (d *FileDictionary) GetEntryIterator() (InputIterator, error) {
	scanner := bufio.NewScanner(d.source)
	var terms [][]byte
	var weights []int64
	var payloads [][]byte
	hasPayload := false
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 3)
		terms = append(terms, []byte(fields[0]))
		weight := int64(1)
		if len(fields) > 1 {
			var w int64
			for _, c := range fields[1] {
				if c < '0' || c > '9' {
					w = 1
					break
				}
				w = w*10 + int64(c-'0')
			}
			weight = w
		}
		weights = append(weights, weight)
		if len(fields) > 2 {
			payloads = append(payloads, []byte(fields[2]))
			hasPayload = true
		} else {
			payloads = append(payloads, nil)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &sliceInputIterator{
		terms:      terms,
		weights:    weights,
		payloads:   payloads,
		hasPayload: hasPayload,
		idx:        -1,
	}, nil
}

// DocumentDictionary materialises an InputIterator from a slice of
// (term, weight, payload, contexts) tuples. Mirrors
// org.apache.lucene.search.suggest.DocumentDictionary in its lookup-time
// shape (the actual indexed-doc traversal lives elsewhere).
type DocumentDictionary struct {
	Entries []*DocumentDictionaryEntry
}

// DocumentDictionaryEntry is the per-document tuple.
type DocumentDictionaryEntry struct {
	Term     []byte
	Weight   int64
	Payload  []byte
	Contexts [][]byte
}

// NewDocumentDictionary builds the dictionary.
func NewDocumentDictionary(entries []*DocumentDictionaryEntry) *DocumentDictionary {
	return &DocumentDictionary{Entries: append([]*DocumentDictionaryEntry(nil), entries...)}
}

// GetEntryIterator returns a slice-backed iterator.
func (d *DocumentDictionary) GetEntryIterator() (InputIterator, error) {
	terms := make([][]byte, len(d.Entries))
	weights := make([]int64, len(d.Entries))
	payloads := make([][]byte, len(d.Entries))
	contexts := make([][][]byte, len(d.Entries))
	hasPayload := false
	hasContexts := false
	for i, e := range d.Entries {
		terms[i] = e.Term
		weights[i] = e.Weight
		payloads[i] = e.Payload
		contexts[i] = e.Contexts
		if e.Payload != nil {
			hasPayload = true
		}
		if len(e.Contexts) > 0 {
			hasContexts = true
		}
	}
	return &sliceInputIterator{
		terms:       terms,
		weights:     weights,
		payloads:    payloads,
		contexts:    contexts,
		hasPayload:  hasPayload,
		hasContexts: hasContexts,
		idx:         -1,
	}, nil
}

// DocumentValueSourceDictionary is the variant whose weights come from a
// ValueSource (DocValues field, expression, ...). Mirrors
// org.apache.lucene.search.suggest.DocumentValueSourceDictionary.
type DocumentValueSourceDictionary struct {
	Field       string
	WeightField string
	Entries     []*DocumentDictionaryEntry
}

// NewDocumentValueSourceDictionary builds the dictionary.
func NewDocumentValueSourceDictionary(field, weightField string, entries []*DocumentDictionaryEntry) *DocumentValueSourceDictionary {
	return &DocumentValueSourceDictionary{
		Field:       field,
		WeightField: weightField,
		Entries:     append([]*DocumentDictionaryEntry(nil), entries...),
	}
}

// GetEntryIterator returns the iterator.
func (d *DocumentValueSourceDictionary) GetEntryIterator() (InputIterator, error) {
	return (&DocumentDictionary{Entries: d.Entries}).GetEntryIterator()
}

type sliceInputIterator struct {
	terms       [][]byte
	weights     []int64
	payloads    [][]byte
	contexts    [][][]byte
	idx         int
	hasPayload  bool
	hasContexts bool
}

func (it *sliceInputIterator) Next() (term []byte, weight int64, payload []byte, contexts [][]byte, ok bool, err error) {
	it.idx++
	if it.idx >= len(it.terms) {
		return nil, 0, nil, nil, false, nil
	}
	var p []byte
	if it.idx < len(it.payloads) {
		p = it.payloads[it.idx]
	}
	var c [][]byte
	if it.idx < len(it.contexts) {
		c = it.contexts[it.idx]
	}
	return it.terms[it.idx], it.weights[it.idx], p, c, true, nil
}

func (it *sliceInputIterator) HasPayloads() bool { return it.hasPayload }
func (it *sliceInputIterator) HasContexts() bool { return it.hasContexts }
