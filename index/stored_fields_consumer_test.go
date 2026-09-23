// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestStoredFieldsConsumer.java
// (Apache Lucene 10.5.0).

package index

import (
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// countingStoredFieldsConsumer renders the anonymous StoredFieldsConsumer
// subclass of testFinish, whose startDocument/finishDocument overrides call
// super and count the invocations.
type countingStoredFieldsConsumer struct {
	*StoredFieldsConsumer
	startDocCounter  *atomic.Int32
	finishDocCounter *atomic.Int32
}

func (c *countingStoredFieldsConsumer) StartDocument(docID int) error {
	if err := c.StoredFieldsConsumer.StartDocument(docID); err != nil {
		return err
	}
	c.startDocCounter.Add(1)
	return nil
}

func (c *countingStoredFieldsConsumer) FinishDocument() error {
	if err := c.StoredFieldsConsumer.FinishDocument(); err != nil {
		return err
	}
	c.finishDocCounter.Add(1)
	return nil
}

func TestStoredFieldsConsumerFinish(t *testing.T) {
	dir := newDirectory()
	iwc := NewIndexWriterConfig()
	// new SegmentInfo(dir, Version.LATEST, null, "_0", -1, false, false,
	//     iwc.getCodec(), Collections.emptyMap(), StringHelper.randomId(),
	//     new HashMap<>(), null)
	si := NewSegmentInfo("_0", -1, dir)
	si.SetVersion(util.Latest.String())
	si.SetCodec(iwc.GetCodec())
	si.SetDiagnostics(map[string]string{})
	if err := si.SetID(util.RandomId()); err != nil {
		t.Fatalf("setID: %v", err)
	}
	si.SetAttributes(map[string]string{})

	var startDocCounter, finishDocCounter atomic.Int32
	consumer := &countingStoredFieldsConsumer{
		StoredFieldsConsumer: NewStoredFieldsConsumer(iwc.GetCodec(), dir, si),
		startDocCounter:      &startDocCounter,
		finishDocCounter:     &finishDocCounter,
	}

	numDocs := 3
	if err := consumer.Finish(numDocs); err != nil {
		t.Fatalf("finish: %v", err)
	}

	si.SetMaxDoc(numDocs)
	state := NewSegmentWriteState(dir, si, NewFieldInfos(), nil,
		store.IOContextFlush(store.NewFlushInfo(numDocs, 10)))
	if err := consumer.Flush(state, nil); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if err := dir.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if got := int(startDocCounter.Load()); got != numDocs {
		t.Fatalf("startDocCounter: expected %d, got %d", numDocs, got)
	}
	if got := int(finishDocCounter.Load()); got != numDocs {
		t.Fatalf("finishDocCounter: expected %d, got %d", numDocs, got)
	}
}
