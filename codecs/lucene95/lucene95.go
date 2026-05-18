// Package lucene95 hosts the Sprint 49 ports for
// org.apache.lucene.codecs.lucene95.
package lucene95

// The Sprint 49 lucene95 port surfaces these types as typed stubs so
// dependent packages keep compiling; concrete behaviour ports (off-heap
// IndexInput-backed vector value scans, OrdToDoc DISI configuration
// blocks) land in follow-up deep-port sprints.

// HasIndexSlice mirrors
// org.apache.lucene.codecs.lucene95.HasIndexSlice.
type HasIndexSlice struct{}

// NewHasIndexSlice builds a HasIndexSlice.
func NewHasIndexSlice() *HasIndexSlice { return &HasIndexSlice{} }

// OffHeapByteVectorValues mirrors
// org.apache.lucene.codecs.lucene95.OffHeapByteVectorValues.
type OffHeapByteVectorValues struct{}

// NewOffHeapByteVectorValues builds an OffHeapByteVectorValues.
func NewOffHeapByteVectorValues() *OffHeapByteVectorValues { return &OffHeapByteVectorValues{} }

// OffHeapFloatVectorValues mirrors
// org.apache.lucene.codecs.lucene95.OffHeapFloatVectorValues.
type OffHeapFloatVectorValues struct{}

// NewOffHeapFloatVectorValues builds an OffHeapFloatVectorValues.
func NewOffHeapFloatVectorValues() *OffHeapFloatVectorValues { return &OffHeapFloatVectorValues{} }

// OrdToDocDISIReaderConfiguration mirrors
// org.apache.lucene.codecs.lucene95.OrdToDocDISIReaderConfiguration.
type OrdToDocDISIReaderConfiguration struct{}

// NewOrdToDocDISIReaderConfiguration builds an
// OrdToDocDISIReaderConfiguration.
func NewOrdToDocDISIReaderConfiguration() *OrdToDocDISIReaderConfiguration {
	return &OrdToDocDISIReaderConfiguration{}
}
