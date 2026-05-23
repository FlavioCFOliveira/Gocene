// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"bytes"
	"fmt"
	"math"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ---------------------------------------------------------------------------
// SimpleTextSkipWriter constants (defined here because SimpleTextSkipReader
// imports them; SimpleTextSkipWriter will reference these same vars when
// ported in task 3200).
// ---------------------------------------------------------------------------

// skipMultiplier is the fan-out between adjacent skip levels.
const skipMultiplier = 3

// maxSkipLevels caps the height of the SimpleText skip list.
const maxSkipLevels = 4

// skipBlockSize is the number of postings between skip entries at level 0.
const skipBlockSize = 8

// Skip-record token prefixes (mirrors SimpleTextSkipWriter static fields).
var (
	stSkipList    = []byte("    skipList ")
	stLevelLength = []byte("      levelLength ")
	stLevel       = []byte("      level ")
	stSkipDoc     = []byte("        skipDoc ")
	stSkipDocFP   = []byte("        skipDocFP ")
	stImpacts     = []byte("        impacts ")
	stImpact      = []byte("          impact ")
	stFreq        = []byte("            freq ")
	stNorm        = []byte("            norm ")
	stImpactsEnd  = []byte("        impactsEnd ")
	stChildPtr    = []byte("        childPointer ")
)

// SimpleTextFieldsWriter constants needed by the skip reader.
var (
	stEnd   = []byte("END")
	stTerm  = []byte("  term ")
	stField = []byte("field ")
)

// ---------------------------------------------------------------------------
// SimpleTextUtil helpers (used by the skip reader; full implementation in
// task 3202 / simple_text_util.go).
// ---------------------------------------------------------------------------

// stReadLine reads one newline-terminated, escape-processed line from in
// into scratch.  Mirrors SimpleTextUtil.readLine.
func stReadLine(in store.DataInput, scratch *util.BytesRefBuilder) error {
	upto := 0
	for {
		b, err := in.ReadByte()
		if err != nil {
			return err
		}
		if b == 92 { // ESCAPE
			esc, err2 := in.ReadByte()
			if err2 != nil {
				return err2
			}
			scratch.Grow(upto + 1)
			scratch.SetByteAt(upto, esc)
			upto++
		} else if b == 10 { // NEWLINE
			break
		} else {
			scratch.Grow(upto + 1)
			scratch.SetByteAt(upto, b)
			upto++
		}
	}
	scratch.SetLength(upto)
	return nil
}

// stStartsWith reports whether b has the given prefix.
func stStartsWith(b []byte, prefix []byte) bool {
	return bytes.HasPrefix(b, prefix)
}

// stParseInt parses an ASCII integer from b[prefixLen:].
func stParseInt(b []byte, prefixLen int) (int, error) {
	return strconv.Atoi(string(b[prefixLen:]))
}

// stParseLong parses an ASCII int64 from b[prefixLen:].
func stParseLong(b []byte, prefixLen int) (int64, error) {
	return strconv.ParseInt(string(b[prefixLen:]), 10, 64)
}

// ---------------------------------------------------------------------------
// SimpleTextSkipReader
// ---------------------------------------------------------------------------

// SimpleTextSkipReader reads skip lists with multiple levels encoded as plain
// text by SimpleTextSkipWriter.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextSkipReader (Lucene
// 10.4.0).
type SimpleTextSkipReader struct {
	base *codecs.MultiLevelSkipListReader

	scratch *util.BytesRefBuilder

	// perLevelImpacts holds decoded impact pairs for each level.
	perLevelImpacts []*index.FreqAndNormBuffer

	// numLevels is the number of active skip levels as of the last SkipTo.
	numLevels int

	// nextSkipDocFP is the doc file pointer stored in the last skip entry.
	nextSkipDocFP int64

	// hasSkipList is true when a non-empty skip pointer was provided at reset.
	hasSkipList bool

	// impactsView is the Impacts implementation exposed via GetImpacts.
	impactsView index.Impacts
}

// NewSimpleTextSkipReader constructs a SimpleTextSkipReader backed by
// skipStream.
//
// Port of SimpleTextSkipReader(IndexInput).
func NewSimpleTextSkipReader(skipStream store.IndexInput) *SimpleTextSkipReader {
	r := &SimpleTextSkipReader{
		scratch:         util.NewBytesRefBuilder(),
		perLevelImpacts: make([]*index.FreqAndNormBuffer, maxSkipLevels),
		numLevels:       1,
	}

	for i := range r.perLevelImpacts {
		b := index.NewFreqAndNormBuffer()
		b.Add(math.MaxInt32, 1)
		r.perLevelImpacts[i] = b
	}

	// Build the Impacts view backed by live state.
	r.impactsView = &simpleTextImpacts{r: r}

	// Construct the base MultiLevelSkipListReader with our hooks.
	r.base = codecs.NewMultiLevelSkipListReader(
		skipStream,
		maxSkipLevels,
		skipBlockSize,
		skipMultiplier,
		r.readSkipData,
	)
	r.base.SetReadLevelLength(r.readLevelLength)
	r.base.SetReadChildPointer(r.readChildPointer)

	return r
}

// SkipTo advances to the largest skip entry whose doc id < target.
// Returns the last doc reached, or -1 if the skip list is empty.
//
// Port of SimpleTextSkipReader.skipTo(int).
func (r *SimpleTextSkipReader) SkipTo(target int) (int, error) {
	if !r.hasSkipList {
		return -1, nil
	}
	result, err := r.base.SkipTo(target)
	if err != nil {
		return result, err
	}
	n := r.base.NumberOfSkipLevels()
	if n > 0 {
		r.numLevels = n
	} else {
		// End of postings: fill with dummy data like SlowImpactsEnum.
		r.numLevels = 1
		b := r.perLevelImpacts[0]
		b.Size = 0
		b.Add(math.MaxInt32, 1)
	}
	return result, nil
}

// readSkipData decodes one text-format skip entry for level from skipStream.
// Returns the doc delta (skipDoc - previousSkipDoc[level]).
//
// Port of SimpleTextSkipReader.readSkipData(int, IndexInput).
func (r *SimpleTextSkipReader) readSkipData(level int, skipStream store.IndexInput) (int, error) {
	r.perLevelImpacts[level].Size = 0
	skipDoc := index.NO_MORE_DOCS
	input := store.NewChecksumIndexInput(skipStream)
	freq := 1
	for {
		if err := stReadLine(input, r.scratch); err != nil {
			return 0, fmt.Errorf("SimpleTextSkipReader.readSkipData: readLine: %w", err)
		}
		line := r.scratch.Bytes()[:r.scratch.Length()]

		switch {
		case bytes.Equal(line, stEnd):
			if err := stCheckFooter(input); err != nil {
				return 0, err
			}
			return 0, nil

		case bytes.Equal(line, stImpactsEnd),
			bytes.Equal(line, stTerm),
			bytes.Equal(line, stField):
			// End of this skip entry.
			if skipDoc == index.NO_MORE_DOCS {
				return 0, nil
			}
			// Convert absolute doc to delta (Java: skipDoc - super.skipDoc[level]).
			delta := skipDoc - r.base.GetSkipDoc(level)
			return delta, nil

		case stStartsWith(line, stSkipList):
			// header line — continue

		case stStartsWith(line, stSkipDoc):
			abs, err := stParseInt(line, len(stSkipDoc))
			if err != nil {
				return 0, fmt.Errorf("SimpleTextSkipReader: parse SKIP_DOC: %w", err)
			}
			skipDoc = abs

		case stStartsWith(line, stSkipDocFP):
			fp, err := stParseLong(line, len(stSkipDocFP))
			if err != nil {
				return 0, fmt.Errorf("SimpleTextSkipReader: parse SKIP_DOC_FP: %w", err)
			}
			r.nextSkipDocFP = fp

		case stStartsWith(line, stImpacts), stStartsWith(line, stImpact):
			// separator lines — continue

		case stStartsWith(line, stFreq):
			f, err := stParseInt(line, len(stFreq))
			if err != nil {
				return 0, fmt.Errorf("SimpleTextSkipReader: parse FREQ: %w", err)
			}
			freq = f

		case stStartsWith(line, stNorm):
			norm, err := stParseLong(line, len(stNorm))
			if err != nil {
				return 0, fmt.Errorf("SimpleTextSkipReader: parse NORM: %w", err)
			}
			r.perLevelImpacts[level].Add(freq, norm)
		}
	}
}

// readLevelLength reads one "levelLength N\n" line.
//
// Port of SimpleTextSkipReader.readLevelLength(IndexInput).
func (r *SimpleTextSkipReader) readLevelLength(skipStream store.IndexInput) (int64, error) {
	if err := stReadLine(skipStream, r.scratch); err != nil {
		return 0, fmt.Errorf("SimpleTextSkipReader.readLevelLength: %w", err)
	}
	line := r.scratch.Bytes()[:r.scratch.Length()]
	v, err := stParseLong(line, len(stLevelLength))
	if err != nil {
		return 0, fmt.Errorf("SimpleTextSkipReader.readLevelLength: parse: %w", err)
	}
	return v, nil
}

// readChildPointer reads one "childPointer N\n" line.
//
// Port of SimpleTextSkipReader.readChildPointer(IndexInput).
func (r *SimpleTextSkipReader) readChildPointer(skipStream store.IndexInput) (int64, error) {
	if err := stReadLine(skipStream, r.scratch); err != nil {
		return 0, fmt.Errorf("SimpleTextSkipReader.readChildPointer: %w", err)
	}
	line := r.scratch.Bytes()[:r.scratch.Length()]
	v, err := stParseLong(line, len(stChildPtr))
	if err != nil {
		return 0, fmt.Errorf("SimpleTextSkipReader.readChildPointer: parse: %w", err)
	}
	return v, nil
}

// Reset reinitialises the reader for a new term.
//
// Port of SimpleTextSkipReader.reset(long, int).
func (r *SimpleTextSkipReader) Reset(skipPointer int64, docFreq int) error {
	r.init()
	if skipPointer > 0 {
		if err := r.base.Init(skipPointer, docFreq); err != nil {
			return err
		}
		r.hasSkipList = true
	}
	return nil
}

// init resets transient state.
func (r *SimpleTextSkipReader) init() {
	r.nextSkipDocFP = -1
	r.numLevels = 1
	for i := range r.perLevelImpacts {
		b := r.perLevelImpacts[i]
		b.Size = 0
		b.Add(math.MaxInt32, 1)
	}
	r.hasSkipList = false
}

// GetImpacts returns the Impacts view for the current position.
func (r *SimpleTextSkipReader) GetImpacts() index.Impacts { return r.impactsView }

// GetNextSkipDocFP returns the doc file pointer from the last skip entry.
func (r *SimpleTextSkipReader) GetNextSkipDocFP() int64 { return r.nextSkipDocFP }

// GetNextSkipDoc returns the next skip doc id, or NO_MORE_DOCS if none.
func (r *SimpleTextSkipReader) GetNextSkipDoc() int {
	if !r.hasSkipList {
		return index.NO_MORE_DOCS
	}
	return r.base.GetSkipDoc(0)
}

// HasSkipList reports whether a skip list was provided at reset.
func (r *SimpleTextSkipReader) HasSkipList() bool { return r.hasSkipList }

// stCheckFooter validates the SimpleText checksum footer.
// This is a minimal inline variant; the full SimpleTextUtil.checkFooter will
// live in simple_text_util.go (task 3202).
func stCheckFooter(input *store.ChecksumIndexInput) error {
	scratch := util.NewBytesRefBuilder()
	if err := stReadLine(input, scratch); err != nil {
		return fmt.Errorf("stCheckFooter: readLine: %w", err)
	}
	line := scratch.Bytes()[:scratch.Length()]
	checksum := []byte("checksum ")
	if !bytes.HasPrefix(line, checksum) {
		return fmt.Errorf("stCheckFooter: expected checksum line, got: %s", line)
	}
	expectedCS := fmt.Sprintf("%020d", input.GetChecksum())
	actualCS := string(line[len(checksum):])
	if expectedCS != actualCS {
		return fmt.Errorf("stCheckFooter: checksum mismatch: expected %s, got %s", expectedCS, actualCS)
	}
	return nil
}

// ---------------------------------------------------------------------------
// simpleTextImpacts: Impacts view backed by SimpleTextSkipReader.
// ---------------------------------------------------------------------------

type simpleTextImpacts struct {
	r *SimpleTextSkipReader
}

// NumLevels returns the number of active skip levels.
func (i *simpleTextImpacts) NumLevels() int { return i.r.numLevels }

// GetDocIDUpTo returns the max doc id covered by this level's impacts.
func (i *simpleTextImpacts) GetDocIDUpTo(level int) int {
	return i.r.base.GetSkipDoc(level)
}

// GetImpacts returns the impact buffer for the given level.
func (i *simpleTextImpacts) GetImpacts(level int) *index.FreqAndNormBuffer {
	return i.r.perLevelImpacts[level]
}

// compile-time assertion.
var _ index.Impacts = (*simpleTextImpacts)(nil)
