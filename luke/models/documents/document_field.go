package documents

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocumentField is a holder for a document field's information and data.
type DocumentField struct {
	name                string
	idxOptions          index.IndexOptions
	hasTermVectors      bool
	hasPayloads         bool
	hasNorms            bool
	norm                int64
	isStored            bool
	stringValue         string
	binaryValue         *util.BytesRef
	numericValue        float64 // Simplified from java.lang.Number
	dvType              index.DocValuesType
	pointDimensionCount int
	pointNumBytes       int
	vectorDimension     int
	vectorSimilarity    index.VectorSimilarityFunction
}

func NewDocumentField(finfo index.FieldInfo, field document.IndexableField, reader index.IndexReader, docID int) (*DocumentField, error) {
	dfield := &DocumentField{
		name:                finfo.Name(),
		idxOptions:          finfo.IndexOptions(),
		hasTermVectors:      finfo.HasTermVectors(),
		hasPayloads:         finfo.HasPayloads(),
		hasNorms:            finfo.HasNorms(),
		dvType:              finfo.DocValuesType(),
		pointDimensionCount: finfo.PointDimensionCount(),
		pointNumBytes:       finfo.PointNumBytes(),
		vectorDimension:     finfo.VectorDimension(),
		vectorSimilarity:    finfo.VectorSimilarityFunction(),
	}

	if finfo.HasNorms() {
		norms, err := index.MultiDocValuesGetNormValues(reader, finfo.Name())
		if err != nil {
			return nil, err
		}
		if norms != nil {
			ok, err := norms.AdvanceExact(docID)
			if err != nil {
				return nil, err
			}
			if ok {
				if dfield.norm, err = norms.LongValue(); err != nil {
					return nil, err
				}
			}
		}
	}

	if field != nil {
		dfield.isStored = field.FieldType().Stored
		dfield.stringValue = field.StringValue()
		if b := field.BinaryValue(); b != nil {
			dfield.binaryValue = util.BytesRefDeepCopyOf(util.NewBytesRef(b))
		}
		// Java keeps the raw java.lang.Number; Gocene's DocumentField narrows
		// it to float64, so the numeric kinds Lucene can store are converted.
		switch v := field.NumericValue().(type) {
		case int:
			dfield.numericValue = float64(v)
		case int32:
			dfield.numericValue = float64(v)
		case int64:
			dfield.numericValue = float64(v)
		case float32:
			dfield.numericValue = float64(v)
		case float64:
			dfield.numericValue = v
		}
	}

	return dfield, nil
}

func (df *DocumentField) Name() string {
	return df.name
}

func (df *DocumentField) IndexOptions() index.IndexOptions {
	return df.idxOptions
}

func (df *DocumentField) HasTermVectors() bool {
	return df.hasTermVectors
}

func (df *DocumentField) HasPayloads() bool {
	return df.hasPayloads
}

func (df *DocumentField) HasNorms() bool {
	return df.hasNorms
}

func (df *DocumentField) Norm() int64 {
	return df.norm
}

func (df *DocumentField) IsStored() bool {
	return df.isStored
}

func (df *DocumentField) StringValue() string {
	return df.stringValue
}

func (df *DocumentField) BinaryValue() *util.BytesRef {
	return df.binaryValue
}

func (df *DocumentField) NumericValue() float64 {
	return df.numericValue
}

func (df *DocumentField) DocValuesType() index.DocValuesType {
	return df.dvType
}

func (df *DocumentField) PointDimensionCount() int {
	return df.pointDimensionCount
}

func (df *DocumentField) PointNumBytes() int {
	return df.pointNumBytes
}

func (df *DocumentField) VectorDimension() int {
	return df.vectorDimension
}

func (df *DocumentField) VectorSimilarity() index.VectorSimilarityFunction {
	return df.vectorSimilarity
}

func (df *DocumentField) String() string {
	return fmt.Sprintf("DocumentField{name='%s', idxOptions=%v, hasTermVectors=%v, isStored=%v, dvType=%v, pointDimensionCount=%d, vectorDimension=%d}",
		df.name, df.idxOptions, df.hasTermVectors, df.isStored, df.dvType, df.pointDimensionCount, df.vectorDimension)
}
