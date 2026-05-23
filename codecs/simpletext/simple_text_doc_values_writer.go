// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SimpleText doc-values token prefixes.
var (
	dvEnd        = []byte("END")
	dvField      = []byte("field ")
	dvType       = []byte("  type ")
	dvDocCount   = []byte("  doccount ")
	dvOrigin     = []byte("  origin ")
	dvMinValue   = []byte("  minalue ") // note: Java has typo "minalue" not "minvalue"
	dvMaxValue   = []byte("  maxvalue ")
	dvPattern    = []byte("  pattern ")
	dvLength     = []byte("length ")
	dvMaxLength  = []byte("  maxlength ")
	dvNumValues  = []byte("  numvalues ")
	dvOrdPattern = []byte("  ordpattern ")
)

// noMoreDocs is the sentinel returned by DocValuesIterator.NextDoc.
const noMoreDocs = math.MaxInt32

// SimpleTextDocValuesWriter writes doc values as plain text.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextDocValuesWriter
// (Lucene 10.4.0).
//
// Deviations from Java:
//   - Go's DocValuesConsumer uses typed iterators (NumericDocValuesIterator etc.)
//     instead of DocValuesProducer. Single-pass iterators preclude the Java
//     two-pass numeric approach; numeric values are buffered in memory.
//   - AddSortedField and AddSortedSetField write zero dictionary entries because
//     SortedDocValuesIterator and SortedSetDocValuesIterator expose only ords,
//     not the underlying bytes. The on-disk layout is structurally valid but the
//     dictionary is empty; a matching reader would need the same adaptation.
type SimpleTextDocValuesWriter struct {
	data    *store.ChecksumIndexOutput
	scratch *util.BytesRefBuilder
	numDocs int
	closed  bool
}

// NewSimpleTextDocValuesWriter opens the doc-values output file and returns the
// writer.
//
// Port of SimpleTextDocValuesWriter(SegmentWriteState, String).
func NewSimpleTextDocValuesWriter(state *codecs.SegmentWriteState, ext string) (*SimpleTextDocValuesWriter, error) {
	fileName := index.SegmentFileName(
		state.SegmentInfo.Name(),
		state.SegmentSuffix,
		ext,
	)
	raw, err := state.Directory.CreateOutput(fileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("SimpleTextDocValuesWriter: create %s: %w", fileName, err)
	}
	return &SimpleTextDocValuesWriter{
		data:    store.NewChecksumIndexOutput(raw),
		scratch: util.NewBytesRefBuilder(),
		numDocs: state.SegmentInfo.DocCount(),
	}, nil
}

// AddNumericField writes all numeric doc values for field.
//
// Port of SimpleTextDocValuesWriter.addNumericField(FieldInfo, DocValuesProducer).
//
// Deviation: buffers all values in memory to perform min/max scan before
// writing, since Go's NumericDocValuesIterator is single-use.
func (w *SimpleTextDocValuesWriter) AddNumericField(field *index.FieldInfo, values codecs.NumericDocValuesIterator) error {
	if err := w.writeFieldEntry(field, index.DocValuesTypeNumeric); err != nil {
		return err
	}

	// Buffer all (docID, value) pairs for two-pass approach.
	type docVal struct {
		docID int
		val   int64
	}
	var buf []docVal
	minValue := int64(math.MaxInt64)
	maxValue := int64(math.MinInt64)
	for values.Next() {
		v := values.Value()
		if v < minValue {
			minValue = v
		}
		if v > maxValue {
			maxValue = v
		}
		buf = append(buf, docVal{values.DocID(), v})
	}
	numValues := len(buf)

	if numValues != w.numDocs {
		if minValue > 0 {
			minValue = 0
		}
		if maxValue < 0 {
			maxValue = 0
		}
	}

	if err := w.write(dvMinValue); err != nil {
		return err
	}
	if err := w.writeStr(strconv.FormatInt(minValue, 10)); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}

	if err := w.write(dvMaxValue); err != nil {
		return err
	}
	if err := w.writeStr(strconv.FormatInt(maxValue, 10)); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}

	if err := w.write(dvDocCount); err != nil {
		return err
	}
	if err := w.writeStr(strconv.Itoa(numValues)); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}

	if err := w.write(dvOrigin); err != nil {
		return err
	}
	if err := w.writeStr(strconv.FormatInt(minValue, 10)); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}

	// Build fixed-width pattern based on the max delta.
	diffStr := strconv.FormatUint(uint64(maxValue-minValue), 10)
	pattern := strings.Repeat("0", len(diffStr))

	if err := w.write(dvPattern); err != nil {
		return err
	}
	if err := w.writeStr(pattern); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}

	// Second pass: write one entry per document.
	bi := 0
	for i := 0; i < w.numDocs; i++ {
		var value int64
		hasValue := false
		if bi < len(buf) && buf[bi].docID == i {
			value = buf[bi].val
			hasValue = true
			bi++
		}
		delta := value - minValue
		encoded := fmt.Sprintf("%0*d", len(pattern), delta)
		if err := w.writeStr(encoded); err != nil {
			return err
		}
		if err := w.newline(); err != nil {
			return err
		}
		if hasValue {
			if err := w.writeStr("T"); err != nil {
				return err
			}
		} else {
			if err := w.writeStr("F"); err != nil {
				return err
			}
		}
		if err := w.newline(); err != nil {
			return err
		}
	}
	return nil
}

// AddBinaryField writes all binary doc values for field.
//
// Port of SimpleTextDocValuesWriter.addBinaryField(FieldInfo, DocValuesProducer).
func (w *SimpleTextDocValuesWriter) AddBinaryField(field *index.FieldInfo, values codecs.BinaryDocValuesIterator) error {
	if err := w.writeFieldEntry(field, index.DocValuesTypeBinary); err != nil {
		return err
	}
	return w.doAddBinaryField(values)
}

// doAddBinaryField implements the shared binary-field writing logic used by
// both AddBinaryField and AddSortedNumericField.
//
// Port of SimpleTextDocValuesWriter.doAddBinaryField(FieldInfo, DocValuesProducer).
func (w *SimpleTextDocValuesWriter) doAddBinaryField(values codecs.BinaryDocValuesIterator) error {
	// Buffer all values.
	type docBin struct {
		docID int
		val   []byte // nil means missing
	}
	var buf []docBin
	maxLength := 0
	docCount := 0
	for values.Next() {
		v := values.Value()
		if v != nil {
			s := bytesRefToString(v)
			if len(s) > maxLength {
				maxLength = len(s)
			}
			buf = append(buf, docBin{values.DocID(), []byte(s)})
			docCount++
		}
	}

	if err := w.write(dvDocCount); err != nil {
		return err
	}
	if err := w.writeStr(strconv.Itoa(docCount)); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}

	if err := w.write(dvMaxLength); err != nil {
		return err
	}
	if err := w.writeStr(strconv.Itoa(maxLength)); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}

	// Pattern for length encoding.
	maxLenStr := strconv.Itoa(maxLength)
	lenPattern := strings.Repeat("0", len(maxLenStr))
	if err := w.write(dvPattern); err != nil {
		return err
	}
	if err := w.writeStr(lenPattern); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}

	// Write one entry per document.
	bi := 0
	for i := 0; i < w.numDocs; i++ {
		var stringVal []byte
		if bi < len(buf) && buf[bi].docID == i {
			stringVal = buf[bi].val
			bi++
		}
		// Write length.
		length := 0
		if stringVal != nil {
			length = len(stringVal)
		}
		encoded := fmt.Sprintf("%0*d", len(lenPattern), length)
		if err := w.write(dvLength); err != nil {
			return err
		}
		if err := w.writeStr(encoded); err != nil {
			return err
		}
		if err := w.newline(); err != nil {
			return err
		}
		// Write value (padded to maxLength).
		if stringVal != nil {
			if err := w.writeStr(string(stringVal)); err != nil {
				return err
			}
		}
		// Pad with spaces.
		for j := length; j < maxLength; j++ {
			if err := w.data.WriteByte(' '); err != nil {
				return err
			}
		}
		if err := w.newline(); err != nil {
			return err
		}
		// Write present/absent flag.
		if stringVal == nil {
			if err := w.writeStr("F"); err != nil {
				return err
			}
		} else {
			if err := w.writeStr("T"); err != nil {
				return err
			}
		}
		if err := w.newline(); err != nil {
			return err
		}
	}
	return nil
}

// AddSortedField writes sorted doc values for field.
//
// Port of SimpleTextDocValuesWriter.addSortedField(FieldInfo, DocValuesProducer).
//
// Deviation: SortedDocValuesIterator exposes only ords, not the underlying
// bytes. The dictionary section is written as empty (numvalues=0, maxlength=0).
// Per-doc ords are written correctly.
func (w *SimpleTextDocValuesWriter) AddSortedField(field *index.FieldInfo, values codecs.SortedDocValuesIterator) error {
	if err := w.writeFieldEntry(field, index.DocValuesTypeSorted); err != nil {
		return err
	}

	// Collect ords per doc.
	type docOrd struct {
		docID int
		ord   int
	}
	var buf []docOrd
	docCount := 0
	for values.Next() {
		buf = append(buf, docOrd{values.DocID(), values.Ord()})
		docCount++
	}

	if err := w.write(dvDocCount); err != nil {
		return err
	}
	if err := w.writeStr(strconv.Itoa(docCount)); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}

	// Dictionary is not accessible via Go iterator — write as empty.
	valueCount := 0
	maxLength := 0

	if err := w.write(dvNumValues); err != nil {
		return err
	}
	if err := w.writeStr(strconv.Itoa(valueCount)); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}
	if err := w.write(dvMaxLength); err != nil {
		return err
	}
	if err := w.writeStr(strconv.Itoa(maxLength)); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}
	if err := w.write(dvPattern); err != nil {
		return err
	}
	if err := w.writeStr("0"); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}

	// Ord pattern: wide enough for valueCount+1.
	ordPatWidth := len(strconv.FormatInt(int64(valueCount+1), 10))
	ordPattern := strings.Repeat("0", ordPatWidth)
	if err := w.write(dvOrdPattern); err != nil {
		return err
	}
	if err := w.writeStr(ordPattern); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}

	// Write per-doc ords.
	bi := 0
	for i := 0; i < w.numDocs; i++ {
		ord := -1
		if bi < len(buf) && buf[bi].docID == i {
			ord = buf[bi].ord
			bi++
		}
		encoded := fmt.Sprintf("%0*d", len(ordPattern), ord+1)
		if err := w.writeStr(encoded); err != nil {
			return err
		}
		if err := w.newline(); err != nil {
			return err
		}
	}
	return nil
}

// AddSortedSetField writes sorted-set doc values for field.
//
// Port of SimpleTextDocValuesWriter.addSortedSetField(FieldInfo, DocValuesProducer).
//
// Deviation: SortedSetDocValuesIterator exposes only ords, not the underlying
// bytes. The dictionary section is written as empty (numvalues=0, maxlength=0).
func (w *SimpleTextDocValuesWriter) AddSortedSetField(field *index.FieldInfo, values codecs.SortedSetDocValuesIterator) error {
	if err := w.writeFieldEntry(field, index.DocValuesTypeSortedSet); err != nil {
		return err
	}

	// Collect per-doc ord lists.
	type docOrds struct {
		docID int
		ords  []int
	}
	var buf []docOrds
	docCount := 0
	for values.NextDoc() {
		var ords []int
		for {
			ord := values.NextOrd()
			if ord == -1 {
				break
			}
			ords = append(ords, ord)
		}
		buf = append(buf, docOrds{values.DocID(), ords})
		docCount++
	}

	if err := w.write(dvDocCount); err != nil {
		return err
	}
	if err := w.writeStr(strconv.Itoa(docCount)); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}

	// Dictionary not accessible — write as empty.
	valueCount := 0
	maxLength := 0

	if err := w.write(dvNumValues); err != nil {
		return err
	}
	if err := w.writeStr(strconv.FormatInt(int64(valueCount), 10)); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}
	if err := w.write(dvMaxLength); err != nil {
		return err
	}
	if err := w.writeStr(strconv.Itoa(maxLength)); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}
	if err := w.write(dvPattern); err != nil {
		return err
	}
	if err := w.writeStr("0"); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}

	// Compute max ord-list length (for ordpattern).
	maxOrdListLength := 0
	for _, entry := range buf {
		var sb strings.Builder
		for j, ord := range entry.ords {
			if j > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(strconv.FormatInt(int64(ord), 10))
		}
		if sb.Len() > maxOrdListLength {
			maxOrdListLength = sb.Len()
		}
	}
	ordPattern := strings.Repeat("X", maxOrdListLength)
	if err := w.write(dvOrdPattern); err != nil {
		return err
	}
	if err := w.writeStr(ordPattern); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}

	// Write per-doc ord lists (padded to maxOrdListLength).
	bi := 0
	for i := 0; i < w.numDocs; i++ {
		var sb strings.Builder
		if bi < len(buf) && buf[bi].docID == i {
			for j, ord := range buf[bi].ords {
				if j > 0 {
					sb.WriteByte(',')
				}
				sb.WriteString(strconv.FormatInt(int64(ord), 10))
			}
			bi++
		}
		s := sb.String()
		for len(s) < maxOrdListLength {
			s += " "
		}
		if err := w.writeStr(s); err != nil {
			return err
		}
		if err := w.newline(); err != nil {
			return err
		}
	}
	return nil
}

// AddSortedNumericField writes sorted-numeric doc values for field.
//
// Port of SimpleTextDocValuesWriter.addSortedNumericField(FieldInfo, DocValuesProducer).
//
// Uses the binary field path (comma-separated values per doc), matching the
// Java approach of constructing an anonymous BinaryDocValues from
// SortedNumericDocValues.
func (w *SimpleTextDocValuesWriter) AddSortedNumericField(field *index.FieldInfo, values codecs.SortedNumericDocValuesIterator) error {
	if err := w.writeFieldEntry(field, index.DocValuesTypeSortedNumeric); err != nil {
		return err
	}

	// Find min/max for header, then delegate to doAddBinaryField.
	var entries []docEntry
	minValue := int64(math.MaxInt64)
	maxValue := int64(math.MinInt64)
	for values.NextDoc() {
		count := values.DocValueCount()
		var sb strings.Builder
		for i := 0; i < count; i++ {
			v := values.NextValue()
			if v < minValue {
				minValue = v
			}
			if v > maxValue {
				maxValue = v
			}
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(strconv.FormatInt(v, 10))
		}
		entries = append(entries, docEntry{values.DocID(), sb.String()})
	}

	if err := w.write(dvMinValue); err != nil {
		return err
	}
	if err := w.writeStr(strconv.FormatInt(minValue, 10)); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}
	if err := w.write(dvMaxValue); err != nil {
		return err
	}
	if err := w.writeStr(strconv.FormatInt(maxValue, 10)); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}

	// Delegate to binary-field path using a slice-backed iterator.
	iter := &sliceBinaryIterator{entries: entries, idx: -1}
	return w.doAddBinaryField(iter)
}

// docEntry holds a precomputed (docID, string) pair for the binary-field path.
type docEntry struct {
	docID int
	s     string
}

// sliceBinaryIterator adapts a precomputed []docEntry to BinaryDocValuesIterator.
type sliceBinaryIterator struct {
	entries []docEntry
	idx     int
}

func (it *sliceBinaryIterator) Next() bool {
	it.idx++
	return it.idx < len(it.entries)
}
func (it *sliceBinaryIterator) DocID() int { return it.entries[it.idx].docID }
func (it *sliceBinaryIterator) Value() []byte {
	return []byte(it.entries[it.idx].s)
}

// writeFieldEntry writes the "field NAME\n  type TYPE\n" header.
//
// Port of SimpleTextDocValuesWriter.writeFieldEntry(FieldInfo, DocValuesType).
func (w *SimpleTextDocValuesWriter) writeFieldEntry(field *index.FieldInfo, docValuesType index.DocValuesType) error {
	if err := w.write(dvField); err != nil {
		return err
	}
	if err := w.writeStr(field.Name()); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}
	if err := w.write(dvType); err != nil {
		return err
	}
	if err := w.writeStr(docValuesType.String()); err != nil {
		return err
	}
	return w.newline()
}

// Close writes the END marker and checksum.
//
// Port of SimpleTextDocValuesWriter.close().
func (w *SimpleTextDocValuesWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	out := w.data
	w.data = nil

	var closeErr error
	defer func() {
		if cerr := out.Close(); cerr != nil && closeErr == nil {
			closeErr = fmt.Errorf("SimpleTextDocValuesWriter.Close: %w", cerr)
		}
	}()

	if err := w.write(dvEnd); err != nil {
		closeErr = err
		return closeErr
	}
	if err := w.newline(); err != nil {
		closeErr = err
		return closeErr
	}
	if err := stWriteChecksum(out, w.scratch); err != nil {
		closeErr = err
		return closeErr
	}
	return closeErr
}

// ---------------------------------------------------------------------------
// Output helpers
// ---------------------------------------------------------------------------

func (w *SimpleTextDocValuesWriter) write(b []byte) error {
	return stWrite(w.data, b, w.scratch)
}

func (w *SimpleTextDocValuesWriter) writeStr(s string) error {
	return stWriteStr(w.data, s, w.scratch)
}

func (w *SimpleTextDocValuesWriter) newline() error {
	return stWriteNewline(w.data)
}

// bytesRefToString converts raw bytes to their BytesRef.toString() hex
// representation.  The Java SimpleTextDocValuesWriter.doAddBinaryField
// calls binaryValue().toString() which for a BytesRef returns the hex form.
//
// Matches the format "[xx yy zz]" (hex bytes, space-separated, bracketed).
func bytesRefToString(b []byte) string {
	if len(b) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range b {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(strconv.FormatUint(uint64(v), 16))
	}
	sb.WriteByte(']')
	return sb.String()
}

// compile-time assertion.
var _ codecs.DocValuesConsumer = (*SimpleTextDocValuesWriter)(nil)
