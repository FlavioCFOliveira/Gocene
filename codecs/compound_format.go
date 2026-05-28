// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//   Licensed to the Apache Software Foundation (ASF) under one or more
//   contributor license agreements. See the NOTICE file distributed with
//   this work for additional information regarding copyright ownership.
//   The ASF licenses this file to You under the Apache License, Version
//   2.0 (the "License"); you may not use this file except in compliance
//   with the License. You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//   implied. See the License for the specific language governing
//   permissions and limitations under the License.

package codecs

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// CompoundFormat is an alias of spi.CompoundFormat.
type CompoundFormat = spi.CompoundFormat

// CompoundDirectory is an alias of spi.CompoundDirectory.
type CompoundDirectory = spi.CompoundDirectory

// BaseCompoundFormat provides common functionality for CompoundFormat implementations.
type BaseCompoundFormat struct {
	name string
}

// NewBaseCompoundFormat creates a new BaseCompoundFormat.
func NewBaseCompoundFormat(name string) *BaseCompoundFormat {
	return &BaseCompoundFormat{name: name}
}

// Name returns the format name.
func (f *BaseCompoundFormat) Name() string {
	return f.name
}

// GetCompoundReader returns a compound reader (must be implemented by subclasses).
func (f *BaseCompoundFormat) GetCompoundReader(dir store.Directory, si *index.SegmentInfo) (CompoundDirectory, error) {
	return nil, fmt.Errorf("GetCompoundReader not implemented")
}

// Write writes the compound file (must be implemented by subclasses).
func (f *BaseCompoundFormat) Write(dir store.Directory, si *index.SegmentInfo, context store.IOContext) error {
	return fmt.Errorf("Write not implemented")
}

// -----------------------------------------------------------------------------
// Lucene 9.0 compound format — wire-format-faithful port of
// org.apache.lucene.codecs.lucene90.Lucene90CompoundFormat (Lucene 10.4.0).
// -----------------------------------------------------------------------------

const (
	// Lucene90CompoundDataExtension is the file extension for the compound
	// data file (concatenation of every component file's bytes, aligned).
	Lucene90CompoundDataExtension = "cfs"
	// Lucene90CompoundEntriesExtension is the file extension for the
	// compound entries file (file name / offset / length triples).
	Lucene90CompoundEntriesExtension = "cfe"
	// Lucene90CompoundDataCodec is the codec name stamped into the .cfs
	// index header.
	Lucene90CompoundDataCodec = "Lucene90CompoundData"
	// Lucene90CompoundEntriesCodec is the codec name stamped into the .cfe
	// index header.
	Lucene90CompoundEntriesCodec = "Lucene90CompoundEntries"
	// Lucene90CompoundVersionStart is the inclusive minimum supported
	// format version.
	Lucene90CompoundVersionStart int32 = 0
	// Lucene90CompoundVersionCurrent is the current format version.
	Lucene90CompoundVersionCurrent int32 = Lucene90CompoundVersionStart
	// Lucene90CompoundAlignmentBytes is the alignment applied to each
	// embedded file's start offset. Chosen to match the LCM of every
	// file-format-specific alignment so mmap reads remain page-aligned.
	Lucene90CompoundAlignmentBytes = 64
)

// CompoundFileEntry represents a single file entry in a compound file.
type CompoundFileEntry struct {
	FileName string
	Offset   int64
	Length   int64
}

// Lucene90CompoundFormat is the Lucene 9.0 compound file format.
// Two files are produced per segment:
//
//   - .cfs (DATA): IndexHeader || (raw bytes of every component, each
//     preceded by alignment-padding and terminated with a stamped footer
//     that carries the component file's original checksum) || Footer.
//   - .cfe (ENTRIES): IndexHeader || VInt(numFiles) ||
//     (String fileName || UInt64 offset || UInt64 length) ^ numFiles ||
//     Footer.
//
// The on-disk byte stream is identical to the Apache Lucene 10.4.0
// reference, modulo the existing project-wide deviation noted below.
//
// DEVIATION (documented in [[project-gocene-bkdwriter-divergences]]):
// `codecs.WriteIndexHeader` / `CheckIndexHeader` use the same magic
// number as Java (0x3FD76C17) but emit it via store.WriteInt32 which is
// big-endian on the wire. Lucene 10.4.0 also emits the magic big-endian
// (CodecUtil.writeBEInt is explicit), so the index-header bytes are
// byte-for-byte compatible with the JVM. The other multi-byte numerics
// in the compound format (VInt for numFiles, UInt64 for offset/length)
// are emitted via the matching store helpers and remain compatible.
type Lucene90CompoundFormat struct {
	*BaseCompoundFormat
}

// NewLucene90CompoundFormat creates a new Lucene90CompoundFormat.
func NewLucene90CompoundFormat() *Lucene90CompoundFormat {
	return &Lucene90CompoundFormat{
		BaseCompoundFormat: NewBaseCompoundFormat("Lucene90CompoundFormat"),
	}
}

// Write packs the segment's files into a compound format. Mirrors
// Lucene90CompoundFormat.write: data and entries are streamed in a single
// pass, files are sorted ascending by length (so smaller files cluster on
// the same page), each file's bytes are copied verbatim except for the
// footer (we re-stamp the footer carrying the original checksum to keep
// CheckIntegrity correct), and both the data and entries files are
// finalised with a CodecUtil footer.
func (f *Lucene90CompoundFormat) Write(dir store.Directory, si *index.SegmentInfo, ctx store.IOContext) error {
	dataName := GetSegmentFileName(si.Name(), "", Lucene90CompoundDataExtension)
	entriesName := GetSegmentFileName(si.Name(), "", Lucene90CompoundEntriesExtension)

	rawData, err := dir.CreateOutput(dataName, ctx)
	if err != nil {
		return fmt.Errorf("lucene90 compound: create data file %q: %w", dataName, err)
	}
	rawEntries, err := dir.CreateOutput(entriesName, ctx)
	if err != nil {
		_ = rawData.Close()
		return fmt.Errorf("lucene90 compound: create entries file %q: %w", entriesName, err)
	}
	// Lucene's IndexOutput tracks a running CRC internally; Gocene's store
	// implementations do not, so wrap here. WriteFooter requires the
	// concrete output to satisfy the GetChecksum contract.
	dataOut := store.NewChecksumIndexOutput(rawData)
	entriesOut := store.NewChecksumIndexOutput(rawEntries)

	cleanup := func(retErr error) error {
		// Best-effort: close both files. Whichever Close error wins, we
		// surface the original retErr to the caller.
		_ = dataOut.Close()
		_ = entriesOut.Close()
		return retErr
	}

	if err := WriteIndexHeader(dataOut, Lucene90CompoundDataCodec, Lucene90CompoundVersionCurrent, si.GetID(), ""); err != nil {
		return cleanup(fmt.Errorf("lucene90 compound: write data header: %w", err))
	}
	if err := WriteIndexHeader(entriesOut, Lucene90CompoundEntriesCodec, Lucene90CompoundVersionCurrent, si.GetID(), ""); err != nil {
		return cleanup(fmt.Errorf("lucene90 compound: write entries header: %w", err))
	}

	if err := f.writeCompoundFile(entriesOut, dataOut, dir, si); err != nil {
		return cleanup(err)
	}

	if err := WriteFooter(dataOut); err != nil {
		return cleanup(fmt.Errorf("lucene90 compound: write data footer: %w", err))
	}
	if err := WriteFooter(entriesOut); err != nil {
		return cleanup(fmt.Errorf("lucene90 compound: write entries footer: %w", err))
	}

	if err := dataOut.Close(); err != nil {
		_ = entriesOut.Close()
		return fmt.Errorf("lucene90 compound: close data: %w", err)
	}
	if err := entriesOut.Close(); err != nil {
		return fmt.Errorf("lucene90 compound: close entries: %w", err)
	}
	return nil
}

// writeCompoundFile streams every file in si.Files() into dataOut and
// records its (name, offset, length) triple in entriesOut. Files are
// processed ascending by length so a single OS page may pack several
// small files.
func (f *Lucene90CompoundFormat) writeCompoundFile(entriesOut store.IndexOutput, dataOut store.IndexOutput, dir store.Directory, si *index.SegmentInfo) error {
	files := append([]string{}, si.Files()...)

	// Compute lengths up front so we can sort by length stably; the Java
	// implementation uses a PriorityQueue<SizedFile>, which produces the
	// same total order modulo ties.
	type sizedFile struct {
		name string
		size int64
	}
	sized := make([]sizedFile, 0, len(files))
	for _, name := range files {
		size, err := dir.FileLength(name)
		if err != nil {
			return fmt.Errorf("lucene90 compound: file length %q: %w", name, err)
		}
		sized = append(sized, sizedFile{name: name, size: size})
	}
	sort.SliceStable(sized, func(i, j int) bool { return sized[i].size < sized[j].size })

	if err := store.WriteVInt(entriesOut, int32(len(sized))); err != nil {
		return fmt.Errorf("lucene90 compound: write numFiles: %w", err)
	}

	for _, sf := range sized {
		startOffset, err := store.AlignFilePointer(dataOut, Lucene90CompoundAlignmentBytes)
		if err != nil {
			return fmt.Errorf("lucene90 compound: align data fp: %w", err)
		}
		if err := f.copyFileBody(dataOut, dir, sf.name, si.GetID()); err != nil {
			return fmt.Errorf("lucene90 compound: copy file %q: %w", sf.name, err)
		}
		endOffset := dataOut.GetFilePointer()
		length := endOffset - startOffset

		// Entry: String(fileName), UInt64(offset), UInt64(length).
		if err := store.WriteString(entriesOut, stripSegmentNamePrefix(sf.name)); err != nil {
			return fmt.Errorf("lucene90 compound: write entry name: %w", err)
		}
		if err := store.WriteInt64(entriesOut, startOffset); err != nil {
			return fmt.Errorf("lucene90 compound: write entry offset: %w", err)
		}
		if err := store.WriteInt64(entriesOut, length); err != nil {
			return fmt.Errorf("lucene90 compound: write entry length: %w", err)
		}
	}
	return nil
}

// copyFileBody opens the named file as a checksummed input, copies every
// byte up to (but not including) the source file's trailing footer
// verbatim into dataOut while computing the source's CRC, then stamps a
// footer onto dataOut that carries the source's original checksum.
//
// DEVIATION from the Java reference: Lucene's writeCompoundFile uses
// CodecUtil.verifyAndCopyIndexHeader(in, data, si.getId()) to assert the
// header's segment id matches the surrounding segment before copying.
// Gocene's CodecUtil does not yet expose a verify-and-copy primitive
// (CheckIndexHeader requires a known codec name, which the compound
// writer does not have — every embedded file is from a different codec
// format). The segment-id mismatch detection is a defensive integrity
// check; its omission does not affect on-disk byte parity but does mean
// a corrupt-segment-id condition will only surface later when the
// embedded file is opened through its own format's reader.
func (f *Lucene90CompoundFormat) copyFileBody(dataOut store.IndexOutput, dir store.Directory, fileName string, _ []byte) error {
	in, err := dir.OpenInput(fileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return err
	}
	defer in.Close()

	totalLen := in.Length()
	footerLen := int64(FooterLength())
	if totalLen < footerLen {
		return fmt.Errorf("lucene90 compound: file %q too short for footer", fileName)
	}

	// Wrap in a checksum stream so we can capture the source file's CRC
	// over [0, totalLen - footerLen) without re-reading.
	csIn := store.NewChecksumIndexInput(in)
	bodyBytes := totalLen - footerLen
	if err := copyDataInputToOutput(csIn, bodyBytes, dataOut); err != nil {
		return err
	}

	// Validate and consume the source file's footer (the 16 trailing
	// bytes). Returns the embedded checksum the source declared.
	checksum, err := CheckFooter(csIn)
	if err != nil {
		return fmt.Errorf("lucene90 compound: verify footer of %q: %w", fileName, err)
	}

	// Stamp a footer onto the data stream that carries the SOURCE file's
	// original checksum (NOT dataOut's running checksum). Mirrors Java's
	// "this is poached from CodecUtil.writeFooter" block.
	if err := store.WriteInt32(dataOut, FOOTER_MAGIC); err != nil {
		return err
	}
	if err := store.WriteInt32(dataOut, 0); err != nil {
		return err
	}
	if err := store.WriteInt64(dataOut, checksum); err != nil {
		return err
	}
	return nil
}

// copyDataInputToOutput streams n bytes from in to out using a fixed-size
// scratch buffer. Mirrors IndexOutput.copyBytes.
func copyDataInputToOutput(in store.DataInput, n int64, out store.IndexOutput) error {
	if n <= 0 {
		return nil
	}
	const chunk = 16 * 1024
	scratch := make([]byte, chunk)
	for n > 0 {
		take := int64(chunk)
		if take > n {
			take = n
		}
		if err := in.ReadBytes(scratch[:take]); err != nil {
			return err
		}
		if err := out.WriteBytes(scratch[:take]); err != nil {
			return err
		}
		n -= take
	}
	return nil
}

// stripSegmentNamePrefix returns the suffix of fileName after the leading
// "_segName" component. Mirrors IndexFileNames.stripSegmentName.
func stripSegmentNamePrefix(fileName string) string {
	if len(fileName) < 2 || fileName[0] != '_' {
		return fileName
	}
	// Find first "_" or "." after position 0.
	for i := 1; i < len(fileName); i++ {
		c := fileName[i]
		if c == '_' || c == '.' {
			return fileName[i:]
		}
	}
	return fileName
}

// -----------------------------------------------------------------------------
// Compound reader (.cfs / .cfe) — see lucene90_compound_reader.go for the
// faithful Lucene90CompoundReader port. GetCompoundReader is a thin
// constructor around it; keeping it here preserves the format/reader
// pairing used by Lucene90CompoundFormat.
// -----------------------------------------------------------------------------

// GetCompoundReader opens a CompoundDirectory backed by the .cfs/.cfe
// pair produced by Write. Mirrors Lucene90CompoundFormat.getCompoundReader,
// which simply forwards to `new Lucene90CompoundReader(dir, si)`.
func (f *Lucene90CompoundFormat) GetCompoundReader(dir store.Directory, si *index.SegmentInfo) (CompoundDirectory, error) {
	return NewLucene90CompoundReader(dir, si)
}

// Ensure implementations satisfy the interfaces.
var (
	_ CompoundFormat = (*BaseCompoundFormat)(nil)
	_ CompoundFormat = (*Lucene90CompoundFormat)(nil)
)
