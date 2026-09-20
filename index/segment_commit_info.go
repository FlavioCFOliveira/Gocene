// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// SegmentCommitInfo is an alias for spi.SegmentCommitInfo.
type SegmentCommitInfo = spi.SegmentCommitInfo

func NewSegmentCommitInfo(info *SegmentInfo, delCount, softDelCount int, delGen, fieldInfosGen, docValuesGen int64, id []byte) *SegmentCommitInfo {
	sci := spi.NewSegmentCommitInfo(info, delCount, delGen)
	sci.SetSoftDelCount(softDelCount)
	sci.SetFieldInfosGen(fieldInfosGen)
	sci.SetDocValuesGen(docValuesGen)
	sci.SetID(id)
	return sci
}
