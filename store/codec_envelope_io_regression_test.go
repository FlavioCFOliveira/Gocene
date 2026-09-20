// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spi"
)

// This file pins the I/O defects that stopped Gocene from opening a file
// Apache Lucene wrote, even after the codec framing itself became byte-faithful.
//
// Every case below is driven off testdata/lucene-10.4.0-fixtures, which holds
// files produced by Apache Lucene itself. The defects were:
//
//	A  SimpleFSIndexInput/NIOFSIndexInput/MMapIndexInput.ReadString recursed
//	   into the free store.ReadString helper, which called them straight back
//	   (stack overflow on the first string of any codec header).
//	B  SimpleFSIndexOutput/NIOFSIndexOutput/OutputStreamIndexOutput.WriteString
//	   had the same shape on the write side.
//	C  ByteBuffersIndexOutput.WriteString used the promoted BaseDataOutput.
//	   WriteVInt whose primitive writer was never wired (nil dereference).
//	F  BufferedChecksumIndexInput.ReadString delegated to the wrapped input, so
//	   the string's bytes never reached the digest.
//	H  PagedBytesDataInput.ReadString read a fixed int length where Java reads a
//	   vInt, and PagedBytesDataOutput.WriteShort/WriteBytes were wrong.
//
// The governing fact: in Java readString/writeString are concrete methods on
// DataInput/DataOutput built from readVInt+readBytes / writeVInt+writeBytes, so
// every subclass inherits them and the checksum and file-pointer bookkeeping
// happen through the primitives that subclass does override.

const fixtureDir = "../testdata/lucene-10.4.0-fixtures"

// luceneFixtures lists the Apache Lucene-written files under testdata together
// with the codec header each one carries.
var luceneFixtures = []struct {
	file     string
	codec    string
	min, max int32
	suffix   string
}{
	{"_0.si", "Lucene90SegmentInfo", 0, 2, ""},
	// SegmentInfos.readCommit passes the generation in base 36 as the suffix.
	{"segments_1", "segments", 0, 100, "1"},
	{"_0.cfe", "Lucene90CompoundEntries", 0, 1, ""},
	{"_0.cfs", "Lucene90CompoundData", 0, 1, ""},
}

// directoryImpls is the set of on-disk Directory implementations that must all
// be able to read a Lucene-written file.
var directoryImpls = []struct {
	name string
	open func(string) (Directory, error)
}{
	{"SimpleFSDirectory", func(p string) (Directory, error) { return NewSimpleFSDirectory(p) }},
	{"NIOFSDirectory", func(p string) (Directory, error) { return NewNIOFSDirectory(p) }},
	{"MMapDirectory", func(p string) (Directory, error) { return NewMMapDirectory(p) }},
}

// TestDirectoryReadsLuceneCodecHeader is the regression test for defect A. Each
// of the three on-disk directories must read the codec header of a real Lucene
// file off disk. Before the fix ReadString recursed until the goroutine stack
// was exhausted, which kills the process rather than failing the test, so a
// regression here shows up as `fatal error: stack overflow`.
func TestDirectoryReadsLuceneCodecHeader(t *testing.T) {
	for _, d := range directoryImpls {
		t.Run(d.name, func(t *testing.T) {
			dir, err := d.open(fixtureDir)
			if err != nil {
				t.Fatalf("open directory: %v", err)
			}
			defer dir.Close()

			for _, f := range luceneFixtures {
				t.Run(f.file, func(t *testing.T) {
					in, err := dir.OpenInput(f.file, IOContextDefault)
					if err != nil {
						t.Fatalf("OpenInput: %v", err)
					}
					defer in.Close()

					version, err := CheckIndexHeader(in, f.codec, f.min, f.max, nil, f.suffix)
					if err != nil {
						t.Fatalf("CheckIndexHeader: %v", err)
					}
					if version < f.min || version > f.max {
						t.Fatalf("version %d outside [%d, %d]", version, f.min, f.max)
					}
				})
			}
		})
	}
}

// TestStreamedFooterValidatesAgainstLucene is the regression test for the read
// leg of defects E and F: a streamed checksummed read of a Lucene file must end
// with a footer whose recorded CRC-32 equals the digest the reader accumulated.
// The digest is also checked against an independent CRC-32 over the file bytes,
// so the test cannot pass by two matching wrong numbers.
func TestStreamedFooterValidatesAgainstLucene(t *testing.T) {
	for _, d := range directoryImpls {
		t.Run(d.name, func(t *testing.T) {
			dir, err := d.open(fixtureDir)
			if err != nil {
				t.Fatalf("open directory: %v", err)
			}
			defer dir.Close()

			for _, f := range luceneFixtures {
				t.Run(f.file, func(t *testing.T) {
					raw, err := os.ReadFile(filepath.Join(fixtureDir, f.file))
					if err != nil {
						t.Fatalf("read fixture: %v", err)
					}
					// CodecUtil.writeFooter emits magic(4) + algo(4) + crc(8);
					// the CRC covers everything before those last 8 bytes.
					independent := crc32.ChecksumIEEE(raw[:len(raw)-8])

					in, err := dir.OpenInput(f.file, IOContextDefault)
					if err != nil {
						t.Fatalf("OpenInput: %v", err)
					}
					defer in.Close()

					ck := spi.NewChecksumIndexInput(in)
					if _, err := CheckIndexHeader(ck, f.codec, f.min, f.max, nil, f.suffix); err != nil {
						t.Fatalf("CheckIndexHeader: %v", err)
					}
					if err := ck.SetPosition(in.Length() - 16); err != nil {
						t.Fatalf("stream to footer: %v", err)
					}
					got, err := CheckFooter(ck)
					if err != nil {
						t.Fatalf("CheckFooter: %v", err)
					}
					if uint32(got) != independent {
						t.Fatalf("footer CRC 0x%08x != independent CRC-32 0x%08x", uint32(got), independent)
					}
				})
			}
		})
	}
}

// parseLuceneHeader recovers the codec name, version, id and suffix that Lucene
// stamped on a file so the write leg can target exactly the same parameters.
func parseLuceneHeader(t *testing.T, raw []byte) (codec string, version int32, id []byte, suffix string, headerLen int) {
	t.Helper()
	p := 4 // big-endian CODEC_MAGIC
	n := int(raw[p])
	p++
	codec = string(raw[p : p+n])
	p += n
	version = int32(binary.BigEndian.Uint32(raw[p : p+4]))
	p += 4
	id = append([]byte(nil), raw[p:p+16]...)
	p += 16
	sl := int(raw[p])
	p++
	suffix = string(raw[p : p+sl])
	p += sl
	return codec, version, id, suffix, p
}

// TestWrittenEnvelopeIsByteIdenticalToLucene is the regression test for defects
// B, C and D on the write side. It reframes each Lucene fixture through the full
// Gocene stack — Directory.CreateOutput, WriteIndexHeader, the payload,
// WriteFooter — and requires the result to be byte-identical to the file Lucene
// wrote. Defect D alone (the checksum skipping the codec-name string) changes
// the eight footer bytes, so this fails if any of them regresses.
func TestWrittenEnvelopeIsByteIdenticalToLucene(t *testing.T) {
	for _, d := range directoryImpls {
		t.Run(d.name, func(t *testing.T) {
			tmp := t.TempDir()
			dir, err := d.open(tmp)
			if err != nil {
				t.Fatalf("open directory: %v", err)
			}
			defer dir.Close()

			for _, f := range luceneFixtures {
				t.Run(f.file, func(t *testing.T) {
					ref, err := os.ReadFile(filepath.Join(fixtureDir, f.file))
					if err != nil {
						t.Fatalf("read fixture: %v", err)
					}
					codec, version, id, suffix, headerLen := parseLuceneHeader(t, ref)
					payload := ref[headerLen : len(ref)-16]

					name := "written_" + f.file
					raw, err := dir.CreateOutput(name, IOContextDefault)
					if err != nil {
						t.Fatalf("CreateOutput: %v", err)
					}
					ck := spi.NewChecksumIndexOutput(raw)
					if err := WriteIndexHeader(ck, codec, version, id, suffix); err != nil {
						t.Fatalf("WriteIndexHeader: %v", err)
					}
					if err := ck.WriteBytes(payload, 0, len(payload)); err != nil {
						t.Fatalf("write payload: %v", err)
					}
					if err := spi.WriteFooter(ck); err != nil {
						t.Fatalf("WriteFooter: %v", err)
					}
					if err := raw.Close(); err != nil {
						t.Fatalf("close: %v", err)
					}

					got, err := os.ReadFile(filepath.Join(tmp, name))
					if err != nil {
						t.Fatalf("read back: %v", err)
					}
					if !bytes.Equal(got, ref) {
						at := -1
						for i := 0; i < len(got) && i < len(ref); i++ {
							if got[i] != ref[i] {
								at = i
								break
							}
						}
						t.Fatalf("not byte-identical to Lucene: len got=%d want=%d first difference at %d",
							len(got), len(ref), at)
					}

					// And the file we just wrote must reopen and revalidate.
					back, err := dir.OpenInput(name, IOContextDefault)
					if err != nil {
						t.Fatalf("reopen: %v", err)
					}
					defer back.Close()
					cin := spi.NewChecksumIndexInput(back)
					if _, err := CheckIndexHeader(cin, codec, version, version, id, suffix); err != nil {
						t.Fatalf("reopen CheckIndexHeader: %v", err)
					}
					if err := cin.SetPosition(back.Length() - 16); err != nil {
						t.Fatalf("reopen stream to footer: %v", err)
					}
					if _, err := CheckFooter(cin); err != nil {
						t.Fatalf("reopen CheckFooter: %v", err)
					}
				})
			}
		})
	}
}

// TestBufferedChecksumIndexInputDigestsStrings is the regression test for defect
// F. BufferedChecksumIndexInput used to hand ReadString, ReadInts, ReadFloats,
// ReadMapOfStrings and ReadSetOfStrings to the wrapped input, so those bytes
// never entered the digest. Java overrides only readByte, readBytes, readShort,
// readInt, readLong and readLongs; everything else is inherited and therefore
// runs through those primitives.
func TestBufferedChecksumIndexInputDigestsStrings(t *testing.T) {
	tmp := t.TempDir()
	dir, err := NewSimpleFSDirectory(tmp)
	if err != nil {
		t.Fatalf("open directory: %v", err)
	}
	defer dir.Close()

	raw, err := dir.CreateOutput("strings.bin", IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	out := spi.NewChecksumIndexOutput(raw)
	if err := out.WriteString("a string that must enter the digest"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := out.WriteMapOfStrings(map[string]string{"key": "value"}); err != nil {
		t.Fatalf("WriteMapOfStrings: %v", err)
	}
	if err := out.WriteSetOfStrings([]string{"one", "two"}); err != nil {
		t.Fatalf("WriteSetOfStrings: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	onDisk, err := os.ReadFile(filepath.Join(tmp, "strings.bin"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	in, err := dir.OpenInput("strings.bin", IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	bc := NewBufferedChecksumIndexInput(in)
	if _, err := bc.ReadString(); err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if _, err := bc.ReadMapOfStrings(); err != nil {
		t.Fatalf("ReadMapOfStrings: %v", err)
	}
	if _, err := bc.ReadSetOfStrings(); err != nil {
		t.Fatalf("ReadSetOfStrings: %v", err)
	}

	fp := bc.GetFilePointer()
	if fp != int64(len(onDisk)) {
		t.Fatalf("file pointer %d did not advance over the whole file (%d bytes)", fp, len(onDisk))
	}
	want := crc32.ChecksumIEEE(onDisk[:fp])
	if bc.GetChecksum() != want {
		t.Fatalf("digest 0x%08x != CRC-32 over the bytes actually read 0x%08x", bc.GetChecksum(), want)
	}
}

// TestBufferedChecksumIndexInputRejectsBackwardSeek pins the ChecksumIndexInput
// seek contract (ChecksumIndexInput.java:56-64): a backward seek must be
// refused, never silently served, because it would invalidate the digest.
func TestBufferedChecksumIndexInputRejectsBackwardSeek(t *testing.T) {
	dir, err := NewSimpleFSDirectory(fixtureDir)
	if err != nil {
		t.Fatalf("open directory: %v", err)
	}
	defer dir.Close()
	in, err := dir.OpenInput("_0.si", IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	bc := NewBufferedChecksumIndexInput(in)
	if err := bc.SetPosition(32); err != nil {
		t.Fatalf("forward seek must be allowed: %v", err)
	}
	before := bc.GetChecksum()
	if err := bc.SetPosition(8); err == nil {
		t.Fatal("backward seek was accepted; Lucene throws IllegalStateException")
	}
	if bc.GetChecksum() != before {
		t.Fatalf("refused seek disturbed the digest: 0x%08x -> 0x%08x", before, bc.GetChecksum())
	}
	if bc.GetFilePointer() != 32 {
		t.Fatalf("refused seek moved the file pointer to %d", bc.GetFilePointer())
	}
	if err := bc.SetPosition(32); err != nil {
		t.Fatalf("seek to the current position must be a no-op: %v", err)
	}
}

// TestByteBuffersIndexOutputWriteString is the regression test for defect C: the
// promoted BaseDataOutput had no primitive writer wired, so WriteVInt — and
// therefore WriteString and every other derived writer — dereferenced nil.
func TestByteBuffersIndexOutputWriteString(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("bb.bin", IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	const s = "hello ByteBuffers"
	if err := out.WriteString(s); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	// vInt length (1 byte for 17) + the UTF-8 bytes.
	if got, want := out.GetFilePointer(), int64(1+len(s)); got != want {
		t.Fatalf("file pointer %d, want %d", got, want)
	}
	if err := out.WriteMapOfStrings(map[string]string{"a": "b"}); err != nil {
		t.Fatalf("WriteMapOfStrings: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	in, err := dir.OpenInput("bb.bin", IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()
	back, err := in.ReadString()
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if back != s {
		t.Fatalf("round trip gave %q, want %q", back, s)
	}
	m, err := in.ReadMapOfStrings()
	if err != nil {
		t.Fatalf("ReadMapOfStrings: %v", err)
	}
	if len(m) != 1 || m["a"] != "b" {
		t.Fatalf("round trip gave %v", m)
	}
}

// TestOutputStreamIndexOutputWriteString is the regression test for defect B on
// OutputStreamIndexOutput, which recursed through the free WriteString helper.
// Java's OutputStreamIndexOutput declares no writeString at all: it inherits
// DataOutput.writeString.
func TestOutputStreamIndexOutputWriteString(t *testing.T) {
	var buf bytes.Buffer
	out := NewOutputStreamIndexOutput("osio", "osio", &buf, 8192)
	const s = "hello OutputStream"
	if err := out.WriteString(s); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if got, want := out.GetFilePointer(), int64(1+len(s)); got != want {
		t.Fatalf("file pointer %d, want %d", got, want)
	}
	if got := buf.Bytes()[0]; got != byte(len(s)) {
		t.Fatalf("length prefix is 0x%02x, want the vInt %d", got, len(s))
	}
	back, err := NewByteArrayDataInput(buf.Bytes()).ReadString()
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if back != s {
		t.Fatalf("round trip gave %q, want %q", back, s)
	}
}

// TestPagedBytesStringAndShortRoundTrip is the regression test for defect H.
// PagedBytesDataInput.ReadString read a fixed 4-byte length where
// DataInput.readString reads a vInt, and PagedBytesDataOutput.WriteShort wrote
// big-endian where DataOutput.writeShort writes low byte first. Java's
// PagedBytes overrides only readByte/readBytes and writeByte/writeBytes, so both
// of those come straight from the base class.
func TestPagedBytesStringAndShortRoundTrip(t *testing.T) {
	pb, err := NewPagedBytes(4) // a 16-byte block, so the writes span blocks
	if err != nil {
		t.Fatalf("NewPagedBytes: %v", err)
	}
	out := pb.GetDataOutput()
	const s = "a paged string long enough to cross a block boundary"
	if err := out.WriteString(s); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := out.WriteShort(0x0102); err != nil {
		t.Fatalf("WriteShort: %v", err)
	}
	if _, err := pb.Freeze(true); err != nil {
		t.Fatalf("Freeze: %v", err)
	}

	in, err := pb.GetDataInput()
	if err != nil {
		t.Fatalf("GetDataInput: %v", err)
	}
	back, err := in.ReadString()
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if back != s {
		t.Fatalf("round trip gave %q, want %q", back, s)
	}
	sh, err := in.ReadShort()
	if err != nil {
		t.Fatalf("ReadShort: %v", err)
	}
	if sh != 0x0102 {
		t.Fatalf("short round trip gave 0x%04x, want 0x0102", uint16(sh))
	}
}

// TestPagedBytesWriteShortIsLittleEndian pins the byte order itself, so the
// round trip above cannot pass with both sides wrong in the same direction.
func TestPagedBytesWriteShortIsLittleEndian(t *testing.T) {
	pb, err := NewPagedBytes(8)
	if err != nil {
		t.Fatalf("NewPagedBytes: %v", err)
	}
	if err := pb.GetDataOutput().WriteShort(0x0102); err != nil {
		t.Fatalf("WriteShort: %v", err)
	}
	if _, err := pb.Freeze(true); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	in, err := pb.GetDataInput()
	if err != nil {
		t.Fatalf("GetDataInput: %v", err)
	}
	lo, err := in.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	hi, err := in.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	// DataOutput.writeShort (DataOutput.java:86-89) writes the low byte first.
	if lo != 0x02 || hi != 0x01 {
		t.Fatalf("wrote 0x%02x 0x%02x, want 0x02 0x01 (little-endian)", lo, hi)
	}
}
