package value

import (
	"bytes"
	"fmt"
)

// this encoding is used by fonts/woff2 (UIntBase128)
// returns decoded value, raw bytes, byte length, error
func ReadVariableLengthU32(r *bytes.Reader) (uint32, []byte, uint64, error) {
	accum := uint32(0)
	raw := []byte{}
	for i := 0; i < 5; i++ {
		v, err := r.ReadByte()
		if err != nil {
			return 0, nil, 0, err
		}
		if i == 0 && v == 0x80 {
			return 0, nil, 0, fmt.Errorf("no leading 0's")
		}
		// If any of top 7 bits are set then << 7 would overflow
		if accum&0xFE000000 != 0 {
			return 0, nil, 0, fmt.Errorf("would overflow")
		}
		raw = append(raw, v)
		accum = (accum << 7) | (uint32(v) & 0x7F)
		if v&0x80 == 0 {
			return accum, raw, uint64(i + 1), nil
		}
	}
	return 0, nil, 0, fmt.Errorf("exceeds 5 bytes")
}
