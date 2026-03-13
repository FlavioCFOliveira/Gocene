// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestSegmentInfoFormat_ReadWrite(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentName := "_0"
	docCount := 123
	segmentID := make([]byte, 16)
	rand.Read(segmentID)

	si := index.NewSegmentInfo(segmentName, docCount, dir)
	si.SetID(segmentID)
	si.SetVersion("10.4.1")
	si.SetCompoundFile(true)
	si.SetDiagnostics(map[string]string{
		"os":     "linux",
		"java":   "21",
		"gocene": "0.1.0",
		"source": "flush",
		"lucene": "10.4.1",
	})
	si.SetFiles([]string{"_0.fdt", "_0.fdx", "_0.nvd", "_0.nvm"})
	si.SetAttribute("test_attr", "test_val")
	si.SetAttribute("another_attr", "another_val")

	format := codecs.NewLucene99SegmentInfoFormat()

	// Write
	err := format.Write(dir, si, store.IOContextWrite)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read back
	si2, err := format.Read(dir, segmentName, segmentID, store.IOContextRead)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Verify
	if si2.Name() != si.Name() {
		t.Errorf("Expected name %s, got %s", si.Name(), si2.Name())
	}
	if si2.DocCount() != si.DocCount() {
		t.Errorf("Expected docCount %d, got %d", si.DocCount(), si2.DocCount())
	}
	if !bytes.Equal(si2.GetID(), si.GetID()) {
		t.Errorf("Expected ID %x, got %x", si.GetID(), si2.GetID())
	}
	if si2.Version() != si.Version() {
		t.Errorf("Expected version %s, got %s", si.Version(), si2.Version())
	}
	if si2.IsCompoundFile() != si.IsCompoundFile() {
		t.Errorf("Expected isCompoundFile %v, got %v", si.IsCompoundFile(), si2.IsCompoundFile())
	}

	// Verify diagnostics
	diag1 := si.GetDiagnostics()
	diag2 := si2.GetDiagnostics()
	if len(diag1) != len(diag2) {
		t.Errorf("Expected %d diagnostics, got %d", len(diag1), len(diag2))
	}
	for k, v := range diag1 {
		if diag2[k] != v {
			t.Errorf("Diagnostic %s: expected %s, got %s", k, v, diag2[k])
		}
	}

	// Verify files
	files1 := si.Files()
	files2 := si2.Files()
	if len(files1) != len(files2) {
		t.Errorf("Expected %d files, got %d", len(files1), len(files2))
	}
	fileMap := make(map[string]bool)
	for _, f := range files2 {
		fileMap[f] = true
	}
	for _, f := range files1 {
		if !fileMap[f] {
			t.Errorf("Missing file %s in read SegmentInfo", f)
		}
	}

	// Verify attributes
	attr1 := si.GetAttributes()
	attr2 := si2.GetAttributes()
	if len(attr1) != len(attr2) {
		t.Errorf("Expected %d attributes, got %d", len(attr1), len(attr2))
	}
	for k, v := range attr1 {
		if attr2[k] != v {
			t.Errorf("Attribute %s: expected %s, got %s", k, v, attr2[k])
		}
	}
}

func TestSegmentInfosFormat_ReadWrite(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sis := index.NewSegmentInfos()
	sis.SetCounter(42)
	sis.SetVersion(12345)
	sis.SetIndexCreatedVersionMajor(10)
	sis.SetLuceneVersion("10.4.1")
	sis.SetUserData(map[string]string{
		"commit_time": "2026-03-13T12:00:00Z",
		"author":      "gocene",
	})

	// Add segments
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("_%d", i)
		si := index.NewSegmentInfo(name, 100+i, dir)

		segID := make([]byte, 16)
		rand.Read(segID)
		si.SetID(segID)

		sci := index.NewSegmentCommitInfo(si, i, int64(i))
		sci.SetFieldInfosGen(int64(i + 1))
		sci.SetDocValuesGen(int64(i + 2))
		sci.SetSoftDelCount(i)

		sciID := make([]byte, 16)
		rand.Read(sciID)
		sci.SetID(sciID)

		// Add some update files
		sci.SetFieldInfosFiles(map[string]struct{}{
			fmt.Sprintf("_%d_1.fnm", i): {},
		})
		sci.SetDocValuesUpdatesFiles(map[int]map[string]struct{}{
			1: {fmt.Sprintf("_%d_1_1.dvd", i): {}},
		})

		sis.Add(sci)
	}

	format := codecs.NewLucene104SegmentInfosFormat()

	// Write
	err := format.Write(dir, sis)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read back
	sis2, err := format.Read(dir)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Verify basic fields
	if sis2.Counter() != sis.Counter() {
		t.Errorf("Expected counter %d, got %d", sis.Counter(), sis2.Counter())
	}
	if sis2.Version() != sis.Version() {
		t.Errorf("Expected version %d, got %d", sis.Version(), sis2.Version())
	}
	if sis2.IndexCreatedVersionMajor() != sis.IndexCreatedVersionMajor() {
		t.Errorf("Expected createdMajor %d, got %d", sis.IndexCreatedVersionMajor(), sis2.IndexCreatedVersionMajor())
	}
	if sis2.LuceneVersion() != sis.LuceneVersion() {
		t.Errorf("Expected luceneVersion %s, got %s", sis.LuceneVersion(), sis2.LuceneVersion())
	}

	// Verify user data
	userData1 := sis.GetUserData()
	userData2 := sis2.GetUserData()
	if len(userData1) != len(userData2) {
		t.Errorf("Expected %d user data entries, got %d", len(userData1), len(userData2))
	}
	for k, v := range userData1 {
		if userData2[k] != v {
			t.Errorf("User data %s: expected %s, got %s", k, v, userData2[k])
		}
	}

	// Verify segments
	if sis2.Size() != sis.Size() {
		t.Fatalf("Expected %d segments, got %d", sis.Size(), sis2.Size())
	}

	for i := 0; i < sis.Size(); i++ {
		sci1 := sis.Get(i)
		sci2 := sis2.Get(i)

		if sci2.Name() != sci1.Name() {
			t.Errorf("Segment %d: expected name %s, got %s", i, sci1.Name(), sci2.Name())
		}
		if !bytes.Equal(sci2.SegmentInfo().GetID(), sci1.SegmentInfo().GetID()) {
			t.Errorf("Segment %d: expected SI ID %x, got %x", i, sci1.SegmentInfo().GetID(), sci2.SegmentInfo().GetID())
		}
		if sci2.DelGen() != sci1.DelGen() {
			t.Errorf("Segment %d: expected delGen %d, got %d", i, sci1.DelGen(), sci2.DelGen())
		}
		if sci2.DelCount() != sci1.DelCount() {
			t.Errorf("Segment %d: expected delCount %d, got %d", i, sci1.DelCount(), sci2.DelCount())
		}
		if sci2.FieldInfosGen() != sci1.FieldInfosGen() {
			t.Errorf("Segment %d: expected fieldInfosGen %d, got %d", i, sci1.FieldInfosGen(), sci2.FieldInfosGen())
		}
		if sci2.DocValuesGen() != sci1.DocValuesGen() {
			t.Errorf("Segment %d: expected docValuesGen %d, got %d", i, sci1.DocValuesGen(), sci2.DocValuesGen())
		}
		if sci2.SoftDelCount() != sci1.SoftDelCount() {
			t.Errorf("Segment %d: expected softDelCount %d, got %d", i, sci1.SoftDelCount(), sci2.SoftDelCount())
		}
		if !bytes.Equal(sci2.GetID(), sci1.GetID()) {
			t.Errorf("Segment %d: expected SCI ID %x, got %x", i, sci1.GetID(), sci2.GetID())
		}

		// Verify update files
		fiFiles1 := sci1.FieldInfosFiles()
		fiFiles2 := sci2.FieldInfosFiles()
		if len(fiFiles1) != len(fiFiles2) {
			t.Errorf("Segment %d: expected %d fi files, got %d", i, len(fiFiles1), len(fiFiles2))
		}

		dvFiles1 := sci1.DocValuesUpdatesFiles()
		dvFiles2 := sci2.DocValuesUpdatesFiles()
		if len(dvFiles1) != len(dvFiles2) {
			t.Errorf("Segment %d: expected %d dv update field counts, got %d", i, len(dvFiles1), len(dvFiles2))
		}
	}
}

func TestSegmentInfoFormat_MissingFile(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	format := codecs.NewLucene99SegmentInfoFormat()
	_, err := format.Read(dir, "_nonexistent", make([]byte, 16), store.IOContextRead)
	if err == nil {
		t.Error("Expected error when reading nonexistent file, got nil")
	}
}

func TestSegmentInfoFormat_CorruptHeader(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segmentName := "_0"
	fileName := codecs.GetSegmentFileName(segmentName, "", "si")
	out, _ := dir.CreateOutput(fileName, store.IOContextWrite)
	store.WriteInt32(out, 0x12345678) // Bad magic
	out.Close()

	format := codecs.NewLucene99SegmentInfoFormat()
	_, err := format.Read(dir, segmentName, make([]byte, 16), store.IOContextRead)
	if err == nil {
		t.Error("Expected error when reading corrupted header, got nil")
	}
}
