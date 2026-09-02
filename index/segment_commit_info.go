//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// SegmentCommitInfo embeds a read-only SegmentInfo and adds per-commit fields.
//
// This is the Go port of Lucene's org.apache.lucene.index.SegmentCommitInfo.
type SegmentCommitInfo struct {
	// Info is the SegmentInfo that we wrap.
	Info *SegmentInfo

	id []byte

	delCount     int
	softDelCount int
	delGen       int64

	nextWriteDelGen int64

	fieldInfosGen     int64
	nextWriteFieldInfosGen int64

	docValuesGen     int64
	nextWriteDocValuesGen int64

	dvUpdatesFiles map[int]map[string]struct{}
	fieldInfosFiles map[string]struct{}

	sizeInBytes int64

	mu sync.RWMutex
}

func NewSegmentCommitInfo(info *SegmentInfo, delCount, softDelCount int, delGen, fieldInfosGen, docValuesGen int64, id []byte) *SegmentCommitInfo {
	return &SegmentCommitInfo{
		Info:                   info,
		delCount:               delCount,
		softDelCount:           softDelCount,
		delGen:                 delGen,
		nextWriteDelGen:        mapDelGen(delGen),
		fieldInfosGen:          fieldInfosGen,
		nextWriteFieldInfosGen: mapFieldInfosGen(fieldInfosGen),
		docValuesGen:           docValuesGen,
		nextWriteDocValuesGen:  mapDocValuesGen(docValuesGen),
		id:                     id,
		dvUpdatesFiles:         make(map[int]map[string]struct{}),
		fieldInfosFiles:        make(map[string]struct{}),
		sizeInBytes:            -1,
	}
}

func mapDelGen(gen int64) int64 {
	if gen == -1 {
		return 1
	}
	return gen + 1
}

func mapFieldInfosGen(gen int64) int64 {
	if gen == -1 {
		return 1
	}
	return gen + 1
}

func mapDocValuesGen(gen int64) int64 {
	if gen == -1 {
		return 1
	}
	return gen + 1
}

func (s *SegmentCommitInfo) GetDocValuesUpdatesFiles() map[int]map[string]struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dvUpdatesFiles
}

func (s *SegmentCommitInfo) SetDocValuesUpdatesFiles(dvUpdates map[int][]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dvUpdatesFiles = make(map[int]map[string]struct{})
	for k, v := range dvUpdates {
		set := make(map[string]struct{})
		for _, file := range v {
			set[s.Info.namedForThisSegment(file)] = struct{}{}
		}
		s.dvUpdatesFiles[k] = set
	}
}

func (s *SegmentCommitInfo) GetFieldInfosFiles() map[string]struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fieldInfosFiles
}

func (s *SegmentCommitInfo) SetFieldInfosFiles(files []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fieldInfosFiles = make(map[string]struct{})
	for _, f := range files {
		s.fieldInfosFiles[s.Info.namedForThisSegment(f)] = struct{}{}
	}
}

func (s *SegmentCommitInfo) AdvanceDelGen() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delGen = s.nextWriteDelGen
	s.nextWriteDelGen = s.delGen + 1
	s.generationAdvanced()
}

func (s *SegmentCommitInfo) AdvanceNextWriteDelGen() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextWriteDelGen++
}

func (s *SegmentCommitInfo) GetNextWriteDelGen() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nextWriteDelGen
}

func (s *SegmentCommitInfo) SetNextWriteDelGen(v int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextWriteDelGen = v
}

func (s *SegmentCommitInfo) AdvanceFieldInfosGen() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fieldInfosGen = s.nextWriteFieldInfosGen
	s.nextWriteFieldInfosGen = s.fieldInfosGen + 1
	s.generationAdvanced()
}

func (s *SegmentCommitInfo) AdvanceNextWriteFieldInfosGen() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextWriteFieldInfosGen++
}

func (s *SegmentCommitInfo) GetNextWriteFieldInfosGen() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nextWriteFieldInfosGen
}

func (s *SegmentCommitInfo) SetNextWriteFieldInfosGen(v int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextWriteFieldInfosGen = v
}

func (s *SegmentCommitInfo) AdvanceDocValuesGen() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docValuesGen = s.nextWriteDocValuesGen
	s.nextWriteDocValuesGen = s.docValuesGen + 1
	s.generationAdvanced()
}

func (s *SegmentCommitInfo) AdvanceNextWriteDocValuesGen() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextWriteDocValuesGen++
}

func (s *SegmentCommitInfo) GetNextWriteDocValuesGen() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nextWriteDocValuesGen
}

func (s *SegmentCommitInfo) SetNextWriteDocValuesGen(v int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextWriteDocValuesGen = v
}

func (s *SegmentCommitInfo) SizeInBytes() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sizeInBytes == -1 {
		var sum int64
		for file := range s.Info.Files() {
			len, err := s.Info.Dir.FileLength(file)
			if err != nil {
				return 0, err
			}
			sum += len
		}
		s.sizeInBytes = sum
	}
	return s.sizeInBytes, nil
}

func (s *SegmentCommitInfo) Files() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	files := make(map[string]struct{})
	for f := range s.Info.Files() {
		files[f] = struct{}{}
	}

	// Note: live docs files, dv updates, and field infos files should be added here.
	// This is a simplified version.
	res := make([]string, 0, len(files))
	for f := range files {
		res = append(res, f)
	}
	return res, nil
}

func (s *SegmentCommitInfo) HasDeletions() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.delGen != -1
}

func (s *SegmentCommitInfo) HasFieldUpdates() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fieldInfosGen != -1
}

func (s *SegmentCommitInfo) GetFieldInfosGen() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fieldInfosGen
}

func (s *SegmentCommitInfo) GetDocValuesGen() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.docValuesGen
}

func (s *SegmentCommitInfo) GetDelGen() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.delGen
}

func (s *SegmentCommitInfo) GetDelCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.delCount
}

func (s *SegmentCommitInfo) GetSoftDelCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.softDelCount
}

func (s *SegmentCommitInfo) SetDelCount(delCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delCount = delCount
}

func (s *SegmentCommitInfo) SetSoftDelCount(softDelCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.softDelCount = softDelCount
}

func (s *SegmentCommitInfo) GetId() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	idCopy := make([]byte, len(s.id))
	copy(idCopy, s.id)
	return idCopy
}

func (s *SegmentCommitInfo) String() string {
	return s.Info.String()
}

func (s *SegmentCommitInfo) generationAdvanced() {
	s.sizeInBytes = -1
	s.id = util.RandomId()
}
