// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Regression tests for the byte order of the codec header and footer.
//
// Background. Lucene's DataOutput.writeInt / writeLong are LITTLE-endian
// (DataOutput.java:73,223), but every fixed-width field of a codec HEADER and
// of a codec FOOTER goes through CodecUtil.writeBEInt / writeBELong and is
// therefore BIG-endian (CodecUtil.java:83,85,308,310,410,411,653,661).
//
// store.WriteHeader / WriteFooter previously used the little-endian
// store.WriteInt32 / WriteInt64 helpers, so Gocene emitted 17 6c d7 3f where
// Apache Lucene emits 3f d7 6c 17, and symmetrically rejected every file
// Lucene had written. These tests pin the correct order against files that
// Apache Lucene itself produced, under testdata/lucene-10.4.0-fixtures/, so
// that the defect cannot come back unnoticed.
package store_test

import (
	"bytes"
	"encoding/binary"
	"hash"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// fixtureDir holds index files written by Apache Lucene itself.
const fixtureDir = "../testdata/lucene-10.4.0-fixtures"

// luceneFixture describes one Lucene-written file and the index header it
// carries. The codec name, the version and the suffix are the values Apache
// Lucene recorded; the segment id is read back out of the file itself.
type luceneFixture struct {
	file    string
	codec   string
	version int32
	suffix  string
}

func luceneFixtures() []luceneFixture {
	return []luceneFixture{
		{file: "_0.si", codec: "Lucene90SegmentInfo", version: 0, suffix: ""},
		{file: "segments_1", codec: "segments", version: 10, suffix: "1"},
		{file: "_0.cfe", codec: "Lucene90CompoundEntries", version: 0, suffix: ""},
		{file: "_0.cfs", codec: "Lucene90CompoundData", version: 0, suffix: ""},
	}
}

// readFixture returns the complete bytes of a Lucene-written fixture.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixtureDir, name))
	if err != nil {
		t.Fatalf("reading Lucene fixture %s: %v", name, err)
	}
	return b
}

// fixtureID extracts the 16-byte object id straight out of a Lucene index
// header: magic (4) + vInt name length (1, every fixture name is < 128 bytes)
// + name + version (4).
func fixtureID(t *testing.T, all []byte, codec string) []byte {
	t.Helper()
	off := 4 + 1 + len(codec) + 4
	if len(all) < off+16 {
		t.Fatalf("fixture too short for an index header of codec %q", codec)
	}
	return append([]byte(nil), all[off:off+16]...)
}

// openBytes exposes b as an IndexInput. The bytes are staged through a
// ByteBuffersDirectory so the input is built by the directory's normal path.
func openBytes(t *testing.T, name string, b []byte) store.IndexInput {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	out, err := dir.CreateOutput(name, store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput(%s): %v", name, err)
	}
	if err := out.WriteBytes(b, 0, len(b)); err != nil {
		t.Fatalf("WriteBytes(%s): %v", name, err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close(%s): %v", name, err)
	}
	in, err := dir.OpenInput(name, store.IOContext{})
	if err != nil {
		t.Fatalf("OpenInput(%s): %v", name, err)
	}
	t.Cleanup(func() { _ = in.Close() })
	return in
}

// recordingOutput is a minimal IndexOutput that keeps every byte written and
// exposes the CRC-32 of all of them, which is the contract of
// org.apache.lucene.store.IndexOutput#getChecksum() that store.WriteCRC needs.
//
// The test defines its own rather than using one of the store outputs because
// every shipped IndexOutput is currently unusable for a codec header:
// SimpleFSIndexOutput.WriteString, NIOFSIndexOutput.WriteString and
// OutputStreamIndexOutput.WriteString each call store.WriteString(out, s),
// which calls out.WriteString(s) straight back and recurses until the stack
// overflows; ByteBuffersIndexOutput.WriteString dereferences an uninitialised
// BaseDataOutput. Those defects are unrelated to byte order and are tracked
// separately; this type keeps the byte-order regression independent of them.
type recordingOutput struct {
	*store.BaseDataOutput
	buf    []byte
	digest hash.Hash32
}

func newRecordingOutput() *recordingOutput {
	o := &recordingOutput{digest: crc32.NewIEEE()}
	o.BaseDataOutput = store.NewBaseDataOutput(o)
	return o
}

func (o *recordingOutput) WriteByte(b byte) error {
	o.buf = append(o.buf, b)
	_, _ = o.digest.Write([]byte{b})
	return nil
}

func (o *recordingOutput) WriteBytes(b []byte, offset, length int) error {
	o.buf = append(o.buf, b[offset:offset+length]...)
	_, _ = o.digest.Write(b[offset : offset+length])
	return nil
}

func (o *recordingOutput) GetName() string         { return "recording" }
func (o *recordingOutput) GetFilePointer() int64   { return int64(len(o.buf)) }
func (o *recordingOutput) Length() int64           { return int64(len(o.buf)) }
func (o *recordingOutput) SetPosition(int64) error { return nil }
func (o *recordingOutput) Close() error            { return nil }
func (o *recordingOutput) GetChecksum() uint32     { return o.digest.Sum32() }

var _ store.IndexOutput = (*recordingOutput)(nil)

// TestCodecMagicIsBigEndianOnDisk pins the four bytes every Apache Lucene
// index file starts with, and the four bytes every codec footer starts with.
// CODEC_MAGIC is 0x3FD76C17 and FOOTER_MAGIC is its bitwise complement; written
// big-endian they are 3f d7 6c 17 and c0 28 93 e8. A little-endian writer would
// emit them reversed.
func TestCodecMagicIsBigEndianOnDisk(t *testing.T) {
	wantHeaderMagic := []byte{0x3f, 0xd7, 0x6c, 0x17}
	wantFooterMagic := []byte{0xc0, 0x28, 0x93, 0xe8}

	for _, f := range luceneFixtures() {
		all := readFixture(t, f.file)
		if !bytes.Equal(all[:4], wantHeaderMagic) {
			t.Fatalf("%s: Lucene header magic = % x, want % x", f.file, all[:4], wantHeaderMagic)
		}
		footer := all[len(all)-store.FooterLength():]
		if !bytes.Equal(footer[:4], wantFooterMagic) {
			t.Fatalf("%s: Lucene footer magic = % x, want % x", f.file, footer[:4], wantFooterMagic)
		}
	}

	// Gocene must emit exactly the same four bytes.
	out := newRecordingOutput()
	if err := store.WriteBEInt(out, store.CODEC_MAGIC); err != nil {
		t.Fatalf("WriteBEInt(CODEC_MAGIC): %v", err)
	}
	if !bytes.Equal(out.buf, wantHeaderMagic) {
		t.Fatalf("WriteBEInt(CODEC_MAGIC) = % x, want % x", out.buf, wantHeaderMagic)
	}

	out = newRecordingOutput()
	if err := store.WriteBEInt(out, store.FOOTER_MAGIC); err != nil {
		t.Fatalf("WriteBEInt(FOOTER_MAGIC): %v", err)
	}
	if !bytes.Equal(out.buf, wantFooterMagic) {
		t.Fatalf("WriteBEInt(FOOTER_MAGIC) = % x, want % x", out.buf, wantFooterMagic)
	}
}

// TestWriteIndexHeaderMatchesLuceneBytes asserts that WriteHeader and
// WriteIndexHeader reproduce, byte for byte, the header prefix that Apache
// Lucene wrote into each fixture.
func TestWriteIndexHeaderMatchesLuceneBytes(t *testing.T) {
	for _, f := range luceneFixtures() {
		t.Run(f.file, func(t *testing.T) {
			all := readFixture(t, f.file)
			id := fixtureID(t, all, f.codec)

			// The full index header.
			wantIndex := all[:store.IndexHeaderLength(f.codec, f.suffix)]
			out := newRecordingOutput()
			if err := store.WriteIndexHeader(out, f.codec, f.version, id, f.suffix); err != nil {
				t.Fatalf("WriteIndexHeader: %v", err)
			}
			if !bytes.Equal(out.buf, wantIndex) {
				t.Fatalf("WriteIndexHeader:\n gocene = % x\n lucene = % x", out.buf, wantIndex)
			}

			// Its codec-header prefix, written on its own.
			wantHeader := all[:store.HeaderLength(f.codec)]
			out = newRecordingOutput()
			if err := store.WriteHeader(out, f.codec, f.version); err != nil {
				t.Fatalf("WriteHeader: %v", err)
			}
			if !bytes.Equal(out.buf, wantHeader) {
				t.Fatalf("WriteHeader:\n gocene = % x\n lucene = % x", out.buf, wantHeader)
			}
		})
	}
}

// TestCheckIndexHeaderAcceptsLuceneFiles asserts that Gocene reads the headers
// Apache Lucene wrote, recovering the same version, id and suffix. Before the
// byte-order repair every one of these failed with a magic mismatch.
func TestCheckIndexHeaderAcceptsLuceneFiles(t *testing.T) {
	for _, f := range luceneFixtures() {
		t.Run(f.file, func(t *testing.T) {
			all := readFixture(t, f.file)
			id := fixtureID(t, all, f.codec)

			in := openBytes(t, f.file, all)
			version, err := store.CheckIndexHeader(in, f.codec, f.version, f.version, id, f.suffix)
			if err != nil {
				t.Fatalf("CheckIndexHeader on a Lucene-written file: %v", err)
			}
			if version != f.version {
				t.Fatalf("CheckIndexHeader version = %d, want %d", version, f.version)
			}
			if got := in.GetFilePointer(); got != int64(store.IndexHeaderLength(f.codec, f.suffix)) {
				t.Fatalf("file pointer after CheckIndexHeader = %d, want %d",
					got, store.IndexHeaderLength(f.codec, f.suffix))
			}

			// CheckHeader alone must consume exactly HeaderLength bytes.
			in2 := openBytes(t, f.file+".hdr", all)
			if _, err := store.CheckHeader(in2, f.codec, f.version, f.version); err != nil {
				t.Fatalf("CheckHeader on a Lucene-written file: %v", err)
			}
			if got := in2.GetFilePointer(); got != int64(store.HeaderLength(f.codec)) {
				t.Fatalf("file pointer after CheckHeader = %d, want %d", got, store.HeaderLength(f.codec))
			}
		})
	}
}

// TestCheckHeaderRejectsLittleEndianMagic is the inverse guard: a file whose
// magic was written little-endian — which is precisely what Gocene used to
// emit — must be rejected.
func TestCheckHeaderRejectsLittleEndianMagic(t *testing.T) {
	all := readFixture(t, "_0.si")
	swapped := append([]byte(nil), all...)
	swapped[0], swapped[1], swapped[2], swapped[3] = all[3], all[2], all[1], all[0]

	in := openBytes(t, "swapped.si", swapped)
	if _, err := store.CheckHeader(in, "Lucene90SegmentInfo", 0, 0); err == nil {
		t.Fatal("CheckHeader accepted a little-endian magic; it must reject it")
	}
}

// TestFooterChecksumIsBigEndianCRC32 pins the footer contract Apache Lucene
// actually wrote: FOOTER_MAGIC and the algorithm id as big-endian int32, then
// the CRC-32 of every preceding byte as a big-endian int64 whose upper four
// bytes are zero.
func TestFooterChecksumIsBigEndianCRC32(t *testing.T) {
	for _, f := range luceneFixtures() {
		t.Run(f.file, func(t *testing.T) {
			all := readFixture(t, f.file)
			footer := all[len(all)-store.FooterLength():]

			if got := binary.BigEndian.Uint32(footer[4:8]); got != 0 {
				t.Fatalf("algorithm id = %d, want 0", got)
			}
			stored := binary.BigEndian.Uint64(footer[8:16])
			if stored>>32 != 0 {
				t.Fatalf("checksum %#016x has bits set above 32; the footer long is not big-endian", stored)
			}
			computed := crc32.ChecksumIEEE(all[:len(all)-8])
			if stored != uint64(computed) {
				t.Fatalf("stored checksum %#016x != CRC-32 of the file %#08x", stored, computed)
			}

			// Gocene must read back exactly that value.
			in := openBytes(t, f.file, all)
			got, err := store.RetrieveChecksum(in)
			if err != nil {
				t.Fatalf("RetrieveChecksum: %v", err)
			}
			if uint64(got) != stored {
				t.Fatalf("RetrieveChecksum = %#x, want %#x", got, stored)
			}

			in2 := openBytes(t, f.file+".all", all)
			sum, err := store.ChecksumEntireFile(in2)
			if err != nil {
				t.Fatalf("ChecksumEntireFile: %v", err)
			}
			if uint64(sum) != stored {
				t.Fatalf("ChecksumEntireFile = %#x, want %#x", sum, stored)
			}

			in3 := openBytes(t, f.file+".footer", all)
			raw, err := store.ReadFooter(in3)
			if err != nil {
				t.Fatalf("ReadFooter: %v", err)
			}
			if !bytes.Equal(raw, footer) {
				t.Fatalf("ReadFooter = % x, want % x", raw, footer)
			}
		})
	}
}

// TestCheckFooterAcceptsLuceneFooter streams every byte of a Lucene-written
// file through a ChecksumIndexInput and then validates the trailing footer,
// which is exactly what Lucene's CodecUtil.checkFooter does.
func TestCheckFooterAcceptsLuceneFooter(t *testing.T) {
	for _, f := range luceneFixtures() {
		t.Run(f.file, func(t *testing.T) {
			all := readFixture(t, f.file)
			in := openBytes(t, f.file, all)
			cin := store.NewChecksumIndexInput(in)
			if err := cin.SetPosition(cin.Length() - int64(store.FooterLength())); err != nil {
				t.Fatalf("SetPosition: %v", err)
			}
			sum, err := store.CheckFooter(cin)
			if err != nil {
				t.Fatalf("CheckFooter on a Lucene-written footer: %v", err)
			}
			want := int64(binary.BigEndian.Uint64(all[len(all)-8:]))
			if sum != want {
				t.Fatalf("CheckFooter = %#x, want %#x", sum, want)
			}
		})
	}
}

// TestWriteFooterRoundTrip writes a complete codec envelope and asserts three
// things: the header prefix equals the one Apache Lucene wrote for the same
// codec name and version; the footer carries the CRC-32 that Lucene itself
// would compute over the file; and Gocene reads that file back and validates
// the checksum.
func TestWriteFooterRoundTrip(t *testing.T) {
	const codec = "Lucene90SegmentInfo"
	si := readFixture(t, "_0.si")
	id := fixtureID(t, si, codec)

	out := newRecordingOutput()
	if err := store.WriteIndexHeader(out, codec, 0, id, ""); err != nil {
		t.Fatalf("WriteIndexHeader: %v", err)
	}
	if err := out.WriteVInt(12345); err != nil {
		t.Fatalf("WriteVInt: %v", err)
	}
	if err := store.WriteFooter(out); err != nil {
		t.Fatalf("WriteFooter: %v", err)
	}
	file := out.buf

	hdrLen := store.IndexHeaderLength(codec, "")
	if !bytes.Equal(file[:hdrLen], si[:hdrLen]) {
		t.Fatalf("header prefix:\n gocene = % x\n lucene = % x", file[:hdrLen], si[:hdrLen])
	}

	footer := file[len(file)-store.FooterLength():]
	if !bytes.Equal(footer[:4], []byte{0xc0, 0x28, 0x93, 0xe8}) {
		t.Fatalf("footer magic = % x, want c0 28 93 e8", footer[:4])
	}
	if got := binary.BigEndian.Uint32(footer[4:8]); got != 0 {
		t.Fatalf("algorithm id = %d, want 0", got)
	}
	stored := binary.BigEndian.Uint64(footer[8:16])
	computed := crc32.ChecksumIEEE(file[:len(file)-8])
	if stored != uint64(computed) {
		t.Fatalf("footer checksum %#016x != CRC-32 that Lucene would compute %#08x", stored, computed)
	}

	in := openBytes(t, "roundtrip.bin", file)
	sum, err := store.ChecksumEntireFile(in)
	if err != nil {
		t.Fatalf("ChecksumEntireFile on our own file: %v", err)
	}
	if uint64(sum) != uint64(computed) {
		t.Fatalf("ChecksumEntireFile = %#x, want %#x", sum, computed)
	}
}

// TestVerifyAndCopyIndexHeader asserts that the compound-file path reproduces a
// Lucene-written index header verbatim.
func TestVerifyAndCopyIndexHeader(t *testing.T) {
	const codec = "Lucene90CompoundData"
	all := readFixture(t, "_0.cfs")
	id := fixtureID(t, all, codec)

	in := openBytes(t, "_0.cfs", all)
	out := newRecordingOutput()
	if err := store.VerifyAndCopyIndexHeader(in, out, id); err != nil {
		t.Fatalf("VerifyAndCopyIndexHeader: %v", err)
	}
	want := all[:store.IndexHeaderLength(codec, "")]
	if !bytes.Equal(out.buf, want) {
		t.Fatalf("copied header:\n gocene = % x\n lucene = % x", out.buf, want)
	}
}

// TestReadIndexHeader asserts that ReadIndexHeader returns the exact header
// bytes of a Lucene-written file.
func TestReadIndexHeader(t *testing.T) {
	const codec = "Lucene90SegmentInfo"
	all := readFixture(t, "_0.si")
	in := openBytes(t, "_0.si", all)
	got, err := store.ReadIndexHeader(in)
	if err != nil {
		t.Fatalf("ReadIndexHeader: %v", err)
	}
	want := all[:store.IndexHeaderLength(codec, "")]
	if !bytes.Equal(got, want) {
		t.Fatalf("ReadIndexHeader:\n got  = % x\n want = % x", got, want)
	}
}

// TestBigEndianPrimitives pins WriteBEInt / WriteBELong against fixed bytes and
// asserts that ReadBEInt / ReadBELong invert them. A value with a high bit set
// catches a sign-extension mistake, which a symmetric round-trip alone would
// not.
func TestBigEndianPrimitives(t *testing.T) {
	intCases := []struct {
		v    int32
		want []byte
	}{
		{0x01020304, []byte{0x01, 0x02, 0x03, 0x04}},
		{store.CODEC_MAGIC, []byte{0x3f, 0xd7, 0x6c, 0x17}},
		{store.FOOTER_MAGIC, []byte{0xc0, 0x28, 0x93, 0xe8}},
		{-1, []byte{0xff, 0xff, 0xff, 0xff}},
	}
	for _, c := range intCases {
		out := newRecordingOutput()
		if err := store.WriteBEInt(out, c.v); err != nil {
			t.Fatalf("WriteBEInt(%#x): %v", c.v, err)
		}
		if !bytes.Equal(out.buf, c.want) {
			t.Fatalf("WriteBEInt(%#x) = % x, want % x", c.v, out.buf, c.want)
		}
		in := openBytes(t, "be-int", out.buf)
		got, err := store.ReadBEInt(in)
		if err != nil {
			t.Fatalf("ReadBEInt: %v", err)
		}
		if got != c.v {
			t.Fatalf("ReadBEInt = %#x, want %#x", got, c.v)
		}
	}

	longCases := []struct {
		v    int64
		want []byte
	}{
		{0x0102030405060708, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}},
		{0x6c494455, []byte{0x00, 0x00, 0x00, 0x00, 0x6c, 0x49, 0x44, 0x55}},
		{-1, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
	}
	for _, c := range longCases {
		out := newRecordingOutput()
		if err := store.WriteBELong(out, c.v); err != nil {
			t.Fatalf("WriteBELong(%#x): %v", c.v, err)
		}
		if !bytes.Equal(out.buf, c.want) {
			t.Fatalf("WriteBELong(%#x) = % x, want % x", c.v, out.buf, c.want)
		}
		in := openBytes(t, "be-long", out.buf)
		got, err := store.ReadBELong(in)
		if err != nil {
			t.Fatalf("ReadBELong: %v", err)
		}
		if got != c.v {
			t.Fatalf("ReadBELong = %#x, want %#x", got, c.v)
		}
	}
}

// TestPlainIntHelpersStayLittleEndian guards the other half of the contract:
// store.WriteInt32 / WriteInt64 are DataOutput.writeInt / writeLong, which
// Lucene defines as LITTLE-endian (DataOutput.java:73,223). They must not be
// "fixed" into big-endian along with the header and footer.
func TestPlainIntHelpersStayLittleEndian(t *testing.T) {
	out := newRecordingOutput()
	if err := store.WriteInt32(out, 0x01020304); err != nil {
		t.Fatalf("WriteInt32: %v", err)
	}
	if want := []byte{0x04, 0x03, 0x02, 0x01}; !bytes.Equal(out.buf, want) {
		t.Fatalf("WriteInt32 = % x, want % x (little-endian)", out.buf, want)
	}

	out = newRecordingOutput()
	if err := store.WriteInt64(out, 0x0102030405060708); err != nil {
		t.Fatalf("WriteInt64: %v", err)
	}
	want := []byte{0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01}
	if !bytes.Equal(out.buf, want) {
		t.Fatalf("WriteInt64 = % x, want % x (little-endian)", out.buf, want)
	}
}

// TestHeaderLengthsMatchLuceneLayout asserts the length helpers agree with the
// real offsets in the Lucene-written files: the byte after IndexHeaderLength is
// where the codec's own payload begins, and FooterLength is 16.
func TestHeaderLengthsMatchLuceneLayout(t *testing.T) {
	if store.FooterLength() != 16 {
		t.Fatalf("FooterLength = %d, want 16", store.FooterLength())
	}
	for _, f := range luceneFixtures() {
		all := readFixture(t, f.file)
		// HeaderLength = magic(4) + vInt len(1) + name + version(4).
		wantHeader := 4 + 1 + len(f.codec) + 4
		if got := store.HeaderLength(f.codec); got != wantHeader {
			t.Fatalf("%s: HeaderLength = %d, want %d", f.file, got, wantHeader)
		}
		// The vInt-encoded name length must really be one byte holding len(name).
		if int(all[4]) != len(f.codec) {
			t.Fatalf("%s: name length byte = %d, want %d", f.file, all[4], len(f.codec))
		}
		// IndexHeaderLength = HeaderLength + id(16) + suffix length byte + suffix.
		wantIndex := wantHeader + 16 + 1 + len(f.suffix)
		if got := store.IndexHeaderLength(f.codec, f.suffix); got != wantIndex {
			t.Fatalf("%s: IndexHeaderLength = %d, want %d", f.file, got, wantIndex)
		}
		if int(all[wantHeader+16]) != len(f.suffix) {
			t.Fatalf("%s: suffix length byte = %d, want %d", f.file, all[wantHeader+16], len(f.suffix))
		}
	}
}
