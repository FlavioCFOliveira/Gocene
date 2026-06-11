package index_test

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestIndexWriterOnDiskFull_AddDocumentOnDiskFull(t *testing.T) {
	for pass := 0; pass < 2; pass++ {
		doAbort := pass == 1
		diskFree := int64(100)
		for {
			base := store.NewByteBuffersDirectory()
			mock := store.NewMockDirectoryWrapper(base)
			mock.SetMaxSizeInBytes(diskFree)

			config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
			config.SetMergeScheduler(index.NewSerialMergeScheduler())
			writer, err := index.NewIndexWriter(mock, config)
			if err != nil {
				mock.Close()
				t.Fatalf("pass=%d diskFree=%d: NewIndexWriter: %v", pass, diskFree, err)
			}

			hitError := false
			indexExists := false
			for i := 0; i < 200; i++ {
				if err := writer.AddDocument(newDiskFullDoc()); err != nil {
					hitError = true
					break
				}
			}
			if !hitError {
				if err := writer.Commit(); err != nil {
					hitError = true
				} else {
					indexExists = true
				}
			}

			if hitError {
				if doAbort {
					_ = writer.Rollback()
				} else {
					if err := writer.Close(); err != nil {
						mock.SetMaxSizeInBytes(0)
						_ = writer.Close()
					}
				}
				if indexExists {
					r, e := index.OpenDirectoryReader(mock)
					if e != nil {
						t.Fatalf("pass=%d diskFree=%d: cannot open reader: %v", pass, diskFree, e)
					}
					r.Close()
				}
				mock.Close()
				diskFree += 3000
			} else {
				mock.SetMaxSizeInBytes(0)
				if err := writer.Close(); err != nil {
					t.Fatalf("pass=%d diskFree=%d: close: %v", pass, diskFree, err)
				}
				mock.Close()
				break
			}
		}
	}
}

func TestIndexWriterOnDiskFull_CorruptionAfterDiskFull(t *testing.T) {
	base := store.NewByteBuffersDirectory()
	mock := store.NewMockDirectoryWrapper(base)

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMergeScheduler(index.NewSerialMergeScheduler())
	mp := index.NewLogDocMergePolicy()
	mp.SetMergeFactor(2)
	config.SetMergePolicy(mp)

	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		mock.Close()
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	f, err := document.NewTextField("f", "doctor who", false)
	if err != nil {
		t.Fatal(err)
	}
	doc.Add(f)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.DeleteDocuments(index.NewTerm("f", "who")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument after delete: %v", err)
	}

	var didFail1, didFail2 bool
	failTwice := &store.Failure{}
	failTwice.SetDoFail()
	failTwice.SetEval(func(dir *store.MockDirectoryWrapper) error {
		if !didFail1 {
			didFail1 = true
			return errors.New("fake disk full during mergeTerms")
		}
		if !didFail2 {
			didFail2 = true
			return errors.New("fake disk full while writing LiveDocs")
		}
		return nil
	})
	mock.FailOn(failTwice)

	if err := writer.Commit(); err == nil {
		t.Fatal("expected IOException, got nil")
	}
	if !didFail1 && !didFail2 {
		t.Fatal("expected at least one failure to fire")
	}

	failTwice.ClearDoFail()
	_ = writer.Rollback()

	ci, err := index.NewCheckIndex(mock)
	if err != nil {
		t.Fatalf("NewCheckIndex: %v", err)
	}
	status, err := ci.CheckIndex()
	ci.Close()
	if err != nil {
		t.Fatalf("CheckIndex error: %v", err)
	}
	if status != nil && status.MissingSegments {
		t.Fatal("CheckIndex: missing segments")
	}

	mock.Close()
}

func TestIndexWriterOnDiskFull_ImmediateDiskFull(t *testing.T) {
	base := store.NewByteBuffersDirectory()
	mock := store.NewMockDirectoryWrapper(base)

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxBufferedDocs(2)
	config.SetMergeScheduler(index.NewSerialMergeScheduler())

	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		mock.Close()
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("empty Commit: %v", err)
	}

	// Total directory size — set max to this so the next write exceeds it
	files, _ := mock.ListAll()
	var totalSize int64
	for _, fn := range files {
		if sz, ferr := mock.FileLength(fn); ferr == nil {
			totalSize += sz
		}
	}
	if totalSize <= 0 {
		totalSize = 1
	}
	mock.SetMaxSizeInBytes(totalSize)

	ff, err := document.NewTextField("field", "aaa bbb ccc ddd eee fff ggg hhh iii jjj", true)
	if err != nil {
		t.Fatal(err)
	}
	doc := document.NewDocument()
	doc.Add(ff)

	err = writer.AddDocument(doc)
	if err == nil {
		// In Gocene, documents are buffered and only flushed during commit.
		// The disk-full error will surface on commit.
		err = writer.Commit()
	}
	if err == nil {
		t.Fatal("expected error from AddDocument/Commit with full disk, got nil")
	}
	mock.Close()
}

func newDiskFullDoc() *document.Document {
	doc := document.NewDocument()
	f, _ := document.NewTextField("content", "aaa", false)
	doc.Add(f)
	dv, _ := document.NewNumericDocValuesField("numericdv", 1)
	doc.Add(dv)
	doc.Add(document.NewIntPoint("point", 1))
	return doc
}
