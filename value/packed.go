package value

import (
	"bytes"
	"fmt"

	"github.com/rs/zerolog/log"
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

// this encoding is used by archive/xz
// returns decoded value, raw bytes, byte length, error
func ReadVariableLengthU64(r *bytes.Reader) (uint64, []byte, uint64, error) {

	accum := uint64(0)
	raw := []byte{}

	for i := 0; i < 9; i++ {
		v, err := r.ReadByte()
		if err != nil {
			return 0, nil, 0, err
		}

		if v != 0 {
			//			panic("XXX") // XXX the xz decode() example returns error here
			raw = append(raw, v)
			accum |= (uint64(v) & 0x7f) << (i * 7)
		}

		if v&0x80 == 0 {
			return accum, raw, uint64(i + 1), nil
		}
	}

	return 0, nil, 0, fmt.Errorf("exceeds 9 bytes")
}

// Codes integers in 7-bit chunks, little-endian order. The high-bit in each byte signifies if it is the last byte.
// used by system/macos/nibarchive
func ReadVariableLengthS64(r *bytes.Reader) (uint64, []byte, uint64, error) {

	accum := uint64(0)
	raw := []byte{}

	for i := 0; i < 9; i++ {
		v, err := r.ReadByte()
		if err != nil {
			return 0, nil, 0, err
		}
		raw = append(raw, v)
		accum |= (uint64(v) & 0x7F) << (i * 7)
		log.Info().Msgf("Read %02x (byte %d)", v, i)
		if (v & 0x80) != 0 {
			return accum, raw, uint64(i + 1), nil
		}
	}
	return 0, nil, 0, fmt.Errorf("exceeds 9 bytes")
}
