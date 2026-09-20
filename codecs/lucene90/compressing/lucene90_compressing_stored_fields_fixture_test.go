// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing_test

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"
	"testing"

	lucene90 "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90/compressing"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// This file pins the Lucene90 compressing stored-fields port against the
// golden fixture in testdata/lucene-10.4.0-fixtures, which was written by
// Apache Lucene 10.4.0 with the default Lucene104Codec (whose
// StoredFieldsFormat is Lucene90StoredFieldsFormat in Mode.BEST_SPEED).
//
// It covers the two legs the Binary Compatibility Mandate requires of the
// format:
//
//   - Lucene-write -> Gocene-read: every stored field of every document, plus
//     the CodecUtil checksum framing of .fdt and .fdx, cross-checked against an
//     independent CRC32 over the file bytes;
//   - Gocene-write -> Lucene-read: the same logical documents written back
//     through the ported writer with the same codec name, version, segment id
//     and chunk parameters Lucene used, compared byte for byte.
//
// The fixture's segment carries four stored fields per document — id, body and
// tag as strings and int_point_stored as an int — whose FieldInfo numbers are
// 0, 1, 2 and 11.

const fixtureRelDir = "../../../testdata/lucene-10.4.0-fixtures"

// ---------------------------------------------------------------------------
// Fixture decoding, independent of the code under test
// ---------------------------------------------------------------------------

// fixtureReader is a minimal decoder for the handful of Lucene primitives the
// fixture's .cfe and .fnm need. It is deliberately independent of Gocene's
// store package, so that the extraction of the Lucene bytes is evidence rather
// than another use of the code being verified.
//
// Note that DataOutput.writeInt/writeShort/writeLong are LITTLE-endian in
// Lucene 10.x (DataOutput.java:73-89, 223-226) while CodecUtil's header and
// footer scalars are written big-endian through writeBEInt/writeBELong
// (CodecUtil.java:83-85, 410-411).
type fixtureReader struct {
	b   []byte
	pos int
}

func (r *fixtureReader) readByte() byte { v := r.b[r.pos]; r.pos++; return v }

func (r *fixtureReader) readBEInt() int32 {
	v := int32(binary.BigEndian.Uint32(r.b[r.pos:]))
	r.pos += 4
	return v
}

func (r *fixtureReader) readLong() int64 {
	v := int64(binary.LittleEndian.Uint64(r.b[r.pos:]))
	r.pos += 8
	return v
}

func (r *fixtureReader) readVInt() int32 {
	b := r.readByte()
	i := int32(b & 0x7F)
	for shift := uint(7); b&0x80 != 0; shift += 7 {
		b = r.readByte()
		i |= int32(b&0x7F) << shift
	}
	return i
}

func (r *fixtureReader) readString() string {
	n := int(r.readVInt())
	s := string(r.b[r.pos : r.pos+n])
	r.pos += n
	return s
}

// readIndexHeader consumes CodecUtil.writeIndexHeader framing.
func (r *fixtureReader) readIndexHeader(t *testing.T) (codec string, version int32, id []byte, suffix string) {
	t.Helper()
	if magic := r.readBEInt(); magic != 0x3fd76c17 {
		t.Fatalf("bad codec magic 0x%08x, want 0x3fd76c17", uint32(magic))
	}
	codec = r.readString()
	version = r.readBEInt()
	id = append([]byte(nil), r.b[r.pos:r.pos+16]...)
	r.pos += 16
	sl := int(r.readByte())
	suffix = string(r.b[r.pos : r.pos+sl])
	r.pos += sl
	return codec, version, id, suffix
}

// splitCompoundFile writes every member of <seg>.cfs out into dstDir under its
// full segment file name, using the directory held in <seg>.cfe. It returns the
// 16-byte segment id recorded in the .cfe index header.
func splitCompoundFile(t *testing.T, fixtureDir, seg, dstDir string) []byte {
	t.Helper()
	cfe, err := os.ReadFile(filepath.Join(fixtureDir, seg+".cfe"))
	if err != nil {
		t.Fatalf("read .cfe: %v", err)
	}
	cfs, err := os.ReadFile(filepath.Join(fixtureDir, seg+".cfs"))
	if err != nil {
		t.Fatalf("read .cfs: %v", err)
	}
	r := &fixtureReader{b: cfe}
	codec, _, id, _ := r.readIndexHeader(t)
	if codec != "Lucene90CompoundEntries" {
		t.Fatalf("unexpected .cfe codec %q", codec)
	}
	n := int(r.readVInt())
	for i := 0; i < n; i++ {
		name := r.readString()
		offset := r.readLong()
		length := r.readLong()
		if err := os.WriteFile(filepath.Join(dstDir, seg+name), cfs[offset:offset+length], 0o600); err != nil {
			t.Fatalf("write %s: %v", seg+name, err)
		}
	}
	return id
}

// fixtureField is a (fieldNumber, name) pair out of the segment's .fnm.
type fixtureField struct {
	number int
	name   string
}

// parseFieldInfos decodes Lucene94FieldInfosFormat far enough to recover every
// (fieldNumber, name) pair, stepping over each record exactly as
// Lucene94FieldInfosFormat#read does (Lucene94FieldInfosFormat.java:131-230).
func parseFieldInfos(t *testing.T, path string) []fixtureField {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .fnm: %v", err)
	}
	r := &fixtureReader{b: b}
	codec, format, _, _ := r.readIndexHeader(t)
	if codec != "Lucene94FieldInfos" {
		t.Fatalf("unexpected .fnm codec %q", codec)
	}
	size := int(r.readVInt())
	out := make([]fixtureField, 0, size)
	for i := 0; i < size; i++ {
		name := r.readString()
		number := int(r.readVInt())
		r.readByte() // bits
		r.readByte() // indexOptions
		r.readByte() // docValuesType
		if format >= 2 {
			r.readByte() // docValuesSkipIndexType (FORMAT_DOCVALUE_SKIPPER)
		}
		r.readLong() // dvGen
		for a, n := 0, int(r.readVInt()); a < n; a++ {
			r.readString()
			r.readString()
		}
		if pdd := int(r.readVInt()); pdd != 0 {
			r.readVInt() // pointIndexDimensionCount
			r.readVInt() // pointNumBytes
		}
		r.readVInt() // vectorDimension
		r.readByte() // vectorEncoding
		r.readByte() // vectorSimilarityFunction
		out = append(out, fixtureField{number: number, name: name})
	}
	return out
}

// prepareFixtureSegment expands _0.cfs into a fresh directory and returns that
// directory, the segment id, and the segment's field infos.
func prepareFixtureSegment(t *testing.T) (dir string, id []byte, fields []fixtureField) {
	t.Helper()
	dir = t.TempDir()
	id = splitCompoundFile(t, fixtureRelDir, "_0", dir)
	fields = parseFieldInfos(t, filepath.Join(dir, "_0.fnm"))
	return dir, id, fields
}

// bestSpeedFormat is Lucene90StoredFieldsFormat.Mode.BEST_SPEED, the stored
// fields format the fixture's Lucene104Codec used
// (Lucene90StoredFieldsFormat.java:159-161):
//
//	new Lucene90CompressingStoredFieldsFormat(
//	    "Lucene90StoredFieldsFastData", BEST_SPEED_MODE, BEST_SPEED_BLOCK_LENGTH, 1024, 10)
//
// with BEST_SPEED_MODE = new LZ4WithPresetDictCompressionMode() and
// BEST_SPEED_BLOCK_LENGTH = 10 * 8 * 1024.
func bestSpeedFormat() *compressing.Lucene90CompressingStoredFieldsFormat {
	return compressing.NewLucene90CompressingStoredFieldsFormatWithOptions(
		"Lucene90StoredFieldsFastData",
		lucene90.NewLZ4WithPresetDictCompressionMode(),
		10*8*1024,
		1024,
		10,
	)
}

func fixtureSegmentInfo(t *testing.T, dir string, id []byte, numDocs int) (*index.SegmentInfo, store.Directory) {
	t.Helper()
	d, err := store.NewNIOFSDirectory(dir)
	if err != nil {
		t.Fatalf("open directory: %v", err)
	}
	si := index.NewSegmentInfo("_0", numDocs, d)
	if err := si.SetID(id); err != nil {
		t.Fatalf("set segment id: %v", err)
	}
	return si, d
}

func fixtureFieldInfos(fields []fixtureField) *index.FieldInfos {
	sorted := append([]fixtureField(nil), fields...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].number < sorted[j].number })
	fis := index.NewFieldInfos()
	for _, f := range sorted {
		fis.Add(index.NewFieldInfo(f.name, f.number, index.FieldInfoOptions{}))
	}
	return fis
}

// ---------------------------------------------------------------------------
// The corpus
// ---------------------------------------------------------------------------

// storedVisit is one StoredFieldVisitor callback.
type storedVisit struct {
	kind  string
	field string
	value any
}

// visitRecorder records every callback VisitDocument makes, in order.
type visitRecorder struct{ visits []storedVisit }

// NeedsField accepts every stored field.
func (v *visitRecorder) NeedsField(*spi.FieldInfo) (spi.StoredFieldVisitorStatus, error) {
	return spi.StoredFieldVisitorStatusYes, nil
}

func (v *visitRecorder) StringField(f *spi.FieldInfo, s string) error {
	v.add("String", f.Name(), s)
	return nil
}
func (v *visitRecorder) BinaryField(f *spi.FieldInfo, b []byte) error {
	v.add("Binary", f.Name(), b)
	return nil
}
func (v *visitRecorder) IntField(f *spi.FieldInfo, i int) error {
	v.add("Int", f.Name(), i)
	return nil
}
func (v *visitRecorder) LongField(f *spi.FieldInfo, i int64) error {
	v.add("Long", f.Name(), i)
	return nil
}
func (v *visitRecorder) FloatField(f *spi.FieldInfo, x float32) error {
	v.add("Float", f.Name(), x)
	return nil
}
func (v *visitRecorder) DoubleField(f *spi.FieldInfo, x float64) error {
	v.add("Double", f.Name(), x)
	return nil
}

func (v *visitRecorder) add(kind, field string, value any) {
	v.visits = append(v.visits, storedVisit{kind: kind, field: field, value: value})
}

// expectedStoredFields is the stored content of document i of the fixture
// corpus, in the order Lucene stored it.
func expectedStoredFields(i int) []storedVisit {
	return []storedVisit{
		{"String", "id", fmt.Sprintf("doc-%d", i)},
		{"String", "body", fmt.Sprintf("lucene document number %d with some words for testing postings", i)},
		{"String", "tag", fmt.Sprintf("tag-%d", i%5)},
		{"Int", "int_point_stored", i},
	}
}

const fixtureNumDocs = 20

// ---------------------------------------------------------------------------
// Leg 1 — Lucene-write -> Gocene-read
// ---------------------------------------------------------------------------

// TestLucene90CompressingStoredFieldsLuceneFixtureRead reads every stored field
// of every document of the Lucene 10.4.0 fixture through the ported reader.
func TestLucene90CompressingStoredFieldsLuceneFixtureRead(t *testing.T) {
	dir, id, fields := prepareFixtureSegment(t)
	si, d := fixtureSegmentInfo(t, dir, id, fixtureNumDocs)

	reader, err := bestSpeedFormat().FieldsReader(d, si, fixtureFieldInfos(fields), store.NewIOContext())
	if err != nil {
		t.Fatalf("open stored fields reader: %v", err)
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Errorf("close reader: %v", err)
		}
	})

	for doc := 0; doc < fixtureNumDocs; doc++ {
		rec := &visitRecorder{}
		if err := reader.VisitDocument(doc, rec); err != nil {
			t.Fatalf("doc %d: VisitDocument: %v", doc, err)
		}
		want := expectedStoredFields(doc)
		if len(rec.visits) != len(want) {
			t.Fatalf("doc %d: got %d stored fields, want %d: %+v", doc, len(rec.visits), len(want), rec.visits)
		}
		for i, w := range want {
			got := rec.visits[i]
			if got.kind != w.kind || got.field != w.field || got.value != w.value {
				t.Errorf("doc %d field %d: got %s(%q)=%v, want %s(%q)=%v",
					doc, i, got.kind, got.field, got.value, w.kind, w.field, w.value)
			}
		}
	}
}

// TestLucene90CompressingStoredFieldsLuceneFixtureCheckIntegrity runs the
// reader's CheckIntegrity over the fixture and cross-checks the CodecUtil
// footers of .fdt, .fdx and .fdm against an independent CRC32 over the file
// bytes.
func TestLucene90CompressingStoredFieldsLuceneFixtureCheckIntegrity(t *testing.T) {
	dir, id, fields := prepareFixtureSegment(t)
	si, d := fixtureSegmentInfo(t, dir, id, fixtureNumDocs)

	reader, err := bestSpeedFormat().FieldsReader(d, si, fixtureFieldInfos(fields), store.NewIOContext())
	if err != nil {
		t.Fatalf("open stored fields reader: %v", err)
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Errorf("close reader: %v", err)
		}
	})

	if err := reader.CheckIntegrity(); err != nil {
		t.Fatalf("CheckIntegrity: %v", err)
	}

	for _, name := range []string{"_0.fdt", "_0.fdx", "_0.fdm"} {
		checkCodecFooter(t, filepath.Join(dir, name))
	}
}

// checkCodecFooter recomputes what CodecUtil.checksumEntireFile validates: the
// footer is FOOTER_MAGIC (big-endian int), algorithmID (big-endian int) and the
// checksum (big-endian long) over every preceding byte.
func checkCodecFooter(t *testing.T, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(b) < 16 {
		t.Fatalf("%s: %d bytes, too short for a codec footer", path, len(b))
	}
	n := len(b)
	const footerMagic = uint32(^uint32(0x3fd76c17))
	if magic := binary.BigEndian.Uint32(b[n-16 : n-12]); magic != footerMagic {
		t.Fatalf("%s: footer magic 0x%08x, want 0x%08x", path, magic, footerMagic)
	}
	if algo := binary.BigEndian.Uint32(b[n-12 : n-8]); algo != 0 {
		t.Fatalf("%s: algorithmID %d, want 0", path, algo)
	}
	stored := binary.BigEndian.Uint64(b[n-8:])
	computed := uint64(crc32.ChecksumIEEE(b[:n-8]))
	if stored != computed {
		t.Fatalf("%s: stored CRC32 0x%016x, independently computed 0x%016x over [0,%d)",
			path, stored, computed, n-8)
	}
}

// ---------------------------------------------------------------------------
// Leg 2 — Gocene-write -> Lucene-read
// ---------------------------------------------------------------------------

// fixtureStoredField is the smallest spi.IndexableField carrying a typed
// StoredValue, which is what the ported writer dispatches on.
type fixtureStoredField struct {
	name  string
	value *document.StoredValue
}

func (f *fixtureStoredField) Name() string { return f.name }

func (f *fixtureStoredField) StringValue() string {
	if f.value.Type() == document.StoredValueTypeString {
		return f.value.StringValue()
	}
	return ""
}

func (f *fixtureStoredField) BinaryValue() []byte { return nil }

func (f *fixtureStoredField) NumericValue() interface{} { return nil }

func (f *fixtureStoredField) StoredValue() *document.StoredValue { return f.value }

var _ spi.IndexableField = (*fixtureStoredField)(nil)

// TestLucene90CompressingStoredFieldsLuceneFixtureWrite writes the fixture's
// logical documents back through the ported writer, with the same codec name,
// version, segment id and chunk parameters Lucene used, and requires the
// emitted .fdt, .fdx and .fdm to be byte-identical to Lucene's.
func TestLucene90CompressingStoredFieldsLuceneFixtureWrite(t *testing.T) {
	luceneDir, id, fields := prepareFixtureSegment(t)
	fis := fixtureFieldInfos(fields)

	goceneDir := t.TempDir()
	si, d := fixtureSegmentInfo(t, goceneDir, id, fixtureNumDocs)
	writer, err := bestSpeedFormat().FieldsWriter(d, si, store.NewIOContext())
	if err != nil {
		t.Fatalf("open stored fields writer: %v", err)
	}
	for doc := 0; doc < fixtureNumDocs; doc++ {
		if err := writer.StartDocument(); err != nil {
			t.Fatalf("doc %d: StartDocument: %v", doc, err)
		}
		for _, f := range []*fixtureStoredField{
			{"id", document.NewStoredValueString(fmt.Sprintf("doc-%d", doc))},
			{"body", document.NewStoredValueString(fmt.Sprintf("lucene document number %d with some words for testing postings", doc))},
			{"tag", document.NewStoredValueString(fmt.Sprintf("tag-%d", doc%5))},
			{"int_point_stored", document.NewStoredValueInt(int32(doc))},
		} {
			info := fis.FieldInfoByName(f.name)
			if info == nil {
				t.Fatalf("doc %d: field %q is absent from the fixture's .fnm", doc, f.name)
			}
			if err := writer.WriteField(info, f); err != nil {
				t.Fatalf("doc %d field %q (number %d): WriteField: %v", doc, f.name, info.Number(), err)
			}
		}
		if err := writer.FinishDocument(); err != nil {
			t.Fatalf("doc %d: FinishDocument: %v", doc, err)
		}
	}
	if err := writer.Finish(fixtureNumDocs); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for _, name := range []string{"_0.fdt", "_0.fdx", "_0.fdm"} {
		requireIdenticalBytes(t, name, filepath.Join(luceneDir, name), filepath.Join(goceneDir, name))
	}
}

// requireIdenticalBytes fails with both hexdumps around the first difference.
func requireIdenticalBytes(t *testing.T, label, lucenePath, gocenePath string) {
	t.Helper()
	want, err := os.ReadFile(lucenePath)
	if err != nil {
		t.Fatalf("%s: read Lucene side: %v", label, err)
	}
	got, err := os.ReadFile(gocenePath)
	if err != nil {
		t.Fatalf("%s: read Gocene side: %v", label, err)
	}
	first := -1
	for i := 0; i < len(want) && i < len(got); i++ {
		if want[i] != got[i] {
			first = i
			break
		}
	}
	if first < 0 && len(want) == len(got) {
		return
	}
	if first < 0 {
		first = min(len(want), len(got))
	}
	var differing []int
	for i := 0; i < min(len(want), len(got)); i++ {
		if want[i] != got[i] {
			differing = append(differing, i)
		}
	}
	lo := max(first-32, 0)
	hi := first + 48
	t.Errorf("%s is not byte-identical to Lucene 10.4.0: Lucene %d bytes, Gocene %d bytes, "+
		"%d differing byte positions, first at offset %d (0x%x)\n"+
		"  differing offsets: %v\n"+
		"  LUCENE 10.4.0 %s [0x%x..]:\n%s"+
		"  GOCENE        %s [0x%x..]:\n%s",
		label, len(want), len(got), len(differing), first, first, differing,
		label, lo, hexDump(want[lo:min(hi, len(want))], lo),
		label, lo, hexDump(got[lo:min(hi, len(got))], lo))
}

func hexDump(b []byte, base int) string {
	out := ""
	for i := 0; i < len(b); i += 16 {
		end := min(i+16, len(b))
		out += fmt.Sprintf("    %08x  ", base+i)
		for j := i; j < i+16; j++ {
			if j < len(b) {
				out += fmt.Sprintf("%02x ", b[j])
			} else {
				out += "   "
			}
			if j == i+7 {
				out += " "
			}
		}
		out += " |"
		for j := i; j < end; j++ {
			c := b[j]
			if c < 0x20 || c > 0x7e {
				c = '.'
			}
			out += string(c)
		}
		out += "|\n"
	}
	return out
}
