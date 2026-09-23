// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// Port of lucene/core/src/test/org/apache/lucene/store/TestMMapDirectory.java
// (Apache Lucene 10.5.0).
//
// Ported methods: getDirectory, testAceWithThreads, testWithNormal,
// testGroupBySegmentFunc. The remaining methods of the Java class depend on
// production members of MMapDirectory that Gocene has not ported
// (supportsMadvise, ADVISE_BY_CONTEXT, PRELOAD_HINT, NO_GROUPING,
// IndexInput.isLoaded, IndexInput.prefetch, the per-group arena attachment and
// arena confinement) or on the inherited BaseDirectoryTestCase suite, which is
// part of the unported test framework; they are not reproduced here.

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"sync"
	"testing"
	"time"
)

func newMMapDirectoryTestRandom(t *testing.T) *rand.Rand {
	t.Helper()
	seed := uint64(time.Now().UnixNano())
	t.Logf("random seed: %d", seed)
	return rand.New(rand.NewPCG(seed, 0x5DEECE66D))
}

// getMMapTestDirectory mirrors TestMMapDirectory.getDirectory(Path).
func getMMapTestDirectory(t *testing.T, path string, random *rand.Rand) *MMapDirectory {
	t.Helper()
	m, err := NewMMapDirectory(path)
	if err != nil {
		t.Fatalf("NewMMapDirectory: %v", err)
	}
	var mu sync.Mutex
	m.SetPreload(func(file string, context IOContext) bool {
		mu.Lock()
		defer mu.Unlock()
		return random.IntN(2) == 0
	})
	return m
}

func TestMMapDirectory_AceWithThreads(t *testing.T) {
	random := newMMapDirectoryTestRandom(t)
	const nInts = 8 * 1024 * 1024

	dir := getMMapTestDirectory(t, t.TempDir(), random)
	defer dir.Close()
	out, err := dir.CreateOutput("test", IOContextDefault)
	mustNoErr(t, err)
	for i := 0; i < nInts; i++ {
		mustNoErr(t, out.WriteInt(int32(random.Uint32())))
	}
	mustNoErr(t, out.Close())

	const iters = 1 * 10 // RANDOM_MULTIPLIER * (TEST_NIGHTLY ? 50 : 10)
	for iter := 0; iter < iters; iter++ {
		in, err := dir.OpenInput("test", IOContextDefault)
		mustNoErr(t, err)
		clone := in.Clone()
		accum := make([]byte, nInts*4)
		shotgun := make(chan struct{})
		var t1Err error
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-shotgun
			for i := 0; i < 10; i++ {
				if err := clone.SetPosition(0); err != nil {
					t1Err = err
					return
				}
				if err := clone.ReadBytes(accum, 0, len(accum)); err != nil {
					t1Err = err
					return
				}
			}
		}()
		close(shotgun)
		// this triggers "bad behaviour": closing input while other threads are running
		mustNoErr(t, in.Close())
		wg.Wait()
		var ace *AlreadyClosedException
		if t1Err != nil && !errors.As(t1Err, &ace) {
			// AlreadyClosedException is OK; anything else is a failure
			t.Fatalf("reader goroutine: %v", t1Err)
		}
	}
}

// RANDOM is the default (see Constants.DEFAULT_READADVICE), so test with
// NORMAL too.
func TestMMapDirectory_WithNormal(t *testing.T) {
	random := newMMapDirectoryTestRandom(t)
	const size = 8 * 1024
	bs := make([]byte, size)
	for i := range bs {
		bs[i] = byte(random.Uint32())
	}

	dir, err := NewMMapDirectory(t.TempDir())
	mustNoErr(t, err)
	defer dir.Close()
	out, err := dir.CreateOutput("test", IOContextDefault)
	mustNoErr(t, err)
	mustNoErr(t, out.WriteBytes(bs, 0, len(bs)))
	mustNoErr(t, out.Close())

	dir.SetReadAdvice(func(s string, c IOContext) *ReadAdvice {
		normal := ReadAdviceNormal
		return &normal
	})
	in, err := dir.OpenInput("test", IOContextDefault)
	mustNoErr(t, err)
	defer in.Close()
	readBytes := make([]byte, size)
	mustNoErr(t, in.ReadBytes(readBytes, 0, len(readBytes)))
	if !bytes.Equal(bs, readBytes) {
		t.Fatal("bytes read differ from bytes written")
	}
}

func TestMMapDirectory_GroupBySegmentFunc(t *testing.T) {
	// MMapDirectory.GROUP_BY_SEGMENT
	fn := groupBySegment
	present := []struct{ name, want string }{
		{"_0.doc", "0"},
		{"_51.si", "51"},
		{"_51_1.si", "51-g"},
		{"_51_1_gg_ff.si", "51-g"},
		{"_51_2_gg_ff.si", "51-g"},
		{"_51_3_gg_ff.si", "51-g"},
		{"_5987654321.si", "5987654321"},
		{"_f.si", "f"},
		{"_ff.si", "ff"},
		{"_51a.si", "51a"},
		{"_f51a.si", "f51a"},
		{"_segment.si", "segment"},
		// old style
		{"_5_Lucene90FieldsIndex-doc_ids_0.tmp", "5"},
	}
	for _, c := range present {
		got, ok := fn(c.name)
		if !ok {
			t.Errorf("GROUP_BY_SEGMENT(%q): got empty, want %q", c.name, c.want)
			continue
		}
		if got != c.want {
			t.Errorf("GROUP_BY_SEGMENT(%q): got %q, want %q", c.name, got, c.want)
		}
	}

	for _, name := range []string{"", "_", "_.si", "foo", "_foo", "__foo", "_segment", "segment.si"} {
		if got, ok := fn(name); ok {
			t.Errorf("GROUP_BY_SEGMENT(%q): got %q, want empty", name, got)
		}
	}
}
