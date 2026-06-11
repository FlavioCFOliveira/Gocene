package index_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestIndexWriterExceptions2_Basics(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	mock := store.NewMockDirectoryWrapper(dir)
	mock.SetRandomIOExceptionRate(0.05)
	mock.SetRandomIOExceptionRateOnOpen(0.01)

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMergeScheduler(index.NewSerialMergeScheduler())

	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		mock.Close()
		t.Fatalf("NewIndexWriter: %v", err)
	}

	const numDocs = 100
	allowAlreadyClosed := false

	for i := 0; i < numDocs; i++ {
		doc := newExceptions2Doc(t, i)

		addErr := writer.AddDocument(doc)
		if addErr != nil {
			var isAce *index.AlreadyClosedException
			if errors.As(addErr, &isAce) {
				if !allowAlreadyClosed {
					t.Fatalf("unexpected AlreadyClosedException at doc %d: %v", i, addErr)
				}
				allowAlreadyClosed = false
				config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
				config2.SetMergeScheduler(index.NewSerialMergeScheduler())
				writer, err = index.NewIndexWriter(mock, config2)
				if err != nil {
					t.Fatalf("reopen writer at doc %d: %v", i, err)
				}
				if err := writer.AddDocument(doc); err != nil {
					t.Fatalf("re-add doc %d: %v", i, err)
				}
			} else {
				allowAlreadyClosed = true
			}
		} else {
			if i%4 == 0 {
				_ = writer.DeleteDocuments(index.NewTerm("id", strconv.Itoa(i)))
			}
		}

		if i%10 == 0 {
			mock.SetRandomIOExceptionRate(0.0)
			mock.SetRandomIOExceptionRateOnOpen(0.0)

			commitErr := writer.Commit()
			if commitErr != nil {
				var ace *index.AlreadyClosedException
				if errors.As(commitErr, &ace) {
					allowAlreadyClosed = true
				}
			}
			_ = writer.Rollback()
			ci, checkErr := index.NewCheckIndex(mock)
			if checkErr != nil {
				t.Fatalf("doc %d: NewCheckIndex: %v", i, checkErr)
			}
			status, checkErr := ci.CheckIndex()
			ci.Close()
			if checkErr != nil {
				t.Fatalf("doc %d: CheckIndex error: %v", i, checkErr)
			}
			if status != nil && status.MissingSegments {
				t.Fatalf("doc %d: CheckIndex: missing segments", i)
			}
			config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
			config2.SetMergeScheduler(index.NewSerialMergeScheduler())
			writer, err = index.NewIndexWriter(mock, config2)
			if err != nil {
				t.Fatalf("reopen writer at doc %d: %v", i, err)
			}

			mock.SetRandomIOExceptionRate(0.05)
			mock.SetRandomIOExceptionRateOnOpen(0.01)
		}
	}

	mock.SetRandomIOExceptionRate(0.0)
	mock.SetRandomIOExceptionRateOnOpen(0.0)

	_ = writer.Rollback()

	ci, err := index.NewCheckIndex(mock)
	if err != nil {
		t.Fatalf("final NewCheckIndex: %v", err)
	}
	status, err := ci.CheckIndex()
	ci.Close()
	if err != nil {
		t.Fatalf("final CheckIndex: %v", err)
	}
	if status != nil && status.MissingSegments {
		t.Fatal("final CheckIndex: missing segments")
	}

	mock.Close()
}

func newExceptions2Doc(t *testing.T, id int) *document.Document {
	t.Helper()
	s := strconv.Itoa(id)
	doc := document.NewDocument()

	idField, err := document.NewStringField("id", s, false)
	if err != nil {
		t.Fatalf("NewStringField(id) error = %v", err)
	}
	doc.Add(idField)

	dv, err := document.NewNumericDocValuesField("dv", int64(id))
	if err != nil {
		t.Fatalf("NewNumericDocValuesField(dv) error = %v", err)
	}
	doc.Add(dv)

	dv2, err := document.NewBinaryDocValuesField("dv2", []byte(s))
	if err != nil {
		t.Fatalf("NewBinaryDocValuesField(dv2) error = %v", err)
	}
	doc.Add(dv2)

	dv4, err := document.NewSortedSetDocValuesField("dv4", [][]byte{
		[]byte(s), []byte(strconv.Itoa(id - 1)),
	})
	if err != nil {
		t.Fatalf("NewSortedSetDocValuesField(dv4) error = %v", err)
	}
	doc.Add(dv4)

	dv5, err := document.NewSortedNumericDocValuesField("dv5", []int64{
		int64(id), int64(id - 1),
	})
	if err != nil {
		t.Fatalf("NewSortedNumericDocValuesField(dv5) error = %v", err)
	}
	doc.Add(dv5)

	text1, err := document.NewTextField("text1", "the quick brown fox "+s, false)
	if err != nil {
		t.Fatalf("NewTextField(text1) error = %v", err)
	}
	doc.Add(text1)

	stored1a, err := document.NewStoredField("stored1", "foo")
	if err != nil {
		t.Fatalf("NewStoredField(stored1=foo) error = %v", err)
	}
	doc.Add(stored1a)
	stored1b, err := document.NewStoredField("stored1", "bar")
	if err != nil {
		t.Fatalf("NewStoredField(stored1=bar) error = %v", err)
	}
	doc.Add(stored1b)

	payloads, err := document.NewTextField("text_payloads", "lorem ipsum "+s, false)
	if err != nil {
		t.Fatalf("NewTextField(text_payloads) error = %v", err)
	}
	doc.Add(payloads)

	vectors, err := document.NewTextField("text_vectors", "dolor sit "+s, false)
	if err != nil {
		t.Fatalf("NewTextField(text_vectors) error = %v", err)
	}
	doc.Add(vectors)

	doc.Add(document.NewIntPoint("point", int32(id)))
	doc.Add(document.NewIntPoints("point2d", int32(id), int32(-id)))

	_ = util.NewBytesRef
	return doc
}
