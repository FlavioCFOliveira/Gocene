// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

import (
	"encoding/binary"
	"fmt"
)

// rbbiData holds the runtime tables extracted from a compiled ICU
// RuleBasedBreakIterator (.brk) blob: the forward state table, the character
// category trie, and the rule-status (tag) table.
//
// Go port of the runtime subset of
// com.ibm.icu.impl.RBBIDataWrapper (unicode-org/icu, tag release-70-1,
// icu4j/main/classes/core/src/com/ibm/icu/impl/RBBIDataWrapper.java).
//
// This struct intentionally captures only what the forward break engine needs.
// The reverse and safe-reverse tables are parsed by ICU but not required for
// Gocene's forward-only handleNext; they are skipped here. (Reverse iteration
// is a documented follow-up — see rbbi_break_iterator.go.)
type rbbiData struct {
	// forward is the forward state table that handleNext drives.
	forward *rbbiStateTable

	// reverse is the reverse state table for handlePrevious (fRTable). May
	// be nil if the .brk does not contain a reverse section, in which case
	// Previous() falls back to re-running the forward engine from the start.
	reverse *rbbiStateTable

	// trie maps a code point to its character category (a column index into
	// each state-table row).
	trie *codePointTrie

	// statusTable is the rule-status (tag) table, a flat []int32 of
	// [count, val0, val1, ...] records. fRuleStatusIndex points at a count.
	statusTable []int32

	// catCount is RBBIDataHeader.fCatCount, the number of character
	// categories. The row stride is catCount + rbbiNextStates.
	catCount int
}

// rbbiStateTable is the parsed forward (or reverse) state-transition table.
//
// Go port of com.ibm.icu.impl.RBBIDataWrapper.RBBIStateTable.
//
// The fTable field is a flat slice of cells (one logical short per cell,
// already widened from the on-disk 8-bit or 16-bit rows). A row for state s
// begins at index rowIndex(s) = s * (catCount + rbbiNextStates). Within a row,
// rbbiAccepting/rbbiLookahead/rbbiTagsIdx are the first three cells and the
// per-category next-state transitions start at rbbiNextStates.
type rbbiStateTable struct {
	numStates            int
	rowLen               int // bytes per row on disk (catCount+3 for 8-bit rows)
	dictCategoriesStart  int // categories >= this require dictionary handling
	lookAheadResultsSize int
	flags                int
	table                []uint16 // flat cells, widened to uint16
}

// RBBI state-table layout constants, ported verbatim from RBBIDataWrapper.
const (
	rbbiAccepting  = 0
	rbbiLookahead  = 1
	rbbiTagsIdx    = 2
	rbbiNextStates = 3

	rbbiAcceptingUnconditional = 1

	rbbiLookaheadHardBreak = 1
	rbbiBOFRequired        = 2
	rbbi8BitsRows          = 4

	// State-machine sentinels (RuleBasedBreakIterator.java).
	rbbiStartState = 1
	rbbiStopState  = 0

	// rbbiStateHeaderSize is the size in bytes of the RBBIStateTable header
	// (five int32 fields) that precedes the row data.
	rbbiStateHeaderSize = 20
)

// rbbiHeaderInts is the number of int32 fields in the RBBIDataHeader
// (RBBIDataWrapper.DH_SIZE). The forward-table/trie/status offsets recorded in
// the header are relative to the start of this header.
const rbbiHeaderInts = 20

// RBBIDataHeader field offsets, in int32 units from the header start. These
// mirror the read order in RBBIDataWrapper.get(ByteBuffer): fMagic,
// fFormatVersion[4-bytes as one int], fLength, fCatCount, then the section
// (offset, length) pairs.
const (
	rbbiHdrCatCount       = 3
	rbbiHdrFTable         = 4
	rbbiHdrFTableLen      = 5
	rbbiHdrRTable         = 6
	rbbiHdrRTableLen      = 7
	rbbiHdrTrie           = 8
	rbbiHdrTrieLen        = 9
	rbbiHdrRuleSource     = 10
	rbbiHdrRuleSourceLen  = 11
	rbbiHdrStatusTable    = 12
	rbbiHdrStatusTableLen = 13
)

// parseRBBIData extracts the forward state table, the category trie, and the
// status table from a parsed BRKDictionary.
//
// The dictionary's Rules slice contains the full .brk blob (UDataInfo header +
// RBBIDataHeader + sections). All RBBIDataHeader section offsets are relative
// to the RBBIDataHeader, which begins at dict.HeaderSize.
func parseRBBIData(dict *BRKDictionary) (*rbbiData, error) {
	if dict == nil {
		return nil, fmt.Errorf("%w: nil dictionary", ErrInvalidBRK)
	}
	order := pickByteOrder(dict.IsBigEndian)
	base := int(dict.HeaderSize)
	blob := dict.Rules

	if base+rbbiHeaderInts*4 > len(blob) {
		return nil, fmt.Errorf("%w: RBBIDataHeader truncated", ErrInvalidBRK)
	}

	hdr := func(i int) int {
		return int(order.Uint32(blob[base+i*4 : base+i*4+4]))
	}

	catCount := hdr(rbbiHdrCatCount)
	if catCount <= 0 {
		return nil, fmt.Errorf("%w: fCatCount %d invalid", ErrInvalidBRK, catCount)
	}

	// section returns the byte slice for a header (offset, length) pair. The
	// offsets are relative to the RBBIDataHeader start (base).
	section := func(offIdx, lenIdx int) ([]byte, error) {
		off := hdr(offIdx)
		ln := hdr(lenIdx)
		if ln == 0 {
			return nil, nil
		}
		start := base + off
		end := start + ln
		if off < 0 || ln < 0 || end > len(blob) || start < base {
			return nil, fmt.Errorf("%w: section [%#x,%#x) out of range (blob %d bytes)",
				ErrInvalidBRK, off, off+ln, len(blob))
		}
		return blob[start:end], nil
	}

	// Forward state table.
	fwdBytes, err := section(rbbiHdrFTable, rbbiHdrFTableLen)
	if err != nil {
		return nil, err
	}
	if fwdBytes == nil {
		return nil, fmt.Errorf("%w: missing forward state table", ErrInvalidBRK)
	}
	forward, err := parseStateTable(fwdBytes, order)
	if err != nil {
		return nil, fmt.Errorf("forward state table: %w", err)
	}

	// Reverse state table (fRTable) — optional; present in all production
	// .brk files but not required for forward tokenisation.
	var reverse *rbbiStateTable
	revBytes, err := section(rbbiHdrRTable, rbbiHdrRTableLen)
	if err == nil && revBytes != nil {
		if rev, parseErr := parseStateTable(revBytes, order); parseErr == nil {
			reverse = rev
		}
		// Silently ignore a corrupt/absent reverse table; Previous() will fall
		// back to the linear forward-scan algorithm.
	}

	// Character category trie.
	trieBytes, err := section(rbbiHdrTrie, rbbiHdrTrieLen)
	if err != nil {
		return nil, err
	}
	if trieBytes == nil {
		return nil, fmt.Errorf("%w: missing character category trie", ErrInvalidBRK)
	}
	trie, _, err := parseCodePointTrie(trieBytes, order)
	if err != nil {
		return nil, fmt.Errorf("category trie: %w", err)
	}

	// Rule status (tag) table — optional.
	statusBytes, err := section(rbbiHdrStatusTable, rbbiHdrStatusTableLen)
	if err != nil {
		return nil, err
	}
	var statusTable []int32
	if statusBytes != nil {
		statusTable = make([]int32, len(statusBytes)/4)
		for i := range statusTable {
			statusTable[i] = int32(order.Uint32(statusBytes[i*4 : i*4+4]))
		}
	}

	// Validate that the forward table row stride is consistent with catCount.
	wantStride := catCount + rbbiNextStates
	if got := forward.numStates * wantStride; got > len(forward.table) {
		return nil, fmt.Errorf("%w: forward table too small (%d cells, need %d)",
			ErrInvalidBRK, len(forward.table), got)
	}

	return &rbbiData{
		forward:     forward,
		reverse:     reverse,
		trie:        trie,
		statusTable: statusTable,
		catCount:    catCount,
	}, nil
}

// parseStateTable decodes an RBBIStateTable from its serialized bytes.
//
// Go port of RBBIDataWrapper.RBBIStateTable.get. The five int32 header fields
// are followed by the row data, stored as bytes when rbbi8BitsRows is set or as
// big-/little-endian shorts otherwise. Either way the cells are widened to
// uint16 (unsigned, matching ICU's "(char)(0xff & b)").
func parseStateTable(buf []byte, order binary.ByteOrder) (*rbbiStateTable, error) {
	if len(buf) < rbbiStateHeaderSize {
		return nil, fmt.Errorf("%w: state table header truncated", ErrInvalidBRK)
	}
	st := &rbbiStateTable{
		numStates:            int(order.Uint32(buf[0:4])),
		rowLen:               int(order.Uint32(buf[4:8])),
		dictCategoriesStart:  int(order.Uint32(buf[8:12])),
		lookAheadResultsSize: int(order.Uint32(buf[12:16])),
		flags:                int(order.Uint32(buf[16:20])),
	}
	rowBytes := buf[rbbiStateHeaderSize:]
	use8Bits := st.flags&rbbi8BitsRows == rbbi8BitsRows
	if use8Bits {
		st.table = make([]uint16, len(rowBytes))
		for i, b := range rowBytes {
			st.table[i] = uint16(b) // treat as unsigned, matching ICU
		}
	} else {
		n := len(rowBytes) / 2
		st.table = make([]uint16, n)
		for i := 0; i < n; i++ {
			st.table[i] = order.Uint16(rowBytes[i*2 : i*2+2])
		}
	}
	return st, nil
}

// rowIndex returns the cell index at which the row for state s begins,
// mirroring RBBIDataWrapper.getRowIndex: state * (catCount + NEXTSTATES).
func (d *rbbiData) rowIndex(state int) int {
	return state * (d.catCount + rbbiNextStates)
}

// ruleStatus returns the largest rule-status (tag) value for the status record
// at ruleStatusIndex, mirroring RuleBasedBreakIterator.getRuleStatus.
//
// The record is [count, val0, val1, ...]; the values are sorted ascending and
// the last is returned. A nil/empty status table or an out-of-range index
// yields RuleStatusWordNone (0).
func (d *rbbiData) ruleStatus(ruleStatusIndex int) int {
	if len(d.statusTable) == 0 || ruleStatusIndex < 0 || ruleStatusIndex >= len(d.statusTable) {
		return RuleStatusWordNone
	}
	count := int(d.statusTable[ruleStatusIndex])
	idx := ruleStatusIndex + count
	if idx < 0 || idx >= len(d.statusTable) {
		return RuleStatusWordNone
	}
	return int(d.statusTable[idx])
}
