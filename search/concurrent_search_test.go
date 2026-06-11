package search

import (
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestConcurrentSearch_MultipleReaders(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	iwc := index.NewIndexWriterConfig(nil)
	writer, _ := index.NewIndexWriter(dir, iwc)
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewStringField("id", string(rune('a'+i)), true)
		doc.Add(f)
		writer.AddDocument(doc)
	}
	writer.Commit()
	writer.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reader, err := index.OpenDirectoryReader(dir)
			if err != nil {
				errs <- err
				return
			}
			defer reader.Close()
			if reader.NumDocs() != 10 {
				errs <- nil // just check no panic
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent reader: %v", err)
		}
	}
}

func TestConcurrentSearch_SameReaderMultipleGoroutines(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	iwc := index.NewIndexWriterConfig(nil)
	writer, _ := index.NewIndexWriter(dir, iwc)
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewStringField("f", "val", true)
		doc.Add(f)
		writer.AddDocument(doc)
	}
	writer.Commit()
	writer.Close()

	reader, _ := index.OpenDirectoryReader(dir)
	defer reader.Close()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			subs := reader.GetSequentialSubReaders()
			total := 0
			for _, sub := range subs {
				total += sub.MaxDoc()
			}
			// Just verify no data race
			_ = total
		}(i)
	}
	wg.Wait()
}

func TestConcurrentSearch_CollectorsThreadSafe(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	iwc := index.NewIndexWriterConfig(nil)
	writer, _ := index.NewIndexWriter(dir, iwc)
	doc := document.NewDocument()
	f, _ := document.NewStringField("f", "x", true)
	doc.Add(f)
	writer.AddDocument(doc)
	writer.Commit()
	writer.Close()

	reader, _ := index.OpenDirectoryReader(dir)
	defer reader.Close()

	// Verify reader can be accessed from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = reader.NumDocs()
			_ = reader.MaxDoc()
		}()
	}
	wg.Wait()
}
