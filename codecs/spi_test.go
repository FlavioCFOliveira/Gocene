// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"

	// Blank import triggers init() registration of every backward-compat
	// PostingsFormat, DocValuesFormat, and KnnVectorsFormat from the
	// backward_codecs sub-packages (mirrors Java ServiceLoader auto-discovery
	// when backward-codecs.jar is on the classpath).
	_ "github.com/FlavioCFOliveira/Gocene/backward_codecs"
)

// TestPostingsFormatByName_CoreFormats verifies that PostingsFormatByName
// resolves every format registered by the codecs package init().
//
// AC1 (core): resolves default and per-field formats by canonical Lucene 10.4.0
// name without programmatic pre-registration by the test itself.
func TestPostingsFormatByName_CoreFormats(t *testing.T) {
	coreFormats := []string{
		"Lucene104",               // org.apache.lucene.codecs.lucene104.Lucene104PostingsFormat
		"Lucene103PostingsFormat", // org.apache.lucene.codecs.lucene103.Lucene103PostingsFormat (current port)
		"PerField40",              // org.apache.lucene.codecs.perfield.PerFieldPostingsFormat
	}
	for _, name := range coreFormats {
		t.Run(name, func(t *testing.T) {
			format, err := codecs.PostingsFormatByName(name)
			if err != nil {
				t.Fatalf("PostingsFormatByName(%q): unexpected error: %v", name, err)
			}
			if format == nil {
				t.Fatalf("PostingsFormatByName(%q): returned nil format", name)
			}
			if format.Name() != name {
				t.Errorf("PostingsFormatByName(%q): Name()=%q, want %q", name, format.Name(), name)
			}
		})
	}
}

// TestPostingsFormatByName_BackwardFormats verifies that PostingsFormatByName
// resolves every backward-compatibility PostingsFormat registered via blank
// import of backward_codecs.
//
// AC1 (backward-codecs): resolves read-only entries by their canonical Lucene
// 10.4.0 name.
func TestPostingsFormatByName_BackwardFormats(t *testing.T) {
	backwardFormats := []string{
		"Lucene50PostingsFormat",  // org.apache.lucene.backward_codecs.lucene50
		"Lucene84PostingsFormat",  // org.apache.lucene.backward_codecs.lucene84
		"Lucene90PostingsFormat",  // org.apache.lucene.backward_codecs.lucene90
		"Lucene99PostingsFormat",  // org.apache.lucene.backward_codecs.lucene99
		"Lucene912",               // org.apache.lucene.backward_codecs.lucene912
		"Lucene101PostingsFormat", // org.apache.lucene.backward_codecs.lucene101
	}
	for _, name := range backwardFormats {
		t.Run(name, func(t *testing.T) {
			format, err := codecs.PostingsFormatByName(name)
			if err != nil {
				t.Fatalf("PostingsFormatByName(%q): unexpected error: %v", name, err)
			}
			if format == nil {
				t.Fatalf("PostingsFormatByName(%q): returned nil format", name)
			}
			if format.Name() != name {
				t.Errorf("PostingsFormatByName(%q): Name()=%q, want %q", name, format.Name(), name)
			}
		})
	}
}

// TestDocValuesFormatByName_CoreFormats verifies that DocValuesFormatByName
// resolves every format registered by the codecs package init().
func TestDocValuesFormatByName_CoreFormats(t *testing.T) {
	coreFormats := []string{
		"Lucene90",     // org.apache.lucene.codecs.lucene90.Lucene90DocValuesFormat
		"PerFieldDV40", // org.apache.lucene.codecs.perfield.PerFieldDocValuesFormat
	}
	for _, name := range coreFormats {
		t.Run(name, func(t *testing.T) {
			format, err := codecs.DocValuesFormatByName(name)
			if err != nil {
				t.Fatalf("DocValuesFormatByName(%q): unexpected error: %v", name, err)
			}
			if format == nil {
				t.Fatalf("DocValuesFormatByName(%q): returned nil format", name)
			}
			if format.Name() != name {
				t.Errorf("DocValuesFormatByName(%q): Name()=%q, want %q", name, format.Name(), name)
			}
		})
	}
}

// TestDocValuesFormatByName_BackwardFormats verifies that DocValuesFormatByName
// resolves backward-compatibility DocValuesFormats.
func TestDocValuesFormatByName_BackwardFormats(t *testing.T) {
	backwardFormats := []string{
		"Lucene80", // org.apache.lucene.backward_codecs.lucene80.Lucene80DocValuesFormat
	}
	for _, name := range backwardFormats {
		t.Run(name, func(t *testing.T) {
			format, err := codecs.DocValuesFormatByName(name)
			if err != nil {
				t.Fatalf("DocValuesFormatByName(%q): unexpected error: %v", name, err)
			}
			if format == nil {
				t.Fatalf("DocValuesFormatByName(%q): returned nil format", name)
			}
			if format.Name() != name {
				t.Errorf("DocValuesFormatByName(%q): Name()=%q, want %q", name, format.Name(), name)
			}
		})
	}
}

// TestKnnVectorsFormatByName_CoreFormats verifies that KnnVectorsFormatByName
// resolves every format registered by the codecs package init().
func TestKnnVectorsFormatByName_CoreFormats(t *testing.T) {
	coreFormats := []string{
		"Lucene99HnswVectorsFormat",                 // org.apache.lucene.codecs.lucene99
		"Lucene104ScalarQuantizedVectorsFormat",     // org.apache.lucene.codecs.lucene104
		"Lucene104HnswScalarQuantizedVectorsFormat", // org.apache.lucene.codecs.lucene104
		"PerFieldVectors90",                         // org.apache.lucene.codecs.perfield
	}
	for _, name := range coreFormats {
		t.Run(name, func(t *testing.T) {
			format, err := codecs.KnnVectorsFormatByName(name)
			if err != nil {
				t.Fatalf("KnnVectorsFormatByName(%q): unexpected error: %v", name, err)
			}
			if format == nil {
				t.Fatalf("KnnVectorsFormatByName(%q): returned nil format", name)
			}
			if format.Name() != name {
				t.Errorf("KnnVectorsFormatByName(%q): Name()=%q, want %q", name, format.Name(), name)
			}
		})
	}
}

// TestKnnVectorsFormatByName_BackwardFormats verifies that
// KnnVectorsFormatByName resolves backward-compat KNN formats.
func TestKnnVectorsFormatByName_BackwardFormats(t *testing.T) {
	backwardFormats := []string{
		"Lucene90HnswVectorsFormat",                 // backward_codecs/lucene90
		"Lucene91HnswVectorsFormat",                 // backward_codecs/lucene91
		"Lucene92HnswVectorsFormat",                 // backward_codecs/lucene92
		"Lucene94HnswVectorsFormat",                 // backward_codecs/lucene94
		"Lucene95HnswVectorsFormat",                 // backward_codecs/lucene95
		"Lucene99HnswScalarQuantizedVectorsFormat",  // backward_codecs/lucene99
		"Lucene99ScalarQuantizedVectorsFormat",      // backward_codecs/lucene99
		"Lucene102BinaryQuantizedVectorsFormat",     // backward_codecs/lucene102
		"Lucene102HnswBinaryQuantizedVectorsFormat", // backward_codecs/lucene102
	}
	for _, name := range backwardFormats {
		t.Run(name, func(t *testing.T) {
			format, err := codecs.KnnVectorsFormatByName(name)
			if err != nil {
				t.Fatalf("KnnVectorsFormatByName(%q): unexpected error: %v", name, err)
			}
			if format == nil {
				t.Fatalf("KnnVectorsFormatByName(%q): returned nil format", name)
			}
			if format.Name() != name {
				t.Errorf("KnnVectorsFormatByName(%q): Name()=%q, want %q", name, format.Name(), name)
			}
		})
	}
}

// TestPostingsFormatByName_UnknownName verifies that PostingsFormatByName
// returns a clear error for unregistered names.
func TestPostingsFormatByName_UnknownName(t *testing.T) {
	_, err := codecs.PostingsFormatByName("NonExistentFormat")
	if err == nil {
		t.Fatal("PostingsFormatByName(unknown): expected error, got nil")
	}
}

// TestDocValuesFormatByName_UnknownName verifies DocValuesFormatByName error.
func TestDocValuesFormatByName_UnknownName(t *testing.T) {
	_, err := codecs.DocValuesFormatByName("NonExistentFormat")
	if err == nil {
		t.Fatal("DocValuesFormatByName(unknown): expected error, got nil")
	}
}

// TestKnnVectorsFormatByName_UnknownName verifies KnnVectorsFormatByName error.
func TestKnnVectorsFormatByName_UnknownName(t *testing.T) {
	_, err := codecs.KnnVectorsFormatByName("NonExistentFormat")
	if err == nil {
		t.Fatal("KnnVectorsFormatByName(unknown): expected error, got nil")
	}
}

// TestPerFieldPostingsFormat_DispatchByName verifies AC2: PerFieldPostingsFormat
// dispatches to the correct delegate by name (via PostingsFormatByName) in a
// multi-field round-trip test, without any programmatic pre-registration by
// this test.
//
// The test writes two fields with the same Lucene104 format (the only one with
// a real FieldsConsumer/FieldsProducer implementation at this sprint) and
// verifies that the PerFieldFieldsProducer resolves postings for each field
// correctly on the read path. The PerFieldPostingsFormat attributes stamped on
// each FieldInfo drive resolution via PostingsFormatByName internally.
func TestPerFieldPostingsFormat_DispatchByName(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	const segmentName = "_0"
	const field1Name = "title"
	const field2Name = "body"

	// Build FieldInfos with two indexed fields using the FieldInfoOptions API.
	opts1 := index.DefaultFieldInfoOptions()
	opts1.IndexOptions = index.IndexOptionsDocsAndFreqs
	fi1 := index.NewFieldInfo(field1Name, 0, opts1)

	opts2 := index.DefaultFieldInfoOptions()
	opts2.IndexOptions = index.IndexOptionsDocsAndFreqs
	fi2 := index.NewFieldInfo(field2Name, 1, opts2)

	fis := index.NewFieldInfos()
	if err := fis.Add(fi1); err != nil {
		t.Fatalf("FieldInfos.Add(fi1): %v", err)
	}
	if err := fis.Add(fi2); err != nil {
		t.Fatalf("FieldInfos.Add(fi2): %v", err)
	}

	si := index.NewSegmentInfo(segmentName, 2, dir)
	si.SetCodec("Lucene104")

	// Write: use PerFieldPostingsFormat backed by Lucene104 as default.
	format := codecs.NewPerFieldPostingsFormatWithDefault(codecs.NewLucene104PostingsFormat())
	writeState := &codecs.SegmentWriteState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fis,
	}
	consumer, err := format.FieldsConsumer(writeState)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}

	// Write a minimal term for each field to produce real postings files.
	for _, fieldName := range []string{field1Name, field2Name} {
		terms := &singleTermTerms{
			field: fieldName,
			term:  index.NewTerm(fieldName, "hello"),
			freq:  1,
		}
		if err := consumer.Write(fieldName, terms); err != nil {
			t.Fatalf("consumer.Write(%q): %v", fieldName, err)
		}
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	// Read: PerFieldFieldsProducer resolves the delegate format name from
	// FieldInfo attributes via PostingsFormatByName — no pre-registration
	// needed because codecs.init() already seeded the registry.
	readState := &codecs.SegmentReadState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fis,
	}
	producer, err := format.FieldsProducer(readState)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}
	defer producer.Close()

	for _, fieldName := range []string{field1Name, field2Name} {
		terms, err := producer.Terms(fieldName)
		if err != nil {
			t.Fatalf("Terms(%q): %v", fieldName, err)
		}
		if terms == nil {
			t.Fatalf("Terms(%q): returned nil", fieldName)
		}
	}
}

// singleTermTerms is a minimal index.Terms implementation that yields exactly
// one term for use in the PerField dispatch test.
type singleTermTerms struct {
	index.TermsBase
	field string
	term  *index.Term
	freq  int
}

func (t *singleTermTerms) GetIterator() (index.TermsEnum, error) {
	return &singleTermEnum{term: t.term, freq: t.freq}, nil
}

func (t *singleTermTerms) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	return t.GetIterator()
}

func (t *singleTermTerms) GetPostingsReader(termText string, flags int) (index.PostingsEnum, error) {
	return nil, nil
}

func (t *singleTermTerms) Size() int64                         { return 1 }
func (t *singleTermTerms) GetDocCount() (int, error)           { return 1, nil }
func (t *singleTermTerms) GetSumDocFreq() (int64, error)       { return 1, nil }
func (t *singleTermTerms) GetSumTotalTermFreq() (int64, error) { return int64(t.freq), nil }
func (t *singleTermTerms) HasFreqs() bool                      { return true }
func (t *singleTermTerms) HasOffsets() bool                    { return false }
func (t *singleTermTerms) HasPositions() bool                  { return false }
func (t *singleTermTerms) HasPayloads() bool                   { return false }
func (t *singleTermTerms) GetMin() (*index.Term, error)        { return t.term, nil }
func (t *singleTermTerms) GetMax() (*index.Term, error)        { return t.term, nil }

// singleTermEnum is a minimal index.TermsEnum that iterates exactly one term.
type singleTermEnum struct {
	index.TermsEnumBase
	term    *index.Term
	freq    int
	advance bool // true once Next() has been called
}

func (e *singleTermEnum) Next() (*index.Term, error) {
	if !e.advance {
		e.advance = true
		e.TermsEnumBase = index.TermsEnumBase{} // keep base consistent
		return e.term, nil
	}
	return nil, nil
}

func (e *singleTermEnum) Term() *index.Term { return e.term }

func (e *singleTermEnum) SeekExact(term *index.Term) (bool, error) {
	if term != nil && e.term != nil && string(term.Bytes.Bytes) == string(e.term.Bytes.Bytes) {
		return true, nil
	}
	return false, nil
}

func (e *singleTermEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	return e.term, nil
}

func (e *singleTermEnum) DocFreq() (int, error)         { return 1, nil }
func (e *singleTermEnum) TotalTermFreq() (int64, error) { return int64(e.freq), nil }

func (e *singleTermEnum) Postings(flags int) (index.PostingsEnum, error) {
	return &singleDocPostingsEnum{docID: 0, freq: e.freq}, nil
}

func (e *singleTermEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

// singleDocPostingsEnum yields exactly one document.
type singleDocPostingsEnum struct {
	index.PostingsEnumBase
	docID int
	freq  int
	pos   int // 0 = before first, 1 = on doc, 2 = exhausted
}

func (e *singleDocPostingsEnum) DocID() int { return e.docID }

func (e *singleDocPostingsEnum) Freq() (int, error) { return e.freq, nil }

func (e *singleDocPostingsEnum) NextDoc() (int, error) {
	if e.pos == 0 {
		e.pos = 1
		return e.docID, nil
	}
	e.pos = 2
	return index.NO_MORE_DOCS, nil
}

func (e *singleDocPostingsEnum) Advance(target int) (int, error) {
	if e.pos < 2 && e.docID >= target {
		e.pos = 1
		return e.docID, nil
	}
	e.pos = 2
	return index.NO_MORE_DOCS, nil
}

func (e *singleDocPostingsEnum) Cost() int64                 { return 1 }
func (e *singleDocPostingsEnum) NextPosition() (int, error)  { return -1, nil }
func (e *singleDocPostingsEnum) StartOffset() (int, error)   { return -1, nil }
func (e *singleDocPostingsEnum) EndOffset() (int, error)     { return -1, nil }
func (e *singleDocPostingsEnum) GetPayload() ([]byte, error) { return nil, nil }
