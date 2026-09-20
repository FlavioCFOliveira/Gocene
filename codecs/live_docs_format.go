// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// LiveDocsFormat is an alias of [spi.LiveDocsFormat], the canonical
// declaration of org.apache.lucene.codecs.LiveDocsFormat. The interface was
// lifted onto the SPI so that index/ (which reaches it through
// spi.Codec.LiveDocsFormat) and codecs/ (which implements it) share one
// declaration site, following the pattern already used for Codec,
// NormsFormat, DocValuesFormat and the rest of the per-component formats.
type LiveDocsFormat = spi.LiveDocsFormat

// BaseLiveDocsFormat is the Go rendering of the abstract
// org.apache.lucene.codecs.LiveDocsFormat base class: it carries the format
// name and leaves every serialization member to the concrete format, which
// Go expresses by reporting an explicit "not implemented" error where Java
// simply has no method body to inherit.
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

// ReadLiveDocs reports that the concrete format did not implement the
// abstract LiveDocsFormat.readLiveDocs member.
func (f *BaseLiveDocsFormat) ReadLiveDocs(dir store.Directory, info *spi.SegmentCommitInfo, ctx store.IOContext) (util.Bits, error) {
	return nil, fmt.Errorf("live docs format %q: ReadLiveDocs not implemented", f.name)
}

// WriteLiveDocs reports that the concrete format did not implement the
// abstract LiveDocsFormat.writeLiveDocs member.
func (f *BaseLiveDocsFormat) WriteLiveDocs(bits util.Bits, dir store.Directory, info *spi.SegmentCommitInfo, newDelCount int, ctx store.IOContext) error {
	return fmt.Errorf("live docs format %q: WriteLiveDocs not implemented", f.name)
}

// Files reports that the concrete format did not implement the abstract
// LiveDocsFormat.files member.
func (f *BaseLiveDocsFormat) Files(info *spi.SegmentCommitInfo, files *[]string) error {
	return fmt.Errorf("live docs format %q: Files not implemented", f.name)
}

// Lucene90LiveDocsFormat is the Lucene 9.0 live docs format. The .liv file
// stores a FixedBitSet (1 = doc is live; 0 = doc is deleted), framed by a
// CodecUtil IndexHeader and Footer. The IndexHeader's suffix carries the
// del-generation in Character.MAX_RADIX (36).
//
// Port of org.apache.lucene.codecs.lucene90.Lucene90LiveDocsFormat from
// Apache Lucene 10.5.0
// (lucene/core/src/java/org/apache/lucene/codecs/lucene90/Lucene90LiveDocsFormat.java).
//
// The read path reproduces the Java sparse/dense split: when the segment's
// deletion rate is at or below SPARSE_DENSE_THRESHOLD (1%) the on-disk words
// are inverted into a SparseFixedBitSet of deleted documents and wrapped in a
// util.SparseLiveDocs; otherwise the dense FixedBitSet is used directly. The
// on-disk bytes are identical either way — the split only selects the
// in-memory representation.
//
// ORGANISATIONAL DIVERGENCE (documented): the Java class lives in
// org.apache.lucene.codecs.lucene90, which maps to the Gocene package
// codecs/lucene90. That package imports codecs, so Go's ban on import cycles
// prevents codecs (which needs the format for BaseCodec's concrete codecs,
// CompressingCodec and Lucene104Codec) from importing it back. The definition
// therefore lives here and codecs/lucene90 re-exports it under its Lucene name
// (see codecs/lucene90/lucene90_live_docs_format.go).
//
// BEHAVIOURAL DIVERGENCE (documented): on the dense branch Java wraps the
// FixedBitSet in org.apache.lucene.util.DenseLiveDocs; Gocene returns the
// *util.FixedBitSet directly because util.DenseLiveDocs does not implement
// util.Bits (it has no Cardinality method). Get, Length and Cardinality answer
// identically, so the divergence is confined to the concrete type returned.
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

// Lucene90LiveDocsSparseDenseThreshold is the deletion rate at or below which
// the read path materialises the live docs as a SparseFixedBitSet of deleted
// documents instead of a dense FixedBitSet. Mirrors
// Lucene90LiveDocsFormat.SPARSE_DENSE_THRESHOLD (1%).
const Lucene90LiveDocsSparseDenseThreshold = 0.01

// NewLiveDocs returns a new FixedBitSet sized for numDocs documents. Lucene
// dropped LiveDocsFormat.newLiveDocs in 9.0; the helper is retained here for
// the package's own writers, which need an all-live starting point.
func (f *Lucene90LiveDocsFormat) NewLiveDocs(numDocs int) (*util.FixedBitSet, error) {
	return util.NewFixedBitSet(numDocs)
}

// ReadLiveDocs reads the .liv file of info from dir. Mirrors
// Lucene90LiveDocsFormat.readLiveDocs(Directory, SegmentCommitInfo,
// IOContext): the generation, the segment identity, the document count and
// the expected delete count all come from the commit info, and the deletion
// rate derived from them selects the sparse or dense representation.
//
// ctx is accepted for interface fidelity and deliberately unused, exactly as
// in Java, where readLiveDocs reaches the file through
// Directory.openChecksumInput (which always uses IOContext.READONCE) and
// never consults its context argument.
func (f *Lucene90LiveDocsFormat) ReadLiveDocs(dir store.Directory, info *spi.SegmentCommitInfo, ctx store.IOContext) (util.Bits, error) {
	si := info.Info
	gen := info.DelGen()
	maxDoc := si.MaxDoc()
	delCount := info.DelCount()
	// Java: (double) delCount / maxDoc. A zero maxDoc yields NaN on both
	// platforms, and NaN <= threshold is false, so the dense branch is taken.
	deletionRate := float64(delCount) / float64(maxDoc)
	name := fileNameFromGeneration(si.Name(), Lucene90LiveDocsExtension, gen)
	bits, _, err := f.readLiveDocsFile(dir, name, si.GetID(), gen, maxDoc, deletionRate, delCount)
	return bits, err
}

// ReadLiveDocsLucene90 reads the .liv file produced by WriteLiveDocsLucene90
// against the given segment info and del-generation. Returns the bitset and
// the actual delCount measured from it. When expectedDelCount >= 0, returns
// an error if the on-disk bitset does not match it, and the expected count
// also feeds the sparse/dense representation choice; when it is negative no
// deletion rate is known and the dense representation is used.
//
// This is the explicit-generation entry point used where no SegmentCommitInfo
// is at hand; ReadLiveDocs is the Lucene-faithful member of the format.
func (f *Lucene90LiveDocsFormat) ReadLiveDocsLucene90(dir store.Directory, si *spi.SegmentInfo, delGen int64, expectedDelCount int, maxDoc int) (util.Bits, int, error) {
	name := fileNameFromGeneration(si.Name(), Lucene90LiveDocsExtension, delGen)
	if !dir.FileExists(name) {
		return nil, 0, nil
	}
	deletionRate := 1.0
	if expectedDelCount >= 0 && maxDoc > 0 {
		deletionRate = float64(expectedDelCount) / float64(maxDoc)
	}
	return f.readLiveDocsFile(dir, name, si.GetID(), delGen, maxDoc, deletionRate, expectedDelCount)
}

// readLiveDocsFile opens name, validates the IndexHeader against the segment
// id and the base-36 generation suffix, decodes the bits and validates the
// footer. It is the shared body of ReadLiveDocs and ReadLiveDocsLucene90.
func (f *Lucene90LiveDocsFormat) readLiveDocsFile(dir store.Directory, name string, segmentID []byte, delGen int64, maxDoc int, deletionRate float64, expectedDelCount int) (util.Bits, int, error) {
	raw, err := dir.OpenInput(name, store.IOContextReadOnce)
	if err != nil {
		return nil, 0, err
	}
	defer raw.Close()
	in := spi.NewChecksumIndexInput(raw)

	if _, err := spi.CheckIndexHeader(in, Lucene90LiveDocsCodec, Lucene90LiveDocsVersionStart, Lucene90LiveDocsVersionCurrent, segmentID, genSuffix(delGen)); err != nil {
		return nil, 0, fmt.Errorf("lucene90 live docs: header: %w", err)
	}

	bits, delCount, err := decodeLiveDocs(in, maxDoc, deletionRate, expectedDelCount)
	if err != nil {
		return nil, 0, err
	}
	if _, err := spi.CheckFooter(in); err != nil {
		return nil, 0, fmt.Errorf("lucene90 live docs: footer: %w", err)
	}
	return bits, delCount, nil
}

// decodeLiveDocs reads the bit payload and chooses the sparse or dense
// in-memory representation from the deletion rate, mirroring the private
// Lucene90LiveDocsFormat.readLiveDocs(IndexInput, int, double, int). When
// expectedDelCount >= 0 the measured delete count is cross-checked against it,
// as Java does before returning.
func decodeLiveDocs(in store.DataInput, maxDoc int, deletionRate float64, expectedDelCount int) (util.Bits, int, error) {
	var liveDocs util.Bits
	var actualDelCount int

	if deletionRate <= Lucene90LiveDocsSparseDenseThreshold {
		sparse, err := readSparseLiveDocsBitSet(in, maxDoc)
		if err != nil {
			return nil, 0, err
		}
		actualDelCount = sparse.Cardinality()
		liveDocs = util.NewSparseLiveDocsBuilder(sparse, maxDoc).Build()
	} else {
		dense, err := readDenseLiveDocsBitSet(in, maxDoc)
		if err != nil {
			return nil, 0, err
		}
		actualDelCount = maxDoc - dense.Cardinality()
		liveDocs = dense
	}

	if expectedDelCount >= 0 && actualDelCount != expectedDelCount {
		return nil, 0, fmt.Errorf("lucene90 live docs: bits.deleted=%d info.delcount=%d", actualDelCount, expectedDelCount)
	}
	return liveDocs, actualDelCount, nil
}

// WriteLiveDocs writes the .liv file for info at the generation reserved by
// info.GetNextWriteDelGen. Mirrors
// Lucene90LiveDocsFormat.writeLiveDocs(Bits, Directory, SegmentCommitInfo,
// int, IOContext), including the final cross-check that the written bits hold
// exactly info.DelCount() + newDelCount deletions.
func (f *Lucene90LiveDocsFormat) WriteLiveDocs(bits util.Bits, dir store.Directory, info *spi.SegmentCommitInfo, newDelCount int, ctx store.IOContext) error {
	si := info.Info
	gen := info.GetNextWriteDelGen()
	return f.writeLiveDocsFile(bits, dir, si, gen, info.DelCount()+newDelCount, ctx)
}

// Files appends the .liv file info keeps in use, when it has deletions.
// Mirrors Lucene90LiveDocsFormat.files(SegmentCommitInfo, Collection<String>).
func (f *Lucene90LiveDocsFormat) Files(info *spi.SegmentCommitInfo, files *[]string) error {
	if info.HasDeletions() {
		*files = append(*files, fileNameFromGeneration(info.Info.Name(), Lucene90LiveDocsExtension, info.DelGen()))
	}
	return nil
}

// WriteLiveDocsLucene90 writes the .liv file at the given del-generation
// with the segment's ID stamped into the IndexHeader. The bits payload is
// a dense FixedBitSet whose ghost bits (past maxDoc within the last word)
// must already be cleared by the caller.
//
// When expectedTotalDelCount >= 0, the method verifies that the bits
// represent exactly that many deletions and returns an error otherwise.
// The Java reference passes info.delCount + newDelCount.
//
// ignoreNewDelCount is retained for call-site compatibility and is unused:
// the Java writer derives its cross-check from the single total
// info.getDelCount() + newDelCount, which callers pass as
// expectedTotalDelCount.
func (f *Lucene90LiveDocsFormat) WriteLiveDocsLucene90(bits util.Bits, dir store.Directory, si *spi.SegmentInfo, delGen int64, expectedTotalDelCount int, ignoreNewDelCount int) error {
	return f.writeLiveDocsFile(bits, dir, si, delGen, expectedTotalDelCount, store.IOContext{Context: store.ContextWrite})
}

// writeLiveDocsFile is the shared body of WriteLiveDocs and
// WriteLiveDocsLucene90: create the generation-suffixed .liv file, stamp the
// IndexHeader with the segment id and the base-36 generation, emit the dense
// bit payload, write the footer, and finally cross-check the number of
// deletions actually written.
func (f *Lucene90LiveDocsFormat) writeLiveDocsFile(bits util.Bits, dir store.Directory, si *spi.SegmentInfo, delGen int64, expectedTotalDelCount int, ctx store.IOContext) error {
	name := fileNameFromGeneration(si.Name(), Lucene90LiveDocsExtension, delGen)
	raw, err := dir.CreateOutput(name, ctx)
	if err != nil {
		return err
	}
	out := spi.NewChecksumIndexOutput(raw)

	if err := spi.WriteIndexHeader(out, Lucene90LiveDocsCodec, Lucene90LiveDocsVersionCurrent, si.GetID(), genSuffix(delGen)); err != nil {
		_ = out.Close()
		return fmt.Errorf("lucene90 live docs: header: %w", err)
	}

	delCount, err := writeDenseLiveDocsBitSet(out, bits)
	if err != nil {
		_ = out.Close()
		return fmt.Errorf("lucene90 live docs: write bits: %w", err)
	}
	if err := spi.WriteFooter(out); err != nil {
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

// readSparseLiveDocsBitSet reads the same dense payload as
// readDenseLiveDocsBitSet and inverts it into a SparseFixedBitSet of DELETED
// documents, mirroring Lucene90LiveDocsFormat.readSparseFixedBitSet: the disk
// format stores live docs (bit set = live) while SparseLiveDocs stores deleted
// docs (bit set = deleted). Words with every bit set carry no deletions and
// are skipped, and bits past maxDoc inside the final word are ignored.
func readSparseLiveDocsBitSet(in store.DataInput, maxDoc int) (*util.SparseFixedBitSet, error) {
	numLongs := (maxDoc + 63) / 64
	words := make([]uint64, numLongs)
	for i := 0; i < numLongs; i++ {
		v, err := readLongLE(in)
		if err != nil {
			return nil, err
		}
		words[i] = uint64(v)
	}

	sparse, err := util.NewSparseFixedBitSet(maxDoc)
	if err != nil {
		return nil, err
	}
	for wordIndex := 0; wordIndex < numLongs; wordIndex++ {
		word := words[wordIndex]
		if word == ^uint64(0) {
			continue
		}
		baseDocID := wordIndex << 6
		maxDocInWord := baseDocID + 64
		if maxDocInWord > maxDoc {
			maxDocInWord = maxDoc
		}
		for docID := baseDocID; docID < maxDocInWord; docID++ {
			if word&(uint64(1)<<uint(docID&63)) == 0 {
				sparse.Set(docID)
			}
		}
	}
	return sparse, nil
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
	format     LiveDocsFormat
	directory  store.Directory
	commitInfo *spi.SegmentCommitInfo
	liveDocs   util.Bits
	mu         sync.RWMutex
}

// NewLiveDocsReader creates a new LiveDocsReader over the live docs of
// commitInfo, loading them eagerly through format.
func NewLiveDocsReader(format LiveDocsFormat, dir store.Directory, commitInfo *spi.SegmentCommitInfo) (*LiveDocsReader, error) {
	reader := &LiveDocsReader{
		format:     format,
		directory:  dir,
		commitInfo: commitInfo,
	}

	liveDocs, err := format.ReadLiveDocs(dir, commitInfo, store.IOContextReadOnce)
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
		return r.commitInfo.Info.DocCount()
	}

	return r.liveDocs.Length()
}

// LiveDocsWriter provides write access to live docs.
type LiveDocsWriter struct {
	format     LiveDocsFormat
	directory  store.Directory
	commitInfo *spi.SegmentCommitInfo
	liveDocs   *util.FixedBitSet
	newDeletes int
	mu         sync.Mutex
}

// NewLiveDocsWriter creates a new LiveDocsWriter whose bitset starts with
// every document of commitInfo marked live.
func NewLiveDocsWriter(format LiveDocsFormat, dir store.Directory, commitInfo *spi.SegmentCommitInfo) (*LiveDocsWriter, error) {
	numDocs := commitInfo.Info.DocCount()
	liveDocs, err := util.NewFixedBitSet(numDocs)
	if err != nil {
		return nil, err
	}

	// Initialize all docs as live
	for i := 0; i < numDocs; i++ {
		liveDocs.Set(i)
	}

	return &LiveDocsWriter{
		format:     format,
		directory:  dir,
		commitInfo: commitInfo,
		liveDocs:   liveDocs,
	}, nil
}

// DeleteDocument marks a document as deleted (not live).
func (w *LiveDocsWriter) DeleteDocument(docID int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if docID < 0 || docID >= w.liveDocs.Length() {
		return fmt.Errorf("document ID %d out of range [0, %d)", docID, w.liveDocs.Length())
	}

	if w.liveDocs.Get(docID) {
		w.liveDocs.Clear(docID)
		w.newDeletes++
	}
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

// Commit writes the live docs to disk at the segment's next delete
// generation, reporting the deletions accumulated since the writer was
// created as the new delete count.
func (w *LiveDocsWriter) Commit() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.format.WriteLiveDocs(w.liveDocs, w.directory, w.commitInfo, w.newDeletes, store.IOContextDefault); err != nil {
		return err
	}
	w.newDeletes = 0
	return nil
}

// Ensure implementations satisfy the interfaces
var _ LiveDocsFormat = (*BaseLiveDocsFormat)(nil)
var _ LiveDocsFormat = (*Lucene90LiveDocsFormat)(nil)
