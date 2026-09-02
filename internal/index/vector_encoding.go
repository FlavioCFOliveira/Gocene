package index

// VectorEncoding describes the numeric datatype of the vector values.
type VectorEncoding int

const (
	VectorEncodingByte VectorEncoding = iota
	VectorEncodingFloat32
)

// ByteSize returns the number of bytes required to encode a scalar in this format.
func (ve VectorEncoding) ByteSize() int {
	switch ve {
	case VectorEncodingByte:
		return 1
	case VectorEncodingFloat32:
		return 4
	default:
		return 0
	}
}
