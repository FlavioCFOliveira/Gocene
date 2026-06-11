package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestIndexWriterOutOfFileDescriptors(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	mock := store.NewMockDirectoryWrapper(dir)

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMergeScheduler(index.NewSerialMergeScheduler())

	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		mock.Close()
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "test document", false)
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("baseline AddDocument(%d): %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("baseline Commit: %v", err)
	}

	mock.SetRandomIOExceptionRateOnOpen(0.1)
	mock.SetRandomIOExceptionRate(0.05)

	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "failing test document", false)
		doc.Add(f)
		_ = writer.AddDocument(doc)
	}

	mock.SetRandomIOExceptionRateOnOpen(0.0)
	mock.SetRandomIOExceptionRate(0.0)

	if err := writer.Commit(); err != nil {
		_ = writer.Rollback()
	}
	_ = writer.Rollback()

	ci, checkErr := index.NewCheckIndex(mock)
	if checkErr != nil {
		t.Fatalf("NewCheckIndex: %v", checkErr)
	}
	status, checkErr := ci.CheckIndex()
	ci.Close()
	if checkErr != nil {
		t.Fatalf("CheckIndex: %v", checkErr)
	}
	if status != nil && status.MissingSegments {
		t.Fatal("CheckIndex: missing segments")
	}

	mock.Close()
}
