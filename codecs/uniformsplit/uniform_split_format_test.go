package uniformsplit

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestBlockHeader_New(t *testing.T) {
	h := NewBlockHeader(10, 0, 0, 0, 0, 0)
	if h == nil {
		t.Fatal("NewBlockHeader returned nil")
	}
}

func TestBlockLine_New(t *testing.T) {
	l := NewBlockLine(NewTermBytes(0, util.NewBytesRef([]byte("term"))), 0)
	if l == nil {
		t.Fatal("NewBlockLine returned nil")
	}
}

func TestFieldMetadata_New(t *testing.T) {
	m, err := NewFieldMetadata(index.NewFieldInfo("f", 0, index.DefaultFieldInfoOptions()), 100)
	if err != nil {
		t.Fatalf("NewFieldMetadata: %v", err)
	}
	if m == nil {
		t.Fatal("NewFieldMetadata returned nil")
	}
}

func TestBlockWriter_New(t *testing.T) {
	out, err := store.NewByteBuffersDirectory().CreateOutput("block", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	w := NewBlockWriter(out, 32, 3, nil)
	if w == nil {
		t.Fatal("NewBlockWriter returned nil")
	}
}
