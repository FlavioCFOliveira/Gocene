package tokenattributes

import "github.com/FlavioCFOliveira/Gocene/util"

// TermToBytesRefAttribute is requested by TermsHashPerField to index the contents.
// This attribute can be used to customize the final byte[] encoding of terms.
type TermToBytesRefAttribute interface {
	// GetBytesRef retrieves this attribute's BytesRef. The bytes are updated
	// from the current term.
	GetBytesRef() *util.BytesRef
}
