// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriterConfig.java
// (Apache Lucene 10.5.0).
//
// Java's reflection over IndexWriterConfig.class.getDeclaredMethods() is
// rendered by parsing the index package sources with go/parser and
// collecting the methods declared on the named receiver type.

package index_test

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// mySimilarity is the private MySimilarity: it does not implement anything,
// it is used only for type checking on IndexWriterConfig.
type mySimilarity struct {
	*search.ClassicSimilarity
}

// declaredMethods renders Class.getDeclaredMethods() for a Go type of the
// index package: it maps each method declared on receiver (value or pointer)
// to the source text of its result list.
func declaredMethods(t *testing.T, receiver string) map[string]string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	fset := token.NewFileSet()
	methods := make(map[string]string)
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			recv := fn.Recv.List[0].Type
			if star, ok := recv.(*ast.StarExpr); ok {
				recv = star.X
			}
			ident, ok := recv.(*ast.Ident)
			if !ok || ident.Name != receiver {
				continue
			}
			var results strings.Builder
			if fn.Type.Results != nil {
				for i, field := range fn.Type.Results.List {
					if i > 0 {
						results.WriteString(", ")
					}
					if err := printer.Fprint(&results, fset, field.Type); err != nil {
						t.Fatalf("print: %v", err)
					}
				}
			}
			methods[fn.Name.Name] = results.String()
		}
	}
	return methods
}

func TestIndexWriterConfigDefaults(t *testing.T) {
	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	if _, ok := conf.GetAnalyzer().(*testanalysis.MockAnalyzer); !ok {
		t.Fatalf("getAnalyzer().getClass(): expected MockAnalyzer, got %T", conf.GetAnalyzer())
	}
	if conf.GetIndexCommit() != nil {
		t.Fatalf("getIndexCommit(): expected null, got %v", conf.GetIndexCommit())
	}
	if _, ok := conf.GetIndexDeletionPolicy().(*index.KeepOnlyLastCommitDeletionPolicy); !ok {
		t.Fatalf("getIndexDeletionPolicy().getClass(): got %T", conf.GetIndexDeletionPolicy())
	}
	if _, ok := conf.GetMergeScheduler().(*index.ConcurrentMergeScheduler); !ok {
		t.Fatalf("getMergeScheduler().getClass(): got %T", conf.GetMergeScheduler())
	}
	if conf.GetOpenMode() != index.CreateOrAppend {
		t.Fatalf("getOpenMode(): expected CREATE_OR_APPEND, got %v", conf.GetOpenMode())
	}
	// we don't need to assert this, it should be unspecified
	if any(search.GetDefaultSimilarity()) != any(conf.GetSimilarity()) {
		t.Fatalf("IndexSearcher.getDefaultSimilarity() == conf.getSimilarity(): %v != %v",
			search.GetDefaultSimilarity(), conf.GetSimilarity())
	}
	if conf.GetRAMBufferSizeMB() != index.DefaultRAMBufferSizeMB {
		t.Fatalf("getRAMBufferSizeMB(): got %v", conf.GetRAMBufferSizeMB())
	}
	if conf.GetMaxBufferedDocs() != index.DefaultMaxBufferedDocs {
		t.Fatalf("getMaxBufferedDocs(): got %d", conf.GetMaxBufferedDocs())
	}
	if conf.GetReaderPooling() != index.DefaultReaderPooling {
		t.Fatalf("getReaderPooling(): got %v", conf.GetReaderPooling())
	}
	if conf.GetMergedSegmentWarmer() != nil {
		t.Fatalf("getMergedSegmentWarmer(): expected null, got %v", conf.GetMergedSegmentWarmer())
	}
	if _, ok := conf.GetMergePolicy().(*index.TieredMergePolicy); !ok {
		t.Fatalf("getMergePolicy().getClass(): got %T", conf.GetMergePolicy())
	}
	if _, ok := conf.GetFlushPolicy().(*index.FlushByRamOrCountsPolicy); !ok {
		t.Fatalf("getFlushPolicy().getClass(): got %T", conf.GetFlushPolicy())
	}
	if conf.GetRAMPerThreadHardLimitMB() != index.DefaultRAMPerThreadHardLimitMB {
		t.Fatalf("getRAMPerThreadHardLimitMB(): got %d", conf.GetRAMPerThreadHardLimitMB())
	}
	defaultCodec, err := codecs.GetDefault()
	if err != nil {
		t.Fatalf("Codec.getDefault(): %v", err)
	}
	if any(defaultCodec) != any(conf.GetCodec()) {
		t.Fatalf("getCodec(): expected %v, got %v", defaultCodec, conf.GetCodec())
	}
	if any(util.DefaultInfoStream()) != any(conf.GetInfoStream()) {
		t.Fatalf("getInfoStream(): expected %v, got %v", util.DefaultInfoStream(), conf.GetInfoStream())
	}
	if conf.GetUseCompoundFile() != index.DefaultUseCompoundFileSystem {
		t.Fatalf("getUseCompoundFile(): got %v", conf.GetUseCompoundFile())
	}
	if !conf.IsCheckPendingFlushOnUpdate() {
		t.Fatal("isCheckPendingFlushOnUpdate() is false")
	}
	// Sanity check - validate that all getters are covered.
	getters := map[string]bool{}
	for _, g := range []string{
		"getAnalyzer", "getIndexCommit", "getIndexDeletionPolicy", "getMaxFieldLength",
		"getMergeScheduler", "getOpenMode", "getSimilarity", "getWriteLockTimeout",
		"getDefaultWriteLockTimeout", "getMaxBufferedDeleteTerms", "getRAMBufferSizeMB",
		"getMaxBufferedDocs", "getIndexingChain", "getMergedSegmentWarmer", "getMergePolicy",
		"getReaderPooling", "getIndexerThreadPool", "getFlushPolicy", "getRAMPerThreadHardLimitMB",
		"getCodec", "getInfoStream", "getUseCompoundFile", "isCheckPendingFlushOnUpdate",
		"getSoftDeletesField",
	} {
		// Go transliteration of the Java method name.
		getters[strings.ToUpper(g[:1])+g[1:]] = true
	}

	for m := range declaredMethods(t, "IndexWriterConfig") {
		if strings.HasPrefix(m, "Get") && !getters[m] {
			t.Errorf("method %s is not tested for defaults", m)
		}
	}
}

func TestIndexWriterConfigSettersChaining(t *testing.T) {
	// Ensures that every setter returns IndexWriterConfig to allow chaining.
	allSetters := map[string]bool{}
	for m, results := range declaredMethods(t, "IndexWriterConfig") {
		if strings.HasPrefix(m, "Set") {
			allSetters[m] = true
			if results != "*IndexWriterConfig" {
				t.Errorf("method %s does not return IndexWriterConfig", m)
			}
		}
	}
	// Setters of LiveIndexWriterConfig reach IndexWriterConfig either overridden
	// (Java's synthetic bridge methods) or, in Go, promoted with the
	// LiveIndexWriterConfig return type; every one must be overridden.
	for m := range declaredMethods(t, "LiveIndexWriterConfig") {
		if strings.HasPrefix(m, "Set") && !allSetters[m] {
			t.Errorf("setter method not overridden by IndexWriterConfig: %s", m)
		}
	}
}

func TestIndexWriterConfigReuse(t *testing.T) {
	dir := newDirectory()
	// test that IWC cannot be reused across two IWs
	conf := index.NewIndexWriterConfigWithAnalyzer(nil)
	mustClose(t, newRandomIndexWriterWithConfig(t, dir, conf))

	// this should fail
	if w, err := newRandomIndexWriterOrError(dir, conf); err == nil {
		mustClose(t, w)
		t.Fatal("expected IllegalStateException reusing an IndexWriterConfig across two IndexWriters")
	}

	mustClose(t, dir)
}

func TestIndexWriterConfigOverrideGetters(t *testing.T) {
	// Test that IndexWriterConfig overrides all getters, so that javadocs
	// contain all methods for the users. Also, ensures that IndexWriterConfig
	// doesn't declare getters that are not declared on LiveIWC.
	liveGetters := map[string]bool{}
	for m := range declaredMethods(t, "LiveIndexWriterConfig") {
		if strings.HasPrefix(m, "Get") {
			liveGetters[m] = true
		}
	}

	for m := range declaredMethods(t, "IndexWriterConfig") {
		if strings.HasPrefix(m, "Get") && !liveGetters[m] {
			t.Errorf("method %s not declared on LiveIndexWriterConfig", m)
		}
	}
}

func TestIndexWriterConfigConstants(t *testing.T) {
	// Tests that the values of the constants does not change
	if index.DisableAutoFlush != -1 {
		t.Errorf("DISABLE_AUTO_FLUSH: got %d", index.DisableAutoFlush)
	}
	if index.DefaultMaxBufferedDeleteTerms != index.DisableAutoFlush {
		t.Errorf("DEFAULT_MAX_BUFFERED_DELETE_TERMS: got %d", index.DefaultMaxBufferedDeleteTerms)
	}
	if index.DefaultMaxBufferedDocs != index.DisableAutoFlush {
		t.Errorf("DEFAULT_MAX_BUFFERED_DOCS: got %d", index.DefaultMaxBufferedDocs)
	}
	if index.DefaultRAMBufferSizeMB != 16.0 {
		t.Errorf("DEFAULT_RAM_BUFFER_SIZE_MB: got %v", index.DefaultRAMBufferSizeMB)
	}
	if index.DefaultReaderPooling != true {
		t.Errorf("DEFAULT_READER_POOLING: got %v", index.DefaultReaderPooling)
	}
	if index.DefaultUseCompoundFileSystem != true {
		t.Errorf("DEFAULT_USE_COMPOUND_FILE_SYSTEM: got %v", index.DefaultUseCompoundFileSystem)
	}
}

func TestIndexWriterConfigToString(t *testing.T) {
	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	if _, ok := any(conf).(interface{ String() string }); !ok {
		t.Fatal("org.apache.lucene.index.IndexWriterConfig#toString() is not ported")
	}
	str := any(conf).(interface{ String() string }).String()
	typ := reflect.TypeOf(*conf)
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Anonymous {
			// the embedded LiveIndexWriterConfig is Java's superclass, not a field
			continue
		}
		if f.Name == "indexingChain" || f.Name == "inUseByIndexWriter" {
			continue
		}
		if !strings.Contains(str, f.Name) {
			t.Errorf("%s not found in toString", f.Name)
		}
	}
}

func TestIndexWriterConfigInvalidValues(t *testing.T) {
	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())

	// Test IndexDeletionPolicy
	if _, ok := conf.GetIndexDeletionPolicy().(*index.KeepOnlyLastCommitDeletionPolicy); !ok {
		t.Fatalf("getIndexDeletionPolicy().getClass(): got %T", conf.GetIndexDeletionPolicy())
	}
	conf.SetIndexDeletionPolicy(index.NewSnapshotDeletionPolicy(nil))
	if _, ok := conf.GetIndexDeletionPolicy().(*index.SnapshotDeletionPolicy); !ok {
		t.Fatalf("getIndexDeletionPolicy().getClass(): got %T", conf.GetIndexDeletionPolicy())
	}
	expectThrowsPanic(t, "setIndexDeletionPolicy(null)", func() { conf.SetIndexDeletionPolicy(nil) })

	// Test MergeScheduler
	if _, ok := conf.GetMergeScheduler().(*index.ConcurrentMergeScheduler); !ok {
		t.Fatalf("getMergeScheduler().getClass(): got %T", conf.GetMergeScheduler())
	}
	conf.SetMergeScheduler(index.NewSerialMergeScheduler())
	if _, ok := conf.GetMergeScheduler().(*index.SerialMergeScheduler); !ok {
		t.Fatalf("getMergeScheduler().getClass(): got %T", conf.GetMergeScheduler())
	}
	expectThrowsPanic(t, "setMergeScheduler(null)", func() { conf.SetMergeScheduler(nil) })

	// Test Similarity:
	// we shouldnt assert what the default is, just that it's not null.
	if any(search.GetDefaultSimilarity()) != any(conf.GetSimilarity()) {
		t.Fatalf("IndexSearcher.getDefaultSimilarity() == conf.getSimilarity(): %v != %v",
			search.GetDefaultSimilarity(), conf.GetSimilarity())
	}
	conf.SetSimilarity(&mySimilarity{ClassicSimilarity: search.NewClassicSimilarity()})
	if _, ok := conf.GetSimilarity().(*mySimilarity); !ok {
		t.Fatalf("getSimilarity().getClass(): got %T", conf.GetSimilarity())
	}
	expectThrowsPanic(t, "setSimilarity(null)", func() { conf.SetSimilarity(nil) })

	expectThrowsPanic(t, "setMaxBufferedDocs(1)", func() { conf.SetMaxBufferedDocs(1) })

	expectThrowsPanic(t, "disabling both MAX_BUF_DOCS and RAM_SIZE_MB", func() {
		// Disable both MAX_BUF_DOCS and RAM_SIZE_MB
		conf.SetMaxBufferedDocs(4)
		conf.SetRAMBufferSizeMB(index.DisableAutoFlush)
		conf.SetMaxBufferedDocs(index.DisableAutoFlush)
	})

	conf.SetRAMBufferSizeMB(index.DefaultRAMBufferSizeMB)
	conf.SetMaxBufferedDocs(index.DefaultMaxBufferedDocs)
	expectThrowsPanic(t, "setRAMBufferSizeMB(DISABLE_AUTO_FLUSH)", func() {
		conf.SetRAMBufferSizeMB(index.DisableAutoFlush)
	})

	expectThrowsPanic(t, "setRAMPerThreadHardLimitMB(2048)", func() { conf.SetRAMPerThreadHardLimitMB(2048) })

	expectThrowsPanic(t, "setRAMPerThreadHardLimitMB(0)", func() { conf.SetRAMPerThreadHardLimitMB(0) })

	// Test MergePolicy
	if _, ok := conf.GetMergePolicy().(*index.TieredMergePolicy); !ok {
		t.Fatalf("getMergePolicy().getClass(): got %T", conf.GetMergePolicy())
	}
	conf.SetMergePolicy(index.NewLogDocMergePolicy())
	if _, ok := conf.GetMergePolicy().(*index.LogDocMergePolicy); !ok {
		t.Fatalf("getMergePolicy().getClass(): got %T", conf.GetMergePolicy())
	}
	expectThrowsPanic(t, "setMergePolicy(null)", func() { conf.SetMergePolicy(nil) })
}

func TestIndexWriterConfigLiveChangeToCFS(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicyUseCFS(true))
	// Start false:
	iwc.SetUseCompoundFile(false)
	iwc.GetMergePolicy().(interface{ SetNoCFSRatio(float64) }).SetNoCFSRatio(0.0)
	w := mustNewIndexWriter(t, dir, iwc)
	defer mustClose(t, w, dir)
	// Change to true:
	w.GetConfig().SetUseCompoundFile(true)

	doc := document.NewDocument()
	doc.Add(newStringField(t, "field", "foo", false))
	mustAddDocument(t, w, doc)
	mustCommit(t, w)
	t.Fatal("org.apache.lucene.index.IndexWriter#newestSegment() is not ported")
}
