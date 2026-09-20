package hppc

// BitMixer provides utilities to evenly distribute key space over int32 range.
type BitMixer struct{}

func Mix32(k int32) int32 {
	uk := uint32(k)
	uk = (uk ^ (uk >> 16)) * 0x85ebca6b
	uk = (uk ^ (uk >> 13)) * 0xc2b2ae35
	return int32(uk ^ (uk >> 16))
}

func Mix64(z int64) int64 {
	uz := uint64(z)
	uz = (uz ^ (uz >> 32)) * 0x4cd6944c5cc20b6
	uz = (uz ^ (uz >> 29)) * 0xfc12c5b19d3259e9
	return int64(uz ^ (uz >> 32))
}

const phiC32 = uint32(0x9e3779b9)
const phiC64 = uint64(0x9e3779b97f4a7c15)

func MixPhi(k int32) int32 {
	uk := uint32(k)
	h := uk * phiC32
	return int32(h ^ (h >> 16))
}

func MixPhi64(k int64) int32 {
	uk := uint64(k)
	h := uk * phiC64
	return int32(h ^ (h >> 32))
}
