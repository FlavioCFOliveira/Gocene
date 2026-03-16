// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockTreeTermsWriter writes terms to a block tree structure.
// This is the Go port of Lucene's BlockTreeTermsWriter.
// It provides efficient term dictionary writing using a trie-based index structure.
type BlockTreeTermsWriter struct {
	state      *SegmentWriteState
	fields     map[string]*blockTreeFieldWriter
	mu         sync.Mutex
	closed     bool

	// Output files
	timOut     store.IndexOutput  // Terms block file (.tim)
	tipOut     store.IndexOutput  // Terms index file (.tip)
}

// blockTreeFieldWriter handles writing for a single field
type blockTreeFieldWriter struct {
	fieldName    string
	indexOptions index.IndexOptions
	hasPayloads  bool
	terms        []*blockTreeTermEntry
}

// blockTreeTermEntry represents a single term entry to be written
type blockTreeTermEntry struct {
	termText     string
	docFreq      int
	totalTermFreq int64
	postings     []index.PostingsEnum
}

// NewBlockTreeTermsWriter creates a new BlockTreeTermsWriter.
// This is the Go port of BlockTreeTermsWriter constructor.
func NewBlockTreeTermsWriter(state *SegmentWriteState) (*BlockTreeTermsWriter, error) {
	segmentName := state.SegmentInfo.Name()
	suffix := state.SegmentSuffix
	if suffix != "" {
		suffix = "_" + suffix
	}

	// Create terms block file (.tim)
	timFileName := fmt.Sprintf("%s%s.tim", segmentName, suffix)
	timOut, err := state.Directory.CreateOutput(timFileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("failed to create terms block file %s: %w", timFileName, err)
	}

	// Create terms index file (.tip)
	tipFileName := fmt.Sprintf("%s%s.tip", segmentName, suffix)
	tipOut, err := state.Directory.CreateOutput(tipFileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		timOut.Close()
		return nil, fmt.Errorf("failed to create terms index file %s: %w", tipFileName, err)
	}

	writer := &BlockTreeTermsWriter{
		state:  state,
		fields: make(map[string]*blockTreeFieldWriter),
		timOut: timOut,
		tipOut: tipOut,
	}

	// Write headers
	if err := writer.writeHeaders(); err != nil {
		timOut.Close()
		tipOut.Close()
		return nil, err
	}

	return writer, nil
}

// writeHeaders writes the file headers.
func (w *BlockTreeTermsWriter) writeHeaders() error {
	// Write TIM header
	if err := store.WriteUint32(w.timOut, 0x54494D00); err != nil { // "TIM\0"
		return fmt.Errorf("failed to write TIM magic: %w", err)
	}
	if err := store.WriteUint32(w.timOut, 1); err != nil { // Version
		return fmt.Errorf("failed to write TIM version: %w", err)
	}

	// Write TIP header
	if err := store.WriteUint32(w.tipOut, 0x54495000); err != nil { // "TIP\0"
		return fmt.Errorf("failed to write TIP magic: %w", err)
	}
	if err := store.WriteUint32(w.tipOut, 1); err != nil { // Version
		return fmt.Errorf("failed to write TIP version: %w", err)
	}

	return nil
}

// Write writes a field's postings to the block tree structure.
// This implements the FieldsConsumer interface.
func (w *BlockTreeTermsWriter) Write(field string, terms index.Terms) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("BlockTreeTermsWriter is closed")
	}

	// Get or create field writer
	fieldWriter, ok := w.fields[field]
	if !ok {
		fieldWriter = &blockTreeFieldWriter{
			fieldName:    field,
			indexOptions: index.IndexOptionsDocsAndFreqsAndPositions, // Default
			hasPayloads:  false,
			terms:        make([]*blockTreeTermEntry, 0),
		}
		w.fields[field] = fieldWriter
	}

	// Iterate over all terms and collect data
	te, err := terms.GetIterator()
	if err != nil {
		return fmt.Errorf("failed to get terms iterator: %w", err)
	}

	for {
		term, err := te.Next()
		if err != nil {
			return fmt.Errorf("error iterating terms: %w", err)
		}
		if term == nil {
			break
		}

		// Get term statistics
		docFreq, err := te.DocFreq()
		if err != nil {
			return fmt.Errorf("error getting doc freq: %w", err)
		}

		totalTermFreq, err := te.TotalTermFreq()
		if err != nil {
			return fmt.Errorf("error getting total term freq: %w", err)
		}

		// Get postings for this term
		pe, err := te.Postings(0)
		if err != nil {
			return fmt.Errorf("error getting postings: %w", err)
		}

		entry := &blockTreeTermEntry{
			termText:      term.Text(),
			docFreq:       docFreq,
			totalTermFreq: totalTermFreq,
		}

		// Collect postings if available
		if pe != nil {
			entry.postings = append(entry.postings, pe)
		}

		fieldWriter.terms = append(fieldWriter.terms, entry)

		// Update field metadata from first term
		if len(fieldWriter.terms) == 1 {
			fieldWriter.indexOptions = index.IndexOptionsDocsAndFreqsAndPositions
			if terms.HasPayloads() {
				fieldWriter.hasPayloads = true
			}
		}
	}

	return nil
}

// Close writes all data to disk and releases resources.
// This implements the FieldsConsumer interface.
func (w *BlockTreeTermsWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	// Write field count to TIP
	if err := store.WriteVInt(w.tipOut, int32(len(w.fields))); err != nil {
		w.timOut.Close()
		w.tipOut.Close()
		return fmt.Errorf("failed to write field count: %w", err)
	}

	// Write each field's terms
	for fieldName, fieldWriter := range w.fields {
		if err := w.writeField(fieldName, fieldWriter); err != nil {
			w.timOut.Close()
			w.tipOut.Close()
			return err
		}
	}

	// Close output files
	if err := w.timOut.Close(); err != nil {
		w.tipOut.Close()
		return fmt.Errorf("failed to close TIM file: %w", err)
	}

	if err := w.tipOut.Close(); err != nil {
		return fmt.Errorf("failed to close TIP file: %w", err)
	}

	return nil
}

// writeField writes a single field's terms to disk.
func (w *BlockTreeTermsWriter) writeField(fieldName string, fieldWriter *blockTreeFieldWriter) error {
	// Write field name to TIP
	if err := store.WriteString(w.tipOut, fieldName); err != nil {
		return fmt.Errorf("failed to write field name: %w", err)
	}

	// Build trie from terms
	trieBuilder := w.buildTrie(fieldWriter.terms)

	// Write terms blocks to TIM and get root file pointer
	rootFP, err := w.writeTermsBlocks(fieldWriter.terms)
	if err != nil {
		return fmt.Errorf("failed to write terms blocks: %w", err)
	}

	// Write root file pointer to TIP
	if err := store.WriteVLong(w.tipOut, rootFP); err != nil {
		return fmt.Errorf("failed to write root file pointer: %w", err)
	}

	// Write number of terms
	if err := store.WriteVLong(w.tipOut, int64(len(fieldWriter.terms))); err != nil {
		return fmt.Errorf("failed to write term count: %w", err)
	}

	// Write index options
	if err := w.tipOut.WriteByte(byte(fieldWriter.indexOptions)); err != nil {
		return fmt.Errorf("failed to write index options: %w", err)
	}

	// Write has payloads flag
	hasPayloadsByte := byte(0)
	if fieldWriter.hasPayloads {
		hasPayloadsByte = 1
	}
	if err := w.tipOut.WriteByte(hasPayloadsByte); err != nil {
		return fmt.Errorf("failed to write has payloads flag: %w", err)
	}

	// Save trie to TIP for index
	if err := trieBuilder.Save(w.tipOut, w.timOut); err != nil {
		return fmt.Errorf("failed to save trie: %w", err)
	}

	_ = trieBuilder // Use the variable

	return nil
}

// buildTrie builds a trie from the given terms.
func (w *BlockTreeTermsWriter) buildTrie(terms []*blockTreeTermEntry) *TrieBuilder {
	if len(terms) == 0 {
		return nil
	}

	// Create trie from first term
	firstTerm := util.NewBytesRef([]byte(terms[0].termText))
	output := NewTrieOutput(0, true, nil)
	trieBuilder := BytesRefToTrie(firstTerm, output)

	// Append remaining terms
	for i := 1; i < len(terms); i++ {
		termBytes := util.NewBytesRef([]byte(terms[i].termText))
		otherTrie := BytesRefToTrie(termBytes, output)
		trieBuilder.Append(otherTrie)
	}

	return trieBuilder
}

// writeTermsBlocks writes the terms blocks to the TIM file.
func (w *BlockTreeTermsWriter) writeTermsBlocks(terms []*blockTreeTermEntry) (int64, error) {
	if len(terms) == 0 {
		return 0, nil
	}

	// Get current file pointer as root
	rootFP := w.timOut.GetFilePointer()

	// Write terms in blocks
	// For simplicity, write all terms in a single block for now
	// A more sophisticated implementation would group terms into blocks
	// based on shared prefixes

	// Write number of terms in this block
	if err := store.WriteVInt(w.timOut, int32(len(terms))); err != nil {
		return 0, fmt.Errorf("failed to write block term count: %w", err)
	}

	// Write each term
	for _, term := range terms {
		// Write term text
		if err := store.WriteString(w.timOut, term.termText); err != nil {
			return 0, fmt.Errorf("failed to write term text: %w", err)
		}

		// Write doc freq
		if err := store.WriteVInt(w.timOut, int32(term.docFreq)); err != nil {
			return 0, fmt.Errorf("failed to write doc freq: %w", err)
		}

		// Write total term freq
		if err := store.WriteVLong(w.timOut, term.totalTermFreq); err != nil {
			return 0, fmt.Errorf("failed to write total term freq: %w", err)
		}

		// Write postings
		if err := w.writePostings(term.postings); err != nil {
			return 0, fmt.Errorf("failed to write postings: %w", err)
		}
	}

	return rootFP, nil
}

// writePostings writes postings data for a term.
func (w *BlockTreeTermsWriter) writePostings(postings []index.PostingsEnum) error {
	// Write number of postings
	if err := store.WriteVInt(w.timOut, int32(len(postings))); err != nil {
		return fmt.Errorf("failed to write posting count: %w", err)
	}

	// Write each posting
	for _, pe := range postings {
		if pe == nil {
			continue
		}

		// Iterate through documents
		for {
			docID, err := pe.NextDoc()
			if err != nil {
				return fmt.Errorf("error iterating postings: %w", err)
			}
			if docID == index.NO_MORE_DOCS {
				break
			}

			// Write doc ID
			if err := store.WriteVInt(w.timOut, int32(docID)); err != nil {
				return fmt.Errorf("failed to write doc ID: %w", err)
			}

			// Write frequency
			freq, err := pe.Freq()
			if err != nil {
				return fmt.Errorf("error getting freq: %w", err)
			}
			if err := store.WriteVInt(w.timOut, int32(freq)); err != nil {
				return fmt.Errorf("failed to write freq: %w", err)
			}

			// Write positions if available
			for i := 0; i < freq; i++ {
				pos, err := pe.NextPosition()
				if err != nil {
					return fmt.Errorf("error getting position: %w", err)
				}
				if pos == index.NO_MORE_POSITIONS {
					break
				}
				if err := store.WriteVInt(w.timOut, int32(pos)); err != nil {
					return fmt.Errorf("failed to write position: %w", err)
				}
			}
		}
	}

	return nil
}

// Ensure BlockTreeTermsWriter implements FieldsConsumer
var _ FieldsConsumer = (*BlockTreeTermsWriter)(nil)
