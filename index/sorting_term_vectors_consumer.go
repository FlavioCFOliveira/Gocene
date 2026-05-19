// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// SortingTermVectorsConsumer buffers per-document term vectors into a
// temporary segment, then on flush reorders the documents into the final
// segment according to the index sort produced by SorterDocMap.
//
// This is the Go port of Apache Lucene 10.4.0's
// org.apache.lucene.index.SortingTermVectorsConsumer (207 lines).
//
// Sprint 55 "option c" deviations (placeholders inlined for cross-task
// independence; replaced when prerequisite ports land):
//
//   - TermVectorsConsumer (GOC-3370) parent type is not yet ported. This
//     file declares an unexported termVectorsConsumerBase struct that
//     carries the codec/directory/info/writer state. The exported
//     SortingTermVectorsConsumer composes that base; once GOC-3370 lands,
//     the base is replaced by the canonical parent (consumer moves to
//     embedding rather than composition).
//
//   - Lucene90CompressingTermVectorsFormat (compressing module) is not yet
//     wired through the codec SPI from package index. The temporary format
//     is exposed as a hook that callers inject via
//     SetTempTermVectorsFormat. The default behavior, when no temporary
//     format is supplied, is to return ErrTempTermVectorsFormatUnset on
//     the first call to InitTermVectorsWriter — making the wiring gap
//     explicit instead of silent.
//
//   - trackingTmpDirectoryWrapper (declared in sorting_stored_fields_consumer.go)
//     is reused as the Sprint 55 stand-in for
//     TrackingTmpOutputDirectoryWrapper. The full Lucene wrapper (with
//     prefix management and CreateTempOutput integration) will replace
//     this stub in its own port task.
//
//   - The TermVectorsWriter contract in package index does not propagate a
//     per-term frequency to StartTerm (the freq is implicit from
//     AddPosition cardinality); Lucene's StartTerm(BytesRef, int) is
//     therefore reduced to StartTerm(BytesRef). When positions/offsets
//     are not present, Lucene relies on the freq passed to StartTerm to
//     reconstruct counts at read time. To preserve Lucene semantics here
//     we replay the docs/positions enum to emit position-less AddPosition
//     calls (with sentinel offsets -1/-1 and a nil payload), one per
//     freq, so the receiving writer sees the correct freq via cardinality.
//
//   - The Lucene parent flushes via super.flush(fieldsToFlush, state,
//     sortMap, norms) before the temp-to-final copy. Because the parent
//     port is deferred, the override here skips that call: there is no
//     buffered TermsHash state to drain in the Sprint 55 surface.
//     Documents written through the temp writer are the only state we
//     need to migrate.
//
// All public symbols mirror Lucene 10.4.0 semantics; deviations are
// scoped to internal plumbing that callers do not observe.

// ErrTempTermVectorsFormatUnset is returned by
// SortingTermVectorsConsumer.InitTermVectorsWriter / Flush when no
// temporary TermVectorsFormat has been supplied via
// SetTempTermVectorsFormat. This surfaces the Sprint 55 wiring gap
// explicitly instead of silently no-oping.
var ErrTempTermVectorsFormatUnset = errors.New("index: SortingTermVectorsConsumer requires a temporary TermVectorsFormat (see SetTempTermVectorsFormat); GOC-3370 / compressing wiring pending")

// termVectorsConsumerBase is the Sprint 55 placeholder for the parent
// type ported in GOC-3370 (TermVectorsConsumer). It carries the fields
// the parent owns in Lucene: codec, directory, segment info, the active
// writer, and the document-stream cursor (lastDocID).
//
// When GOC-3370 lands, this struct disappears: SortingTermVectorsConsumer
// embeds the canonical *TermVectorsConsumer and the field-access paths
// here switch to the embedded receiver. The migration is mechanical.
type termVectorsConsumerBase struct {
	codec     Codec
	directory store.Directory
	info      *SegmentInfo
	writer    TermVectorsWriter

	// lastDocID is the count of documents accepted by writer so far.
	// Lucene's parent maintains it for fill() and IOContext.flush(); the
	// sorting subclass resets it to 0 each time InitTermVectorsWriter
	// installs a fresh writer.
	lastDocID int
}

// SortingTermVectorsConsumer specializes the term-vectors consumer for
// segments that are sorted at flush time. Term vectors are first
// buffered in document-write order to a temporary segment, then
// reordered into the codec-provided TermVectorsWriter using the
// SorterDocMap supplied at flush.
//
// Mirrors org.apache.lucene.index.SortingTermVectorsConsumer.
type SortingTermVectorsConsumer struct {
	termVectorsConsumerBase

	// tmpDirectory is the tracking wrapper around the segment directory
	// where the buffered (pre-sort) term vectors live. nil until
	// InitTermVectorsWriter creates it.
	tmpDirectory *trackingTmpDirectoryWrapper

	// tempFormat is the TermVectorsFormat used for the buffered segment.
	// In Lucene this is Lucene90CompressingTermVectorsFormat with the
	// NO_COMPRESSION mode; until GOC-3370 + the compressing wiring land,
	// callers must inject it via SetTempTermVectorsFormat.
	tempFormat TermVectorsFormat
}

// NewSortingTermVectorsConsumer constructs the consumer for the given
// segment. It mirrors the Lucene constructor
// SortingTermVectorsConsumer(IntBlockPool.Allocator, ByteBlockPool.Allocator,
// Directory, SegmentInfo, Codec); the per-block allocators are owned by
// the (not-yet-ported) TermsHash parent and so are absent from the
// Sprint 55 signature.
//
// The temporary TermVectorsFormat used to buffer the pre-sort segment
// must be supplied separately via SetTempTermVectorsFormat before the
// first call to InitTermVectorsWriter; otherwise that call returns
// ErrTempTermVectorsFormatUnset. See the Sprint 55 deviation note on
// the type doc.
func NewSortingTermVectorsConsumer(codec Codec, directory store.Directory, info *SegmentInfo) *SortingTermVectorsConsumer {
	if info == nil {
		// Mirrors Lucene's reliance on a non-null SegmentInfo: there is
		// no useful behaviour the consumer can perform without it.
		return nil
	}
	return &SortingTermVectorsConsumer{
		termVectorsConsumerBase: termVectorsConsumerBase{
			codec:     codec,
			directory: directory,
			info:      info,
		},
	}
}

// SetTempTermVectorsFormat injects the TermVectorsFormat used for the
// temporary, pre-sort segment. This exists only to bridge the Sprint 55
// wiring gap; once Lucene90CompressingTermVectorsFormat is reachable
// from package index without import cycles (post GOC-3370), the field
// becomes a package-private constant initialised at package load and
// this setter is removed.
func (c *SortingTermVectorsConsumer) SetTempTermVectorsFormat(format TermVectorsFormat) {
	c.tempFormat = format
}

// TempDirectory returns the tracking wrapper around the segment
// directory that holds the temporary pre-sort term-vectors files.
// Returns nil if InitTermVectorsWriter has not run yet.
//
// Exposed (Lucene's field is package-private) for test inspection of
// the temporary-file lifecycle.
func (c *SortingTermVectorsConsumer) TempDirectory() *trackingTmpDirectoryWrapper {
	return c.tmpDirectory
}

// Writer returns the active TermVectorsWriter, or nil if
// InitTermVectorsWriter has not run yet. Exposed (Lucene's field is
// package-private) for test inspection and to let consumers stream
// vectors into the buffered writer without depending on the unported
// TermsHashPerField machinery.
func (c *SortingTermVectorsConsumer) Writer() TermVectorsWriter {
	return c.writer
}

// InitTermVectorsWriter lazily creates the temporary TermVectorsWriter
// the first time it is called. Subsequent calls are no-ops.
//
// Mirrors the package-private initTermVectorsWriter() in Lucene.
func (c *SortingTermVectorsConsumer) InitTermVectorsWriter() error {
	if c.writer != nil {
		return nil
	}
	if c.tempFormat == nil {
		return ErrTempTermVectorsFormatUnset
	}
	c.tmpDirectory = newTrackingTmpDirectoryWrapper(c.directory)
	state := &SegmentWriteState{
		Directory:   c.tmpDirectory,
		SegmentInfo: c.info,
	}
	w, err := c.tempFormat.VectorsWriter(state)
	if err != nil {
		c.tmpDirectory = nil
		return fmt.Errorf("index: SortingTermVectorsConsumer init temp writer: %w", err)
	}
	c.writer = w
	c.lastDocID = 0
	return nil
}

// Flush completes the buffered segment, then copies term vectors into
// the codec's TermVectorsWriter, reordering documents through sortMap
// when non-nil.
//
// Mirrors org.apache.lucene.index.SortingTermVectorsConsumer.flush.
//
// The integrity check that Lucene performs via reader.checkIntegrity()
// is elided here (the integrity contract belongs to the compressing
// reader port that lands with GOC-3370); structurally the call site is
// preserved by a comment so the future hook is obvious.
func (c *SortingTermVectorsConsumer) Flush(state *SegmentWriteState, sortMap SorterDocMap) error {
	if state == nil || state.SegmentInfo == nil {
		return errors.New("index: SortingTermVectorsConsumer.Flush requires a non-nil state with SegmentInfo")
	}
	if c.tempFormat == nil || c.tmpDirectory == nil || c.writer == nil {
		// Nothing was buffered. Mirrors the implicit no-op path Lucene
		// gets when initTermVectorsWriter was never invoked.
		return nil
	}

	// Close the temporary writer (super.flush() in Lucene flushes the
	// buffered writer; in the port we just close it before reopening
	// for read since TermVectorsWriter does not expose a Flush method).
	if err := c.writer.Close(); err != nil {
		c.cleanupTempFiles()
		return fmt.Errorf("index: SortingTermVectorsConsumer flush close temp writer: %w", err)
	}
	c.writer = nil

	reader, err := c.tempFormat.VectorsReader(c.tmpDirectory, state.SegmentInfo, state.FieldInfos, store.IOContextDefault)
	if err != nil {
		c.cleanupTempFiles()
		return fmt.Errorf("index: SortingTermVectorsConsumer flush open temp reader: %w", err)
	}

	// Don't pull a merge instance: term vectors are consumed in random
	// order here, not sequentially. (Mirrors the Lucene comment.)
	// reader.checkIntegrity() goes here when GOC-3370 lands.
	sortWriter, err := c.codec.TermVectorsFormat().VectorsWriter(state)
	if err != nil {
		_ = reader.Close()
		c.cleanupTempFiles()
		return fmt.Errorf("index: SortingTermVectorsConsumer flush open sort writer: %w", err)
	}

	flushErr := c.copyDocuments(reader, sortWriter, state.SegmentInfo.DocCount(), state.FieldInfos, sortMap)
	closeErr := closeAllTermVectors(reader, sortWriter)
	c.cleanupTempFiles()

	switch {
	case flushErr != nil:
		return flushErr
	case closeErr != nil:
		return fmt.Errorf("index: SortingTermVectorsConsumer flush close: %w", closeErr)
	}
	return nil
}

// copyDocuments walks the buffered reader in sorted order, copying every
// vector field of every document into the codec writer.
func (c *SortingTermVectorsConsumer) copyDocuments(reader TermVectorsReader, sortWriter TermVectorsWriter, maxDoc int, fieldInfos *FieldInfos, sortMap SorterDocMap) error {
	for docID := 0; docID < maxDoc; docID++ {
		sourceDoc := docID
		if sortMap != nil {
			sourceDoc = sortMap.NewToOld(docID)
		}
		vectors, err := reader.Get(sourceDoc)
		if err != nil {
			return fmt.Errorf("index: SortingTermVectorsConsumer flush read doc %d (source %d): %w", docID, sourceDoc, err)
		}
		if err := writeTermVectorsDoc(sortWriter, vectors, fieldInfos); err != nil {
			return fmt.Errorf("index: SortingTermVectorsConsumer flush write doc %d (source %d): %w", docID, sourceDoc, err)
		}
	}
	return nil
}

// Abort closes the in-flight writer and deletes any buffered temporary
// files, swallowing per-file errors as Lucene does.
//
// Mirrors org.apache.lucene.index.SortingTermVectorsConsumer.abort.
func (c *SortingTermVectorsConsumer) Abort() {
	if c.writer != nil {
		_ = c.writer.Close()
		c.writer = nil
	}
	if c.tmpDirectory != nil {
		for _, name := range c.tmpDirectory.TemporaryFiles() {
			_ = c.directory.DeleteFile(name)
		}
		c.tmpDirectory = nil
	}
}

// cleanupTempFiles is the flush-path equivalent of Abort's temp-file
// removal: it deletes every file the tracking wrapper recorded,
// ignoring individual delete errors, then clears the wrapper.
func (c *SortingTermVectorsConsumer) cleanupTempFiles() {
	if c.tmpDirectory == nil {
		return
	}
	for _, name := range c.tmpDirectory.TemporaryFiles() {
		_ = c.directory.DeleteFile(name)
	}
	c.tmpDirectory = nil
}

// closeAllTermVectors closes both arguments and returns the first
// non-nil error. Mirrors IOUtils.close semantics for the two-arg flush
// path. Distinct name from the stored-fields helper to keep the
// type-switch on the receiver unambiguous.
func closeAllTermVectors(reader TermVectorsReader, writer TermVectorsWriter) error {
	rerr := reader.Close()
	werr := writer.Close()
	if rerr != nil {
		return rerr
	}
	return werr
}

// writeTermVectorsDoc is the Go port of
// SortingTermVectorsConsumer.writeTermVectors (the private static
// helper). It copies every vector field for a single document into the
// supplied writer.
//
// Pre-conditions mirror Lucene:
//   - If vectors is nil, the document is emitted as an empty
//     start/finish pair.
//   - Field names visited via vectors.Iterator() must be ascending; the
//     port enforces the same invariant.
//
// Deviations from Lucene (Sprint 55):
//   - StartTerm is freq-less in the index-package contract; positions
//     and offsets are emitted via AddPosition calls whose cardinality
//     equals freq. When the term vector has neither positions nor
//     offsets, Lucene relies on the freq passed to StartTerm to encode
//     the term's count. The port emits freq sentinel AddPosition calls
//     (position -1, offsets -1/-1, nil payload) in that case so the
//     receiving writer can still count occurrences. Writers that
//     consume only AddPosition for cardinality (the in-tree
//     MemoryTermVectorsWriter equivalent) see the same count Lucene
//     would record.
//   - assertions on field/term ordering and counts mirror Lucene; on
//     violation the function returns an error rather than throwing
//     AssertionError.
func writeTermVectorsDoc(writer TermVectorsWriter, vectors Fields, fieldInfos *FieldInfos) error {
	if vectors == nil {
		if err := writer.StartDocument(0); err != nil {
			return fmt.Errorf("empty doc start: %w", err)
		}
		return writer.FinishDocument()
	}

	numFields, err := countFields(vectors)
	if err != nil {
		return err
	}

	if err := writer.StartDocument(numFields); err != nil {
		return fmt.Errorf("start doc (numFields=%d): %w", numFields, err)
	}

	iter, err := vectors.Iterator()
	if err != nil {
		return fmt.Errorf("vectors iterator: %w", err)
	}

	var lastFieldName string
	fieldCount := 0
	for {
		fieldName, err := iter.Next()
		if err != nil {
			return fmt.Errorf("vectors next: %w", err)
		}
		if fieldName == "" {
			break
		}
		fieldCount++
		if lastFieldName != "" && fieldName <= lastFieldName {
			return fmt.Errorf("vector fields not in ascending order: lastFieldName=%q fieldName=%q", lastFieldName, fieldName)
		}
		lastFieldName = fieldName

		var fieldInfo *FieldInfo
		if fieldInfos != nil {
			fieldInfo = fieldInfos.GetByName(fieldName)
		}

		terms, err := vectors.Terms(fieldName)
		if err != nil {
			return fmt.Errorf("vectors terms(%q): %w", fieldName, err)
		}
		if terms == nil {
			// Mirrors Lucene: "FieldsEnum shouldn't lie..."; skip silently.
			continue
		}

		hasPositions := terms.HasPositions()
		hasOffsets := terms.HasOffsets()
		hasPayloads := terms.HasPayloads()
		if hasPayloads && !hasPositions {
			return fmt.Errorf("field %q reports payloads without positions", fieldName)
		}

		numTerms, err := countTerms(terms)
		if err != nil {
			return fmt.Errorf("count terms for %q: %w", fieldName, err)
		}

		if err := writer.StartField(fieldInfo, numTerms, hasPositions, hasOffsets, hasPayloads); err != nil {
			return fmt.Errorf("start field %q: %w", fieldName, err)
		}

		termsEnum, err := terms.GetIterator()
		if err != nil {
			return fmt.Errorf("terms iterator for %q: %w", fieldName, err)
		}

		termCount := 0
		for {
			term, err := termsEnum.Next()
			if err != nil {
				return fmt.Errorf("terms.next for %q: %w", fieldName, err)
			}
			if term == nil {
				break
			}
			termCount++

			freq64, err := termsEnum.TotalTermFreq()
			if err != nil {
				return fmt.Errorf("totalTermFreq for %q: %w", fieldName, err)
			}
			freq := int(freq64)

			if err := writer.StartTerm(termBytes(term)); err != nil {
				return fmt.Errorf("start term in %q: %w", fieldName, err)
			}

			if err := emitPositions(writer, termsEnum, hasPositions, hasOffsets, hasPayloads, freq); err != nil {
				return fmt.Errorf("emit positions in %q: %w", fieldName, err)
			}

			if err := writer.FinishTerm(); err != nil {
				return fmt.Errorf("finish term in %q: %w", fieldName, err)
			}
		}
		if termCount != numTerms {
			return fmt.Errorf("field %q: termCount %d != numTerms %d", fieldName, termCount, numTerms)
		}

		if err := writer.FinishField(); err != nil {
			return fmt.Errorf("finish field %q: %w", fieldName, err)
		}
	}
	if fieldCount != numFields {
		return fmt.Errorf("fieldCount %d != numFields %d", fieldCount, numFields)
	}

	return writer.FinishDocument()
}

// countFields returns Fields.Size() when known, otherwise iterates to
// count the fields manually (mirrors Lucene's "FieldsEnum shouldn't
// lie" fallback path).
func countFields(vectors Fields) (int, error) {
	if n := vectors.Size(); n >= 0 {
		return n, nil
	}
	iter, err := vectors.Iterator()
	if err != nil {
		return 0, fmt.Errorf("count fields iterator: %w", err)
	}
	count := 0
	for {
		name, err := iter.Next()
		if err != nil {
			return 0, fmt.Errorf("count fields next: %w", err)
		}
		if name == "" {
			break
		}
		count++
	}
	return count, nil
}

// countTerms returns Terms.Size() when known and non-negative,
// otherwise iterates a fresh enum to count manually (mirrors Lucene's
// "Terms.size() is not mandatory" fallback).
func countTerms(terms Terms) (int, error) {
	if n := terms.Size(); n >= 0 {
		return int(n), nil
	}
	enum, err := terms.GetIterator()
	if err != nil {
		return 0, fmt.Errorf("count terms iterator: %w", err)
	}
	count := 0
	for {
		t, err := enum.Next()
		if err != nil {
			return 0, fmt.Errorf("count terms next: %w", err)
		}
		if t == nil {
			break
		}
		count++
	}
	return count, nil
}

// emitPositions reproduces the Lucene per-term inner loop: pull a
// PostingsEnum, walk freq positions, and call AddPosition for each.
//
// When the underlying enum lacks positions/offsets/payloads, the port
// emits the loop using sentinels (-1) so the receiving writer still
// observes the correct cardinality and can reconstruct the term freq.
// This compensates for Gocene's freq-less StartTerm signature; see the
// type-doc deviation note.
func emitPositions(writer TermVectorsWriter, termsEnum TermsEnum, hasPositions, hasOffsets, hasPayloads bool, freq int) error {
	if !hasPositions && !hasOffsets {
		// Lucene does nothing here; the freq was conveyed via StartTerm.
		// In the port we emit freq sentinel positions so the writer
		// records the same count.
		for i := 0; i < freq; i++ {
			if err := writer.AddPosition(-1, -1, -1, nil); err != nil {
				return fmt.Errorf("sentinel position %d/%d: %w", i, freq, err)
			}
		}
		return nil
	}

	posEnum, err := termsEnum.Postings(0)
	if err != nil {
		return fmt.Errorf("postings: %w", err)
	}
	if posEnum == nil {
		return errors.New("postings enum is nil but positions/offsets were declared")
	}

	docID, err := posEnum.NextDoc()
	if err != nil {
		return fmt.Errorf("postings.nextDoc: %w", err)
	}
	if docID == NO_MORE_DOCS {
		return errors.New("postings enum advanced past the only doc unexpectedly")
	}
	gotFreq, err := posEnum.Freq()
	if err != nil {
		return fmt.Errorf("postings.freq: %w", err)
	}
	if gotFreq != freq {
		return fmt.Errorf("postings.freq %d != totalTermFreq %d", gotFreq, freq)
	}

	for i := 0; i < freq; i++ {
		pos, err := posEnum.NextPosition()
		if err != nil {
			return fmt.Errorf("postings.nextPosition %d/%d: %w", i, freq, err)
		}
		start := -1
		end := -1
		if hasOffsets {
			if start, err = posEnum.StartOffset(); err != nil {
				return fmt.Errorf("postings.startOffset %d/%d: %w", i, freq, err)
			}
			if end, err = posEnum.EndOffset(); err != nil {
				return fmt.Errorf("postings.endOffset %d/%d: %w", i, freq, err)
			}
		}
		var payload []byte
		if hasPayloads {
			if payload, err = posEnum.GetPayload(); err != nil {
				return fmt.Errorf("postings.getPayload %d/%d: %w", i, freq, err)
			}
		}
		if hasPositions && pos < 0 {
			return fmt.Errorf("position %d/%d is negative (%d) but hasPositions=true", i, freq, pos)
		}
		if err := writer.AddPosition(pos, start, end, payload); err != nil {
			return fmt.Errorf("addPosition %d/%d: %w", i, freq, err)
		}
	}
	return nil
}

// termBytes extracts the raw bytes of a Term, honouring the BytesRef
// offset/length window. Returns nil when the Term has no bytes.
func termBytes(t *Term) []byte {
	if t == nil || t.Bytes == nil {
		return nil
	}
	br := t.Bytes
	if br.Length <= 0 {
		return nil
	}
	return br.Bytes[br.Offset : br.Offset+br.Length]
}
