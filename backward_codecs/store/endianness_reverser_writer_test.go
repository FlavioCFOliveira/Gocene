package store

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestEndiannessReverser_WriteBytes_RoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	ctx := store.IOContext{Context: store.ContextRead}
	out, err := dir.CreateOutput("test_file", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	data := []byte("hello endianness")
	if err := out.WriteBytes(data); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	out.Close()

	in, err := dir.OpenInput("test_file", ctx)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()
	buf := make([]byte, len(data))
	in.ReadBytes(buf)
	if string(buf) != string(data) {
		t.Fatalf("data mismatch: %q != %q", buf, data)
	}
}

func TestEndiannessReverser_ListAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	ctx := store.IOContext{Context: store.ContextRead}
	out, _ := dir.CreateOutput("endo_test", ctx)
	out.WriteBytes([]byte("data"))
	out.Close()
	files, _ := dir.ListAll()
	found := false
	for _, f := range files {
		if f == "endo_test" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("file not found after write")
	}
}
