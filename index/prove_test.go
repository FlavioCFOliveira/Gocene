package index_test

import (
    "testing"
    
    "github.com/FlavioCFOliveira/Gocene/codecs"
    "github.com/FlavioCFOliveira/Gocene/index"
    "github.com/FlavioCFOliveira/Gocene/spi"
    "github.com/FlavioCFOliveira/Gocene/store"
    _ "github.com/FlavioCFOliveira/Gocene/codecs"
)

type localDelegate interface {
    AddSortedFieldFromReader(field *index.FieldInfo, reset func() (index.SortedDocValues, error)) error
    AddSortedSetFieldFromReader(field *index.FieldInfo, reset func() (index.SortedSetDocValues, error)) error
}

func TestConsumerImplementsDelegate(t *testing.T) {
    c := index.GetDefaultCodec()
    t.Logf("codec: %T %s", c, c.Name())
    dvf := c.DocValuesFormat()
    t.Logf("dvf: %T %s", dvf, dvf.Name())
    
    // Create segment write state
    dir := store.NewByteBuffersDirectory()
    segInfo := index.NewSegmentInfo("_0", 10, dir)
    fi := index.NewFieldInfos()
    state := &codecs.SegmentWriteState{
        SegmentInfo:   segInfo,
        FieldInfos:    fi,
        Directory:     dir,
        SegmentSuffix: "",
    }
    
    consumer, err := dvf.FieldsConsumer(state)
    if err != nil {
        t.Fatalf("FieldsConsumer: %v", err)
    }
    t.Logf("consumer type: %T", consumer)
    
    // Direct type assert
    pc, ok := consumer.(*codecs.PerFieldDocValuesConsumer)
    if ok {
        t.Logf("consumer is *PerFieldDocValuesConsumer: YES")
        _ = pc
    } else {
        t.Logf("consumer is *PerFieldDocValuesConsumer: NO")
    }
    
    // Local delegate interface
    ld, ok := consumer.(localDelegate)
    if ok {
        t.Logf("consumer implements localDelegate: YES")
        _ = ld
    } else {
        t.Logf("consumer implements localDelegate: NO")
    }
    
    // Real delegate type
    // Try creating the delegate by calling AddSortedFieldFromReader
    // Just verify compile-time matching
    var _ localDelegate = &codecs.PerFieldDocValuesConsumer{}
    _ = consumer
}
