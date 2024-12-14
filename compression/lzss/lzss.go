package lzss

import (
	"bytes"
	"fmt"
	"io"

	"github.com/spf13/afero"
)

// Decompression algorithm tested working correctly with:
// Blinx 2
// Mario Party 4
// Namco Museum Megamix

// Decompress decompresses LZSS a data stream.
// Implementation based on https://github.com/blacktop/lzss/blob/5db4a74c19d62a8e41860aa404cd76a3ac5a49ac/lzss.go
func Decompress(in afero.File, compressedSize uint) ([]byte, error) {
	// n is the size of ring buffer - must be power of 2
	n := 4096

	// f is the upper limit for match_length
	f := 18

	threshold := 2

	var i, j, r, c int
	var flags uint

	dst := bytes.Buffer{}

	// ring buffer of size n, with extra f-1 bytes to aid string comparison
	textBuf := make([]byte, n+f-1)

	r = n - f
	flags = 0

	for {
		flags = flags >> 1
		if ((flags) & 0x100) == 0 {
			bite, err := readByte(in)
			if err != nil {
				break
			}
			c = int(bite)
			flags = uint(c | 0xFF00) /* uses higher byte cleverly to count eight*/
		}
		if flags&1 == 1 {
			bite, err := readByte(in)
			if err != nil {
				break
			}
			c = int(bite)
			dst.WriteByte(byte(c))
			textBuf[r] = byte(c)
			r++
			r &= (n - 1)
		} else {
			bite, err := readByte(in)
			if err != nil {
				break
			}
			i = int(bite)

			bite, err = readByte(in)
			if err != nil {
				break
			}
			j = int(bite)

			i |= ((j & 0xF0) << 4)
			j = (j & 0x0F) + threshold
			for k := 0; k <= j; k++ {
				c = int(textBuf[(i+k)&(n-1)])
				dst.WriteByte(byte(c))
				textBuf[r] = byte(c)
				r++
				r &= (n - 1)
			}
		}
	}

	return dst.Bytes(), nil
}

func Compress(in []byte, w io.Writer) error {
	return fmt.Errorf("TODO lzss compression is not implemented")
}

func readByte(f io.Reader) (byte, error) {
	buf := make([]byte, 1)
	_, err := f.Read(buf)
	return buf[0], err
}
