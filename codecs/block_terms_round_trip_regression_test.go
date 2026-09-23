// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

// This file pins the codecs/blockterms defects found while porting the write
// side (TermsIndexWriterBase, FixedGapTermsIndexWriter,
// VariableGapTermsIndexWriter, BlockTermsWriter) of Apache Lucene 10.5.0:
//
//	A  BlockTermsReader lacked iterator(), checkIntegrity() and
//	   getMergeInstance(), so it was not a FieldsProducer;
//	B  SegmentTermsEnum.decodeMetaData looked the FieldReader up by the term
//	   text instead of using its enclosing FieldReader (nil dereference).
//
// The postings formats below render the test-framework
// org.apache.lucene.tests.codecs.blockterms.LuceneFixedGap and
// LuceneVarGapFixedInterval (Lucene104 postings + a terms index writer +
// BlockTermsWriter, and the matching readers). The Lucene test classes that use
// them (TestFixedGapPostingsFormat, TestVarGapFixedIntervalPostingsFormat) need
// BasePostingsFormatTestCase, which is not ported, so this regression drives
// the Gocene PostingsTester over DOCS postings: the tester passes no norms,
// and PushPostingsWriterBase.writeTerm reads norms for fields that have them.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/blockterms"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

type blockTermsRegressionFormat struct {
	varGap   bool
	interval int
}

func (f blockTermsRegressionFormat) Name() string {
	if f.varGap {
		return "LuceneVarGapFixedInterval"
	}
	return "LuceneFixedGap"
}

func (f blockTermsRegressionFormat) FieldsConsumer(state *spi.SegmentWriteState) (spi.FieldsConsumer, error) {
	docs, err := codecs.NewLucene104PostingsWriter(state)
	if err != nil {
		return nil, err
	}
	var indexWriter blockterms.TermsIndexWriterBase
	if f.varGap {
		indexWriter, err = blockterms.NewVariableGapTermsIndexWriter(state, blockterms.NewEveryNTermSelector(f.interval))
	} else {
		indexWriter, err = blockterms.NewFixedGapTermsIndexWriterWithTermIndexInterval(state, f.interval)
	}
	if err != nil {
		return nil, err
	}
	return blockterms.NewBlockTermsWriter(indexWriter, state, docs)
}

func (f blockTermsRegressionFormat) FieldsProducer(state *spi.SegmentReadState) (spi.FieldsProducer, error) {
	postings, err := codecs.NewLucene104PostingsReader(state)
	if err != nil {
		return nil, err
	}
	var indexReader blockterms.TermsIndexReader
	if f.varGap {
		indexReader, err = blockterms.NewVariableGapTermsIndexReader(state)
	} else {
		indexReader, err = blockterms.NewFixedGapTermsIndexReader(state)
	}
	if err != nil {
		return nil, err
	}
	return blockterms.NewBlockTermsReader(indexReader, postings, state)
}

func TestBlockTerms_RoundTripRegression(t *testing.T) {
	for _, varGap := range []bool{false, true} {
		codecs.NewPostingsTester(t).TestFull(
			blockTermsRegressionFormat{varGap: varGap, interval: 3}, index.IndexOptionsDocs, store.NewByteBuffersDirectory())
	}
}
