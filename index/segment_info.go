// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"strings"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

const (
	No  = -1
	Yes = 1
)

// SegmentInfo provides information about a segment such as its name, directory, and files related to the segment.
//
// This is the Go port of Lucene's org.apache.lucene.index.SegmentInfo.
type SegmentInfo struct {
	// name is the unique segment name in the directory.
	Name string

	// dir is where this segment resides.
	Dir util.Directory

	maxDoc int

	isCompoundFile bool

	// id uniquely identifies this segment.
	id []byte

	codec spi.Codec

	diagnostics map[string]string

	attributes map[string]string

	// indexSort tracks the sort order of this segment.
	indexSort any // Simplified as 'any' for now, will refine if a Sort type is available.

	// version tracks the Lucene version this segment was created with.
	version string

	// minVersion tracks the minimum version that contributed documents to a segment.
	minVersion string

	hasBlocks bool

	setFiles map[string]struct{}

	mu sync.RWMutex
}

func NewSegmentInfo(dir util.Directory, version string, minVersion string, name string, maxDoc int, isCompoundFile bool, hasBlocks bool, codec spi.Codec, diagnostics map[string]string, id []byte, attributes map[string]string, indexSort any) *SegmentInfo {
	return &SegmentInfo{
		Dir:               dir,
		version:           version,
		minVersion:        minVersion,
		Name:              name,
		maxDoc:            maxDoc,
		isCompoundFile:    isCompoundFile,
		hasBlocks:         hasBlocks,
		codec:             codec,
		diagnostics:       diagnostics,
		id:                id,
		attributes:        attributes,
		indexSort:         indexSort,
		setFiles:          make(map[string]struct{}),
	}
}

func (s *SegmentInfo) GetUseCompoundFile() bool {
	return s.isCompoundFile
}

func (s *SegmentInfo) SetUseCompoundFile(isCompoundFile bool) {
	s.isCompoundFile = isCompoundFile
}

func (s *SegmentInfo) GetHasBlocks() bool {
	return s.hasBlocks
}

func (s *SegmentInfo) SetHasBlocks() {
	s.hasBlocks = true
}

func (s *SegmentInfo) SetCodec(codec spi.Codec) {
	if codec == nil {
		panic("codec must be non-null")
	}
	s.codec = codec
}

func (s *SegmentInfo) GetCodec() spi.Codec {
	return s.codec
}

func (s *SegmentInfo) MaxDoc() int {
	if s.maxDoc == -1 {
		panic("maxDoc isn't set yet")
	}
	return s.maxDoc
}

func (s *SegmentInfo) SetMaxDoc(maxDoc int) {
	if s.maxDoc != -1 {
		panic(fmt.Sprintf("maxDoc was already set: this.maxDoc=%d vs maxDoc=%d", s.maxDoc, maxDoc))
	}
	s.maxDoc = maxDoc
}

func (s *SegmentInfo) Files() map[string]struct{} {
	if s.setFiles == nil {
		panic("files were not computed yet")
	}
	return s.setFiles
}

func (s *SegmentInfo) SetFiles(files []string) {
	s.setFiles = make(map[string]struct{})
	for _, f := range files {
		s.AddFile(f)
	}
}

func (s *SegmentInfo) AddFile(file string) {
	// In Lucene, it uses IndexFileNames.stripSegmentName(file).
	// For now, we'll assume it's already formatted or just add it.
	s.setFiles[file] = struct{}{}
}

func (s *SegmentInfo) GetAttribute(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.attributes[key]
}

func (s *SegmentInfo) PutAttribute(key, value string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	oldValue := s.attributes[key]
	s.attributes[key] = value
	return oldValue
}

func (s *SegmentInfo) GetAttributes() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.attributes
}

func (s *SegmentInfo) GetVersion() string {
	return s.version
}

func (s *SegmentInfo) GetMinVersion() string {
	return s.minVersion
}

func (s *SegmentInfo) GetId() []byte {
	idCopy := make([]byte, len(s.id))
	copy(idCopy, s.id)
	return idCopy
}

func (s *SegmentInfo) String() string {
	return fmt.Sprintf("%s(%s):%c%d", s.Name, s.version, byte('c'), s.maxDoc)
}
