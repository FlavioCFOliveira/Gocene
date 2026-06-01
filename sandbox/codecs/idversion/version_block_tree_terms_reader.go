// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.codecs.idversion.VersionBlockTreeTermsReader.
package idversion

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// VersionBlockTreeTermsReader reads the IDVersion block-tree term dictionary
// written by VersionBlockTreeTermsWriter.
//
// It opens the .tiv (terms data) and .tipv (terms index) files, verifies
// codec headers, validates cross-file version consistency, reads per-field
// metadata, and constructs a VersionFieldReader for each field.
//
// Mirrors org.apache.lucene.sandbox.codecs.idversion.VersionBlockTreeTermsReader.
type VersionBlockTreeTermsReader struct {
	// In is the open IndexInput for the terms (.tiv) file.
	// Kept as the concrete store.IndexInput type (not interface{}) after the
	// stub was replaced with the full port.
	In store.IndexInput

	// PostingsReader decodes per-term postings.
	PostingsReader *IDVersionPostingsReader

	// Fields maps field name → VersionFieldReader (sorted iteration order).
	Fields map[string]*VersionFieldReader

	// fieldOrder preserves the sorted insertion order for iterator().
	fieldOrder []string
}

// NewVersionBlockTreeTermsReader opens the .tiv and .tipv files, validates
// headers, and reads all per-field metadata.
//
// Mirrors VersionBlockTreeTermsReader(PostingsReaderBase, SegmentReadState).
func NewVersionBlockTreeTermsReader(
	postingsReader *IDVersionPostingsReader,
	state *codecs.SegmentReadState,
) (*VersionBlockTreeTermsReader, error) {

	termsFile := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, vbtTermsExtension,
	)
	termsIn, err := state.Directory.OpenInput(termsFile, store.IOContext{})
	if err != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: open %s: %w", termsFile, err)
	}

	r := &VersionBlockTreeTermsReader{
		In:             termsIn,
		PostingsReader: postingsReader,
		Fields:         make(map[string]*VersionFieldReader),
	}

	var indexIn store.IndexInput
	success := false
	defer func() {
		if !success {
			_ = r.Close()
			if indexIn != nil {
				_ = indexIn.Close()
			}
		}
	}()

	termsVersion, err := codecs.CheckIndexHeader(
		termsIn, vbtTermsCodecName,
		vbtVersionStart, vbtVersionCurrent,
		state.SegmentInfo.GetID(), state.SegmentSuffix,
	)
	if err != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: check terms header: %w", err)
	}

	indexFile := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, vbtTermsIndexExtension,
	)
	indexIn, err = state.Directory.OpenInput(indexFile, store.IOContext{})
	if err != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: open %s: %w", indexFile, err)
	}

	indexVersion, err := codecs.CheckIndexHeader(
		indexIn, vbtTermsIndexCodecName,
		vbtVersionStart, vbtVersionCurrent,
		state.SegmentInfo.GetID(), state.SegmentSuffix,
	)
	if err != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: check index header: %w", err)
	}

	if indexVersion != termsVersion {
		return nil, fmt.Errorf(
			"NewVersionBlockTreeTermsReader: version mismatch: terms=%d index=%d",
			termsVersion, indexVersion,
		)
	}

	// Verify the index file is internally consistent (checksum covers the
	// entire file including the header we already read).
	if _, cerr := codecs.ChecksumEntireFile(indexIn); cerr != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: index checksum: %w", cerr)
	}

	// Let the postings reader validate its own inline codec header.
	if err := postingsReader.Init(termsIn, state); err != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: postingsReader.Init: %w", err)
	}

	// Verify footer structure in the data file without reading all bytes
	// (RetrieveChecksum reads the last 8 bytes before the footer magic).
	if _, cerr := codecs.RetrieveChecksum(termsIn); cerr != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: terms footer structure: %w", cerr)
	}

	// Seek both files to their respective directory sections.
	if err := seekDir(termsIn); err != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: seekDir(termsIn): %w", err)
	}
	// Re-open index for reading the per-field indexStartFP values; after
	// ChecksumEntireFile the position is undefined, so seek it fresh.
	if err := seekDir(indexIn); err != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: seekDir(indexIn): %w", err)
	}

	// Read per-field records.
	vli, ok := termsIn.(store.VariableLengthInput)
	if !ok {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: termsIn does not implement VariableLengthInput")
	}
	vliIdx, ok := indexIn.(store.VariableLengthInput)
	if !ok {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: indexIn does not implement VariableLengthInput")
	}

	numFields, err := vli.ReadVInt()
	if err != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: read numFields: %w", err)
	}
	if numFields < 0 {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: invalid numFields: %d", numFields)
	}

	for i := int32(0); i < numFields; i++ {
		fieldNumber, err := vli.ReadVInt()
		if err != nil {
			return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: read fieldNumber[%d]: %w", i, err)
		}
		numTerms, err := vli.ReadVLong()
		if err != nil {
			return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: read numTerms[%d]: %w", i, err)
		}

		numBytes, err := vli.ReadVInt()
		if err != nil {
			return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: read rootCode len[%d]: %w", i, err)
		}
		codeBytes := make([]byte, numBytes)
		if rerr := termsIn.ReadBytes(codeBytes); rerr != nil {
			return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: read rootCode bytes[%d]: %w", i, rerr)
		}
		maxVersion, err := vli.ReadVLong()
		if err != nil {
			return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: read maxVersion[%d]: %w", i, err)
		}
		rootCode := vbtFSTOutputsW.NewPair(
			&util.BytesRef{Bytes: codeBytes, Offset: 0, Length: int(numBytes)},
			maxVersion,
		)

		fi := state.FieldInfos.GetByNumber(int(fieldNumber))
		if fi == nil {
			return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: unknown field number %d", fieldNumber)
		}

		// sum values: for IDVersion, numTerms == sumDocFreq == sumTotalTermFreq.
		sumTotalTermFreq := numTerms
		sumDocFreq := numTerms
		docCount := int(numTerms)

		minTerm, err := readBytesRefVBT(termsIn, vli)
		if err != nil {
			return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: read minTerm[%d]: %w", i, err)
		}
		maxTerm, err := readBytesRefVBT(termsIn, vli)
		if err != nil {
			return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: read maxTerm[%d]: %w", i, err)
		}

		// Validate.
		if docCount < 0 || docCount > state.SegmentInfo.DocCount() {
			return nil, fmt.Errorf(
				"NewVersionBlockTreeTermsReader: invalid docCount %d (maxDoc=%d) for field %s",
				docCount, state.SegmentInfo.DocCount(), fi.Name(),
			)
		}
		if sumDocFreq < int64(docCount) {
			return nil, fmt.Errorf(
				"NewVersionBlockTreeTermsReader: invalid sumDocFreq %d < docCount %d for field %s",
				sumDocFreq, docCount, fi.Name(),
			)
		}
		if sumTotalTermFreq < sumDocFreq {
			return nil, fmt.Errorf(
				"NewVersionBlockTreeTermsReader: invalid sumTotalTermFreq %d < sumDocFreq %d for field %s",
				sumTotalTermFreq, sumDocFreq, fi.Name(),
			)
		}

		indexStartFP, err := vliIdx.ReadVLong()
		if err != nil {
			return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: read indexStartFP[%d]: %w", i, err)
		}

		if _, dup := r.Fields[fi.Name()]; dup {
			return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: duplicate field %s", fi.Name())
		}

		fr, ferr := NewVersionFieldReader(
			r, fi, numTerms,
			rootCode,
			sumTotalTermFreq, sumDocFreq, docCount,
			indexStartFP,
			indexIn, // reader clones indexIn internally
			minTerm, maxTerm,
		)
		if ferr != nil {
			return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: NewVersionFieldReader(%s): %w", fi.Name(), ferr)
		}
		r.Fields[fi.Name()] = fr
		r.fieldOrder = append(r.fieldOrder, fi.Name())
	}

	// Sort field names for deterministic iteration (mirrors TreeMap in Java).
	sort.Strings(r.fieldOrder)

	// indexIn is no longer needed after we have consumed all FST data.
	if err := indexIn.Close(); err != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsReader: close indexIn: %w", err)
	}
	indexIn = nil // prevent double-close in deferred cleanup

	success = true
	return r, nil
}

// Terms returns the VersionFieldReader for the named field, or nil if the
// field has no terms in this segment.
//
// Mirrors VersionBlockTreeTermsReader.terms(String).
func (r *VersionBlockTreeTermsReader) Terms(field string) (schema.Terms, error) {
	fr, ok := r.Fields[field]
	if !ok {
		return nil, nil
	}
	return fr, nil
}

// Close releases all resources: closes the .tiv file and the postings reader.
//
// Mirrors VersionBlockTreeTermsReader.close().
func (r *VersionBlockTreeTermsReader) Close() error {
	var firstErr error
	setErr := func(e error) {
		if firstErr == nil && e != nil {
			firstErr = e
		}
	}
	if r.In != nil {
		setErr(r.In.Close())
	}
	if r.PostingsReader != nil {
		setErr(r.PostingsReader.Close())
	}
	r.Fields = nil
	r.fieldOrder = nil
	return firstErr
}

// CheckIntegrity verifies the checksum of the .tiv file and delegates to the
// postings reader's integrity check.
//
// Mirrors VersionBlockTreeTermsReader.checkIntegrity().
func (r *VersionBlockTreeTermsReader) CheckIntegrity() error {
	if r.In != nil {
		if _, err := codecs.ChecksumEntireFile(r.In); err != nil {
			return fmt.Errorf("VersionBlockTreeTermsReader.CheckIntegrity: %w", err)
		}
	}
	if r.PostingsReader != nil {
		return r.PostingsReader.CheckIntegrity()
	}
	return nil
}

// String returns a human-readable description of this reader.
func (r *VersionBlockTreeTermsReader) String() string {
	return fmt.Sprintf("VersionBlockTreeTermsReader(fields=%d,delegate=%s)",
		len(r.Fields), r.PostingsReader)
}

// seekDir seeks input to the directory offset: (length - footerLength - 8)
// bytes into the file, then reads the stored dirOffset and seeks there.
//
// Mirrors VersionBlockTreeTermsReader.seekDir(IndexInput).
func seekDir(input store.IndexInput) error {
	// Position just before the trailing dirOffset long (8 bytes) that precedes
	// the codec footer.
	dirPtrPos := input.Length() - int64(codecs.FooterLength()) - 8
	if err := input.SetPosition(dirPtrPos); err != nil {
		return fmt.Errorf("seekDir: seek to dirPtr position %d: %w", dirPtrPos, err)
	}
	vli, ok := input.(store.VariableLengthInput)
	if !ok {
		return fmt.Errorf("seekDir: input does not implement VariableLengthInput")
	}
	// ReadLong — VariableLengthInput doesn't expose ReadLong, use the
	// store.IndexInput's ReadLong method via type assertion.
	type longReader interface {
		ReadLong() (int64, error)
	}
	lr, ok := input.(longReader)
	if !ok {
		// Fallback: read 8 bytes manually.
		buf := make([]byte, 8)
		if err := input.ReadBytes(buf); err != nil {
			return fmt.Errorf("seekDir: read dirOffset bytes: %w", err)
		}
		dirOffset := int64(buf[0])<<56 | int64(buf[1])<<48 | int64(buf[2])<<40 | int64(buf[3])<<32 |
			int64(buf[4])<<24 | int64(buf[5])<<16 | int64(buf[6])<<8 | int64(buf[7])
		_ = vli // satisfy compiler
		return input.SetPosition(dirOffset)
	}
	dirOffset, err := lr.ReadLong()
	if err != nil {
		return fmt.Errorf("seekDir: read dirOffset: %w", err)
	}
	return input.SetPosition(dirOffset)
}

// readBytesRefVBT reads a BytesRef written by writeBytesRefVBT: (vint length, raw bytes).
func readBytesRefVBT(input store.IndexInput, vli store.VariableLengthInput) (*util.BytesRef, error) {
	length, err := vli.ReadVInt()
	if err != nil {
		return nil, fmt.Errorf("readBytesRefVBT: read length: %w", err)
	}
	b := make([]byte, length)
	if length > 0 {
		if rerr := input.ReadBytes(b); rerr != nil {
			return nil, fmt.Errorf("readBytesRefVBT: read bytes: %w", rerr)
		}
	}
	return &util.BytesRef{Bytes: b, Offset: 0, Length: int(length)}, nil
}

// ─── compile-time interface assertions ───────────────────────────────────────

var _ codecs.FieldsProducer = (*VersionBlockTreeTermsReader)(nil)

// VersionFieldReader also needs to satisfy schema.Terms for r.Terms to return it.
var _ schema.Terms = (*VersionFieldReader)(nil)

// Ensure FST outputs type matches what VersionFieldReader expects.
var _ fst.Outputs[*fst.Pair[*util.BytesRef, int64]] = vbtFSTOutputsW
