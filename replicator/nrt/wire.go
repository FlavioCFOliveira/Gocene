// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package nrt

// wire.go ports the CopyState wire encoder/decoder that lives in
// SimplePrimaryNode.writeCopyState + TestSimpleServer.writeFilesMetaData (and
// their inverse counterparts) from Apache Lucene 10.4.0.
//
// The binary layout is:
//
//	vInt    infosBytes.length
//	bytes   infosBytes
//	vLong   gen
//	vLong   version
//	// writeFilesMetaData:
//	vInt    files.size()
//	for each (name, FileMetaData):
//	  String  name          (vInt length + UTF-8 bytes)
//	  vLong   length
//	  vLong   checksum
//	  vInt    header.length
//	  bytes   header
//	  vInt    footer.length
//	  bytes   footer
//	vInt    completedMergeFiles.size()
//	for each: String fileName
//	vLong   primaryGen
//
// Port of:
//   - org.apache.lucene.replicator.nrt.SimplePrimaryNode#writeCopyState
//   - org.apache.lucene.replicator.nrt.TestSimpleServer#writeFilesMetaData
//   - org.apache.lucene.replicator.nrt.TestSimpleServer#readFilesMetaData
//   - org.apache.lucene.replicator.nrt.TestSimpleServer#readCopyState

import (
	"encoding/binary"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// WriteCopyState encodes state onto out using the Lucene 10.4.0
// SimplePrimaryNode.writeCopyState wire layout.
//
// Port of org.apache.lucene.replicator.nrt.SimplePrimaryNode#writeCopyState.
func WriteCopyState(state *CopyState, out store.DataOutput) error {
	if err := store.WriteVInt(out, int32(len(state.InfosBytes))); err != nil {
		return err
	}
	if err := out.WriteBytes(state.InfosBytes); err != nil {
		return err
	}
	if err := store.WriteVLong(out, state.Gen); err != nil {
		return err
	}
	if err := store.WriteVLong(out, state.Version); err != nil {
		return err
	}
	if err := WriteFilesMetaData(state.Files, out); err != nil {
		return err
	}
	if err := store.WriteVInt(out, int32(len(state.CompletedMergeFiles))); err != nil {
		return err
	}
	for name := range state.CompletedMergeFiles {
		if err := store.WriteString(out, name); err != nil {
			return err
		}
	}
	return store.WriteVLong(out, state.PrimaryGen)
}

// ReadCopyState decodes a CopyState from in using the Lucene 10.4.0
// TestSimpleServer.readCopyState wire layout.
//
// Port of org.apache.lucene.replicator.nrt.TestSimpleServer#readCopyState.
func ReadCopyState(in store.DataInput) (*CopyState, error) {
	infosLen, err := store.ReadVInt(in)
	if err != nil {
		return nil, err
	}
	infosBytes := make([]byte, infosLen)
	if err := in.ReadBytes(infosBytes); err != nil {
		return nil, err
	}
	gen, err := store.ReadVLong(in)
	if err != nil {
		return nil, err
	}
	version, err := store.ReadVLong(in)
	if err != nil {
		return nil, err
	}
	files, err := ReadFilesMetaData(in)
	if err != nil {
		return nil, err
	}
	count, err := store.ReadVInt(in)
	if err != nil {
		return nil, err
	}
	completedMergeFiles := make(map[string]struct{}, count)
	for i := int32(0); i < count; i++ {
		name, err := store.ReadString(in)
		if err != nil {
			return nil, err
		}
		completedMergeFiles[name] = struct{}{}
	}
	primaryGen, err := store.ReadVLong(in)
	if err != nil {
		return nil, err
	}
	return &CopyState{
		Files:               files,
		Version:             version,
		Gen:                 gen,
		InfosBytes:          infosBytes,
		CompletedMergeFiles: completedMergeFiles,
		PrimaryGen:          primaryGen,
	}, nil
}

// WriteFilesMetaData encodes files onto out using the Lucene 10.4.0
// TestSimpleServer.writeFilesMetaData wire layout.
//
// The files map is written in its natural iteration order (callers must use
// an ordered-insertion map if determinism is required — see BuildCopyState).
//
// Port of org.apache.lucene.replicator.nrt.TestSimpleServer#writeFilesMetaData.
func WriteFilesMetaData(files map[string]*FileMetaData, out store.DataOutput) error {
	if err := store.WriteVInt(out, int32(len(files))); err != nil {
		return err
	}
		// Sort file names for deterministic output (Go map iteration is non-deterministic).
	sortedNames := make([]string, 0, len(files))
	for name := range files {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)
	for _, name := range sortedNames {
		fmd := files[name]
		if err := store.WriteString(out, name); err != nil {
			return err
		}
		if err := store.WriteVLong(out, fmd.Length); err != nil {
			return err
		}
		if err := store.WriteVLong(out, fmd.Checksum); err != nil {
			return err
		}
		if err := store.WriteVInt(out, int32(len(fmd.Header))); err != nil {
			return err
		}
		if err := out.WriteBytes(fmd.Header); err != nil {
			return err
		}
		if err := store.WriteVInt(out, int32(len(fmd.Footer))); err != nil {
			return err
		}
		if err := out.WriteBytes(fmd.Footer); err != nil {
			return err
		}
	}
	return nil
}

// ReadFilesMetaData decodes a files map from in using the Lucene 10.4.0
// TestSimpleServer.readFilesMetaData wire layout.
//
// Port of org.apache.lucene.replicator.nrt.TestSimpleServer#readFilesMetaData.
func ReadFilesMetaData(in store.DataInput) (map[string]*FileMetaData, error) {
	count, err := store.ReadVInt(in)
	if err != nil {
		return nil, err
	}
	files := make(map[string]*FileMetaData, count)
	for i := int32(0); i < count; i++ {
		name, err := store.ReadString(in)
		if err != nil {
			return nil, err
		}
		length, err := store.ReadVLong(in)
		if err != nil {
			return nil, err
		}
		checksum, err := store.ReadVLong(in)
		if err != nil {
			return nil, err
		}
		headerLen, err := store.ReadVInt(in)
		if err != nil {
			return nil, err
		}
		header := make([]byte, headerLen)
		if err := in.ReadBytes(header); err != nil {
			return nil, err
		}
		footerLen, err := store.ReadVInt(in)
		if err != nil {
			return nil, err
		}
		footer := make([]byte, footerLen)
		if err := in.ReadBytes(footer); err != nil {
			return nil, err
		}
		files[name] = &FileMetaData{Header: header, Footer: footer, Length: length, Checksum: checksum}
	}
	return files, nil
}

// ---------------------------------------------------------------------------
// Deterministic helpers
//
// These replicate the Java harness's ReplicatorNrtCopyStateScenario so that
// Gocene can produce the same CopyState as the Java side for a given seed.
// ---------------------------------------------------------------------------

// IDFromSeed returns the deterministic 16-byte codec-header id for seed.
// Mirrors Determinism.idBytes on the Java side:
//
//	buf.putLong(seed); buf.putLong(~seed);
func IDFromSeed(seed int64) []byte {
	id := make([]byte, 16)
	binary.BigEndian.PutUint64(id[0:8], uint64(seed))
	binary.BigEndian.PutUint64(id[8:16], uint64(^seed))
	return id
}

// SeedBytes produces a deterministic pseudo-random byte slice of length len
// from seed and a salt string, using the SplitMix64 mix function.
//
// Mirrors ReplicatorNrtCopyStateScenario.seedBytes (private static).
func SeedBytes(seed int64, salt string, length int) []byte {
	state := uint64(seed)
	for i := 0; i < len(salt); i++ {
		state ^= uint64(salt[i])
		state = splitMix64(state)
	}
	out := make([]byte, length)
	for i := 0; i < length; i++ {
		state = splitMix64(state)
		out[i] = byte(state & 0xFF)
	}
	return out
}

// splitMix64 is the SplitMix64 finaliser used by the Java harness.
//
// Mirrors the private mix() method in ReplicatorNrtCopyStateScenario.
func splitMix64(z uint64) uint64 {
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// orderedFiles is a slice-backed ordered file map that preserves insertion
// order for deterministic wire serialisation. It implements the minimal
// interface needed by WriteCopyState.
//
// Lucene's Java buildCopyState uses a LinkedHashMap, so insertion order
// determines the byte stream; Go maps have non-deterministic iteration.
// We keep an ordered list of names and a map for O(1) lookup.
type orderedFiles struct {
	names []string
	m     map[string]*FileMetaData
}

func newOrderedFiles() *orderedFiles {
	return &orderedFiles{m: make(map[string]*FileMetaData)}
}

func (f *orderedFiles) Put(name string, fmd *FileMetaData) {
	if _, exists := f.m[name]; !exists {
		f.names = append(f.names, name)
	}
	f.m[name] = fmd
}

func (f *orderedFiles) Len() int { return len(f.names) }

// Names returns the file names in insertion order.
func (f *orderedFiles) Names() []string { return f.names }

func (f *orderedFiles) Get(name string) (*FileMetaData, bool) {
	fmd, ok := f.m[name]
	return fmd, ok
}

// WriteFilesMetaDataOrdered encodes an orderedFiles onto out in insertion order.
func writeFilesMetaDataOrdered(files *orderedFiles, out store.DataOutput) error {
	if err := store.WriteVInt(out, int32(files.Len())); err != nil {
		return err
	}
	for _, name := range files.names {
		fmd := files.m[name]
		if err := store.WriteString(out, name); err != nil {
			return err
		}
		if err := store.WriteVLong(out, fmd.Length); err != nil {
			return err
		}
		if err := store.WriteVLong(out, fmd.Checksum); err != nil {
			return err
		}
		if err := store.WriteVInt(out, int32(len(fmd.Header))); err != nil {
			return err
		}
		if err := out.WriteBytes(fmd.Header); err != nil {
			return err
		}
		if err := store.WriteVInt(out, int32(len(fmd.Footer))); err != nil {
			return err
		}
		if err := out.WriteBytes(fmd.Footer); err != nil {
			return err
		}
	}
	return nil
}

// orderedSet is a slice-backed ordered string set (like Java LinkedHashSet).
type orderedSet struct {
	names []string
	m     map[string]struct{}
}

func newOrderedSet() *orderedSet {
	return &orderedSet{m: make(map[string]struct{})}
}

func (s *orderedSet) Add(name string) {
	if _, exists := s.m[name]; !exists {
		s.names = append(s.names, name)
		s.m[name] = struct{}{}
	}
}

func (s *orderedSet) Len() int { return len(s.names) }

// CopyStateOrdered is an ordered-collection variant of CopyState used for
// deterministic wire serialisation (Gocene-write scenarios only). The public
// CopyState uses Go maps, which have non-deterministic iteration order.
type CopyStateOrdered struct {
	Files               *orderedFiles
	CompletedMergeFiles *orderedSet
	Version             int64
	Gen                 int64
	InfosBytes          []byte
	PrimaryGen          int64
}

// WriteCopyStateOrdered encodes an ordered copy state onto out.
// This is the variant used by the Gocene-write leg to produce byte-identical
// output to the Java harness (which uses LinkedHashMap / LinkedHashSet).
func WriteCopyStateOrdered(state *CopyStateOrdered, out store.DataOutput) error {
	if err := store.WriteVInt(out, int32(len(state.InfosBytes))); err != nil {
		return err
	}
	if err := out.WriteBytes(state.InfosBytes); err != nil {
		return err
	}
	if err := store.WriteVLong(out, state.Gen); err != nil {
		return err
	}
	if err := store.WriteVLong(out, state.Version); err != nil {
		return err
	}
	if err := writeFilesMetaDataOrdered(state.Files, out); err != nil {
		return err
	}
	if err := store.WriteVInt(out, int32(state.CompletedMergeFiles.Len())); err != nil {
		return err
	}
	for _, name := range state.CompletedMergeFiles.names {
		if err := store.WriteString(out, name); err != nil {
			return err
		}
	}
	return store.WriteVLong(out, state.PrimaryGen)
}

// BuildCopyStateOrdered replicates ReplicatorNrtCopyStateScenario.buildCopyState(seed).
//
// It uses orderedFiles and orderedSet to preserve insertion order and produce
// a byte-identical wire frame to the Java reference at the same seed.
func BuildCopyStateOrdered(seed int64) *CopyStateOrdered {
	files := newOrderedFiles()
	files.Put("segments_1", buildFileMetaData(seed, 1, 64))
	files.Put("_0.cfe", buildFileMetaData(seed, 2, 128))
	files.Put("_0.cfs", buildFileMetaData(seed, 3, 4096))

	completed := newOrderedSet()
	completed.Add("_0.cfs")
	completed.Add("_0.cfe")

	infosBytes := SeedBytes(seed, "infos", 96)
	gen := seed | 0x10
	version := seed | 0x20
	primaryGen := seed | 0x40

	return &CopyStateOrdered{
		Files:               files,
		CompletedMergeFiles: completed,
		Version:             version,
		Gen:                 gen,
		InfosBytes:          infosBytes,
		PrimaryGen:          primaryGen,
	}
}

// buildFileMetaData mirrors ReplicatorNrtCopyStateScenario.fileMetaData(seed, idx, length).
func buildFileMetaData(seed int64, idx int, length int64) *FileMetaData {
	header := SeedBytes(seed, "h"+string(rune('0'+idx)), 16)
	footer := SeedBytes(seed, "f"+string(rune('0'+idx)), 16)
	checksum := (seed ^ (int64(0xA5A5A5A5) * int64(idx))) & 0x7FFFFFFFFFFFFFFF
	return &FileMetaData{Header: header, Footer: footer, Length: length, Checksum: checksum}
}
