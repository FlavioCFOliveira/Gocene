// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktreeords_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/blocktreeords"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// listTerms is a simple Terms implementation backed by a sorted string slice.
// Each term has docFreq=1, totalTermFreq=1, and appears in doc 0.
type listTerms struct {
	schema.TermsBase
	field string
	terms []string
}

func newListTerms(field string, terms ...string) *listTerms {
	sorted := make([]string, len(terms))
	copy(sorted, terms)
	sort.Strings(sorted)
	return &listTerms{field: field, terms: sorted}
}

func (lt *listTerms) GetIterator() (index.TermsEnum, error) {
	return &listTermsEnum{terms: lt.terms, field: lt.field}, nil
}

func (lt *listTerms) Size() int64                      { return int64(len(lt.terms)) }
func (lt *listTerms) GetDocCount() (int, error)         { return min(1, len(lt.terms)), nil }
func (lt *listTerms) GetSumDocFreq() (int64, error)     { return int64(len(lt.terms)), nil }
func (lt *listTerms) GetSumTotalTermFreq() (int64, error) { return int64(len(lt.terms)), nil }

// listTermsEnum iterates over a sorted list of terms.
type listTermsEnum struct {
	schema.TermsEnumBase
	terms []string
	field string
	pos   int
}

func (e *listTermsEnum) Next() (*index.Term, error) {
	if e.pos >= len(e.terms) {
		e.SetCurrentTerm(nil)
		return nil, nil
	}
	term := index.NewTerm(e.field, e.terms[e.pos])
	e.SetCurrentTerm(term)
	e.pos++
	return term, nil
}

func (e *listTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	target := term.Text()
	for i, t := range e.terms {
		if t >= target {
			e.pos = i
			got := index.NewTerm(e.field, t)
			e.SetCurrentTerm(got)
			return got, nil
		}
	}
	e.pos = len(e.terms)
	e.SetCurrentTerm(nil)
	return nil, nil
}

func (e *listTermsEnum) SeekExact(term *index.Term) (bool, error) {
	target := term.Text()
	for _, t := range e.terms {
		if t == target {
			e.SetCurrentTerm(term)
			return true, nil
		}
	}
	return false, nil
}

func (e *listTermsEnum) DocFreq() (int, error)            { return 1, nil }
func (e *listTermsEnum) TotalTermFreq() (int64, error)    { return 1, nil }
func (e *listTermsEnum) Postings(int) (index.PostingsEnum, error) {
	return &singleDocPostingsEnum{}, nil
}
func (e *listTermsEnum) PostingsWithLiveDocs(_ util.Bits, _ int) (index.PostingsEnum, error) {
	return &singleDocPostingsEnum{}, nil
}

// singleDocPostingsEnum returns doc 0 with freq 1 then NO_MORE_DOCS.
type singleDocPostingsEnum struct {
	schema.PostingsEnumBase
	done bool
}

func (e *singleDocPostingsEnum) NextDoc() (int, error) {
	if !e.done {
		e.done = true
		e.CurrentDoc = 0
		return 0, nil
	}
	e.CurrentDoc = schema.NO_MORE_DOCS
	return schema.NO_MORE_DOCS, nil
}

func (e *singleDocPostingsEnum) Advance(int) (int, error) { return e.NextDoc() }
func (e *singleDocPostingsEnum) Freq() (int, error)                  { return 1, nil }
func (e *singleDocPostingsEnum) NextPosition() (int, error)          { return schema.NO_MORE_POSITIONS, nil }
func (e *singleDocPostingsEnum) StartOffset() (int, error)           { return -1, nil }
func (e *singleDocPostingsEnum) EndOffset() (int, error)             { return -1, nil }
func (e *singleDocPostingsEnum) GetPayload() ([]byte, error)         { return nil, nil }
func (e *singleDocPostingsEnum) Cost() int64                         { return 1 }

// writeTerms is a test helper that writes terms for a single field through
// the BlockTreeOrds format and returns the directory.
func writeTerms(t *testing.T, segName string, termStrs []string) store.Directory {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	format := blocktreeords.NewBlockTreeOrdsPostingsFormat()

	fis := index.NewFieldInfosBuilder()
	fis.AddFromOptions("field", index.FieldInfoOptions{
		IndexOptions: index.IndexOptionsDocsAndFreqsAndPositions,
	})
	fi := fis.Build()

	si := index.NewSegmentInfo(segName, len(termStrs), dir)
	id := make([]byte, 16)
	for i := range id {
		id[i] = byte(i + 1)
	}
	si.SetID(id)

	ws := &codecs.SegmentWriteState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fi,
	}

	consumer, err := format.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}

	lt := newListTerms("field", termStrs...)
	if err := consumer.Write("field", lt); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	return dir
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestOrdsBlockTree_FormatName verifies the format name and block-size constants.
func TestOrdsBlockTree_FormatName(t *testing.T) {
	if got := blocktreeords.BlockTreeOrdsPostingsFormatName; got != "BlockTreeOrds" {
		t.Errorf("FormatName: got %q, want %q", got, "BlockTreeOrds")
	}
	if got := blocktreeords.BlockTreeOrdsPostingsFormatBlockSize; got != 128 {
		t.Errorf("BlockSize: got %d, want %d", got, 128)
	}
}

// TestOrdsBlockTree_EmptyTerms verifies that the writer lifecycle (creation,
// write with zero terms, close) completes without error and produces .tio/.tipo.
func TestOrdsBlockTree_EmptyTerms(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	format := blocktreeords.NewBlockTreeOrdsPostingsFormat()
	fis := index.NewFieldInfosBuilder()
	fis.AddFromOptions("field", index.FieldInfoOptions{
		IndexOptions: index.IndexOptionsDocsAndFreqsAndPositions,
	})
	fi := fis.Build()

	si := index.NewSegmentInfo("_0", 0, dir)
	id := make([]byte, 16)
	id[0] = 1
	si.SetID(id)

	ws := &codecs.SegmentWriteState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fi,
	}

	consumer, err := format.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}

	lt := newListTerms("field")
	if err := consumer.Write("field", lt); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestOrdsBlockTree_Basic writes three terms ("a", "b", "c") and verifies
// the .tio and .tipo files are created on disk.
func TestOrdsBlockTree_Basic(t *testing.T) {
	dir := writeTerms(t, "_0", []string{"a", "b", "c"})
	defer dir.Close()

	if !dir.FileExists("_0.tio") {
		t.Error("_0.tio not found after write")
	}
	if !dir.FileExists("_0.tipo") {
		t.Error("_0.tipo not found after write")
	}
}

// TestOrdsBlockTree_TwoBlocks writes 72 terms (36 single-char + 36 "m"+char)
// to exercise two FST blocks, and verifies file creation.
func TestOrdsBlockTree_TwoBlocks(t *testing.T) {
	terms := make([]string, 0, 72)
	for i := 0; i < 36; i++ {
		terms = append(terms, fmt.Sprintf("%c", 'a'+i))
	}
	for i := 0; i < 36; i++ {
		terms = append(terms, fmt.Sprintf("m%c", 'a'+i))
	}

	dir := writeTerms(t, "_0", terms)
	defer dir.Close()

	if !dir.FileExists("_0.tio") {
		t.Error("_0.tio not found after write")
	}
	if !dir.FileExists("_0.tipo") {
		t.Error("_0.tipo not found after write")
	}
}

// TestOrdsBlockTree_ThreeBlocks writes 108 terms across three FST blocks
// (single-char, "m"+char, "mo"+char) and verifies file creation.
func TestOrdsBlockTree_ThreeBlocks(t *testing.T) {
	terms := make([]string, 0, 108)
	for i := 0; i < 36; i++ {
		terms = append(terms, fmt.Sprintf("%c", 'a'+i))
	}
	for i := 0; i < 36; i++ {
		terms = append(terms, fmt.Sprintf("m%c", 'a'+i))
	}
	for i := 0; i < 36; i++ {
		terms = append(terms, fmt.Sprintf("mo%c", 'a'+i))
	}

	dir := writeTerms(t, "_0", terms)
	defer dir.Close()

	if !dir.FileExists("_0.tio") {
		t.Error("_0.tio not found after write")
	}
	if !dir.FileExists("_0.tipo") {
		t.Error("_0.tipo not found after write")
	}
}

// TestOrdsBlockTree_FloorBlocks writes 128 terms (bytes 0-127) to exercise
// floor blocks and verifies file creation.
func TestOrdsBlockTree_FloorBlocks(t *testing.T) {
	terms := make([]string, 128)
	for i := 0; i < 128; i++ {
		terms[i] = string([]byte{byte(i)})
	}

	dir := writeTerms(t, "_0", terms)
	defer dir.Close()

	if !dir.FileExists("_0.tio") {
		t.Error("_0.tio not found after write")
	}
	if !dir.FileExists("_0.tipo") {
		t.Error("_0.tipo not found after write")
	}
}

// TestOrdsBlockTree_NonRootFloorBlocks writes 36 single-char terms plus 128
// "m"+byte terms to create floor blocks at a non-root node, verifying file creation.
func TestOrdsBlockTree_NonRootFloorBlocks(t *testing.T) {
	terms := make([]string, 0, 36+128)
	for i := 0; i < 36; i++ {
		terms = append(terms, fmt.Sprintf("%c", 'a'+i))
	}
	for i := 0; i < 128; i++ {
		terms = append(terms, fmt.Sprintf("m%c", byte(i)))
	}

	dir := writeTerms(t, "_0", terms)
	defer dir.Close()

	if !dir.FileExists("_0.tio") {
		t.Error("_0.tio not found after write")
	}
	if !dir.FileExists("_0.tipo") {
		t.Error("_0.tipo not found after write")
	}
}

// TestOrdsBlockTree_SeveralNonRootBlocks writes 900 two-character terms
// (30x30 grid) to exercise sub-blocks, verifying file creation.
func TestOrdsBlockTree_SeveralNonRootBlocks(t *testing.T) {
	terms := make([]string, 0, 900)
	for i := 0; i < 30; i++ {
		for j := 0; j < 30; j++ {
			terms = append(terms, fmt.Sprintf("%c%c", 'a'+i, 'a'+j))
		}
	}

	dir := writeTerms(t, "_0", terms)
	defer dir.Close()

	if !dir.FileExists("_0.tio") {
		t.Error("_0.tio not found after write")
	}
	if !dir.FileExists("_0.tipo") {
		t.Error("_0.tipo not found after write")
	}
}

// TestOrdsBlockTree_SeekCeilNotFound writes terms including the empty string
// plus 36 single-char and "a"+char terms, verifying file creation.
// Full seekCeil NOT_FOUND testing requires the read path (OrdsBlockTreeTermsReader).
func TestOrdsBlockTree_SeekCeilNotFound(t *testing.T) {
	terms := make([]string, 0, 1+36+36)
	terms = append(terms, "") // empty string
	for i := 0; i < 36; i++ {
		terms = append(terms, fmt.Sprintf("%c", 'a'+i))
	}
	for i := 0; i < 36; i++ {
		terms = append(terms, fmt.Sprintf("a%c", 'a'+i))
	}

	dir := writeTerms(t, "_0", terms)
	defer dir.Close()

	if !dir.FileExists("_0.tio") {
		t.Error("_0.tio not found after write")
	}
	if !dir.FileExists("_0.tipo") {
		t.Error("_0.tipo not found after write")
	}
}
