package mapper

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/martinlindhe/feng/value"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
)

const DEBUG_READ = false

var fs1 = afero.NewOsFs()

func (fl *FileLayout) peekU32(offset int64) (uint32, error) {
	prevOffset, err := fl._f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	_, _ = fl._f.Seek(fl.startOffset+offset, io.SeekStart)
	buf := make([]byte, 4)
	n, _ := fl._f.Read(buf)
	fl.bytesRead += n
	val := binary.LittleEndian.Uint32(buf)
	_, _ = fl._f.Seek(prevOffset, io.SeekStart)
	return val, nil
}

func (fl *FileLayout) peekU16(offset int64) (uint16, error) {
	prevOffset, err := fl._f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	_, _ = fl._f.Seek(fl.startOffset+offset, io.SeekStart)
	buf := make([]byte, 2)
	n, _ := fl._f.Read(buf)
	fl.bytesRead += n
	val := binary.LittleEndian.Uint16(buf)
	_, _ = fl._f.Seek(prevOffset, io.SeekStart)
	return val, nil
}

func (fl *FileLayout) peekU8(offset int64) (uint8, error) {
	prevOffset, err := fl._f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	_, _ = fl._f.Seek(fl.startOffset+offset, io.SeekStart)
	buf := make([]byte, 1)
	n, _ := fl._f.Read(buf)
	fl.bytesRead += n
	_, _ = fl._f.Seek(prevOffset, io.SeekStart)
	return buf[0], nil
}

// returns a reader pointing to start of data for the given `Field`.
func (fl *FileLayout) readerToField(field *Field) (r afero.File, err error) {

	if field.ImportFile != "" {
		size := field.Format.RangeVal
		log.Info().Msgf("IMPORT % 2d bytes from %06x in %s", size, field.Offset, field.ImportFile)

		f, err := fs1.Open(field.ImportFile)
		if err != nil {
			return nil, err
		}
		//		defer f.Close()

		_, err = f.Seek(fl.startOffset+field.Offset, io.SeekStart)
		return f, err
	}

	_, err = fl._f.Seek(fl.startOffset+field.Offset, io.SeekStart)
	return fl._f, err
}

// returns a slice of bytes from file, otherwise unmodified
func (fl *FileLayout) peekBytes(field *Field) (b []byte, err error) { // XXX deprecate, use readerToField and use io.Reader all over

	if field.ImportFile != "" {
		size := field.Format.RangeVal
		log.Info().Msgf("IMPORT % 2d bytes from %06x in %s", size, field.Offset, field.ImportFile)

		f, err2 := os.Open(field.ImportFile) // XXX use afero
		if err2 != nil {
			return nil, err2
		}
		defer f.Close()

		_, err = f.Seek(field.Offset, io.SeekStart)
		if err != nil {
			return nil, err
		}

		b = make([]byte, size)
		var n int
		n, err = f.Read(b)
		fl.bytesImported += n
	} else {
		b, err = fl.peekBytesMainFile(field.Offset, field.Length)
	}

	// convert to network byte order
	unitLength, _ := fl.GetAddressLengthPair(&field.Format)
	if unitLength > 1 && field.Endian == "little" {
		b = value.ReverseBytes(b, int(unitLength))
	}
	return
}

func (fl *FileLayout) peekBytesMainFile(offset int64, size int64) ([]uint8, error) {
	if DEBUG_READ {
		log.Info().Msgf("PEEK % 2d from %06x", size, offset)
	}
	prevOffset, err := fl._f.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	_, _ = fl._f.Seek(fl.startOffset+offset, io.SeekStart)
	data := make([]byte, size)
	n, _ := fl._f.Read(data)
	fl.bytesRead += n
	_, _ = fl._f.Seek(prevOffset, io.SeekStart)

	data = fl.xorResult(data)
	return data, nil
}

// reads bytes from reader and returns them in network byte order (big endian)
func (fl *FileLayout) readBytes(totalLength, unitLength int64, endian string) ([]byte, error) {
	if unitLength > 1 && endian == "" {
		return nil, fmt.Errorf("endian is not set in file format template, don't know how to read data")
	}

	if totalLength > 1024*1024*1024 {
		return nil, fmt.Errorf("readBytes: attempt to read unexpected amount of data %d", totalLength)
	}
	if totalLength < 0 {
		return nil, fmt.Errorf("readBytes: negative totalLength %d", totalLength)
	}

	val := make([]byte, totalLength)
	if DEBUG_READ {
		log.Info().Msgf("READ % 2d from %06x", totalLength, fl.startOffset+fl.offset)
	}
	if _, err := io.ReadFull(fl._f, val); err != nil {
		return nil, err
	}
	fl.bytesRead += int(totalLength)

	// convert to network byte order
	if unitLength > 1 && endian == "little" {
		val = value.ReverseBytes(val, int(unitLength))
	}

	return fl.xorResult(val), nil
}

// transforms data according to fl.xorKey
func (fl *FileLayout) xorResult(in []byte) []byte {
	if len(fl.xorKey) == 0 {
		return in
	}

	if len(fl.xorKey) != 1 {
		log.Fatal().Msgf("TODO impl multi-byte xor key")
	}

	out := make([]byte, len(in))
	for idx, b := range in {
		out[idx] = b ^ fl.xorKey[0]
	}
	//log.Info().Msgf("xor %02x to %02x", in, out)
	return out
}

// this encoding is used by fonts/woff2 (UIntBase128)
// returns decoded value, raw bytes, byte length, error
func (fl *FileLayout) ReadVariableLengthU32() (uint32, []byte, int64, error) {
	accum := uint32(0)
	raw := []byte{}
	for i := 0; i < 5; i++ {

		buf, err := fl.readBytes(1, 1, fl.endian)
		v := buf[0]

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
			return accum, raw, int64(i + 1), nil
		}
	}
	return 0, nil, 0, fmt.Errorf("exceeds 5 bytes")
}

// this encoding is used by archive/xz
// returns decoded value, raw bytes, byte length, error
func (fl *FileLayout) ReadVariableLengthU64() (uint64, []byte, int64, error) {

	accum := uint64(0)
	raw := []byte{}

	for i := 0; i < 9; i++ {

		buf, err := fl.readBytes(1, 1, fl.endian)
		v := buf[0]

		if err != nil {
			return 0, nil, 0, err
		}

		if v != 0 {
			//			panic("XXX") // XXX the xz decode() example returns error here
			raw = append(raw, v)
			accum |= (uint64(v) & 0x7f) << (i * 7)
		}

		if v&0x80 == 0 {
			return accum, raw, int64(i + 1), nil
		}
	}

	return 0, nil, 0, fmt.Errorf("exceeds 9 bytes")
}

// Codes integers in 7-bit chunks, little-endian order. The high-bit in each byte signifies if it is the last byte.
// used by system/macos/nibarchive
func (fl *FileLayout) ReadVariableLengthS64() (uint64, []byte, int64, error) {

	accum := uint64(0)
	raw := []byte{}

	for i := 0; i < 9; i++ {

		buf, err := fl.readBytes(1, 1, fl.endian)
		v := buf[0]

		if err != nil {
			return 0, nil, 0, err
		}
		raw = append(raw, v)
		accum |= (uint64(v) & 0x7F) << (i * 7)
		log.Info().Msgf("Read %02x (byte %d)", v, i)
		if (v & 0x80) != 0 {
			return accum, raw, int64(i + 1), nil
		}
	}
	return 0, nil, 0, fmt.Errorf("exceeds 9 bytes")
}

// reads bytes from reader until 0x00 is found. returned data includes the terminating 0x00
func (fl *FileLayout) readBytesUntilMarkerByte(marker byte) ([]byte, error) {

	b := make([]byte, 1)

	res := []byte{}

	for {
		if DEBUG_READ {
			log.Info().Msgf("Reading % 2d (READ UNTIL MARKER %02x)", len(b), marker)
		}

		n, err := io.ReadFull(fl._f, b)
		if err != nil {
			return nil, err
		}

		b = fl.xorResult(b)

		fl.bytesRead += int(n)

		res = append(res, b[0])
		if b[0] == marker {
			break
		}
	}
	return res, nil
}

// reads bytes from reader until the marker byte sequence is found. returned data excludes the marker
// FIXME: won't find patterns overlapping chunks
func (fl *FileLayout) readBytesUntilMarkerSequence(chunkSize int64, search []byte) ([]byte, error) {

	if int(chunkSize) < len(search) {
		panic("unlikely")
	}

	chunk := make([]byte, int(chunkSize)+len(search))

	if DEBUG_READ {
		log.Info().Msgf("Reading % 2d (READ #1 UNTIL MARKER %02x)", len(chunk), search)
	}
	n, err := fl._f.Read(chunk[:chunkSize])
	fl.bytesRead += int(n)

	res := []byte{}

	var offset int64
	idx := bytes.Index(chunk[:chunkSize], search)
	for {
		//log.Printf("Read a slice of len %d, Index %d: % 02x", n, idx, chunk[:4])
		if idx >= 0 {
			res = append(res, chunk[:idx]...)

			// rewind to before marker
			_, err = fl._f.Seek(int64(-(n - idx)), io.SeekCurrent)
			return res, err
		} else {
			//log.Printf("appended %d bytes: % 02x, res is %d len", len(chunk[:chunkSize]), chunk[:4], len(res))
			res = append(res, chunk[:chunkSize]...)
		}
		if err == io.EOF {
			log.Error().Msgf("reached EOF")
			return nil, nil
		} else if err != nil {
			return nil, err
		}

		offset += chunkSize

		if DEBUG_READ {
			log.Info().Msgf("Reading % 2d (READ #2 UNTIL MARKER %02x)", chunkSize, search)
		}
		n, err = fl._f.Read(chunk[:chunkSize])
		fl.bytesRead += int(n)

		idx = bytes.Index(chunk[:chunkSize], search)
	}
}
