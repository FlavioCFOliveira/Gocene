// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package codecs

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Reference: lucene/core/src/java/org/apache/lucene/codecs/lucene103/blocktree/
// Lucene103BlockTreeTermsReader.java (Apache Lucene 10.4.0).
//
// This file declares the wire-format constants shared by the Writer (Sprint
// 17 task #99), the FieldReader (Sprint 17 task #95), and the Stats helper
// (Sprint 17 task #102) plus the full segment-level reader (Sprint 17 task
// #98). The strict reader opens .tim / .tip / .tmd at construction time,
// validates codec headers via CheckIndexHeader, walks the meta block to
// materialise one [Lucene103FieldReader] per indexed field, and verifies
// the index/terms CRC footers against the lengths recorded in the meta tail.

// Lucene103BlockTreeTermsExtension is the file-extension constant used for
// the per-segment .tim file (term dictionary). Matches
// {@code Lucene103BlockTreeTermsReader.TERMS_EXTENSION}.
const Lucene103BlockTreeTermsExtension = "tim"

// Lucene103BlockTreeTermsIndexExtension is the file-extension constant for
// the per-segment .tip file (terms index). Matches
// {@code Lucene103BlockTreeTermsReader.TERMS_INDEX_EXTENSION}.
const Lucene103BlockTreeTermsIndexExtension = "tip"

// Lucene103BlockTreeTermsMetaExtension is the file-extension constant for
// the per-segment .tmd file (terms metadata / footer tail). Matches
// {@code Lucene103BlockTreeTermsReader.TERMS_META_EXTENSION}.
const Lucene103BlockTreeTermsMetaExtension = "tmd"

// Codec-name constants — these are passed verbatim to
// CodecUtil.writeIndexHeader / CodecUtil.checkIndexHeader and are part of
// the on-disk format.
const (
	Lucene103BlockTreeTermsCodecName      = "BlockTreeTermsDict"
	Lucene103BlockTreeTermsIndexCodecName = "BlockTreeTermsIndex"
	Lucene103BlockTreeTermsMetaCodecName  = "BlockTreeTermsMeta"
)

// Wire-format version range. The initial format is 0; bumping these
// requires a matching change in both Reader and Writer.
const (
	Lucene103BlockTreeVersionStart   int32 = 0
	Lucene103BlockTreeVersionCurrent int32 = 0
)

// Lucene103BlockTreeTermsReader is the strict Go port of
// org.apache.lucene.codecs.lucene103.blocktree.Lucene103BlockTreeTermsReader.
//
// Construction opens .tim, .tip, .tmd in turn, validates codec headers,
// then walks the meta file to materialise one [Lucene103FieldReader] per
// field. The trailing two longs of the meta file record the lengths of
// the index and terms files so the constructor can validate their CRC
// footers without re-scanning every byte.
//
// The reader is safe for concurrent term lookup once construction has
// returned; per-term iteration (Iterator / Intersect on FieldReader)
// requires the deferred SegmentTermsEnum port (backlog task #2692).
type Lucene103BlockTreeTermsReader struct {
	termsIn        store.IndexInput
	indexIn        store.IndexInput
	postingsReader PostingsReaderBase
	fieldInfos     *index.FieldInfos
	segment        string
	version        int32

	mu        sync.RWMutex
	fieldMap  map[int]*Lucene103FieldReader
	fieldList []string

	closed bool
}

// NewLucene103BlockTreeTermsReader is the strict port of the canonical
// Lucene103BlockTreeTermsReader constructor. It performs the full
// open-and-validate dance for the three segment files (.tim / .tip /
// .tmd), wires the wrapped PostingsReaderBase, and materialises one
// FieldReader per indexed field.
//
// On any error during construction the partially opened files are closed
// before the error surfaces, mirroring Lucene's
// IOUtils.closeWhileHandlingException pattern.
func NewLucene103BlockTreeTermsReader(postingsReader PostingsReaderBase, state *SegmentReadState) (*Lucene103BlockTreeTermsReader, error) {
	if state == nil {
		return nil, errors.New("Lucene103BlockTreeTermsReader: state must not be nil")
	}
	if postingsReader == nil {
		return nil, errors.New("Lucene103BlockTreeTermsReader: postingsReader must not be nil")
	}

	r := &Lucene103BlockTreeTermsReader{
		postingsReader: postingsReader,
		segment:        state.SegmentInfo.Name(),
		fieldInfos:     state.FieldInfos,
	}

	segmentName := r.segment
	segmentSuffix := state.SegmentSuffix
	segmentID := state.SegmentInfo.GetID()
	directory := state.Directory

	termsName := GetSegmentFileName(segmentName, segmentSuffix, Lucene103BlockTreeTermsExtension)
	termsIn, err := directory.OpenInput(termsName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("Lucene103BlockTreeTermsReader: open %s: %w", termsName, err)
	}

	success := false
	var indexIn store.IndexInput
	defer func() {
		if success {
			return
		}
		closeQuietly(termsIn, indexIn)
	}()

	version, err := CheckIndexHeader(termsIn, Lucene103BlockTreeTermsCodecName,
		Lucene103BlockTreeVersionStart, Lucene103BlockTreeVersionCurrent,
		segmentID, segmentSuffix)
	if err != nil {
		return nil, fmt.Errorf("Lucene103BlockTreeTermsReader: check %s header: %w", termsName, err)
	}
	r.version = version

	indexName := GetSegmentFileName(segmentName, segmentSuffix, Lucene103BlockTreeTermsIndexExtension)
	indexIn, err = directory.OpenInput(indexName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("Lucene103BlockTreeTermsReader: open %s: %w", indexName, err)
	}
	if _, err := CheckIndexHeader(indexIn, Lucene103BlockTreeTermsIndexCodecName,
		version, version, segmentID, segmentSuffix); err != nil {
		return nil, fmt.Errorf("Lucene103BlockTreeTermsReader: check %s header: %w", indexName, err)
	}

	// Meta file is opened as a ChecksumIndexInput so the CRC32 footer is
	// verified once the meta block has been fully consumed. Lucene's
	// Directory.openChecksumInput returns a Buffered* variant; Gocene's
	// helper takes a raw IndexInput and wraps it inline.
	metaName := GetSegmentFileName(segmentName, segmentSuffix, Lucene103BlockTreeTermsMetaExtension)
	rawMetaIn, err := directory.OpenInput(metaName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("Lucene103BlockTreeTermsReader: open %s: %w", metaName, err)
	}
	metaIn := store.NewBufferedChecksumIndexInput(rawMetaIn)

	indexLength, termsLength, fieldMap, fieldList, metaErr := r.loadMeta(metaIn, indexIn, state, segmentID, segmentSuffix, version)
	if cerr := metaIn.Close(); cerr != nil && metaErr == nil {
		metaErr = cerr
	}
	if metaErr != nil {
		return nil, metaErr
	}

	// Lucene's "the meta file's checksum has been verified so the lengths
	// are likely correct" comment: the per-file length validation
	// duplicates what's already in the wire format so a torn write
	// produces a clear error before the user hits a corrupt block.
	if _, err := retrieveChecksumExpectingLength(indexIn, indexLength); err != nil {
		return nil, fmt.Errorf("Lucene103BlockTreeTermsReader: validate %s checksum: %w", indexName, err)
	}
	if _, err := retrieveChecksumExpectingLength(termsIn, termsLength); err != nil {
		return nil, fmt.Errorf("Lucene103BlockTreeTermsReader: validate %s checksum: %w", termsName, err)
	}

	r.termsIn = termsIn
	r.indexIn = indexIn
	r.fieldMap = fieldMap
	r.fieldList = fieldList
	success = true
	return r, nil
}

// loadMeta walks the .tmd file from the postings-reader Init through every
// per-field record, returning the trailing (indexLength, termsLength) pair
// plus the populated field map and sorted field name list.
func (r *Lucene103BlockTreeTermsReader) loadMeta(
	metaIn *store.BufferedChecksumIndexInput,
	indexIn store.IndexInput,
	state *SegmentReadState,
	segmentID []byte,
	segmentSuffix string,
	version int32,
) (indexLength, termsLength int64, fieldMap map[int]*Lucene103FieldReader, fieldList []string, err error) {
	// Wrap the buffered checksum input in a ChecksumIndexInput so that
	// CheckIndexHeader / CheckFooter (which both take the concrete type)
	// can drive the same underlying read cursor.
	checksum := store.NewChecksumIndexInput(metaIn)

	var priorErr error
	defer func() {
		if cerr := checkFooterCapture(checksum, priorErr); cerr != nil && err == nil {
			err = cerr
		}
	}()

	if _, hdrErr := CheckIndexHeader(checksum, Lucene103BlockTreeTermsMetaCodecName,
		version, version, segmentID, segmentSuffix); hdrErr != nil {
		priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: check %s header: %w",
			GetSegmentFileName(r.segment, segmentSuffix, Lucene103BlockTreeTermsMetaExtension), hdrErr)
		return 0, 0, nil, nil, priorErr
	}
	if pwErr := r.postingsReader.Init(checksum, state); pwErr != nil {
		priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: postingsReader.Init: %w", pwErr)
		return 0, 0, nil, nil, priorErr
	}

	numFields, vErr := store.ReadVInt(checksum)
	if vErr != nil {
		priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: read numFields: %w", vErr)
		return 0, 0, nil, nil, priorErr
	}
	if numFields < 0 {
		priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: invalid numFields: %d", numFields)
		return 0, 0, nil, nil, priorErr
	}

	fieldMap = make(map[int]*Lucene103FieldReader, numFields)
	for i := int32(0); i < numFields; i++ {
		fieldNumber, ferr := store.ReadVInt(checksum)
		if ferr != nil {
			priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: read fieldNumber[%d]: %w", i, ferr)
			return 0, 0, nil, nil, priorErr
		}
		numTerms, ferr := store.ReadVLong(checksum)
		if ferr != nil {
			priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: read numTerms for field %d: %w", fieldNumber, ferr)
			return 0, 0, nil, nil, priorErr
		}
		if numTerms <= 0 {
			priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: illegal numTerms %d for field number %d", numTerms, fieldNumber)
			return 0, 0, nil, nil, priorErr
		}
		fieldInfo := state.FieldInfos.GetByNumber(int(fieldNumber))
		if fieldInfo == nil {
			priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: invalid field number: %d", fieldNumber)
			return 0, 0, nil, nil, priorErr
		}
		sumTotalTermFreq, ferr := store.ReadVLong(checksum)
		if ferr != nil {
			priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: read sumTotalTermFreq for field %d: %w", fieldNumber, ferr)
			return 0, 0, nil, nil, priorErr
		}
		// When frequencies are omitted (IndexOptions.DOCS) sumDocFreq
		// equals sumTotalTermFreq and only the single value is on disk.
		var sumDocFreq int64
		if fieldInfo.IndexOptions() == index.IndexOptionsDocs {
			sumDocFreq = sumTotalTermFreq
		} else {
			sumDocFreq, ferr = store.ReadVLong(checksum)
			if ferr != nil {
				priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: read sumDocFreq for field %d: %w", fieldNumber, ferr)
				return 0, 0, nil, nil, priorErr
			}
		}
		docCount, ferr := store.ReadVInt(checksum)
		if ferr != nil {
			priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: read docCount for field %d: %w", fieldNumber, ferr)
			return 0, 0, nil, nil, priorErr
		}
		minTerm, minErr := readMetaBytesRef(checksum)
		if minErr != nil {
			priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: read minTerm for field %d: %w", fieldNumber, minErr)
			return 0, 0, nil, nil, priorErr
		}
		maxTerm, maxErr := readMetaBytesRef(checksum)
		if maxErr != nil {
			priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: read maxTerm for field %d: %w", fieldNumber, maxErr)
			return 0, 0, nil, nil, priorErr
		}
		// Lucene collapses min == max into a single BytesRef when there
		// is only one term in the field; we honour the same heap-saving
		// trick.
		if numTerms == 1 {
			maxTerm = minTerm
		}
		if docCount < 0 || docCount > int32(state.SegmentInfo.DocCount()) {
			priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: invalid docCount %d (maxDoc=%d) for field %d",
				docCount, state.SegmentInfo.DocCount(), fieldNumber)
			return 0, 0, nil, nil, priorErr
		}
		if sumDocFreq < int64(docCount) {
			priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: invalid sumDocFreq %d (docCount=%d) for field %d",
				sumDocFreq, docCount, fieldNumber)
			return 0, 0, nil, nil, priorErr
		}
		if sumTotalTermFreq < sumDocFreq {
			priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: invalid sumTotalTermFreq %d (sumDocFreq=%d) for field %d",
				sumTotalTermFreq, sumDocFreq, fieldNumber)
			return 0, 0, nil, nil, priorErr
		}

		fr, frErr := NewLucene103FieldReader(r, fieldInfo, numTerms, sumTotalTermFreq, sumDocFreq, int(docCount), checksum, indexIn, minTerm, maxTerm)
		if frErr != nil {
			priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: build FieldReader for field %d: %w", fieldNumber, frErr)
			return 0, 0, nil, nil, priorErr
		}
		if _, dup := fieldMap[fieldInfo.Number()]; dup {
			priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: duplicate field: %s", fieldInfo.Name())
			return 0, 0, nil, nil, priorErr
		}
		fieldMap[fieldInfo.Number()] = fr
	}

	indexLength, ilErr := checksum.ReadLong()
	if ilErr != nil {
		priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: read indexLength: %w", ilErr)
		return 0, 0, nil, nil, priorErr
	}
	termsLength, tlErr := checksum.ReadLong()
	if tlErr != nil {
		priorErr = fmt.Errorf("Lucene103BlockTreeTermsReader: read termsLength: %w", tlErr)
		return 0, 0, nil, nil, priorErr
	}

	fieldList = sortFieldNames(fieldMap, state.FieldInfos)
	return indexLength, termsLength, fieldMap, fieldList, nil
}

// readMetaBytesRef mirrors Lucene's private readBytesRef helper: vInt(len)
// followed by the raw bytes.
func readMetaBytesRef(in store.IndexInput) (*util.BytesRef, error) {
	n, err := store.ReadVInt(in)
	if err != nil {
		return nil, err
	}
	if n < 0 {
		return nil, fmt.Errorf("invalid bytes length: %d", n)
	}
	if n == 0 {
		return util.NewBytesRefEmpty(), nil
	}
	buf := make([]byte, n)
	if err := in.ReadBytes(buf); err != nil {
		return nil, err
	}
	return util.NewBytesRef(buf), nil
}

// retrieveChecksumExpectingLength is the Go equivalent of Lucene's
// CodecUtil.retrieveChecksum(IndexInput, long) overload that asserts the
// declared length matches the actual file length before delegating to the
// single-arg form.
func retrieveChecksumExpectingLength(in store.IndexInput, expected int64) (int64, error) {
	if got := in.Length(); got != expected {
		return 0, fmt.Errorf("misplaced codec footer: expected length %d, got %d", expected, got)
	}
	return RetrieveChecksum(in)
}

// checkFooterCapture wraps CheckFooter so the loadMeta deferred-call site
// can fall through to the codec footer validation while still reporting
// any prior decoding error as the primary cause. Mirrors Lucene's
// try-with-resources + CodecUtil.checkFooter idiom.
func checkFooterCapture(in *store.ChecksumIndexInput, priorErr error) error {
	_, err := CheckFooter(in)
	if priorErr != nil {
		return priorErr
	}
	return err
}

// sortFieldNames builds the sorted list of indexed field names. Mirrors
// Lucene's sortFieldNames helper.
func sortFieldNames(fieldMap map[int]*Lucene103FieldReader, infos *index.FieldInfos) []string {
	names := make([]string, 0, len(fieldMap))
	for num := range fieldMap {
		fi := infos.GetByNumber(num)
		if fi == nil {
			continue
		}
		names = append(names, fi.Name())
	}
	sort.Strings(names)
	return names
}

// SegmentName returns the segment name this reader was opened for. Mirrors
// Lucene103BlockTreeTermsReader.segment.
func (r *Lucene103BlockTreeTermsReader) SegmentName() string {
	if r == nil {
		return ""
	}
	return r.segment
}

// Version returns the wire-format version of the segment files this reader
// is bound to. Mirrors Lucene103BlockTreeTermsReader.version.
func (r *Lucene103BlockTreeTermsReader) Version() int32 { return r.version }

// Size returns the number of indexed fields. Mirrors FieldsProducer.size().
func (r *Lucene103BlockTreeTermsReader) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.fieldMap)
}

// FieldNames returns the indexed field names in ascending order.
func (r *Lucene103BlockTreeTermsReader) FieldNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.fieldList))
	copy(out, r.fieldList)
	return out
}

// Terms returns the [index.Terms] for the requested field, or nil when
// the field is not indexed. Mirrors FieldsProducer.terms(String).
func (r *Lucene103BlockTreeTermsReader) Terms(field string) (index.Terms, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return nil, errors.New("Lucene103BlockTreeTermsReader: closed")
	}
	fi := r.fieldInfos.GetByName(field)
	if fi == nil {
		return nil, nil
	}
	fr, ok := r.fieldMap[fi.Number()]
	if !ok {
		return nil, nil
	}
	return fr, nil
}

// CheckIntegrity validates the CRC footers of every file this reader owns
// plus the wrapped PostingsReaderBase. Mirrors
// FieldsProducer.checkIntegrity().
func (r *Lucene103BlockTreeTermsReader) CheckIntegrity() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return errors.New("Lucene103BlockTreeTermsReader: closed")
	}
	if _, err := ChecksumEntireFile(r.indexIn); err != nil {
		return fmt.Errorf("CheckIntegrity: index: %w", err)
	}
	if _, err := ChecksumEntireFile(r.termsIn); err != nil {
		return fmt.Errorf("CheckIntegrity: terms: %w", err)
	}
	if err := r.postingsReader.CheckIntegrity(); err != nil {
		return fmt.Errorf("CheckIntegrity: postings: %w", err)
	}
	return nil
}

// Close releases the three on-disk files and the wrapped postings reader.
// Subsequent calls are no-ops. Mirrors FieldsProducer.close().
func (r *Lucene103BlockTreeTermsReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true

	var firstErr error
	setErr := func(err error) {
		if firstErr == nil {
			firstErr = err
		}
	}
	closeAll(setErr, r.indexIn, r.termsIn, r.postingsReader)
	// Drop the field map so the per-field tries become GC-eligible even
	// if the caller hangs onto the reader pointer.
	r.fieldMap = nil
	r.fieldList = nil
	return firstErr
}

// String mirrors Lucene103BlockTreeTermsReader.toString().
func (r *Lucene103BlockTreeTermsReader) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return fmt.Sprintf("Lucene103BlockTreeTermsReader(fields=%d,delegate=%T)", len(r.fieldMap), r.postingsReader)
}

// Ensure Lucene103BlockTreeTermsReader satisfies the FieldsProducer SPI.
var _ FieldsProducer = (*Lucene103BlockTreeTermsReader)(nil)
