package codecs

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// NormsProducer is an abstract API that produces field normalization values.
type NormsProducer interface {
	io.Closer

	// GetNorms returns NumericDocValues for this field. The returned instance need not be thread-safe:
	// it will only be used by a single thread. The behavior is undefined if the given field doesn't
	// have norms enabled on its FieldInfo. The return value is never nil.
	GetNorms(field index.FieldInfo) (index.NumericDocValues, error)

	// CheckIntegrity checks consistency of this producer.
	//
	// Note that this may be costly in terms of I/O, e.g. may involve computing a checksum value
	// against large data files.
	CheckIntegrity() error

	// GetMergeInstance returns an instance optimized for merging. This instance may only be used from the thread that
	// acquires it.
	GetMergeInstance() NormsProducer
}
