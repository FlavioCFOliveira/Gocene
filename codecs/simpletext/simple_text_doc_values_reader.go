// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	store2 "github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// dvFieldMeta holds the parsed metadata for one field written by
// SimpleTextDocValuesWriter.
type dvFieldMeta struct {
	// docCount is the number of documents that have a value for this field.
	docCount int

	// dataStartFilePointer is the byte offset in the data file at which the
	// per-document records for this field begin.
	dataStartFilePointer int64

	// pattern is the DecimalFormat pattern used for numeric/length widths.
	pattern string

	// ordPattern is the DecimalFormat pattern used for ord widths.
	ordPattern string

	// maxLength is the maximum binary value length (BINARY / SORTED / SORTED_SET).
	maxLength int

	// minValue and maxValue are the global min/max (NUMERIC / SORTED_NUMERIC).
	minValue, maxValue int64

	// origin is the fixed delta subtracted from every stored decimal (NUMERIC).
	origin int64

	// numValues is the number of unique values (SORTED / SORTED_SET).
	numValues int64
}

// SimpleTextDocValuesReader reads per-document values from the plain-text
// ".dat" file written by SimpleTextDocValuesWriter.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextDocValuesReader
// (Lucene 10.4.0).
type SimpleTextDocValuesReader struct {
	maxDoc int
	data   store2.IndexInput
	fields map[string]*dvFieldMeta
}

// NewSimpleTextDocValuesReader opens the doc-values data file and parses the
// per-field index headers stored at the beginning of the file.
//
// Port of SimpleTextDocValuesReader(SegmentReadState, String).
func NewSimpleTextDocValuesReader(state *codecs.SegmentReadState, ext string) (*SimpleTextDocValuesReader, error) {
	fileName := index.SegmentFileName(
		state.SegmentInfo.Name(),
		state.SegmentSuffix,
		ext,
	)
	in, err := state.Directory.OpenInput(fileName, store2.IOContext{Context: store2.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("SimpleTextDocValuesReader: open %s: %w", fileName, err)
	}

	r := &SimpleTextDocValuesReader{
		maxDoc: state.SegmentInfo.DocCount(),
		data:   in,
		fields: make(map[string]*dvFieldMeta),
	}

	scratch := util.NewBytesRefBuilder()
	readLine := func() error { return stReadLine(r.data, scratch) }
	line := func() []byte { return scratch.Bytes()[:scratch.Length()] }
	startsWith := func(prefix []byte) bool {
		if scratch.Length() < len(prefix) {
			return false
		}
		for i, b := range prefix {
			if scratch.ByteAt(i) != b {
				return false
			}
		}
		return true
	}
	stripPrefix := func(prefix []byte) string {
		return string(scratch.Bytes()[len(prefix):scratch.Length()])
	}

	for {
		if err := readLine(); err != nil {
			_ = in.Close()
			return nil, fmt.Errorf("SimpleTextDocValuesReader: readLine: %w", err)
		}
		if startsWith(dvEnd) {
			break
		}
		if !startsWith(dvField) {
			_ = in.Close()
			return nil, fmt.Errorf("SimpleTextDocValuesReader: expected %q, got %q", dvField, line())
		}
		fieldName := stripPrefix(dvField)

		f := &dvFieldMeta{}
		r.fields[fieldName] = f

		if err := readLine(); err != nil {
			_ = in.Close()
			return nil, fmt.Errorf("SimpleTextDocValuesReader: read type: %w", err)
		}
		if !startsWith(dvType) {
			_ = in.Close()
			return nil, fmt.Errorf("SimpleTextDocValuesReader: expected type prefix, got %q", line())
		}
		dvTypeName := stripPrefix(dvType)
		docValType, err2 := parseDVType(dvTypeName)
		if err2 != nil {
			_ = in.Close()
			return nil, fmt.Errorf("SimpleTextDocValuesReader: field %q: %w", fieldName, err2)
		}

		// NUMERIC and SORTED_NUMERIC have min/max headers.
		if docValType == index.DocValuesTypeNumeric || docValType == index.DocValuesTypeSortedNumeric {
			if err := readLine(); err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: read minvalue: %w", err)
			}
			if !startsWith(dvMinValue) {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: expected minvalue, got %q", line())
			}
			f.minValue, err = strconv.ParseInt(stripPrefix(dvMinValue), 10, 64)
			if err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: parse minvalue: %w", err)
			}
			if err := readLine(); err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: read maxvalue: %w", err)
			}
			if !startsWith(dvMaxValue) {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: expected maxvalue, got %q", line())
			}
			f.maxValue, err = strconv.ParseInt(stripPrefix(dvMaxValue), 10, 64)
			if err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: parse maxvalue: %w", err)
			}
		}

		// docCount
		if err := readLine(); err != nil {
			_ = in.Close()
			return nil, fmt.Errorf("SimpleTextDocValuesReader: read doccount: %w", err)
		}
		if !startsWith(dvDocCount) {
			_ = in.Close()
			return nil, fmt.Errorf("SimpleTextDocValuesReader: expected doccount, got %q", line())
		}
		f.docCount, err = strconv.Atoi(stripPrefix(dvDocCount))
		if err != nil {
			_ = in.Close()
			return nil, fmt.Errorf("SimpleTextDocValuesReader: parse doccount: %w", err)
		}

		switch docValType {
		case index.DocValuesTypeNumeric:
			// origin + pattern, then skip per-doc records
			if err := readLine(); err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: read origin: %w", err)
			}
			if !startsWith(dvOrigin) {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: expected origin, got %q", line())
			}
			f.origin, err = strconv.ParseInt(stripPrefix(dvOrigin), 10, 64)
			if err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: parse origin: %w", err)
			}
			if err := readLine(); err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: read pattern: %w", err)
			}
			if !startsWith(dvPattern) {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: expected pattern, got %q", line())
			}
			f.pattern = stripPrefix(dvPattern)
			f.dataStartFilePointer = r.data.GetFilePointer()
			// Skip past per-doc records: each is (1 + patternLen + 2) bytes
			skip := int64(1+len(f.pattern)+2) * int64(r.maxDoc)
			if err := r.data.SetPosition(r.data.GetFilePointer() + skip); err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: seek past numeric records: %w", err)
			}

		case index.DocValuesTypeBinary, index.DocValuesTypeSortedNumeric:
			// maxLength + pattern, then skip per-doc records
			if err := readLine(); err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: read maxlength: %w", err)
			}
			if !startsWith(dvMaxLength) {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: expected maxlength, got %q", line())
			}
			f.maxLength, err = strconv.Atoi(stripPrefix(dvMaxLength))
			if err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: parse maxlength: %w", err)
			}
			if err := readLine(); err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: read pattern: %w", err)
			}
			if !startsWith(dvPattern) {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: expected pattern, got %q", line())
			}
			f.pattern = stripPrefix(dvPattern)
			f.dataStartFilePointer = r.data.GetFilePointer()
			// Each record: (9 + patternLen + maxLength + 2) bytes
			skip := int64(9+len(f.pattern)+f.maxLength+2) * int64(r.maxDoc)
			if err := r.data.SetPosition(r.data.GetFilePointer() + skip); err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: seek past binary records: %w", err)
			}

		case index.DocValuesTypeSorted, index.DocValuesTypeSortedSet:
			// numValues + maxLength + pattern + ordPattern, then skip
			if err := readLine(); err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: read numvalues: %w", err)
			}
			if !startsWith(dvNumValues) {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: expected numvalues, got %q", line())
			}
			f.numValues, err = strconv.ParseInt(stripPrefix(dvNumValues), 10, 64)
			if err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: parse numvalues: %w", err)
			}
			if err := readLine(); err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: read maxlength: %w", err)
			}
			if !startsWith(dvMaxLength) {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: expected maxlength, got %q", line())
			}
			f.maxLength, err = strconv.Atoi(stripPrefix(dvMaxLength))
			if err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: parse maxlength: %w", err)
			}
			if err := readLine(); err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: read pattern: %w", err)
			}
			if !startsWith(dvPattern) {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: expected pattern, got %q", line())
			}
			f.pattern = stripPrefix(dvPattern)
			if err := readLine(); err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: read ordpattern: %w", err)
			}
			if !startsWith(dvOrdPattern) {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: expected ordpattern, got %q", line())
			}
			f.ordPattern = stripPrefix(dvOrdPattern)
			f.dataStartFilePointer = r.data.GetFilePointer()
			// Skip value dictionary then per-doc ord records
			skip := f.numValues*int64(9+len(f.pattern)+f.maxLength) +
				int64(1+len(f.ordPattern))*int64(r.maxDoc)
			if err := r.data.SetPosition(r.data.GetFilePointer() + skip); err != nil {
				_ = in.Close()
				return nil, fmt.Errorf("SimpleTextDocValuesReader: seek past sorted records: %w", err)
			}

		default:
			_ = in.Close()
			return nil, fmt.Errorf("SimpleTextDocValuesReader: unrecognized DocValues type %q", dvTypeName)
		}
	}

	return r, nil
}

// parseDVType maps the on-disk type string to a Go DocValuesType constant.
func parseDVType(s string) (index.DocValuesType, error) {
	switch s {
	case "NUMERIC":
		return index.DocValuesTypeNumeric, nil
	case "BINARY":
		return index.DocValuesTypeBinary, nil
	case "SORTED":
		return index.DocValuesTypeSorted, nil
	case "SORTED_NUMERIC":
		return index.DocValuesTypeSortedNumeric, nil
	case "SORTED_SET":
		return index.DocValuesTypeSortedSet, nil
	default:
		return index.DocValuesTypeNone, fmt.Errorf("parseDVType: unrecognized type %q", s)
	}
}

// GetNumeric returns a NumericDocValues iterator for the given field.
//
// Port of SimpleTextDocValuesReader.getNumeric(FieldInfo).
func (r *SimpleTextDocValuesReader) GetNumeric(field *index.FieldInfo) (codecs.NumericDocValues, error) {
	f := r.fields[field.Name()]
	if f == nil {
		return nil, fmt.Errorf("SimpleTextDocValuesReader.GetNumeric: field %q not found", field.Name())
	}
	in := r.data.Clone()
	return &dvNumericIter{
		f:      f,
		in:     in,
		maxDoc: r.maxDoc,
		doc:    -1,
	}, nil
}

// dvNumericIter is an iterator over NUMERIC doc values.
type dvNumericIter struct {
	f      *dvFieldMeta
	in     store2.IndexInput
	maxDoc int
	doc    int
}

func (it *dvNumericIter) DocID() int { return it.doc }

func (it *dvNumericIter) Cost() int64 { return int64(it.maxDoc) }

func (it *dvNumericIter) NextDoc() (int, error) { return it.Advance(it.doc + 1) }

func (it *dvNumericIter) Advance(target int) (int, error) {
	f := it.f
	scratch := util.NewBytesRefBuilder()
	for i := target; i < it.maxDoc; i++ {
		pos := f.dataStartFilePointer + int64(1+len(f.pattern)+2)*int64(i)
		if err := it.in.SetPosition(pos); err != nil {
			return 0, fmt.Errorf("dvNumericIter.Advance: SetPosition: %w", err)
		}
		if err := stReadLine(it.in, scratch); err != nil { // value line
			return 0, fmt.Errorf("dvNumericIter.Advance: readLine value: %w", err)
		}
		if err := stReadLine(it.in, scratch); err != nil { // 'T' or 'F' line
			return 0, fmt.Errorf("dvNumericIter.Advance: readLine flag: %w", err)
		}
		if scratch.ByteAt(0) == 'T' {
			it.doc = i
			return i, nil
		}
	}
	it.doc = noMoreDocs
	return noMoreDocs, nil
}

func (it *dvNumericIter) LongValue() (int64, error) {
	return dvReadNumericValue(it.in, it.f, it.doc)
}

// dvReadNumericValue seeks to the record for docID and parses the stored
// decimal delta, returning origin + delta.
func dvReadNumericValue(in store2.IndexInput, f *dvFieldMeta, docID int) (int64, error) {
	pos := f.dataStartFilePointer + int64(1+len(f.pattern)+2)*int64(docID)
	if err := in.SetPosition(pos); err != nil {
		return 0, fmt.Errorf("dvReadNumericValue: SetPosition: %w", err)
	}
	scratch := util.NewBytesRefBuilder()
	if err := stReadLine(in, scratch); err != nil {
		return 0, fmt.Errorf("dvReadNumericValue: readLine: %w", err)
	}
	s := string(scratch.Bytes()[:scratch.Length()])
	// s is zero-padded (e.g. "0000000000000000042"); parse as int64 directly.
	delta, err := strconv.ParseInt(strings.TrimLeft(s, "0 "), 10, 64)
	if err != nil && strings.TrimLeft(s, "0 ") == "" {
		delta = 0
		err = nil
	}
	if err != nil {
		return 0, fmt.Errorf("dvReadNumericValue: parse %q: %w", s, err)
	}
	return f.origin + delta, nil
}

// GetBinary returns a BinaryDocValues iterator for the given field.
//
// Port of SimpleTextDocValuesReader.getBinary(FieldInfo).
func (r *SimpleTextDocValuesReader) GetBinary(field *index.FieldInfo) (codecs.BinaryDocValues, error) {
	f := r.fields[field.Name()]
	if f == nil {
		return nil, fmt.Errorf("SimpleTextDocValuesReader.GetBinary: field %q not found", field.Name())
	}
	in := r.data.Clone()
	return &dvBinaryIter{
		f:      f,
		in:     in,
		maxDoc: r.maxDoc,
		doc:    -1,
	}, nil
}

// dvBinaryIter is an iterator over BINARY doc values.
type dvBinaryIter struct {
	f      *dvFieldMeta
	in     store2.IndexInput
	maxDoc int
	doc    int
}

func (it *dvBinaryIter) DocID() int  { return it.doc }
func (it *dvBinaryIter) Cost() int64 { return int64(it.maxDoc) }

func (it *dvBinaryIter) NextDoc() (int, error) { return it.Advance(it.doc + 1) }

func (it *dvBinaryIter) Advance(target int) (int, error) {
	f := it.f
	recLen := int64(9 + len(f.pattern) + f.maxLength + 2)
	scratch := util.NewBytesRefBuilder()
	for i := target; i < it.maxDoc; i++ {
		if err := it.in.SetPosition(f.dataStartFilePointer + recLen*int64(i)); err != nil {
			return 0, fmt.Errorf("dvBinaryIter.Advance: SetPosition: %w", err)
		}
		// length line
		if err := stReadLine(it.in, scratch); err != nil {
			return 0, fmt.Errorf("dvBinaryIter.Advance: readLine length: %w", err)
		}
		lenStr := string(scratch.Bytes()[len(dvLength):scratch.Length()])
		length, err := dvParsePatternInt(lenStr, f.pattern)
		if err != nil {
			return 0, fmt.Errorf("dvBinaryIter.Advance: parse length: %w", err)
		}
		// skip raw bytes
		rawBuf := make([]byte, length)
		if err := it.in.ReadBytes(rawBuf); err != nil {
			return 0, fmt.Errorf("dvBinaryIter.Advance: readBytes: %w", err)
		}
		// newline after raw bytes
		if err := stReadLine(it.in, scratch); err != nil {
			return 0, fmt.Errorf("dvBinaryIter.Advance: readLine newline: %w", err)
		}
		// T/F flag
		if err := stReadLine(it.in, scratch); err != nil {
			return 0, fmt.Errorf("dvBinaryIter.Advance: readLine flag: %w", err)
		}
		if scratch.ByteAt(0) == 'T' {
			it.doc = i
			return i, nil
		}
	}
	it.doc = noMoreDocs
	return noMoreDocs, nil
}

func (it *dvBinaryIter) BinaryValue() ([]byte, error) {
	return dvReadBinaryValue(it.in, it.f, it.doc)
}

// dvReadBinaryValue seeks to the record for docID and returns the decoded
// binary value.
func dvReadBinaryValue(in store2.IndexInput, f *dvFieldMeta, docID int) ([]byte, error) {
	recLen := int64(9 + len(f.pattern) + f.maxLength + 2)
	if err := in.SetPosition(f.dataStartFilePointer + recLen*int64(docID)); err != nil {
		return nil, fmt.Errorf("dvReadBinaryValue: SetPosition: %w", err)
	}
	scratch := util.NewBytesRefBuilder()
	// length line
	if err := stReadLine(in, scratch); err != nil {
		return nil, fmt.Errorf("dvReadBinaryValue: readLine length: %w", err)
	}
	lenStr := string(scratch.Bytes()[len(dvLength):scratch.Length()])
	length, err := dvParsePatternInt(lenStr, f.pattern)
	if err != nil {
		return nil, fmt.Errorf("dvReadBinaryValue: parse length: %w", err)
	}
	rawBuf := make([]byte, length)
	if err := in.ReadBytes(rawBuf); err != nil {
		return nil, fmt.Errorf("dvReadBinaryValue: readBytes: %w", err)
	}
	// rawBuf is the hex-encoded BytesRef string stored by bytesRefToString
	decoded, err := fromBytesRefString(string(rawBuf))
	if err != nil {
		return nil, fmt.Errorf("dvReadBinaryValue: fromBytesRefString: %w", err)
	}
	return decoded, nil
}

// GetSorted returns a SortedDocValues iterator for the given field.
//
// Port of SimpleTextDocValuesReader.getSorted(FieldInfo).
func (r *SimpleTextDocValuesReader) GetSorted(field *index.FieldInfo) (codecs.SortedDocValues, error) {
	f := r.fields[field.Name()]
	if f == nil {
		return nil, fmt.Errorf("SimpleTextDocValuesReader.GetSorted: field %q not found", field.Name())
	}
	in := r.data.Clone()
	return &dvSortedIter{
		f:      f,
		in:     in,
		maxDoc: r.maxDoc,
		doc:    -1,
		ord:    -1,
	}, nil
}

// dvSortedIter is an iterator over SORTED doc values.
// It embeds the ord as the LongValue to satisfy the NumericDocValues contract
// embedded by SortedDocValues.
type dvSortedIter struct {
	f      *dvFieldMeta
	in     store2.IndexInput
	maxDoc int
	doc    int
	ord    int
}

func (it *dvSortedIter) DocID() int  { return it.doc }
func (it *dvSortedIter) Cost() int64 { return int64(it.maxDoc) }

func (it *dvSortedIter) NextDoc() (int, error) { return it.Advance(it.doc + 1) }

func (it *dvSortedIter) Advance(target int) (int, error) {
	f := it.f
	scratch := util.NewBytesRefBuilder()
	for i := target; i < it.maxDoc; i++ {
		pos := f.dataStartFilePointer +
			f.numValues*int64(9+len(f.pattern)+f.maxLength) +
			int64(i)*int64(1+len(f.ordPattern))
		if err := it.in.SetPosition(pos); err != nil {
			return 0, fmt.Errorf("dvSortedIter.Advance: SetPosition: %w", err)
		}
		if err := stReadLine(it.in, scratch); err != nil {
			return 0, fmt.Errorf("dvSortedIter.Advance: readLine: %w", err)
		}
		ord, err := dvParsePatternInt(string(scratch.Bytes()[:scratch.Length()]), f.ordPattern)
		if err != nil {
			return 0, fmt.Errorf("dvSortedIter.Advance: parse ord: %w", err)
		}
		// stored ord is 1-based; -1 means no value
		it.ord = ord - 1
		if it.ord >= 0 {
			it.doc = i
			return i, nil
		}
	}
	it.doc = noMoreDocs
	return noMoreDocs, nil
}

// LongValue returns the current document's ordinal as int64 (satisfies
// NumericDocValues embedded by SortedDocValues).
func (it *dvSortedIter) LongValue() (int64, error) { return int64(it.ord), nil }

// OrdValue returns the current document's ordinal.
func (it *dvSortedIter) OrdValue() (int, error) { return it.ord, nil }

// LookupOrd returns the stored byte value for the given ordinal.
//
// Port of SortedDocValues.lookupOrd(int) inside getSorted.
func (it *dvSortedIter) LookupOrd(ord int) ([]byte, error) {
	f := it.f
	if ord < 0 || int64(ord) >= f.numValues {
		return nil, fmt.Errorf("dvSortedIter.LookupOrd: ord %d out of range [0, %d)", ord, f.numValues)
	}
	pos := f.dataStartFilePointer + int64(ord)*int64(9+len(f.pattern)+f.maxLength)
	if err := it.in.SetPosition(pos); err != nil {
		return nil, fmt.Errorf("dvSortedIter.LookupOrd: SetPosition: %w", err)
	}
	scratch := util.NewBytesRefBuilder()
	if err := stReadLine(it.in, scratch); err != nil {
		return nil, fmt.Errorf("dvSortedIter.LookupOrd: readLine length: %w", err)
	}
	lenStr := string(scratch.Bytes()[len(dvLength):scratch.Length()])
	length, err := dvParsePatternInt(lenStr, f.pattern)
	if err != nil {
		return nil, fmt.Errorf("dvSortedIter.LookupOrd: parse length: %w", err)
	}
	buf := make([]byte, length)
	if err := it.in.ReadBytes(buf); err != nil {
		return nil, fmt.Errorf("dvSortedIter.LookupOrd: readBytes: %w", err)
	}
	return buf, nil
}

// GetValueCount returns the number of unique values.
func (it *dvSortedIter) GetValueCount() int { return int(it.f.numValues) }

// GetSortedNumeric returns a SortedNumericDocValues iterator for the given field.
//
// Port of SimpleTextDocValuesReader.getSortedNumeric(FieldInfo).
func (r *SimpleTextDocValuesReader) GetSortedNumeric(field *index.FieldInfo) (codecs.SortedNumericDocValues, error) {
	// SORTED_NUMERIC is stored as binary CSV; reuse dvBinaryIter.
	bin, err := r.GetBinary(field)
	if err != nil {
		return nil, fmt.Errorf("SimpleTextDocValuesReader.GetSortedNumeric: %w", err)
	}
	return &dvSortedNumericIter{bin: bin}, nil
}

// dvSortedNumericIter wraps a BinaryDocValues and parses comma-separated
// int64 values on each document transition.
type dvSortedNumericIter struct {
	bin    codecs.BinaryDocValues
	values []int64
	idx    int
}

func (it *dvSortedNumericIter) DocID() int  { return it.bin.DocID() }
func (it *dvSortedNumericIter) Cost() int64 { return it.bin.Cost() }

func (it *dvSortedNumericIter) NextDoc() (int, error) {
	doc, err := it.bin.NextDoc()
	if err != nil {
		return 0, err
	}
	return doc, it.setCurrentDoc()
}

func (it *dvSortedNumericIter) Advance(target int) (int, error) {
	doc, err := it.bin.Advance(target)
	if err != nil {
		return 0, err
	}
	return doc, it.setCurrentDoc()
}

// LongValue returns the current value at the iterator position advanced by
// NextValue. It is only valid after calling NextValue.
func (it *dvSortedNumericIter) LongValue() (int64, error) {
	if it.idx == 0 || it.idx > len(it.values) {
		return 0, nil
	}
	return it.values[it.idx-1], nil
}

func (it *dvSortedNumericIter) NextValue() (int64, error) {
	v := it.values[it.idx]
	it.idx++
	return v, nil
}

func (it *dvSortedNumericIter) DocValueCount() (int, error) {
	return len(it.values), nil
}

func (it *dvSortedNumericIter) setCurrentDoc() error {
	if it.bin.DocID() == noMoreDocs {
		it.values = it.values[:0]
		it.idx = 0
		return nil
	}
	raw, err := it.bin.BinaryValue()
	if err != nil {
		return err
	}
	csv := string(raw)
	if csv == "" {
		it.values = it.values[:0]
		it.idx = 0
		return nil
	}
	parts := strings.Split(csv, ",")
	if cap(it.values) < len(parts) {
		it.values = make([]int64, len(parts))
	} else {
		it.values = it.values[:len(parts)]
	}
	for i, p := range parts {
		v, err2 := strconv.ParseInt(p, 10, 64)
		if err2 != nil {
			return fmt.Errorf("dvSortedNumericIter: parse value %q: %w", p, err2)
		}
		it.values[i] = v
	}
	it.idx = 0
	return nil
}

// GetSortedSet returns a SortedSetDocValues iterator for the given field.
//
// Port of SimpleTextDocValuesReader.getSortedSet(FieldInfo).
func (r *SimpleTextDocValuesReader) GetSortedSet(field *index.FieldInfo) (codecs.SortedSetDocValues, error) {
	f := r.fields[field.Name()]
	if f == nil {
		return nil, fmt.Errorf("SimpleTextDocValuesReader.GetSortedSet: field %q not found", field.Name())
	}
	in := r.data.Clone()
	return &dvSortedSetIter{
		f:      f,
		in:     in,
		maxDoc: r.maxDoc,
		doc:    -1,
	}, nil
}

// dvSortedSetIter is an iterator over SORTED_SET doc values.
type dvSortedSetIter struct {
	f           *dvFieldMeta
	in          store2.IndexInput
	maxDoc      int
	doc         int
	currentOrds []int
	currentIdx  int
}

func (it *dvSortedSetIter) DocID() int  { return it.doc }
func (it *dvSortedSetIter) Cost() int64 { return int64(it.maxDoc) }

func (it *dvSortedSetIter) NextDoc() (int, error) { return it.Advance(it.doc + 1) }

func (it *dvSortedSetIter) Advance(target int) (int, error) {
	f := it.f
	scratch := util.NewBytesRefBuilder()
	for i := target; i < it.maxDoc; i++ {
		pos := f.dataStartFilePointer +
			f.numValues*int64(9+len(f.pattern)+f.maxLength) +
			int64(i)*int64(1+len(f.ordPattern))
		if err := it.in.SetPosition(pos); err != nil {
			return 0, fmt.Errorf("dvSortedSetIter.Advance: SetPosition: %w", err)
		}
		if err := stReadLine(it.in, scratch); err != nil {
			return 0, fmt.Errorf("dvSortedSetIter.Advance: readLine: %w", err)
		}
		ordList := strings.TrimSpace(string(scratch.Bytes()[:scratch.Length()]))
		if ordList != "" {
			if err := it.parseOrds(ordList); err != nil {
				return 0, err
			}
			it.currentIdx = 0
			it.doc = i
			return i, nil
		}
	}
	it.doc = noMoreDocs
	return noMoreDocs, nil
}

func (it *dvSortedSetIter) parseOrds(ordList string) error {
	parts := strings.Split(ordList, ",")
	if cap(it.currentOrds) < len(parts) {
		it.currentOrds = make([]int, len(parts))
	} else {
		it.currentOrds = it.currentOrds[:len(parts)]
	}
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return fmt.Errorf("dvSortedSetIter.parseOrds: %w", err)
		}
		it.currentOrds[i] = v
	}
	return nil
}

func (it *dvSortedSetIter) NextOrd() (int, error) {
	if it.currentIdx >= len(it.currentOrds) {
		return -1, nil
	}
	v := it.currentOrds[it.currentIdx]
	it.currentIdx++
	return v, nil
}

// LookupOrd returns the stored byte value for the given ordinal.
//
// Port of SortedSetDocValues.lookupOrd(long) inside getSortedSet.
func (it *dvSortedSetIter) LookupOrd(ord int) ([]byte, error) {
	f := it.f
	if ord < 0 || int64(ord) >= f.numValues {
		return nil, fmt.Errorf("dvSortedSetIter.LookupOrd: ord %d out of range [0, %d)", ord, f.numValues)
	}
	pos := f.dataStartFilePointer + int64(ord)*int64(9+len(f.pattern)+f.maxLength)
	if err := it.in.SetPosition(pos); err != nil {
		return nil, fmt.Errorf("dvSortedSetIter.LookupOrd: SetPosition: %w", err)
	}
	scratch := util.NewBytesRefBuilder()
	if err := stReadLine(it.in, scratch); err != nil {
		return nil, fmt.Errorf("dvSortedSetIter.LookupOrd: readLine length: %w", err)
	}
	lenStr := string(scratch.Bytes()[len(dvLength):scratch.Length()])
	length, err := dvParsePatternInt(lenStr, f.pattern)
	if err != nil {
		return nil, fmt.Errorf("dvSortedSetIter.LookupOrd: parse length: %w", err)
	}
	buf := make([]byte, length)
	if err := it.in.ReadBytes(buf); err != nil {
		return nil, fmt.Errorf("dvSortedSetIter.LookupOrd: readBytes: %w", err)
	}
	return buf, nil
}

// GetValueCount returns the number of unique values.
func (it *dvSortedSetIter) GetValueCount() int { return int(it.f.numValues) }

// CheckIntegrity validates the checksum of the data file.
//
// Port of SimpleTextDocValuesReader.checkIntegrity().
func (r *SimpleTextDocValuesReader) CheckIntegrity() error {
	clone := r.data.Clone()
	if err := clone.SetPosition(0); err != nil {
		return fmt.Errorf("SimpleTextDocValuesReader.CheckIntegrity: seek(0): %w", err)
	}
	input := store2.NewBufferedChecksumIndexInput(clone)
	scratch := util.NewBytesRefBuilder()

	// checksum footer: "checksum " (9 bytes) + 20 decimal digits + 1 newline = 30 bytes
	footerStartPos := clone.Length() - int64(len(SimpleTextChecksumPrefix)+21)

	for {
		if err := stReadLine(input, scratch); err != nil {
			return fmt.Errorf("SimpleTextDocValuesReader.CheckIntegrity: readLine: %w", err)
		}
		if input.GetFilePointer() >= footerStartPos {
			if input.GetFilePointer() != footerStartPos {
				return fmt.Errorf(
					"SimpleTextDocValuesReader.CheckIntegrity: footer at wrong position %d, expected %d",
					input.GetFilePointer(), footerStartPos)
			}
			break
		}
	}

	expectedCS := fmt.Sprintf("%020d", input.GetChecksum())
	if err := stReadLine(input, scratch); err != nil {
		return fmt.Errorf("SimpleTextDocValuesReader.CheckIntegrity: read checksum line: %w", err)
	}
	line := scratch.Bytes()[:scratch.Length()]
	if len(line) < len(SimpleTextChecksumPrefix) {
		return fmt.Errorf("SimpleTextDocValuesReader.CheckIntegrity: expected checksum line, got: %s", line)
	}
	for i, b := range SimpleTextChecksumPrefix {
		if line[i] != b {
			return fmt.Errorf("SimpleTextDocValuesReader.CheckIntegrity: expected checksum line, got: %s", line)
		}
	}
	actualCS := string(line[len(SimpleTextChecksumPrefix):])
	if expectedCS != actualCS {
		return fmt.Errorf("SimpleTextDocValuesReader.CheckIntegrity: checksum mismatch: expected %s got %s",
			expectedCS, actualCS)
	}
	return nil
}

// Close releases the data file.
//
// Port of SimpleTextDocValuesReader.close().
func (r *SimpleTextDocValuesReader) Close() error {
	if err := r.data.Close(); err != nil {
		return fmt.Errorf("SimpleTextDocValuesReader.Close: %w", err)
	}
	return nil
}

// dvParsePatternInt parses an integer from a zero-padded pattern-formatted
// string. The pattern determines how many digits were written; we strip
// leading zeros and parse the remainder.
func dvParsePatternInt(s, _ string) (int, error) {
	trimmed := strings.TrimLeft(s, "0 ")
	if trimmed == "" {
		return 0, nil
	}
	v, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("dvParsePatternInt: %w", err)
	}
	return v, nil
}

// compile-time assertion.
var _ codecs.DocValuesProducer = (*SimpleTextDocValuesReader)(nil)
