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

// LiveDocsFormat handles encoding/decoding of live docs (deleted documents).
// This is the Go port of Lucene's org.apache.lucene.codecs.LiveDocsFormat.
//
// Live docs are stored in files like _X.liv and contain a bitset indicating
// which documents are still "live" (not deleted) in the index.
type LiveDocsFormat interface {
	// Name returns the name of this format.
	Name() string

	// NewLiveDocs returns a new FixedBitSet for tracking live documents.
	NewLiveDocs(numDocs int) (*util.FixedBitSet, error)

	// ReadLiveDocs reads the live docs from the directory.
	ReadLiveDocs(dir store.Directory, segmentInfo *index.SegmentInfo) (util.Bits, error)

	// WriteLiveDocs writes the live docs to the directory.
	WriteLiveDocs(bits util.Bits, dir store.Directory, segmentInfo *index.SegmentInfo) error

	// Files returns the files used by this format for the given segment.
	Files(segmentInfo *index.SegmentInfo) []string
}

// BaseLiveDocsFormat provides common functionality for LiveDocsFormat implementations.
type BaseLiveDocsFormat struct {
	name string
}

// NewBaseLiveDocsFormat creates a new BaseLiveDocsFormat.
func NewBaseLiveDocsFormat(name string) *BaseLiveDocsFormat {
	return &BaseLiveDocsFormat{name: name}
}

// Name returns the format name.
func (f *BaseLiveDocsFormat) Name() string {
	return f.name
}

// NewLiveDocs returns a new FixedBitSet (must be implemented by subclasses).
func (f *BaseLiveDocsFormat) NewLiveDocs(numDocs int) (*util.FixedBitSet, error) {
	return nil, fmt.Errorf("NewLiveDocs not implemented")
}

// ReadLiveDocs reads the live docs (must be implemented by subclasses).
func (f *BaseLiveDocsFormat) ReadLiveDocs(dir store.Directory, segmentInfo *index.SegmentInfo) (util.Bits, error) {
	return nil, fmt.Errorf("ReadLiveDocs not implemented")
}

// WriteLiveDocs writes the live docs (must be implemented by subclasses).
func (f *BaseLiveDocsFormat) WriteLiveDocs(bits util.Bits, dir store.Directory, segmentInfo *index.SegmentInfo) error {
	return fmt.Errorf("WriteLiveDocs not implemented")
}

// Files returns the files used by this format (must be implemented by subclasses).
func (f *BaseLiveDocsFormat) Files(segmentInfo *index.SegmentInfo) []string {
	return nil
}

// Lucene90LiveDocsFormat is the Lucene 9.0 live docs format. The .liv file
// stores a FixedBitSet (1 = doc is live; 0 = doc is deleted), framed by a
// CodecUtil IndexHeader and Footer. The IndexHeader's suffix carries the
// del-generation in Character.MAX_RADIX (36).
//
// Wire-format-faithful port of
// org.apache.lucene.codecs.lucene90.Lucene90LiveDocsFormat.
//
// DEVIATIONS from the Java reference (documented):
//
//   - SparseLiveDocs / DenseLiveDocs / Bits.applyMask are not yet ported;
//     the read path always returns a util.FixedBitSet (the dense
//     representation). The format is wire-format equivalent regardless of
//     in-memory representation.
//   - The simple LiveDocsFormat interface (ReadLiveDocs / WriteLiveDocs
//     without del-gen + del-count) is kept for backward compatibility but
//     defaults gen=0 and skips the cross-check against the expected
//     del-count. The lucene-faithful methods Lucene90LiveDocsFormat
//     .ReadLiveDocsLucene90 / .WriteLiveDocsLucene90 take an explicit
//     del-gen and expectedDelCount.
type Lucene90LiveDocsFormat struct {
	*BaseLiveDocsFormat
}

// NewLucene90LiveDocsFormat creates a new Lucene90LiveDocsFormat.
func NewLucene90LiveDocsFormat() *Lucene90LiveDocsFormat {
	return &Lucene90LiveDocsFormat{
		BaseLiveDocsFormat: NewBaseLiveDocsFormat("Lucene90LiveDocsFormat"),
	}
}

// Lucene90LiveDocsCodec is the codec name stamped into the .liv IndexHeader.
const Lucene90LiveDocsCodec = "Lucene90LiveDocs"

// Lucene90LiveDocsVersionStart is the inclusive minimum supported version.
const Lucene90LiveDocsVersionStart int32 = 0

// Lucene90LiveDocsVersionCurrent is the current version of the format.
const Lucene90LiveDocsVersionCurrent int32 = Lucene90LiveDocsVersionStart

// Lucene90LiveDocsExtension is the file extension for the .liv file.
const Lucene90LiveDocsExtension = "liv"

// NewLiveDocs returns a new FixedBitSet for tracking live documents.
func (f *Lucene90LiveDocsFormat) NewLiveDocs(numDocs int) (*util.FixedBitSet, error) {
	return util.NewFixedBitSet(numDocs)
}

// ReadLiveDocs (backward-compat overload) reads live docs using del-gen 0
// and no expected-del-count cross-check. Prefer ReadLiveDocsLucene90 for
// faithful semantics.
func (f *Lucene90LiveDocsFormat) ReadLiveDocs(dir store.Directory, si *index.SegmentInfo) (util.Bits, error) {
	bits, _, err := f.ReadLiveDocsLucene90(dir, si, 0, -1, si.DocCount())
	return bits, err
}

// ReadLiveDocsLucene90 reads the .liv file produced by WriteLiveDocsLucene90
// against the given segment info and del-generation. Returns the bitset and
// the actual delCount measured from it. When expectedDelCount >= 0, returns
// an error if the on-disk bitset does not match it.
func (f *Lucene90LiveDocsFormat) ReadLiveDocsLucene90(dir store.Directory, si *index.SegmentInfo, delGen int64, expectedDelCount int, maxDoc int) (util.Bits, int, error) {
	name := fileNameFromGeneration(si.Name(), Lucene90LiveDocsExtension, delGen)
	if !dir.FileExists(name) {
		return nil, 0, nil
	}
	raw, err := dir.OpenInput(name, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, 0, err
	}
	defer raw.Close()
	in := store.NewChecksumIndexInput(raw)

	if _, err := CheckIndexHeader(in, Lucene90LiveDocsCodec, Lucene90LiveDocsVersionStart, Lucene90LiveDocsVersionCurrent, si.GetID(), genSuffix(delGen)); err != nil {
		return nil, 0, fmt.Errorf("lucene90 live docs: header: %w", err)
	}

	bits, err := readDenseLiveDocsBitSet(in, maxDoc)
	if err != nil {
		return nil, 0, err
	}
	delCount := maxDoc - bits.Cardinality()
	if _, err := CheckFooter(in); err != nil {
		return nil, 0, fmt.Errorf("lucene90 live docs: footer: %w", err)
	}
	if expectedDelCount >= 0 && delCount != expectedDelCount {
		return nil, 0, fmt.Errorf("lucene90 live docs: bits.deleted=%d expected=%d", delCount, expectedDelCount)
	}
	return bits, delCount, nil
}

// WriteLiveDocs (backward-compat overload) writes with delGen=0 and no
// cross-check on the resulting del-count.
func (f *Lucene90LiveDocsFormat) WriteLiveDocs(bits util.Bits, dir store.Directory, si *index.SegmentInfo) error {
	if bits == nil {
		return nil
	}
	return f.WriteLiveDocsLucene90(bits, dir, si, 0, -1, -1)
}

// WriteLiveDocsLucene90 writes the .liv file at the given del-generation
// with the segment's ID stamped into the IndexHeader. The bits payload is
// a dense FixedBitSet whose ghost bits (past maxDoc within the last word)
// must already be cleared by the caller.
//
// When expectedTotalDelCount >= 0, the method verifies that the bits
// represent exactly that many deletions and returns an error otherwise.
// The Java reference passes info.delCount + newDelCount.
func (f *Lucene90LiveDocsFormat) WriteLiveDocsLucene90(bits util.Bits, dir store.Directory, si *index.SegmentInfo, delGen int64, expectedTotalDelCount int, ignoreNewDelCount int) error {
	name := fileNameFromGeneration(si.Name(), Lucene90LiveDocsExtension, delGen)
	raw, err := dir.CreateOutput(name, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return err
	}
	out := store.NewChecksumIndexOutput(raw)

	if err := WriteIndexHeader(out, Lucene90LiveDocsCodec, Lucene90LiveDocsVersionCurrent, si.GetID(), genSuffix(delGen)); err != nil {
		_ = out.Close()
		return fmt.Errorf("lucene90 live docs: header: %w", err)
	}

	delCount, err := writeDenseLiveDocsBitSet(out, bits)
	if err != nil {
		_ = out.Close()
		return fmt.Errorf("lucene90 live docs: write bits: %w", err)
	}
	if err := WriteFooter(out); err != nil {
		_ = out.Close()
		return fmt.Errorf("lucene90 live docs: footer: %w", err)
	}
	if err := out.Close(); err != nil {
		return err
	}
	if expectedTotalDelCount >= 0 && delCount != expectedTotalDelCount {
		return fmt.Errorf("lucene90 live docs: bits.deleted=%d expected=%d", delCount, expectedTotalDelCount)
	}
	return nil
}

// Files returns the files used by this format for the given segment. The
// backward-compat overload assumes del-gen 0.
func (f *Lucene90LiveDocsFormat) Files(si *index.SegmentInfo) []string {
	return []string{fileNameFromGeneration(si.Name(), Lucene90LiveDocsExtension, 0)}
}

// readDenseLiveDocsBitSet reads a dense FixedBitSet of length maxDoc, 64
// bits per long, little-endian, matching IndexInput.readLongs.
func readDenseLiveDocsBitSet(in store.DataInput, maxDoc int) (*util.FixedBitSet, error) {
	numLongs := (maxDoc + 63) / 64
	words := make([]uint64, numLongs)
	for i := 0; i < numLongs; i++ {
		v, err := readLongLE(in)
		if err != nil {
			return nil, err
		}
		words[i] = uint64(v)
	}
	return util.NewFixedBitSetOfBits(words, maxDoc)
}

// writeDenseLiveDocsBitSet writes the bits as 64-bit little-endian longs
// in 1024-bit batches (matches Java's writeBits). Returns the number of
// deleted docs (= total bits length - cardinality), used by the caller to
// cross-check against expectedTotalDelCount.
func writeDenseLiveDocsBitSet(out store.IndexOutput, bits util.Bits) (int, error) {
	length := bits.Length()
	delCount := length

	const batchBits = 1024
	for offset := 0; offset < length; offset += batchBits {
		numBitsToCopy := batchBits
		if length-offset < numBitsToCopy {
			numBitsToCopy = length - offset
		}
		// Materialise the chunk as 16 longs, all bits initially set; for
		// each position in [0, numBitsToCopy), copy bit from source.
		const numLongs = batchBits / 64 // 16
		var words [numLongs]uint64
		for i := 0; i < numLongs; i++ {
			words[i] = ^uint64(0)
		}
		if numBitsToCopy < batchBits {
			// Clear ghost bits at the tail.
			for b := numBitsToCopy; b < batchBits; b++ {
				words[b>>6] &^= uint64(1) << uint(b&63)
			}
		}
		// Apply the source bits.
		for b := 0; b < numBitsToCopy; b++ {
			doc := offset + b
			if !bits.Get(doc) {
				words[b>>6] &^= uint64(1) << uint(b&63)
			}
		}
		// Count cardinality (live count) of this chunk and subtract.
		cardinality := 0
		copyLongs := (numBitsToCopy + 63) / 64
		for i := 0; i < copyLongs; i++ {
			cardinality += popcountUint64(words[i])
		}
		delCount -= cardinality
		// Emit the longs (only the meaningful chunk longCount).
		for i := 0; i < copyLongs; i++ {
			if err := writeLongLEOut(out, int64(words[i])); err != nil {
				return 0, err
			}
		}
	}
	return delCount, nil
}

// readLongLE reads an 8-byte little-endian signed long via DataInput.ReadByte
// to remain endian-correct against the project's BE/LE divergence.
func readLongLE(in store.DataInput) (int64, error) {
	var v uint64
	for i := 0; i < 8; i++ {
		b, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		v |= uint64(b) << (8 * uint(i))
	}
	return int64(v), nil
}

// writeLongLEOut writes an 8-byte little-endian signed long via WriteByte.
func writeLongLEOut(out store.IndexOutput, v int64) error {
	uv := uint64(v)
	for i := 0; i < 8; i++ {
		if err := out.WriteByte(byte(uv >> (8 * uint(i)))); err != nil {
			return err
		}
	}
	return nil
}

// popcountUint64 mirrors math/bits.OnesCount64 in a function form for use
// in hot loops.
func popcountUint64(v uint64) int {
	v = v - ((v >> 1) & 0x5555555555555555)
	v = (v & 0x3333333333333333) + ((v >> 2) & 0x3333333333333333)
	v = (v + (v >> 4)) & 0x0F0F0F0F0F0F0F0F
	return int((v * 0x0101010101010101) >> 56)
}

// fileNameFromGeneration mirrors IndexFileNames.fileNameFromGeneration:
// when generation is 0, returns "<segment>.<ext>"; otherwise it appends
// "_<generation in base 36>.<ext>" to disambiguate per-generation files.
func fileNameFromGeneration(segmentName, ext string, generation int64) string {
	if generation == 0 {
		return segmentName + "." + ext
	}
	return segmentName + "_" + strconvBase36(generation) + "." + ext
}

// genSuffix encodes the del-generation as the index header suffix using
// base 36 (Character.MAX_RADIX in Java).
func genSuffix(generation int64) string {
	return strconvBase36(generation)
}

// strconvBase36 returns generation as a base-36 string (lowercase a-z).
// Equivalent to Long.toString(g, Character.MAX_RADIX) in Java.
func strconvBase36(generation int64) string {
	if generation == 0 {
		return "0"
	}
	neg := generation < 0
	g := uint64(generation)
	if neg {
		g = uint64(-generation)
	}
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	var buf [13]byte // 64-bit unsigned fits in 13 base-36 digits
	i := len(buf)
	for g > 0 {
		i--
		buf[i] = alphabet[g%36]
		g /= 36
	}
	out := string(buf[i:])
	if neg {
		out = "-" + out
	}
	return out
}

// LiveDocsReader provides read access to live docs.
type LiveDocsReader struct {
	format      LiveDocsFormat
	directory   store.Directory
	segmentInfo *index.SegmentInfo
	liveDocs    util.Bits
	mu          sync.RWMutex
}

// NewLiveDocsReader creates a new LiveDocsReader.
func NewLiveDocsReader(format LiveDocsFormat, dir store.Directory, segmentInfo *index.SegmentInfo) (*LiveDocsReader, error) {
	reader := &LiveDocsReader{
		format:      format,
		directory:   dir,
		segmentInfo: segmentInfo,
	}

	liveDocs, err := format.ReadLiveDocs(dir, segmentInfo)
	if err != nil {
		return nil, err
	}
	reader.liveDocs = liveDocs

	return reader, nil
}

// IsLive returns true if the document is live (not deleted).
func (r *LiveDocsReader) IsLive(docID int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.liveDocs == nil {
		return true
	}

	if docID < 0 || docID >= r.liveDocs.Length() {
		return false
	}

	return r.liveDocs.Get(docID)
}

// NumDocs returns the number of documents in the live docs.
func (r *LiveDocsReader) NumDocs() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.liveDocs == nil {
		return r.segmentInfo.DocCount()
	}

	return r.liveDocs.Length()
}

// LiveDocsWriter provides write access to live docs.
type LiveDocsWriter struct {
	format      LiveDocsFormat
	directory   store.Directory
	segmentInfo *index.SegmentInfo
	liveDocs    *util.FixedBitSet
	mu          sync.Mutex
}

// NewLiveDocsWriter creates a new LiveDocsWriter.
func NewLiveDocsWriter(format LiveDocsFormat, dir store.Directory, segmentInfo *index.SegmentInfo) (*LiveDocsWriter, error) {
	numDocs := segmentInfo.DocCount()
	liveDocs, err := format.NewLiveDocs(numDocs)
	if err != nil {
		return nil, err
	}

	// Initialize all docs as live
	for i := 0; i < numDocs; i++ {
		liveDocs.Set(i)
	}

	return &LiveDocsWriter{
		format:      format,
		directory:   dir,
		segmentInfo: segmentInfo,
		liveDocs:    liveDocs,
	}, nil
}

// DeleteDocument marks a document as deleted (not live).
func (w *LiveDocsWriter) DeleteDocument(docID int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if docID < 0 || docID >= w.liveDocs.Length() {
		return fmt.Errorf("document ID %d out of range [0, %d)", docID, w.liveDocs.Length())
	}

	w.liveDocs.Clear(docID)
	return nil
}

// IsLive returns true if the document is live (not deleted).
func (w *LiveDocsWriter) IsLive(docID int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if docID < 0 || docID >= w.liveDocs.Length() {
		return false
	}

	return w.liveDocs.Get(docID)
}

// Commit writes the live docs to disk.
func (w *LiveDocsWriter) Commit() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.format.WriteLiveDocs(w.liveDocs, w.directory, w.segmentInfo)
}

// Ensure implementations satisfy the interfaces
var _ LiveDocsFormat = (*BaseLiveDocsFormat)(nil)
var _ LiveDocsFormat = (*Lucene90LiveDocsFormat)(nil)
