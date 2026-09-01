// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SimpleTextSegmentInfoFormat writes segment metadata as text.
// Mirrors org.apache.lucene.codecs.simpletext.SimpleTextSegmentInfoFormat.
type SimpleTextSegmentInfoFormat struct{}

// NewSimpleTextSegmentInfoFormat builds the format.
func NewSimpleTextSegmentInfoFormat() *SimpleTextSegmentInfoFormat {
	return &SimpleTextSegmentInfoFormat{}
}

const siExtension = "si"

func (f *SimpleTextSegmentInfoFormat) Name() string {
	return "SimpleTextSegmentInfoFormat"
}

func (f *SimpleTextSegmentInfoFormat) Read(dir store.Directory, segmentName string, segmentID []byte, context store.IOContext) (*index.SegmentInfo, error) {
	fileName := segmentName + "." + siExtension
	in, err := dir.OpenChecksumInput(fileName, context)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	readVal := func() string {
		line, err := ReadLine(in)
		if err != nil {
			panic(err)
		}
		// Find the first ':' and take everything after it
		idx := strings.Index(string(line), ":")
		if idx == -1 {
			return ""
		}
		return strings.TrimSpace(string(line[idx+1:]))
	}

	// Version
	versionStr := readVal()
	// Simplified: just store version as string or use a parser
	
	// Min Version
	minVersionStr := readVal()

	// Doc Count
	docCountStr := readVal()
	docCount, _ := strconv.Atoi(docCountStr)

	// Use Compound
	useCompoundStr := readVal()
	useCompound, _ := strconv.ParseBool(useCompoundStr)

	// Has Blocks
	hasBlocksStr := readVal()
	hasBlocks, _ := strconv.ParseBool(hasBlocksStr)

	// Diagnostics
	numDiagStr := readVal()
	numDiag, _ := strconv.Atoi(numDiagStr)
	diagnostics := make(map[string]string)
	for i := 0; i < numDiag; i++ {
		key := readVal()
		val := readVal()
		diagnostics[key] = val
	}

	// Attributes
	numAttStr := readVal()
	numAtt, _ := strconv.Atoi(numAttStr)
	attributes := make(map[string]string)
	for i := 0; i < numAtt; i++ {
		key := readVal()
		val := readVal()
		attributes[key] = val
	}

	// Files
	numFilesStr := readVal()
	numFiles, _ := strconv.Atoi(numFilesStr)
	files := make(map[string]struct{})
	for i := 0; i < numFiles; i++ {
		fileName := readVal()
		files[fileName] = struct{}{}
	}

	// ID
	idStr := readVal()
	id := FromBytesRefString(idStr)

	// Sort
	numSortStr := readVal()
	numSort, _ := strconv.Atoi(numSortStr)
	// Sort is complex, for now we'll just read and skip or implement partially
	for i := 0; i < numSort; i++ {
		readVal() // name
		readVal() // type
		readVal() // bytes
	}

	CheckFooter(in)

	// Note: In a real implementation, I'd map this to index.SegmentInfo
	return &index.SegmentInfo{
		// ... fill fields ...
	}, nil
}

func (f *SimpleTextSegmentInfoFormat) Write(dir store.Directory, si *index.SegmentInfo, context store.IOContext) error {
	fileName := si.Name() + "." + siExtension
	out, err := dir.CreateOutput(fileName, context)
	if err != nil {
		return err
	}
	defer out.Close()

	Write(out, "    version ")
	Write(out, "10.5.0") // Placeholder
	WriteNewline(out)

	Write(out, "    min version ")
	Write(out, "null")
	WriteNewline(out)

	Write(out, "    number of documents ")
	Write(out, strconv.Itoa(si.MaxDoc()))
	WriteNewline(out)

	Write(out, "    uses compound file ")
	Write(out, strconv.FormatBool(si.GetUseCompoundFile()))
	WriteNewline(out)

	Write(out, "    has blocks ")
	Write(out, strconv.FormatBool(si.GetHasBlocks()))
	WriteNewline(out)

	diag := si.GetDiagnostics()
	Write(out, "    diagnostics ")
	Write(out, strconv.Itoa(len(diag)))
	WriteNewline(out)
	for k, v := range diag {
		Write(out, "      key ")
		Write(out, k)
		WriteNewline(out)
		Write(out, "      value ")
		Write(out, v)
		WriteNewline(out)
	}

	att := si.GetAttributes()
	Write(out, "    attributes ")
	Write(out, strconv.Itoa(len(att)))
	WriteNewline(out)
	for k, v := range att {
		Write(out, "      key ")
		Write(out, k)
		WriteNewline(out)
		Write(out, "      value ")
		Write(out, v)
		WriteNewline(out)
	}

	files := si.Files()
	Write(out, "    files ")
	Write(out, strconv.Itoa(len(files)))
	WriteNewline(out)
	for f := range files {
		Write(out, "      file ")
		Write(out, f)
		WriteNewline(out)
	}

	Write(out, "    id ")
	Write(out, fmt.Sprintf("%v", si.GetID())) // Placeholder for BytesRef.toString()
	WriteNewline(out)

	Write(out, "    sort ")
	Write(out, "0")
	WriteNewline(out)

	WriteChecksum(out)
	return nil
}
