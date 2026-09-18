// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SimpleTextFieldInfosFormat writes the per-field FieldInfos as a text file.
// Mirrors org.apache.lucene.codecs.simpletext.SimpleTextFieldInfosFormat.
type SimpleTextFieldInfosFormat struct{}

// NewSimpleTextFieldInfosFormat builds the format.
func NewSimpleTextFieldInfosFormat() *SimpleTextFieldInfosFormat {
	return &SimpleTextFieldInfosFormat{}
}

const fieldInfosExtension = "inf"

func (f *SimpleTextFieldInfosFormat) Name() string {
	return "SimpleTextFieldInfosFormat"
}

func (f *SimpleTextFieldInfosFormat) Read(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, context store.IOContext) (*index.FieldInfos, error) {
	fileName := segmentInfo.Name() + segmentSuffix + "." + fieldInfosExtension
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
		idx := strings.Index(string(line), ":")
		if idx == -1 {
			return ""
		}
		return strings.TrimSpace(string(line[idx+1:]))
	}

	numFieldsStr := readVal()
	numFields, _ := strconv.Atoi(numFieldsStr)
	infos := make([]*index.FieldInfo, numFields)

	for i := 0; i < numFields; i++ {
		name := readVal()
		numStr := readVal()
		num, _ := strconv.Atoi(numStr)
		optStr := readVal()
		// Simplified: a real implementation would parse IndexOptions

		tvStr := readVal()
		tv, _ := strconv.ParseBool(tvStr)

		payStr := readVal()
		pay, _ := strconv.ParseBool(payStr)

		normStr := readVal()
		norm, _ := strconv.ParseBool(normStr)

		dvStr := readVal()
		// Simplified: a real implementation would parse DocValuesType

		skipStr := readVal()
		// Simplified: a real implementation would parse DocValuesSkipIndexType

		genStr := readVal()
		gen, _ := strconv.ParseInt(genStr, 10, 64)

		numAttStr := readVal()
		numAtt, _ := strconv.Atoi(numAttStr)
		atts := make(map[string]string)
		for j := 0; j < numAtt; j++ {
			k := readVal()
			v := readVal()
			atts[k] = v
		}

		dimCountStr := readVal()
		dimCount, _ := strconv.Atoi(dimCountStr)

		idxDimCountStr := readVal()
		idxDimCount, _ := strconv.Atoi(idxDimCountStr)

		dimBytesStr := readVal()
		dimBytes, _ := strconv.Atoi(dimBytesStr)

		vecDimStr := readVal()
		vecDim, _ := strconv.Atoi(vecDimStr)

		encodingStr := readVal()
		// Simplified: parse VectorEncoding

		similarityStr := readVal()
		// Simplified: parse VectorSimilarityFunction

		softDelStr := readVal()
		softDel, _ := strconv.ParseBool(softDelStr)

		parentStr := readVal()
		parent, _ := strconv.ParseBool(parentStr)

		infos[i] = &index.FieldInfo{
			Name:   name,
			Number: num,
			// ... other fields ...
		}
	}

	CheckFooter(in)
	return index.NewFieldInfos(infos), nil
}

func (f *SimpleTextFieldInfosFormat) Write(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, infos *index.FieldInfos, context store.IOContext) error {
	fileName := segmentInfo.Name() + segmentSuffix + "." + fieldInfosExtension
	out, err := dir.CreateOutput(fileName, context)
	if err != nil {
		return err
	}
	defer out.Close()

	Write(out, "number of fields ")
	Write(out, strconv.Itoa(infos.Size()))
	WriteNewline(out)

	for _, fi := range infos.Fields() {
		Write(out, "  name ")
		Write(out, fi.Name())
		WriteNewline(out)

		Write(out, "  number ")
		Write(out, strconv.Itoa(fi.Number()))
		WriteNewline(out)

		Write(out, "  index options ")
		Write(out, fi.IndexOptions().String())
		WriteNewline(out)

		Write(out, "  term vectors ")
		Write(out, strconv.FormatBool(fi.HasTermVectors()))
		WriteNewline(out)

		Write(out, "  payloads ")
		Write(out, strconv.FormatBool(fi.HasPayloads()))
		WriteNewline(out)

		Write(out, "  norms ")
		Write(out, strconv.FormatBool(!fi.OmitsNorms()))
		WriteNewline(out)

		Write(out, "  doc values ")
		Write(out, fi.DocValuesType().String())
		WriteNewline(out)

		Write(out, "  doc values skip index")
		Write(out, fi.DocValuesSkipIndexType().String())
		WriteNewline(out)

		Write(out, "  doc values gen ")
		Write(out, strconv.FormatInt(fi.DocValuesGen(), 10))
		WriteNewline(out)

		atts := fi.Attributes()
		Write(out, "  attributes ")
		Write(out, strconv.Itoa(len(atts)))
		WriteNewline(out)
		for k, v := range atts {
			Write(out, "    key ")
			Write(out, k)
			WriteNewline(out)
			Write(out, "    value ")
			Write(out, v)
			WriteNewline(out)
		}

		Write(out, "  data dimensional count ")
		Write(out, strconv.Itoa(fi.PointDimensionCount()))
		WriteNewline(out)

		Write(out, "  index dimensional count ")
		Write(out, strconv.Itoa(fi.PointIndexDimensionCount()))
		WriteNewline(out)

		Write(out, "  dimensional num bytes ")
		Write(out, strconv.Itoa(fi.PointNumBytes()))
		WriteNewline(out)

		Write(out, "  vector number of dimensions ")
		Write(out, strconv.Itoa(fi.VectorDimension()))
		WriteNewline(out)

		Write(out, "  vector encoding ")
		Write(out, fi.VectorEncoding().String())
		WriteNewline(out)

		Write(out, "  vector similarity ")
		Write(out, fi.VectorSimilarityFunction().String())
		WriteNewline(out)

		Write(out, "  soft-deletes ")
		Write(out, strconv.FormatBool(fi.IsSoftDeletesField()))
		WriteNewline(out)

		Write(out, "  parent ")
		Write(out, strconv.FormatBool(fi.IsParentField()))
		WriteNewline(out)
	}

	WriteChecksum(out)
	return nil
}
