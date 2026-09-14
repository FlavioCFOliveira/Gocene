// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SimpleText doc-values token prefixes (SimpleTextDocValuesWriter's static
// BytesRef constants).
var (
	dvEnd      = []byte("END")
	dvField    = []byte("field ")
	dvType     = []byte("  type ")
	dvDocCount = []byte("  doccount ")
	// used for numerics
	dvOrigin = []byte("  origin ") // for deltas

	dvMinValue      = []byte("  minalue ") // Java spells MINVALUE "  minalue "
	dvMaxValue      = []byte("  maxvalue ")
	dvMaxValueCount = []byte("  maxvaluecount ")

	dvPattern = []byte("  pattern ")
	// used for bytes
	dvLength    = []byte("length ")
	dvMaxLength = []byte("  maxlength ")
	// used for sorted bytes
	dvNumValues  = []byte("  numvalues ")
	dvOrdPattern = []byte("  ordpattern ")
	// used for skip data
	dvSkipData     = []byte("  skipdata")
	dvSkipInterval = []byte("    interval")
	dvSkipMinDocID = []byte("      minDocID ")
	dvSkipMaxDocID = []byte("      maxDocID ")
	dvSkipMinValue = []byte("      minValue ")
	dvSkipMaxValue = []byte("      maxValue ")
	dvSkipDocCount = []byte("      docCount ")
	dvSkipEnd      = []byte("  skipend")
)

// dvSkipIntervalSize mirrors SimpleTextDocValuesWriter.SKIP_INTERVAL_SIZE.
const dvSkipIntervalSize = 8

// noMoreDocs is the sentinel returned by DocValuesIterator.NextDoc.
const noMoreDocs = math.MaxInt32

// SimpleTextDocValuesWriter writes doc values as plain text.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextDocValuesWriter
// (Apache Lucene 10.5.0), which extends DocValuesConsumer and therefore
// inherits its merge members through the embedded BaseDocValuesConsumer.
// The Java assertions (fieldSeen, the per-document asserts) are not rendered.
type SimpleTextDocValuesWriter struct {
	*codecs.BaseDocValuesConsumer

	data    *store.ChecksumIndexOutput
	scratch *util.BytesRefBuilder
	numDocs int
}

// NewSimpleTextDocValuesWriter opens the doc-values output file and returns the
// writer.
//
// Port of SimpleTextDocValuesWriter(SegmentWriteState, String).
func NewSimpleTextDocValuesWriter(state *codecs.SegmentWriteState, ext string) (*SimpleTextDocValuesWriter, error) {
	fileName := store.SegmentFileName(
		state.SegmentInfo.Name(),
		state.SegmentSuffix,
		ext,
	)
	raw, err := state.Directory.CreateOutput(fileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("SimpleTextDocValuesWriter: create %s: %w", fileName, err)
	}
	w := &SimpleTextDocValuesWriter{
		data:    store.NewChecksumIndexOutput(raw),
		scratch: util.NewBytesRefBuilder(),
		numDocs: state.SegmentInfo.MaxDoc(),
	}
	w.BaseDocValuesConsumer = codecs.NewBaseDocValuesConsumer(w)
	return w, nil
}

// skipInterval mirrors the record SimpleTextDocValuesWriter.SkipInterval.
type skipIntervalRecord struct {
	minDocID int
	maxDocID int
	minValue int64
	maxValue int64
	docCount int
}

// AddNumericField writes all numeric doc values for field.
//
// Port of SimpleTextDocValuesWriter.addNumericField(FieldInfo, DocValuesProducer).
func (w *SimpleTextDocValuesWriter) AddNumericField(field *index.FieldInfo, valuesProducer codecs.DocValuesProducer) error {
	if err := w.writeFieldEntry(field, index.DocValuesTypeNumeric); err != nil {
		return err
	}

	// first pass to find min/max and accumulate skip intervals
	minValue := int64(math.MaxInt64)
	maxValue := int64(math.MinInt64)
	values, err := valuesProducer.GetNumeric(field)
	if err != nil {
		return err
	}
	numValues := 0
	var skipIntervals []skipIntervalRecord
	intervalDocCount := 0
	intervalMinDocID := -1
	intervalMaxDocID := -1
	intervalMinValue := int64(math.MaxInt64)
	intervalMaxValue := int64(math.MinInt64)
	for {
		doc, err := values.NextDoc()
		if err != nil {
			return err
		}
		if doc == noMoreDocs {
			break
		}
		v, err := values.LongValue()
		if err != nil {
			return err
		}
		minValue = minInt64(minValue, v)
		maxValue = maxInt64(maxValue, v)
		numValues++
		if intervalDocCount == 0 {
			intervalMinDocID = doc
		}
		intervalMaxDocID = doc
		intervalMinValue = minInt64(intervalMinValue, v)
		intervalMaxValue = maxInt64(intervalMaxValue, v)
		intervalDocCount++
		if intervalDocCount == dvSkipIntervalSize {
			skipIntervals = append(skipIntervals, skipIntervalRecord{
				intervalMinDocID, intervalMaxDocID, intervalMinValue, intervalMaxValue, intervalDocCount,
			})
			intervalDocCount = 0
			intervalMinValue = math.MaxInt64
			intervalMaxValue = math.MinInt64
		}
	}
	if intervalDocCount > 0 {
		skipIntervals = append(skipIntervals, skipIntervalRecord{
			intervalMinDocID, intervalMaxDocID, intervalMinValue, intervalMaxValue, intervalDocCount,
		})
	}

	// write absolute min and max for skipper
	if err := w.writeLine(dvMinValue, strconv.FormatInt(minValue, 10)); err != nil {
		return err
	}
	if err := w.writeLine(dvMaxValue, strconv.FormatInt(maxValue, 10)); err != nil {
		return err
	}
	if err := w.writeLine(dvDocCount, strconv.Itoa(numValues)); err != nil {
		return err
	}
	maxValueCount := 1
	if numValues == 0 {
		maxValueCount = 0
	}
	if err := w.writeLine(dvMaxValueCount, strconv.Itoa(maxValueCount)); err != nil {
		return err
	}

	if numValues != w.numDocs {
		minValue = minInt64(minValue, 0)
		maxValue = maxInt64(maxValue, 0)
	}

	// write our minimum value to the .dat, all entries are deltas from that
	if err := w.writeLine(dvOrigin, strconv.FormatInt(minValue, 10)); err != nil {
		return err
	}

	// build up our fixed-width "simple text packed ints" format
	maxBig := big.NewInt(maxValue)
	minBig := big.NewInt(minValue)
	diffBig := new(big.Int).Sub(maxBig, minBig)
	maxBytesPerValue := len(diffBig.String())
	patternString := strings.Repeat("0", maxBytesPerValue)

	// write our pattern to the .dat
	if err := w.writeLine(dvPattern, patternString); err != nil {
		return err
	}

	// second pass to write the values
	values, err = valuesProducer.GetNumeric(field)
	if err != nil {
		return err
	}
	for i := 0; i < w.numDocs; i++ {
		if values.DocID() < i {
			if _, err := values.NextDoc(); err != nil {
				return err
			}
		}
		var value int64
		if values.DocID() == i {
			value, err = values.LongValue()
			if err != nil {
				return err
			}
		}
		delta := new(big.Int).Sub(big.NewInt(value), minBig)
		if err := w.writeStr(decimalFormat(delta.String(), len(patternString))); err != nil {
			return err
		}
		if err := w.newline(); err != nil {
			return err
		}
		flag := "T"
		if values.DocID() != i {
			flag = "F"
		}
		if err := w.writeStr(flag); err != nil {
			return err
		}
		if err := w.newline(); err != nil {
			return err
		}
	}

	return w.writeSkipData(skipIntervals)
}

// AddBinaryField writes all binary doc values for field.
//
// Port of SimpleTextDocValuesWriter.addBinaryField(FieldInfo, DocValuesProducer).
func (w *SimpleTextDocValuesWriter) AddBinaryField(field *index.FieldInfo, valuesProducer codecs.DocValuesProducer) error {
	if err := w.writeFieldEntry(field, index.DocValuesTypeBinary); err != nil {
		return err
	}
	return w.doAddBinaryField(field, valuesProducer)
}

// doAddBinaryField is doAddBinaryField(FieldInfo, DocValuesProducer).
func (w *SimpleTextDocValuesWriter) doAddBinaryField(field *index.FieldInfo, valuesProducer codecs.DocValuesProducer) error {
	return w.doAddBinaryFieldWithMaxValueCount(field, valuesProducer, -1)
}

// doAddBinaryFieldWithMaxValueCount is
// doAddBinaryField(FieldInfo, DocValuesProducer, int), shared by
// AddBinaryField and AddSortedNumericField.
func (w *SimpleTextDocValuesWriter) doAddBinaryFieldWithMaxValueCount(
	field *index.FieldInfo,
	valuesProducer codecs.DocValuesProducer,
	maxValueCount int,
) error {
	maxLength := 0
	values, err := valuesProducer.GetBinary(field)
	if err != nil {
		return err
	}
	docCount := 0
	for {
		doc, err := values.NextDoc()
		if err != nil {
			return err
		}
		if doc == noMoreDocs {
			break
		}
		docCount++
		v, err := values.BinaryValue()
		if err != nil {
			return err
		}
		if l := len(bytesRefToString(v)); l > maxLength {
			maxLength = l
		}
	}

	if err := w.writeLine(dvDocCount, strconv.Itoa(docCount)); err != nil {
		return err
	}

	writtenMaxValueCount := maxValueCount
	if maxValueCount == -1 {
		writtenMaxValueCount = 1
		if docCount == 0 {
			writtenMaxValueCount = 0
		}
	}
	if err := w.writeLine(dvMaxValueCount, strconv.Itoa(writtenMaxValueCount)); err != nil {
		return err
	}

	// write maxLength
	if err := w.writeLine(dvMaxLength, strconv.Itoa(maxLength)); err != nil {
		return err
	}

	maxBytesLength := len(strconv.FormatInt(int64(maxLength), 10))
	// write our pattern for encoding lengths
	if err := w.writeLine(dvPattern, strings.Repeat("0", maxBytesLength)); err != nil {
		return err
	}

	values, err = valuesProducer.GetBinary(field)
	if err != nil {
		return err
	}
	for i := 0; i < w.numDocs; i++ {
		if values.DocID() < i {
			if _, err := values.NextDoc(); err != nil {
				return err
			}
		}
		hasValue := values.DocID() == i
		stringVal := ""
		if hasValue {
			v, err := values.BinaryValue()
			if err != nil {
				return err
			}
			stringVal = bytesRefToString(v)
		}
		// write length
		length := len(stringVal)
		if err := w.write(dvLength); err != nil {
			return err
		}
		if err := w.writeStr(decimalFormat(strconv.Itoa(length), maxBytesLength)); err != nil {
			return err
		}
		if err := w.newline(); err != nil {
			return err
		}

		// write bytes as hex array
		if hasValue {
			if err := w.writeStr(stringVal); err != nil {
				return err
			}
		}

		// pad to fit
		for j := length; j < maxLength; j++ {
			if err := w.data.WriteByte(' '); err != nil {
				return err
			}
		}
		if err := w.newline(); err != nil {
			return err
		}
		flag := "T"
		if !hasValue {
			flag = "F"
		}
		if err := w.writeStr(flag); err != nil {
			return err
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
func (w *SimpleTextDocValuesWriter) AddSortedField(field *index.FieldInfo, valuesProducer codecs.DocValuesProducer) error {
	if err := w.writeFieldEntry(field, index.DocValuesTypeSorted); err != nil {
		return err
	}

	docCount := 0
	var skipIntervals []skipIntervalRecord
	intervalDocCount := 0
	intervalMinDocID := -1
	intervalMaxDocID := -1
	intervalMinValue := int64(math.MaxInt64)
	intervalMaxValue := int64(math.MinInt64)
	values, err := valuesProducer.GetSorted(field)
	if err != nil {
		return err
	}
	for {
		doc, err := values.NextDoc()
		if err != nil {
			return err
		}
		if doc == noMoreDocs {
			break
		}
		docCount++
		ord, err := values.OrdValue()
		if err != nil {
			return err
		}
		if intervalDocCount == 0 {
			intervalMinDocID = doc
		}
		intervalMaxDocID = doc
		intervalMinValue = minInt64(intervalMinValue, int64(ord))
		intervalMaxValue = maxInt64(intervalMaxValue, int64(ord))
		intervalDocCount++
		if intervalDocCount == dvSkipIntervalSize {
			skipIntervals = append(skipIntervals, skipIntervalRecord{
				intervalMinDocID, intervalMaxDocID, intervalMinValue, intervalMaxValue, intervalDocCount,
			})
			intervalDocCount = 0
			intervalMinValue = math.MaxInt64
			intervalMaxValue = math.MinInt64
		}
	}
	if intervalDocCount > 0 {
		skipIntervals = append(skipIntervals, skipIntervalRecord{
			intervalMinDocID, intervalMaxDocID, intervalMinValue, intervalMaxValue, intervalDocCount,
		})
	}

	if err := w.writeLine(dvDocCount, strconv.Itoa(docCount)); err != nil {
		return err
	}
	maxValueCount := 1
	if docCount == 0 {
		maxValueCount = 0
	}
	if err := w.writeLine(dvMaxValueCount, strconv.Itoa(maxValueCount)); err != nil {
		return err
	}

	valueCount := 0
	maxLength := -1
	sorted, err := valuesProducer.GetSorted(field)
	if err != nil {
		return err
	}
	terms, err := dvWriterSortedTermsEnum(sorted)
	if err != nil {
		return err
	}
	for {
		value, err := terms.Next()
		if err != nil {
			return err
		}
		if value == nil {
			break
		}
		if l := value.Bytes.Length; l > maxLength {
			maxLength = l
		}
		valueCount++
	}

	// write numValues
	if err := w.writeLine(dvNumValues, strconv.Itoa(valueCount)); err != nil {
		return err
	}

	// write maxLength
	if err := w.writeLine(dvMaxLength, strconv.Itoa(maxLength)); err != nil {
		return err
	}

	maxBytesLength := len(strconv.Itoa(maxLength))
	// write our pattern for encoding lengths
	if err := w.writeLine(dvPattern, strings.Repeat("0", maxBytesLength)); err != nil {
		return err
	}

	maxOrdBytes := len(strconv.FormatInt(int64(valueCount)+1, 10))
	// write our pattern for ords
	if err := w.writeLine(dvOrdPattern, strings.Repeat("0", maxOrdBytes)); err != nil {
		return err
	}

	sorted, err = valuesProducer.GetSorted(field)
	if err != nil {
		return err
	}
	terms, err = dvWriterSortedTermsEnum(sorted)
	if err != nil {
		return err
	}
	if err := w.writeTermsDict(terms, maxBytesLength, maxLength); err != nil {
		return err
	}

	values, err = valuesProducer.GetSorted(field)
	if err != nil {
		return err
	}
	for i := 0; i < w.numDocs; i++ {
		if values.DocID() < i {
			if _, err := values.NextDoc(); err != nil {
				return err
			}
		}
		ord := -1
		if values.DocID() == i {
			ord, err = values.OrdValue()
			if err != nil {
				return err
			}
		}
		if err := w.writeStr(decimalFormat(strconv.FormatInt(int64(ord)+1, 10), maxOrdBytes)); err != nil {
			return err
		}
		if err := w.newline(); err != nil {
			return err
		}
	}

	return w.writeSkipData(skipIntervals)
}

// AddSortedNumericField writes sorted-numeric doc values for field.
//
// Port of SimpleTextDocValuesWriter.addSortedNumericField(FieldInfo, DocValuesProducer).
func (w *SimpleTextDocValuesWriter) AddSortedNumericField(field *index.FieldInfo, valuesProducer codecs.DocValuesProducer) error {
	if err := w.writeFieldEntry(field, index.DocValuesTypeSortedNumeric); err != nil {
		return err
	}

	minValue := int64(math.MaxInt64)
	maxValue := int64(math.MinInt64)
	maxValueCount := 0
	var skipIntervals []skipIntervalRecord
	intervalDocCount := 0
	intervalMinDocID := -1
	intervalMaxDocID := -1
	intervalMinValue := int64(math.MaxInt64)
	intervalMaxValue := int64(math.MinInt64)
	values, err := valuesProducer.GetSortedNumeric(field)
	if err != nil {
		return err
	}
	for {
		doc, err := values.NextDoc()
		if err != nil {
			return err
		}
		if doc == noMoreDocs {
			break
		}
		valueCount, err := values.DocValueCount()
		if err != nil {
			return err
		}
		if valueCount > maxValueCount {
			maxValueCount = valueCount
		}
		if intervalDocCount == 0 {
			intervalMinDocID = doc
		}
		intervalMaxDocID = doc
		for i := 0; i < valueCount; i++ {
			v, err := values.NextValue()
			if err != nil {
				return err
			}
			minValue = minInt64(minValue, v)
			maxValue = maxInt64(maxValue, v)
			intervalMinValue = minInt64(intervalMinValue, v)
			intervalMaxValue = maxInt64(intervalMaxValue, v)
		}
		intervalDocCount++
		if intervalDocCount == dvSkipIntervalSize {
			skipIntervals = append(skipIntervals, skipIntervalRecord{
				intervalMinDocID, intervalMaxDocID, intervalMinValue, intervalMaxValue, intervalDocCount,
			})
			intervalDocCount = 0
			intervalMinValue = math.MaxInt64
			intervalMaxValue = math.MinInt64
		}
	}
	if intervalDocCount > 0 {
		skipIntervals = append(skipIntervals, skipIntervalRecord{
			intervalMinDocID, intervalMaxDocID, intervalMinValue, intervalMaxValue, intervalDocCount,
		})
	}

	// write absolute min and max for skipper
	if err := w.writeLine(dvMinValue, strconv.FormatInt(minValue, 10)); err != nil {
		return err
	}
	if err := w.writeLine(dvMaxValue, strconv.FormatInt(maxValue, 10)); err != nil {
		return err
	}

	if err := w.doAddBinaryFieldWithMaxValueCount(
		field,
		&sortedNumericAsBinaryDocValuesProducer{valuesProducer: valuesProducer},
		maxValueCount,
	); err != nil {
		return err
	}

	return w.writeSkipData(skipIntervals)
}

// sortedNumericAsBinaryDocValuesProducer is the anonymous
// EmptyDocValuesProducer subclass addSortedNumericField hands to
// doAddBinaryField.
type sortedNumericAsBinaryDocValuesProducer struct {
	index.EmptyDocValuesProducer
	valuesProducer codecs.DocValuesProducer
}

// GetBinary presents each document's sorted numeric values as the comma
// separated decimal string of its values.
func (p *sortedNumericAsBinaryDocValuesProducer) GetBinary(field *index.FieldInfo) (codecs.BinaryDocValues, error) {
	values, err := p.valuesProducer.GetSortedNumeric(field)
	if err != nil {
		return nil, err
	}
	return &sortedNumericAsBinaryDocValues{values: values}, nil
}

// GetMergeInstance returns the receiver (DocValuesProducer default).
func (p *sortedNumericAsBinaryDocValuesProducer) GetMergeInstance() codecs.DocValuesProducer {
	return p
}

// sortedNumericAsBinaryDocValues is the anonymous BinaryDocValues built by
// the producer above.
type sortedNumericAsBinaryDocValues struct {
	values      codecs.SortedNumericDocValues
	builder     strings.Builder
	binaryValue []byte
}

func (b *sortedNumericAsBinaryDocValues) NextDoc() (int, error) {
	doc, err := b.values.NextDoc()
	if err != nil {
		return 0, err
	}
	if err := b.setCurrentDoc(); err != nil {
		return 0, err
	}
	return doc, nil
}

func (b *sortedNumericAsBinaryDocValues) DocID() int {
	return b.values.DocID()
}

func (b *sortedNumericAsBinaryDocValues) Cost() int64 {
	return b.values.Cost()
}

func (b *sortedNumericAsBinaryDocValues) Advance(target int) (int, error) {
	doc, err := b.values.Advance(target)
	if err != nil {
		return 0, err
	}
	if err := b.setCurrentDoc(); err != nil {
		return 0, err
	}
	return doc, nil
}

func (b *sortedNumericAsBinaryDocValues) AdvanceExact(target int) (bool, error) {
	ok, err := b.values.AdvanceExact(target)
	if err != nil {
		return false, err
	}
	if ok {
		if err := b.setCurrentDoc(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (b *sortedNumericAsBinaryDocValues) setCurrentDoc() error {
	if b.DocID() == noMoreDocs {
		return nil
	}
	b.builder.Reset()
	count, err := b.values.DocValueCount()
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		if i > 0 {
			b.builder.WriteByte(',')
		}
		v, err := b.values.NextValue()
		if err != nil {
			return err
		}
		b.builder.WriteString(strconv.FormatInt(v, 10))
	}
	b.binaryValue = []byte(b.builder.String())
	return nil
}

func (b *sortedNumericAsBinaryDocValues) BinaryValue() ([]byte, error) {
	return b.binaryValue, nil
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int).
func (b *sortedNumericAsBinaryDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(b, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd().
func (b *sortedNumericAsBinaryDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(b)
}

// AddSortedSetField writes sorted-set doc values for field.
//
// Port of SimpleTextDocValuesWriter.addSortedSetField(FieldInfo, DocValuesProducer).
func (w *SimpleTextDocValuesWriter) AddSortedSetField(field *index.FieldInfo, valuesProducer codecs.DocValuesProducer) error {
	if err := w.writeFieldEntry(field, index.DocValuesTypeSortedSet); err != nil {
		return err
	}

	docCount := 0
	maxValueCount := 0
	var skipIntervals []skipIntervalRecord
	intervalDocCount := 0
	intervalMinDocID := -1
	intervalMaxDocID := -1
	intervalMinValue := int64(math.MaxInt64)
	intervalMaxValue := int64(math.MinInt64)
	values, err := valuesProducer.GetSortedSet(field)
	if err != nil {
		return err
	}
	for {
		doc, err := values.NextDoc()
		if err != nil {
			return err
		}
		if doc == noMoreDocs {
			break
		}
		docCount++
		valueCount := values.DocValueCount()
		if valueCount > maxValueCount {
			maxValueCount = valueCount
		}
		if intervalDocCount == 0 {
			intervalMinDocID = doc
		}
		intervalMaxDocID = doc
		for i := 0; i < valueCount; i++ {
			ord, err := values.NextOrd()
			if err != nil {
				return err
			}
			intervalMinValue = minInt64(intervalMinValue, int64(ord))
			intervalMaxValue = maxInt64(intervalMaxValue, int64(ord))
		}
		intervalDocCount++
		if intervalDocCount == dvSkipIntervalSize {
			skipIntervals = append(skipIntervals, skipIntervalRecord{
				intervalMinDocID, intervalMaxDocID, intervalMinValue, intervalMaxValue, intervalDocCount,
			})
			intervalDocCount = 0
			intervalMinValue = math.MaxInt64
			intervalMaxValue = math.MinInt64
		}
	}
	if intervalDocCount > 0 {
		skipIntervals = append(skipIntervals, skipIntervalRecord{
			intervalMinDocID, intervalMaxDocID, intervalMinValue, intervalMaxValue, intervalDocCount,
		})
	}

	if err := w.writeLine(dvDocCount, strconv.Itoa(docCount)); err != nil {
		return err
	}
	if err := w.writeLine(dvMaxValueCount, strconv.Itoa(maxValueCount)); err != nil {
		return err
	}

	var valueCount int64
	maxLength := 0
	sortedSet, err := valuesProducer.GetSortedSet(field)
	if err != nil {
		return err
	}
	terms, err := dvWriterSortedSetTermsEnum(sortedSet)
	if err != nil {
		return err
	}
	for {
		value, err := terms.Next()
		if err != nil {
			return err
		}
		if value == nil {
			break
		}
		if l := value.Bytes.Length; l > maxLength {
			maxLength = l
		}
		valueCount++
	}

	// write numValues
	if err := w.writeLine(dvNumValues, strconv.FormatInt(valueCount, 10)); err != nil {
		return err
	}

	// write maxLength
	if err := w.writeLine(dvMaxLength, strconv.Itoa(maxLength)); err != nil {
		return err
	}

	maxBytesLength := len(strconv.Itoa(maxLength))
	// write our pattern for encoding lengths
	if err := w.writeLine(dvPattern, strings.Repeat("0", maxBytesLength)); err != nil {
		return err
	}

	// compute ord pattern: this is funny, we encode all values for all docs to
	// find the maximum length
	maxOrdListLength := 0
	var sb2 strings.Builder
	values, err = valuesProducer.GetSortedSet(field)
	if err != nil {
		return err
	}
	for {
		doc, err := values.NextDoc()
		if err != nil {
			return err
		}
		if doc == noMoreDocs {
			break
		}
		sb2.Reset()
		for i := 0; i < values.DocValueCount(); i++ {
			if sb2.Len() > 0 {
				sb2.WriteString(",")
			}
			ord, err := values.NextOrd()
			if err != nil {
				return err
			}
			sb2.WriteString(strconv.FormatInt(int64(ord), 10))
		}
		if sb2.Len() > maxOrdListLength {
			maxOrdListLength = sb2.Len()
		}
	}

	// write our pattern for ord lists
	if err := w.writeLine(dvOrdPattern, strings.Repeat("X", maxOrdListLength)); err != nil {
		return err
	}

	sortedSet, err = valuesProducer.GetSortedSet(field)
	if err != nil {
		return err
	}
	terms, err = dvWriterSortedSetTermsEnum(sortedSet)
	if err != nil {
		return err
	}
	if err := w.writeTermsDict(terms, maxBytesLength, maxLength); err != nil {
		return err
	}

	values, err = valuesProducer.GetSortedSet(field)
	if err != nil {
		return err
	}

	// write the ords for each doc comma-separated
	for i := 0; i < w.numDocs; i++ {
		if values.DocID() < i {
			if _, err := values.NextDoc(); err != nil {
				return err
			}
		}
		sb2.Reset()
		if values.DocID() == i {
			for j := 0; j < values.DocValueCount(); j++ {
				if sb2.Len() > 0 {
					sb2.WriteString(",")
				}
				ord, err := values.NextOrd()
				if err != nil {
					return err
				}
				sb2.WriteString(strconv.FormatInt(int64(ord), 10))
			}
		}
		// now pad to fit: these are numbers so spaces work well. reader calls trim()
		numPadding := maxOrdListLength - sb2.Len()
		for j := 0; j < numPadding; j++ {
			sb2.WriteByte(' ')
		}
		if err := w.writeStr(sb2.String()); err != nil {
			return err
		}
		if err := w.newline(); err != nil {
			return err
		}
	}

	return w.writeSkipData(skipIntervals)
}

// writeTermsDict writes every term of terms as "length N\n<bytes><padding>\n",
// the dictionary loop shared by addSortedField and addSortedSetField. The
// bytes are written raw: SimpleText.write would escape them.
func (w *SimpleTextDocValuesWriter) writeTermsDict(terms spi.TermsEnum, maxBytesLength, maxLength int) error {
	for {
		value, err := terms.Next()
		if err != nil {
			return err
		}
		if value == nil {
			return nil
		}
		bytes := value.Bytes
		// write length
		if err := w.write(dvLength); err != nil {
			return err
		}
		if err := w.writeStr(decimalFormat(strconv.Itoa(bytes.Length), maxBytesLength)); err != nil {
			return err
		}
		if err := w.newline(); err != nil {
			return err
		}

		// write bytes -- don't use SimpleText.write because it escapes:
		if err := w.data.WriteBytes(bytes.Bytes, bytes.Offset, bytes.Length); err != nil {
			return err
		}

		// pad to fit
		for i := bytes.Length; i < maxLength; i++ {
			if err := w.data.WriteByte(' '); err != nil {
				return err
			}
		}
		if err := w.newline(); err != nil {
			return err
		}
	}
}

// writeSkipData writes the skip intervals.
//
// Port of SimpleTextDocValuesWriter.writeSkipData(List<SkipInterval>).
func (w *SimpleTextDocValuesWriter) writeSkipData(intervals []skipIntervalRecord) error {
	if err := w.write(dvSkipData); err != nil {
		return err
	}
	if err := w.newline(); err != nil {
		return err
	}
	for _, interval := range intervals {
		if err := w.write(dvSkipInterval); err != nil {
			return err
		}
		if err := w.newline(); err != nil {
			return err
		}
		if err := w.writeLine(dvSkipMinDocID, strconv.Itoa(interval.minDocID)); err != nil {
			return err
		}
		if err := w.writeLine(dvSkipMaxDocID, strconv.Itoa(interval.maxDocID)); err != nil {
			return err
		}
		if err := w.writeLine(dvSkipMinValue, strconv.FormatInt(interval.minValue, 10)); err != nil {
			return err
		}
		if err := w.writeLine(dvSkipMaxValue, strconv.FormatInt(interval.maxValue, 10)); err != nil {
			return err
		}
		if err := w.writeLine(dvSkipDocCount, strconv.Itoa(interval.docCount)); err != nil {
			return err
		}
	}
	if err := w.write(dvSkipEnd); err != nil {
		return err
	}
	return w.newline()
}

// writeFieldEntry writes the header for this field.
//
// Port of SimpleTextDocValuesWriter.writeFieldEntry(FieldInfo, DocValuesType).
func (w *SimpleTextDocValuesWriter) writeFieldEntry(field *index.FieldInfo, docValuesType index.DocValuesType) error {
	if err := w.writeLine(dvField, field.Name()); err != nil {
		return err
	}
	return w.writeLine(dvType, docValuesType.String())
}

// Close writes the END marker and checksum.
//
// Port of SimpleTextDocValuesWriter.close().
func (w *SimpleTextDocValuesWriter) Close() error {
	if w.data == nil {
		return nil
	}
	out := w.data
	err := w.write(dvEnd)
	if err == nil {
		err = w.newline()
	}
	if err == nil {
		err = stWriteChecksum(out, w.scratch)
	}
	w.data = nil
	if err != nil {
		// IOUtils.closeWhileHandlingException(data)
		if closeErr := out.Close(); closeErr != nil {
			return fmt.Errorf("%w (close: %v)", err, closeErr)
		}
		return err
	}
	return out.Close()
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

// writeLine writes prefix, s and a newline: the
// SimpleTextUtil.write(data, PREFIX); write(data, s, scratch); writeNewline
// triple the Java writer repeats for every header line.
func (w *SimpleTextDocValuesWriter) writeLine(prefix []byte, s string) error {
	if err := w.write(prefix); err != nil {
		return err
	}
	if err := w.writeStr(s); err != nil {
		return err
	}
	return w.newline()
}

// decimalFormat renders new DecimalFormat(pattern).format(n) for a pattern of
// minDigits zeros: the decimal digits of n left-padded with zeros to at least
// minDigits digits, keeping a leading minus sign.
func decimalFormat(digits string, minDigits int) string {
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign = "-"
		digits = digits[1:]
	}
	if len(digits) < minDigits {
		digits = strings.Repeat("0", minDigits-len(digits)) + digits
	}
	return sign + digits
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// dvWriterSortedTermsEnum renders the virtual call SortedDocValues.termsEnum():
// the implementation's own override when it declares one, otherwise the
// default body, new SortedDocValuesTermsEnum(this).
func dvWriterSortedTermsEnum(values codecs.SortedDocValues) (spi.TermsEnum, error) {
	if override, ok := values.(interface {
		TermsEnum() (spi.TermsEnum, error)
	}); ok {
		return override.TermsEnum()
	}
	return index.OpenTermsEnum("", values)
}

// dvWriterSortedSetTermsEnum renders the virtual call
// SortedSetDocValues.termsEnum(): the implementation's own override when it
// declares one, otherwise the default body, new
// SortedSetDocValuesTermsEnum(this).
func dvWriterSortedSetTermsEnum(values codecs.SortedSetDocValues) (spi.TermsEnum, error) {
	if override, ok := values.(interface {
		TermsEnum() (spi.TermsEnum, error)
	}); ok {
		return override.TermsEnum()
	}
	return index.OpenSetTermsEnum("", values)
}

// bytesRefToString converts raw bytes to their BytesRef.toString() hex
// representation. SimpleTextDocValuesWriter.doAddBinaryField calls
// binaryValue().toString(), which for a BytesRef returns the hex form
// "[xx yy zz]" (hex bytes, space-separated, bracketed).
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
