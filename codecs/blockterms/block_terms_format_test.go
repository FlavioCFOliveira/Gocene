package blockterms

import "testing"

func TestTermsIndexReaderBase_New(t *testing.T) {
	b := NewTermsIndexReaderBase("path")
	if b == nil {
		t.Fatal("NewTermsIndexReaderBase returned nil")
	}
}

func TestTermsIndexWriterBase_New(t *testing.T) {
	w := NewTermsIndexWriterBase("path")
	if w == nil {
		t.Fatal("NewTermsIndexWriterBase returned nil")
	}
}

func TestFixedGapTermsIndexReader_New(t *testing.T) {
	base := NewTermsIndexReaderBase("path")
	r := NewFixedGapTermsIndexReader(base, 20)
	if r == nil {
		t.Fatal("NewFixedGapTermsIndexReader returned nil")
	}
}

func TestVariableGapTermsIndexWriter_New(t *testing.T) {
	base := NewTermsIndexWriterBase("path")
	w := NewVariableGapTermsIndexWriter(base)
	if w == nil {
		t.Fatal("NewVariableGapTermsIndexWriter returned nil")
	}
}
