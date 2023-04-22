// SOURCE: https://github.com/yole/Gibbed.RED/blob/bcf096521adb919723ca0bb78a113b4fbbb2c6ac/Gibbed.RED.Unpack/LZF.case

// Used in The Witcher 2 (DZIP files)

package lzf

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
)

// C#      rust       go
// int     i32        int32
// uint    u32        uint32
// long    i64        int64
// ulong   u64        uint64
// https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/builtin-types/built-in-types

func Decompress(r io.ReadSeeker, uncompressedSize uint32) ([]byte, error) {

	baseOffset := uint32(0)
	var blocks = int(uncompressedSize+0xFFFF) >> 16 // .Align(0x10000) / 0x10000;

	var offsets = make([]uint32, blocks+1)

	buf := make([]byte, 4)
	log.Printf("Reading %d blocks ...", blocks)
	for i := 0; i < len(offsets); i++ {

		n, err := r.Read(buf)
		if err != nil {
			return nil, err
		}

		tt := binary.LittleEndian.Uint32(buf[:n])
		log.Printf("Reading block %d offset = %08x", i, tt)

		offsets[i] = baseOffset + tt
	}
	offsets[blocks] = baseOffset + uncompressedSize

	var left = uncompressedSize

	var uncompressed = make([]byte, 0x10000)

	var output = []byte{}

	for i := 0; i < blocks; i++ {
		var compressed = make([]byte, offsets[i+1]-offsets[i+0])
		log.Printf("Expects next compressed block i=%d, [i+1]=%d and [i]=%d   = len %d", i, offsets[i+1], offsets[i+0], len(compressed))
		log.Printf("Reading data from block %d at 0x%04x", i, offsets[i])
		r.Seek(int64(offsets[i]), io.SeekStart)
		n, err := r.Read(compressed)
		if err != nil {
			return nil, err
		}
		if n != len(compressed) {
			log.Fatalf("did not read enough data. wanted %d, got %d", len(compressed), n)
		}

		decompressedSize, err := decompressBlock(compressed, uncompressed)
		if err != nil {
			return nil, err
		}
		if i+1 < blocks && decompressedSize != n {
			panic("Decompress InvalidOperation0")
		}

		//output.Write(uncompressed, 0, min(int(left), decompressedSize))
		output = append(output, uncompressed[:min(int(left), decompressedSize)]...)
		left -= uint32(decompressedSize)
	}

	return output, nil

}

func decompressBlock(input, output []byte) (int, error) {
	// ---
	var i = 0
	var o = 0

	var inputLength = len(input)
	var outputLength = len(output)

	for i < inputLength {
		var control = uint32(input[i])
		i++

		if control < (1 << 5) { // 0x20
			var length = int(control + 1)

			if o+length > outputLength {
				return 0, fmt.Errorf("InvalidOperation1 at %d", i)
			}

			//Array.Copy(input, i, output, o, length)
			copy(output[o:length], input[i:]) // XXX length or o+length ?
			log.Printf("Copied %d bytes from input [0x%04x]", length, i)

			i += length
			o += length

		} else {

			var length = (int)(control >> 5)
			var offset = (int)((control & 0x1F) << 8)

			if length == 7 {
				length += int(input[i])
				i++
			}
			length += 2

			log.Printf("offset=%d, input[i]=%d", offset, input[i])
			offset |= int(input[i])
			log.Printf("offset | input[i] = %d", offset)
			i++

			if o+length > outputLength {
				return 0, fmt.Errorf("InvalidOperation2 at %d: o=%d, length=%d, outputLength=%d", i, o, length, outputLength)
			}

			offset = o - 1 - offset
			log.Printf("offset=%d, o=%d", offset, o)
			if offset < 0 {
				return 0, fmt.Errorf("InvalidOperation3 at %d: o=%d, offset=%d", i, o, offset)
			}

			var block = min(length, o-offset)

			//Array.Copy(output, offset, output, o, block)
			copy(output[o:block], output[offset:]) // XXX block or o+block ?
			log.Printf("Copied %d bytes from input[0x%04x]", block, offset)

			o += block
			offset += block
			length -= block

			for length > 0 {
				output[o] = output[offset]
				o++
				offset++
				length--
			}
		}
	}

	return o, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
