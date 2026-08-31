package index

// VectorEncoding represents the numeric datatype of the vector values.
type VectorEncoding int

const (
	// VectorEncodingByte encodes vector using 8 bits of precision per sample.
	// Values provided with higher precision (e.g. queries provided as float)
	// must be in the range [-128, 127]. NOTE: this can enable significant
	// storage savings and faster searches, at the cost of some possible loss of precision.
	VectorEncodingByte VectorEncoding = 1

	// VectorEncodingFloat32 encodes vector using 32 bits of precision per sample
	// in IEEE floating point format.
	VectorEncodingFloat32 VectorEncoding = 4
)

// ByteSize returns the number of bytes required to encode a scalar in this format.
// A vector will nominally require dimension * byteSize bytes of storage.
func (ve VectorEncoding) ByteSize() int {
	return int(ve)
}
