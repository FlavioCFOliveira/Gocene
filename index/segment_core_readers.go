// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SegmentCoreReaders holds core readers that are shared (unchanged) when SegmentReader is cloned or reopened.
//
// This is the Go port of Lucene's org.apache.lucene.index.SegmentCoreReaders.
type SegmentCoreReaders struct {
	ref int32

	Fields spi.FieldsProducer
	Norms  spi.NormsProducer

	fieldsReaderOrig    spi.StoredFieldsReader
	termVectorsReaderOrig spi.TermVectorsReader
	pointsReader        spi.PointsReader
	knnVectorsReader    spi.KnnVectorsReader
	cfsReader           spi.CompoundDirectory
	segment             string
	coreFieldInfos      *FieldInfos
}

func NewSegmentCoreReaders(dir store.Directory, si *SegmentCommitInfo, context store.IOContext) (*SegmentCoreReaders, error) {
	codec := si.Info.GetCodec()
	var cfsDir store.Directory
	var cfsReader spi.CompoundDirectory

	if si.Info.GetUseCompoundFile() {
		var err error
		cfsReader, cfsDir, err = codec.CompoundFormat().GetCompoundReader(dir, si.Info)
		if err != nil {
			return nil, err
		}
	} else {
		cfsReader = nil
		cfsDir = dir
	}

	segment := si.Info.Name()
	coreFieldInfos, err := codec.FieldInfosFormat().Read(cfsDir, si.Info, "", context)
	if err != nil {
		return nil, err
	}

	segmentReadState := NewSegmentReadState(cfsDir, si.Info, coreFieldInfos, context)

	var fields spi.FieldsProducer
	if coreFieldInfos.HasPostings() {
		fields, err = codec.PostingsFormat().FieldsProducer(segmentReadState)
		if err != nil {
			return nil, err
		}
	}

	var norms spi.NormsProducer
	if coreFieldInfos.HasNorms() {
		norms, err = codec.NormsFormat().NormsProducer(segmentReadState)
		if err != nil {
			return nil, err
		}
	}

	fieldsReader, err := codec.StoredFieldsFormat().FieldsReader(cfsDir, si.Info, coreFieldInfos, context)
	if err != nil {
		return nil, err
	}

	var tvReader spi.TermVectorsReader
	if coreFieldInfos.HasTermVectors() {
		tvReader, err = codec.TermVectorsFormat().VectorsReader(cfsDir, si.Info, coreFieldInfos, context)
		if err != nil {
			return nil, err
		}
	}

	var pointsReader spi.PointsReader
	if coreFieldInfos.HasPointValues() {
		pointsReader, err = codec.PointsFormat().FieldsReader(segmentReadState)
		if err != nil {
			return nil, err
		}
	}

	var knnReader spi.KnnVectorsReader
	if coreFieldInfos.HasVectorValues() {
		knnReader, err = codec.KnnVectorsFormat().FieldsReader(segmentReadState)
		if err != nil {
			return nil, err
		}
	}

	return &SegmentCoreReaders{
		ref:                  1,
		Fields:               fields,
		Norms:                norms,
		fieldsReaderOrig:    fieldsReader,
		termVectorsReaderOrig: tvReader,
		pointsReader:        pointsReader,
		knnVectorsReader:    knnReader,
		cfsReader:           cfsReader,
		segment:             segment,
		coreFieldInfos:      coreFieldInfos,
	}, nil
}

func (s *SegmentCoreReaders) GetRefCount() int32 {
	return atomic.LoadInt32(&s.ref)
}

func (s *SegmentCoreReaders) IncRef() error {
	for {
		count := atomic.LoadInt32(&s.ref)
		if count <= 0 {
			return fmt.Errorf("SegmentCoreReaders is already closed")
		}
		if atomic.CompareAndSwapInt32(&s.ref, count, count+1) {
			return nil
		}
	}
}

func (s *SegmentCoreReaders) DecRef() error {
	if atomic.AddInt32(&s.ref, -1) == 0 {
		// Close all readers
		if s.Fields != nil {
			s.Fields.Close()
		}
		if s.termVectorsReaderOrig != nil {
			s.termVectorsReaderOrig.Close()
		}
		if s.fieldsReaderOrig != nil {
			s.fieldsReaderOrig.Close()
		}
		if s.cfsReader != nil {
			s.cfsReader.Close()
		}
		if s.Norms != nil {
			s.Norms.Close()
		}
		if s.pointsReader != nil {
			s.pointsReader.Close()
		}
		if s.knnVectorsReader != nil {
			s.knnVectorsReader.Close()
		}
	}
	return nil
}

func (s *SegmentCoreReaders) GetSegmentName() string {
	return s.segment
}

func (s *SegmentCoreReaders) GetFieldInfos() *FieldInfos {
	return s.coreFieldInfos
}

func (s *SegmentCoreReaders) GetStoredFieldsReader() spi.StoredFieldsReader {
	return s.fieldsReaderOrig
}

func (s *SegmentCoreReaders) GetTermVectorsReader() spi.TermVectorsReader {
	return s.termVectorsReaderOrig
}

func (s *SegmentCoreReaders) GetFields() spi.FieldsProducer {
	return s.Fields
}

func (s *SegmentCoreReaders) GetNormsProducer() spi.NormsProducer {
	return s.Norms
}

func (s *SegmentCoreReaders) GetPointsReader() spi.PointsReader {
	return s.pointsReader
}

func (s *SegmentCoreReaders) GetVectorReader() spi.KnnVectorsReader {
	return s.knnVectorsReader
}
