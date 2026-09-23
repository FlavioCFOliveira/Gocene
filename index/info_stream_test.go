// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestInfoStream.java
// (Apache Lucene 10.5.0): tests IndexWriter's infostream.

package index

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// funcInfoStream renders the anonymous InfoStream subclasses of the Java test.
type funcInfoStream struct {
	message   func(component, message string)
	isEnabled func(component string) bool
}

func (s *funcInfoStream) Close() error                      { return nil }
func (s *funcInfoStream) Message(component, message string) { s.message(component, message) }
func (s *funcInfoStream) IsEnabled(component string) bool   { return s.isEnabled(component) }

// TestInfoStreamTestPointsOff: we shouldn't have test points unless we ask.
func TestInfoStreamTestPointsOff(t *testing.T) {
	dir := newDirectory()
	iwc := NewIndexWriterConfigWithAnalyzer(nil)
	iwc.SetInfoStream(&funcInfoStream{
		message: func(component, message string) {
			if component == "TP" {
				t.Errorf("assertFalse(\"TP\".equals(component)): %s", message)
			}
		},
		isEnabled: func(component string) bool {
			if component == "TP" {
				t.Errorf("assertFalse(\"TP\".equals(component))")
			}
			return true
		},
	})
	iw, err := NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	if _, err := iw.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	if err := iw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := dir.Close(); err != nil {
		t.Fatalf("close dir: %v", err)
	}
}

// TestInfoStreamTestPointsOn: but they should work when we need.
//
// The Java test overrides the protected IndexWriter#isEnableTestPoints() in an
// anonymous IndexWriter subclass. Gocene's IndexWriter does not declare that
// member, so the override cannot be rendered.
func TestInfoStreamTestPointsOn(t *testing.T) {
	t.Fatal("org.apache.lucene.index.IndexWriter#isEnableTestPoints() is not ported: " +
		"TestInfoStream.testTestPointsOn overrides it to emit the \"TP\" test points")
}

func TestInfoStreamMergeSmallIndex(t *testing.T) {
	// examine info stream output from IW, MP, MS
	iwc := NewIndexWriterConfigWithAnalyzer(nil)
	components := map[string]bool{"IW": true, "MP": true, "MS": true, "BD": true, "BU": true}
	var mu sync.Mutex
	var infoStream []string
	iwc.SetInfoStream(&funcInfoStream{
		message: func(component, message string) {
			mu.Lock()
			defer mu.Unlock()
			if components[component] {
				infoStream = append(infoStream, fmt.Sprintf("[%s] %s\n", component, message))
			}
		},
		isEnabled: func(component string) bool {
			return components[component]
		},
	})
	func() {
		dir := newDirectory()
		defer dir.Close()
		iw, err := NewIndexWriter(dir, iwc)
		if err != nil {
			t.Fatalf("new IndexWriter: %v", err)
		}
		defer iw.Close()
		if _, err := iw.AddDocument(document.NewDocument()); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
		if _, err := iw.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		if _, err := iw.AddDocument(document.NewDocument()); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
		if _, err := iw.AddDocument(document.NewDocument()); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
		func() {
			reader, err := OpenDirectoryReaderFromWriter(iw)
			if err != nil {
				t.Fatalf("DirectoryReader.open(iw): %v", err)
			}
			defer reader.Close()
			// Java: assertTrue(iw.tryDeleteDocument(reader, 2) > 0). Gocene's
			// IndexWriter.TryDeleteDocument takes only the docID and reports
			// success as a boolean.
			deleted, err := iw.TryDeleteDocument(2)
			if err != nil {
				t.Fatalf("tryDeleteDocument: %v", err)
			}
			if !deleted {
				t.Fatal("assertTrue(iw.tryDeleteDocument(reader, 2) > 0)")
			}
		}()
		if _, err := iw.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		if err := iw.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	}()
	foundInit := false
	flushedCount := 0
	mergedCount := 0
	for _, message := range infoStream {
		// we don't want to be printing full diagnostics every time a segment is
		// mentioned in the log, only when it is first flushed
		if strings.Contains(message, "diagnostics") {
			if strings.HasPrefix(message, "[IW] publishFlushedSegment") {
				flushedCount++
			} else if strings.HasPrefix(message, "[IW] merged new segment") {
				mergedCount++
			} else {
				t.Fatalf("message contains diagnostics: %s", message)
			}
		}
		if strings.Contains(message, "init segments") {
			if message != "[IW] init segments: \n" {
				t.Fatalf("expected %q, got %q", "[IW] init segments: \n", message)
			}
			foundInit = true
		}
	}
	if !foundInit {
		t.Fatal("init message not found")
	}
	if flushedCount != 2 {
		t.Fatalf("flushedCount: expected 2, got %d", flushedCount)
	}
	if mergedCount != 1 {
		t.Fatalf("mergedCount: expected 1, got %d", mergedCount)
	}
}
