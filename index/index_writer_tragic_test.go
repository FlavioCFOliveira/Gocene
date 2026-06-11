package index_test

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

var errSimulatedFatal = errors.New("simulated fatal I/O error during segment write")

func TestIndexWriter_TragicEvent(t *testing.T) {
	base := store.NewByteBuffersDirectory()
	mock := store.NewMockDirectoryWrapper(base)

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMergeScheduler(index.NewSerialMergeScheduler())

	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		mock.Close()
		t.Fatalf("NewIndexWriter: %v", err)
	}

	failOnCreate := &store.Failure{}
	failOnCreate.SetDoFail()
	failOnCreate.SetEval(func(dir *store.MockDirectoryWrapper) error {
		return errSimulatedFatal
	})
	mock.FailOn(failOnCreate)

	doc := document.NewDocument()
	f, err := document.NewTextField("field", "test content", false)
	if err != nil {
		t.Fatal(err)
	}
	doc.Add(f)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	err = writer.Commit()
	if err == nil {
		t.Fatal("expected error from Commit with failure injected, got nil")
	}
	t.Logf("Got expected commit error: %v", err)

	mock.Close()
}

func TestIndexWriter_TragicErrorIntegration(t *testing.T) {
	base := store.NewByteBuffersDirectory()
	mock := store.NewMockDirectoryWrapper(base)

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMergeScheduler(index.NewSerialMergeScheduler())

	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		mock.Close()
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	f, err := document.NewTextField("field", "some content", false)
	if err != nil {
		t.Fatal(err)
	}
	doc.Add(f)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	failOnCreate := &store.Failure{}
	failOnCreate.SetDoFail()
	failOnCreate.SetEval(func(dir *store.MockDirectoryWrapper) error {
		return errSimulatedFatal
	})
	mock.FailOn(failOnCreate)

	err = writer.Commit()
	if err == nil {
		t.Fatal("expected error from Commit with failure injected, got nil")
	}
	t.Logf("Got expected commit error: %v", err)

	mock.Close()
}
