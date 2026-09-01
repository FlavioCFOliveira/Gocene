// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package cranky

import (
	"fmt"
	"math/rand"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// CrankyFieldInfosFormat is a FieldInfosFormat that randomly throws IOExceptions
// to test the robustness of the indexing chain.
type CrankyFieldInfosFormat struct {
	delegate spi.FieldInfosFormat
	random   *rand.Rand
}

// NewCrankyFieldInfosFormat creates a new CrankyFieldInfosFormat.
func NewCrankyFieldInfosFormat(delegate spi.FieldInfosFormat, random *rand.Rand) *CrankyFieldInfosFormat {
	return &CrankyFieldInfosFormat{
		delegate: delegate,
		random:   random,
	}
}

// Name returns the format name.
func (f *CrankyFieldInfosFormat) Name() string {
	return f.delegate.Name()
}

// Read reads the field infos.
func (f *CrankyFieldInfosFormat) Read(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, context store.IOContext) (*index.FieldInfos, error) {
	return f.delegate.Read(dir, segmentInfo, segmentSuffix, context)
}

// Write writes the field infos.
func (f *CrankyFieldInfosFormat) Write(dir store.Directory, segmentInfo *index.SegmentInfo, segmentSuffix string, infos *index.FieldInfos, context store.IOContext) error {
	if f.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from FieldInfosFormat.Write()")
	}
	return f.delegate.Write(dir, segmentInfo, segmentSuffix, infos, context)
}
