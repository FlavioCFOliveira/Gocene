// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy

import (
	"io"
	"os"
	"encoding/binary"
)

// OrdinalMap is a mapping from old ordinal to new ordinals, used when merging indexes with separate taxonomies.
type OrdinalMap interface {
	SetSize(size int) error
	AddMapping(origOrdinal, newOrdinal int) error
	AddDone() error
	GetMap() ([]int, error)
}

// MemoryOrdinalMap is an OrdinalMap maintained in memory.
type MemoryOrdinalMap struct {
	mapping []int
}

func NewMemoryOrdinalMap() *MemoryOrdinalMap {
	return &MemoryOrdinalMap{}
}

func (m *MemoryOrdinalMap) SetSize(size int) error {
	m.mapping = make([]int, size)
	return nil
}

func (m *MemoryOrdinalMap) AddMapping(origOrdinal, newOrdinal int) error {
	m.mapping[origOrdinal] = newOrdinal
	return nil
}

func (m *MemoryOrdinalMap) AddDone() error {
	return nil
}

func (m *MemoryOrdinalMap) GetMap() ([]int, error) {
	return m.mapping, nil
}

// DiskOrdinalMap is an OrdinalMap maintained on the file system.
type DiskOrdinalMap struct {
	mapFile string
	mapping []int
	out     *os.File
}

func NewDiskOrdinalMap(mapFile string) (*DiskOrdinalMap, error) {
	f, err := os.Create(mapFile)
	if err != nil {
		return nil, err
	}
	return &DiskOrdinalMap{
		mapFile: mapFile,
		out:     f,
	}, nil
}

func (m *DiskOrdinalMap) SetSize(size int) error {
	return binary.Write(m.out, binary.BigEndian, int32(size))
}

func (m *DiskOrdinalMap) AddMapping(origOrdinal, newOrdinal int) error {
	if err := binary.Write(m.out, binary.BigEndian, int32(origOrdinal)); err != nil {
		return err
	}
	return binary.Write(m.out, binary.BigEndian, int32(newOrdinal))
}

func (m *DiskOrdinalMap) AddDone() error {
	if m.out != nil {
		err := m.out.Close()
		m.out = nil
		return err
	}
	return nil
}

func (m *DiskOrdinalMap) GetMap() ([]int, error) {
	if m.mapping != nil {
		return m.mapping, nil
	}
	if m.out != nil {
		if err := m.AddDone(); err != nil {
			return nil, err
		}
	}

	f, err := os.Open(m.mapFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	defer os.Remove(m.mapFile)

	var size int32
	if err := binary.Read(f, binary.BigEndian, &size); err != nil {
		return nil, err
	}

	m.mapping = make([]int, size)
	for i := 0; i < int(size); i++ {
		var orig, newOrd int32
		if err := binary.Read(f, binary.BigEndian, &orig); err != nil {
			return nil, err
		}
		if err := binary.Read(f, binary.BigEndian, &newOrd); err != nil {
			return nil, err
		}
		m.mapping[orig] = int(newOrd)
	}

	return m.mapping, nil
}
