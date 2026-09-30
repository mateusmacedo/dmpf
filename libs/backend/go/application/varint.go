package application

// The layout of encoding/binary's Uvarint and Varint, rewritten because the
// depguard list of this block leaves that package out.
const maxVarintLen = 10

func appendUvarint(buf []byte, value uint64) []byte {
	for value >= 0x80 {
		buf = append(buf, byte(value)|0x80)
		value >>= 7
	}
	return append(buf, byte(value))
}

func appendVarint(buf []byte, value int64) []byte {
	zigzag := uint64(value) << 1
	if value < 0 {
		zigzag = ^zigzag
	}
	return appendUvarint(buf, zigzag)
}

func readUvarint(buf []byte) (uint64, int) {
	var value uint64
	var shift uint
	for i, b := range buf {
		if i == maxVarintLen || (i == maxVarintLen-1 && b > 1) {
			return 0, -(i + 1)
		}
		if b < 0x80 {
			return value | uint64(b)<<shift, i + 1
		}
		value |= uint64(b&0x7f) << shift
		shift += 7
	}
	return 0, 0
}

func readVarint(buf []byte) (int64, int) {
	zigzag, n := readUvarint(buf)
	value := int64(zigzag >> 1)
	if zigzag&1 != 0 {
		value = ^value
	}
	return value, n
}
