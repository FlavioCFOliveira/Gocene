// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SimpleTextFieldInfosFormat writes the per-field FieldInfos as a text file.
// Mirrors org.apache.lucene.codecs.simpletext.SimpleTextFieldInfosFormat.
type SimpleTextFieldInfosFormat struct{}

// NewSimpleTextFieldInfosFormat builds the format.
func NewSimpleTextFieldInfosFormat() *SimpleTextFieldInfosFormat {
	return &SimpleTextFieldInfosFormat{}
}

// fieldInfosExtension is the extension of the field infos file.
// Port of SimpleTextFieldInfosFormat.FIELD_INFOS_EXTENSION (line 53).
const fieldInfosExtension = "inf"

// The line prefixes of the .inf file. Each one is a byte-for-byte port of the
// matching BytesRef constant of SimpleTextFieldInfosFormat (lines 55-77); they
// are the on-disk contract, so the spelling — trailing spaces included, and the
// missing trailing space of fiDocValuesSkipIndex — must not be altered.
var (
	fiNumFields = []byte("number of fields ")
	fiName      = []byte("  name ")
	fiNumber    = []byte("  number ")
	fiStoreTV   = []byte("  term vectors ")
	// fiStoreTVPos and fiStoreTVOff are declared by
	// SimpleTextFieldInfosFormat (lines 59-60) and neither read nor written
	// by it; they are ported as they stand, exactly as Lucene carries them.
	fiStoreTVPos         = []byte("  term vector positions ")
	fiStoreTVOff         = []byte("  term vector offsets ")
	fiPayloads           = []byte("  payloads ")
	fiNorms              = []byte("  norms ")
	fiDocValues          = []byte("  doc values ")
	fiDocValuesSkipIndex = []byte("  doc values skip index")
	fiDocValuesGen       = []byte("  doc values gen ")
	fiIndexOptions       = []byte("  index options ")
	fiNumAtts            = []byte("  attributes ")
	fiAttKey             = []byte("    key ")
	fiAttValue           = []byte("    value ")
	fiDataDimCount       = []byte("  data dimensional count ")
	fiIndexDimCount      = []byte("  index dimensional count ")
	fiDimNumBytes        = []byte("  dimensional num bytes ")
	fiVectorNumDims      = []byte("  vector number of dimensions ")
	fiVectorEncoding     = []byte("  vector encoding ")
	fiVectorSimilarity   = []byte("  vector similarity ")
	fiSoftDeletes        = []byte("  soft-deletes ")
	fiParent             = []byte("  parent ")
)

func (f *SimpleTextFieldInfosFormat) Name() string {
	return "SimpleTextFieldInfosFormat"
}

// readString returns everything after the prefix of the line just read.
// Port of SimpleTextFieldInfosFormat.readString(int, BytesRefBuilder)
// (line 237), which takes the bytes of scratch from offset to its length.
func (f *SimpleTextFieldInfosFormat) readString(line, prefix []byte) (string, error) {
	if !bytes.HasPrefix(line, prefix) {
		return "", fmt.Errorf("SimpleTextFieldInfosFormat: expected %q, got %q", prefix, line)
	}
	return string(line[len(prefix):]), nil
}

func (f *SimpleTextFieldInfosFormat) Read(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, context store.IOContext) (*index.FieldInfos, error) {
	fileName := index.SegmentFileName(segmentInfo.Name(), segmentSuffix, fieldInfosExtension)
	// Java: ChecksumIndexInput input = directory.openChecksumInput(fileName)
	// (line 85). Gocene's store.Directory has no OpenChecksumInput; the
	// in-tree rendering of Directory.openChecksumInput is OpenInput followed
	// by store.NewChecksumIndexInput (codecs/segment_info_format.go:32-36).
	raw, err := dir.OpenInput(fileName, context)
	if err != nil {
		return nil, err
	}
	in := store.NewChecksumIndexInput(raw)
	defer in.Close()

	// readVal reads one line and strips the given prefix.
	readVal := func(prefix []byte) (string, error) {
		line, err := ReadLine(in)
		if err != nil {
			return "", err
		}
		return f.readString(line, prefix)
	}

	numFieldsStr, err := readVal(fiNumFields)
	if err != nil {
		return nil, err
	}
	size, err := strconv.Atoi(numFieldsStr)
	if err != nil {
		return nil, fmt.Errorf("SimpleTextFieldInfosFormat: number of fields: %w", err)
	}
	infos := make([]*index.FieldInfo, size)

	for i := 0; i < size; i++ {
		// String name = readString(NAME.length, scratch);   (line 99)
		name, err := readVal(fiName)
		if err != nil {
			return nil, err
		}

		// int fieldNumber = Integer.parseInt(...);          (line 103)
		numStr, err := readVal(fiNumber)
		if err != nil {
			return nil, err
		}
		fieldNumber, err := strconv.Atoi(numStr)
		if err != nil {
			return nil, fmt.Errorf("SimpleTextFieldInfosFormat: number: %w", err)
		}

		// final IndexOptions indexOptions = IndexOptions.valueOf(s); (line 108)
		optStr, err := readVal(fiIndexOptions)
		if err != nil {
			return nil, err
		}
		indexOptions, err := spi.ParseIndexOptions(optStr)
		if err != nil {
			return nil, err
		}

		// boolean storeTermVector = Boolean.parseBoolean(...); (line 112)
		tvStr, err := readVal(fiStoreTV)
		if err != nil {
			return nil, err
		}
		storeTermVector := tvStr == "true"

		// boolean storePayloads = Boolean.parseBoolean(...);  (line 116)
		payStr, err := readVal(fiPayloads)
		if err != nil {
			return nil, err
		}
		storePayloads := payStr == "true"

		// boolean omitNorms = !Boolean.parseBoolean(...);     (line 120)
		normStr, err := readVal(fiNorms)
		if err != nil {
			return nil, err
		}
		omitNorms := normStr != "true"

		// final DocValuesType docValuesType = docValuesType(dvType); (line 125)
		dvStr, err := readVal(fiDocValues)
		if err != nil {
			return nil, err
		}
		docValuesType, err := f.DocValuesType(dvStr)
		if err != nil {
			return nil, err
		}

		// DocValuesSkipIndexType docValueSkipper = ...;       (line 129)
		skipStr, err := readVal(fiDocValuesSkipIndex)
		if err != nil {
			return nil, err
		}
		docValueSkipper, err := f.DocValuesSkipIndexType(skipStr)
		if err != nil {
			return nil, err
		}

		// final long dvGen = Long.parseLong(...);             (line 134)
		genStr, err := readVal(fiDocValuesGen)
		if err != nil {
			return nil, err
		}
		dvGen, err := strconv.ParseInt(genStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("SimpleTextFieldInfosFormat: doc values gen: %w", err)
		}

		// int numAtts = Integer.parseInt(...); Map<String,String> atts; (lines 138-150)
		numAttStr, err := readVal(fiNumAtts)
		if err != nil {
			return nil, err
		}
		numAtts, err := strconv.Atoi(numAttStr)
		if err != nil {
			return nil, fmt.Errorf("SimpleTextFieldInfosFormat: attributes: %w", err)
		}
		atts := make(map[string]string, numAtts)
		for j := 0; j < numAtts; j++ {
			key, err := readVal(fiAttKey)
			if err != nil {
				return nil, err
			}
			value, err := readVal(fiAttValue)
			if err != nil {
				return nil, err
			}
			atts[key] = value
		}

		// int dimensionalCount = Integer.parseInt(...);       (line 154)
		dimCountStr, err := readVal(fiDataDimCount)
		if err != nil {
			return nil, err
		}
		dimensionalCount, err := strconv.Atoi(dimCountStr)
		if err != nil {
			return nil, fmt.Errorf("SimpleTextFieldInfosFormat: data dimensional count: %w", err)
		}

		// int indexDimensionalCount = Integer.parseInt(...);  (line 158)
		idxDimCountStr, err := readVal(fiIndexDimCount)
		if err != nil {
			return nil, err
		}
		indexDimensionalCount, err := strconv.Atoi(idxDimCountStr)
		if err != nil {
			return nil, fmt.Errorf("SimpleTextFieldInfosFormat: index dimensional count: %w", err)
		}

		// int dimensionalNumBytes = Integer.parseInt(...);    (line 162)
		dimBytesStr, err := readVal(fiDimNumBytes)
		if err != nil {
			return nil, err
		}
		dimensionalNumBytes, err := strconv.Atoi(dimBytesStr)
		if err != nil {
			return nil, fmt.Errorf("SimpleTextFieldInfosFormat: dimensional num bytes: %w", err)
		}

		// int vectorNumDimensions = Integer.parseInt(...);    (line 166)
		vecDimStr, err := readVal(fiVectorNumDims)
		if err != nil {
			return nil, err
		}
		vectorNumDimensions, err := strconv.Atoi(vecDimStr)
		if err != nil {
			return nil, fmt.Errorf("SimpleTextFieldInfosFormat: vector number of dimensions: %w", err)
		}

		// VectorEncoding vectorEncoding = vectorEncoding(encoding); (line 171)
		encodingStr, err := readVal(fiVectorEncoding)
		if err != nil {
			return nil, err
		}
		vectorEncoding, err := f.VectorEncoding(encodingStr)
		if err != nil {
			return nil, err
		}

		// VectorSimilarityFunction vectorDistFunc = distanceFunction(...); (line 176)
		similarityStr, err := readVal(fiVectorSimilarity)
		if err != nil {
			return nil, err
		}
		vectorDistFunc, err := f.DistanceFunction(similarityStr)
		if err != nil {
			return nil, err
		}

		// boolean isSoftDeletesField = Boolean.parseBoolean(...); (line 180)
		softDelStr, err := readVal(fiSoftDeletes)
		if err != nil {
			return nil, err
		}
		isSoftDeletesField := softDelStr == "true"

		// boolean isParentField = Boolean.parseBoolean(...);  (line 183)
		parentStr, err := readVal(fiParent)
		if err != nil {
			return nil, err
		}
		isParentField := parentStr == "true"

		// infos[i] = new FieldInfo(name, fieldNumber, storeTermVector,
		//     omitNorms, storePayloads, indexOptions, docValuesType,
		//     docValueSkipper, dvGen, unmodifiableMap(atts), dimensionalCount,
		//     indexDimensionalCount, dimensionalNumBytes, vectorNumDimensions,
		//     vectorEncoding, vectorDistFunc, isSoftDeletesField, isParentField);
		//                                                     (lines 185-204)
		//
		// Gocene renders that constructor as the FieldInfoBuilder chain used by
		// every other FieldInfosFormat in the tree
		// (codecs/lucene94_field_infos_format.go:219-242); storePayloads is
		// applied afterwards through SetStorePayloads, the port of Java's
		// package-private FieldInfo.setStorePayloads.
		fib := index.NewFieldInfoBuilder(name, fieldNumber).
			SetIndexOptions(indexOptions).
			SetDocValuesType(docValuesType).
			SetDocValuesSkipIndexType(docValueSkipper).
			SetDocValuesGen(dvGen).
			SetOmitNorms(omitNorms).
			SetStoreTermVectors(storeTermVector).
			SetPointDimensions(dimensionalCount, indexDimensionalCount, dimensionalNumBytes).
			SetVectorAttributes(vectorNumDimensions, vectorEncoding, vectorDistFunc).
			SetSoftDeletesField(isSoftDeletesField).
			SetParentField(isParentField)
		for k, v := range atts {
			fib.SetAttribute(k, v)
		}
		fi := fib.Build()
		if storePayloads {
			fi.SetStorePayloads()
		}
		infos[i] = fi
	}

	// SimpleTextUtil.checkFooter(input);                      (line 207)
	if err := CheckFooter(in); err != nil {
		return nil, err
	}

	// FieldInfos fieldInfos = new FieldInfos(infos);          (line 209)
	return spi.NewFieldInfos(infos...), nil
}

// DocValuesType resolves the doc-values type named dvType.
// Port of SimpleTextFieldInfosFormat.docValuesType(String) (line 221).
func (f *SimpleTextFieldInfosFormat) DocValuesType(dvType string) (index.DocValuesType, error) {
	return spi.ParseDocValuesType(dvType)
}

// DocValuesSkipIndexType resolves the skip-index type named dvSkipIndexType.
// Port of SimpleTextFieldInfosFormat.docValuesSkipIndexType(String) (line 225).
func (f *SimpleTextFieldInfosFormat) DocValuesSkipIndexType(dvSkipIndexType string) (index.DocValuesSkipIndexType, error) {
	return spi.ParseDocValuesSkipIndexType(dvSkipIndexType)
}

// VectorEncoding resolves the vector encoding named vectorEncoding.
// Port of SimpleTextFieldInfosFormat.vectorEncoding(String) (line 229).
func (f *SimpleTextFieldInfosFormat) VectorEncoding(vectorEncoding string) (util.VectorEncoding, error) {
	return util.ParseVectorEncoding(vectorEncoding)
}

// DistanceFunction resolves the vector similarity function named scoreFunction.
// Port of SimpleTextFieldInfosFormat.distanceFunction(String) (line 233).
func (f *SimpleTextFieldInfosFormat) DistanceFunction(scoreFunction string) (util.VectorSimilarityFunction, error) {
	id, err := util.ParseVectorSimilarityID(scoreFunction)
	if err != nil {
		return nil, err
	}
	return util.GetSimilarityFunction(id), nil
}

func (f *SimpleTextFieldInfosFormat) Write(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, infos *index.FieldInfos, context store.IOContext) error {
	fileName := index.SegmentFileName(segmentInfo.Name(), segmentSuffix, fieldInfosExtension)
	raw, err := dir.CreateOutput(fileName, context)
	if err != nil {
		return err
	}
	// Java's Directory.createOutput hands back an IndexOutput that already
	// tracks a CRC32; in Gocene the running checksum lives on
	// store.ChecksumIndexOutput, which every other SimpleText writer wraps the
	// raw output in (simple_text_fields_writer.go:63-68).
	out := store.NewChecksumIndexOutput(raw)
	defer out.Close()

	// writeVal writes one "<prefix><value>\n" line.
	writeVal := func(prefix []byte, value string) error {
		if err := out.WriteBytes(prefix, 0, len(prefix)); err != nil {
			return err
		}
		if err := Write(out, value); err != nil {
			return err
		}
		return WriteNewline(out)
	}

	if err := writeVal(fiNumFields, strconv.Itoa(infos.Size())); err != nil {
		return err
	}

	for _, fi := range infos.Fields() {
		if err := writeVal(fiName, fi.Name()); err != nil {
			return err
		}
		if err := writeVal(fiNumber, strconv.Itoa(fi.Number())); err != nil {
			return err
		}
		if err := writeVal(fiIndexOptions, fi.IndexOptions().String()); err != nil {
			return err
		}
		if err := writeVal(fiStoreTV, strconv.FormatBool(fi.HasTermVectors())); err != nil {
			return err
		}
		if err := writeVal(fiPayloads, strconv.FormatBool(fi.HasPayloads())); err != nil {
			return err
		}
		// Boolean.toString(!fi.omitsNorms())                  (line 284)
		if err := writeVal(fiNorms, strconv.FormatBool(!fi.OmitNorms())); err != nil {
			return err
		}
		if err := writeVal(fiDocValues, fi.DocValuesType().String()); err != nil {
			return err
		}
		if err := writeVal(fiDocValuesSkipIndex, fi.DocValuesSkipIndexType().String()); err != nil {
			return err
		}
		if err := writeVal(fiDocValuesGen, strconv.FormatInt(fi.DocValuesGen(), 10)); err != nil {
			return err
		}

		atts := fi.GetAttributes()
		if err := writeVal(fiNumAtts, strconv.Itoa(len(atts))); err != nil {
			return err
		}
		for k, v := range atts {
			if err := writeVal(fiAttKey, k); err != nil {
				return err
			}
			if err := writeVal(fiAttValue, v); err != nil {
				return err
			}
		}

		if err := writeVal(fiDataDimCount, strconv.Itoa(fi.PointDimensionCount())); err != nil {
			return err
		}
		if err := writeVal(fiIndexDimCount, strconv.Itoa(fi.PointIndexDimensionCount())); err != nil {
			return err
		}
		if err := writeVal(fiDimNumBytes, strconv.Itoa(fi.PointNumBytes())); err != nil {
			return err
		}
		if err := writeVal(fiVectorNumDims, strconv.Itoa(fi.VectorDimension())); err != nil {
			return err
		}
		// fi.getVectorEncoding().name()                       (line 334)
		if err := writeVal(fiVectorEncoding, fi.VectorEncoding().String()); err != nil {
			return err
		}
		// fi.getVectorSimilarityFunction().name()             (line 338).
		// Gocene splits Java's VectorSimilarityFunction enum into a behaviour
		// interface plus the VectorSimilarityID ordinal; the enum constant name
		// is carried by the ordinal's String().
		if err := writeVal(fiVectorSimilarity, fi.VectorSimilarityFunction().ID().String()); err != nil {
			return err
		}
		if err := writeVal(fiSoftDeletes, strconv.FormatBool(fi.IsSoftDeletesField())); err != nil {
			return err
		}
		if err := writeVal(fiParent, strconv.FormatBool(fi.IsParentField())); err != nil {
			return err
		}
	}

	return WriteChecksum(out)
}
