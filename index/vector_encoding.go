package index

// VectorEncoding represents the numeric datatype of the vector values.
type VectorEncoding int

const (
	// Byte encodes vector using 8 bits of precision per sample.
	Byte VectorEncoding = iota
	// Float32 encodes vector using 32 bits of precision per sample in IEEE floating point format.
	Float32
)

// ByteSize returns the number of bytes required to encode a scalar in this format.
func (ve VectorEncoding) ByteSize() int {
	switch ve {
	case Byte:
		return 1
	case Float32:
		return 4
	default:
		panic("unknown vector encoding")
	}
}
