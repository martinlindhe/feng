// Compression format used in:
// The Witcher 2 (DZIP files) - TODO confirm
// Viva Pinata: Party Animals (RKV files) - confirmed decompression works

// TODO: implement compression

package lzf

import (
	"fmt"
	"io"
	"log"
)

func Decompress(r io.Reader, compressedSize uint) ([]byte, error) {

	// TODO: stream-based decompression ?

	compressed := make([]byte, compressedSize)

	n, err := r.Read(compressed)
	if err != nil {
		return nil, err
	}
	if n != int(compressedSize) {
		log.Fatalf("did not read enough data. wanted %d, got %d", len(compressed), n)
	}

	return decompressBlock(compressed, 9_500_000) // the embedded M048_env_X3.rkv contains M048_Pinata_Safari_01.mdg with expanded size 9126464
}

// based on https://github.com/badboy/lzf-rs/blob/main/src/decompress.rs#L20
func decompressBlock(input []byte, uncompressedSize uint) ([]byte, error) {

	output := make([]byte, uncompressedSize)
	outLen := uint(0)
	currentOffset := uint(0)
	inLen := uint(len(input))

	if inLen == 0 {
		panic("Err(LzfError::DataCorrupted) 0")
	}

	for currentOffset < inLen {
		ctrl := uint(input[currentOffset])

		//log.Printf("ctrl %02x from %06x", ctrl, currentOffset)

		currentOffset += 1

		if ctrl < (1 << 5) {
			ctrl += 1

			if outLen+ctrl > uncompressedSize {
				panic(fmt.Errorf("Err(LzfError::BufferTooSmall) 1: %d > %d", outLen+ctrl, uncompressedSize))
			}

			if currentOffset+ctrl > inLen {
				panic("Err(LzfError::DataCorrupted) 1")
			}

			copy(output[outLen:outLen+ctrl], input[currentOffset:currentOffset+ctrl])

			currentOffset += ctrl
			outLen += ctrl
		} else {
			val := ctrl >> 5

			refOffset := int32(((ctrl & 0x1f) << 8) + 1)

			if currentOffset >= inLen {
				panic("Err(LzfError::DataCorrupted) 2")
			}

			if val == 7 {
				val += uint(input[currentOffset])
				currentOffset += 1

				if currentOffset >= inLen {
					panic("Err(LzfError::DataCorrupted) 3")
				}
			}

			refOffset += int32(input[currentOffset])
			currentOffset += 1

			if outLen+val+2 > uncompressedSize {
				panic(fmt.Errorf("Err(LzfError::BufferTooSmall) 2: %d > %d", outLen+val+2, uncompressedSize))
			}

			refPos := int32(outLen) - refOffset
			if refPos < 0 {
				panic("Err(LzfError::DataCorrupted) 4")
			}

			c := output[refPos]
			output[outLen] = c
			outLen += 1
			refPos += 1

			c = output[refPos]
			output[outLen] = c
			outLen += 1
			refPos += 1

			for val > 0 {
				c = output[refPos]
				output[outLen] = c
				outLen += 1
				refPos += 1
				val -= 1
			}
		}
	}

	return output[:outLen], nil
}
