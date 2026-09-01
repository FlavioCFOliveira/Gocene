package cranky

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

type mockStoredFieldsWriter struct {
	spi.BaseStoredFieldsWriter
}

func (w *mockStoredFieldsWriter) StartDocument() error { return nil }
func (w *mockStoredFieldsWriter) FinishDocument() error { return nil }
func (w *mockStoredFieldsWriter) WriteField(f spi.IndexableField) error { return nil }
func (w *mockStoredFieldsWriter) Finish(n int) error { return nil }
func (w *mockStoredFieldsWriter) Close() error { return nil }

type mockStoredFieldsFormat struct {
	spi.BaseStoredFieldsFormat
}

func (f *mockStoredFieldsFormat) FieldsReader(dir store.Directory, si *index.SegmentInfo, fi *index.FieldInfos, ctx store.IOContext) (spi.StoredFieldsReader, error) {
	return nil, nil
}
func (f *mockStoredFieldsFormat) FieldsWriter(dir store.Directory, si *index.SegmentInfo, ctx store.IOContext) (spi.StoredFieldsWriter, error) {
	return &mockStoredFieldsWriter{}, nil
}

func TestCrankyStoredFieldsFormat(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	fmt := NewCrankyStoredFieldsFormat(&mockStoredFieldsFormat{}, rng)

	failed := false
	for i := 0; i < 1000; i++ {
		_, err := fmt.FieldsWriter(nil, nil, store.IOContext{})
		if err != nil {
			failed = true
			break
		}
	}
	if !failed {
		t.Error("CrankyStoredFieldsFormat should have failed at least once in 1000 iterations")
	}
}

func TestCrankyPostingsFormat(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	delegate := &mockPostingsFormat{}
	fmt := NewCrankyPostingsFormat(delegate, rng)

	failed := false
	for i := 0; i < 1000; i++ {
		_, err := fmt.FieldsConsumer(nil)
		if err != nil {
			failed = true
			break
		}
	}
	if !failed {
		t.Error("CrankyPostingsFormat should have failed at least once in 1000 iterations")
	}
}

type mockPostingsFormat struct {
	spi.BasePostingsFormat
}

func (f *mockPostingsFormat) FieldsConsumer(state *index.SegmentWriteState) (spi.FieldsConsumer, error) {
	return &mockFieldsConsumer{}, nil
}
func (f *mockPostingsFormat) FieldsProducer(state *index.SegmentReadState) (spi.FieldsProducer, error) {
	return nil, nil
}

type mockFieldsConsumer struct {
	spi.BaseFieldsConsumer
}

func (c *mockFieldsConsumer) Write(field string, terms spi.Terms) error { return nil }
func (c *mockFieldsConsumer) Close() error { return nil }

func TestCrankyNormsFormat(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	delegate := &mockNormsFormat{}
	fmt := NewCrankyNormsFormat(delegate, rng)

	failed := false
	for i := 0; i < 1000; i++ {
		_, err := fmt.NormsConsumer(nil)
		if err != nil {
			failed = true
			break
		}
	}
	if !failed {
		t.Error("CrankyNormsFormat should have failed at least once in 1000 iterations")
	}
}

type mockNormsFormat struct {
	spi.BaseNormsFormat
}

func (f *mockNormsFormat) NormsConsumer(state *index.SegmentWriteState) (spi.NormsConsumer, error) {
	return &mockNormsConsumer{}, nil
}
func (f *mockNormsFormat) NormsProducer(state *index.SegmentReadState) (spi.NormsProducer, error) {
	return nil, nil
}

type mockNormsConsumer struct {
	spi.BaseNormsConsumer
}

func (c *mockNormsConsumer) AddNormsField(f *index.FieldInfo, p spi.NormsProducer) error { return nil }
func (c *mockNormsConsumer) Close() error { return nil }

func TestCrankyFieldInfosFormat(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	delegate := &mockFieldInfosFormat{}
	fmt := NewCrankyFieldInfosFormat(delegate, rng)

	failed := false
	for i := 0; i < 1000; i++ {
		err := fmt.Write(nil, nil, "", nil, store.IOContext{})
		if err != nil {
			failed = true
			break
		}
	}
	if !failed {
		t.Error("CrankyFieldInfosFormat should have failed at least once in 1000 iterations")
	}
}

type mockFieldInfosFormat struct {
	spi.BaseFieldInfosFormat
}

func (f *mockFieldInfosFormat) Read(dir store.Directory, si *index.SegmentInfo, s string, ctx store.IOContext) (*index.FieldInfos, error) {
	return nil, nil
}
func (f *mockFieldInfosFormat) Write(dir store.Directory, si *index.SegmentInfo, s string, fi *index.FieldInfos, ctx store.IOContext) error {
	return nil
}
