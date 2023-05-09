// Compression format used in:
// The Witcher 2 (DZIP files) - TODO confirm
// Viva Pinata: Party Animals (RKV files) - confirmed decompression works

// TODO: implement compression

package lzf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// based on https://github.com/badboy/lzf-rs/blob/main/src/decompress.rs#L20
func Decompress(r io.Reader, compressedSize uint) ([]byte, error) {

	output := new(bytes.Buffer)
	currentOffset := uint(0)

	if compressedSize == 0 {
		panic("Err(LzfError::DataCorrupted) 0")
	}

	for currentOffset < compressedSize {
		var v uint8
		if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
			panic(err)
		}
		ctrl := uint(v)
		//log.Printf("ctrl %02x from %06x", ctrl, currentOffset)
		currentOffset += 1

		if ctrl < (1 << 5) {
			ctrl += 1

			if currentOffset+ctrl > compressedSize {
				panic("Err(LzfError::DataCorrupted) 1")
			}

			var buf = make([]byte, ctrl)
			nRead, err := io.ReadFull(r, buf)
			if err != nil {
				return nil, err
			}
			if uint(nRead) != ctrl {
				panic("didnt read expected len")
			}

			output.Write(buf[:nRead])
			currentOffset += ctrl

		} else {
			val := ctrl >> 5

			refOffset := int32(((ctrl & 0x1f) << 8) + 1)

			if currentOffset >= compressedSize {
				panic("Err(LzfError::DataCorrupted) 2")
			}

			if val == 7 {
				if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
					panic(err)
				}

				val += uint(v)
				currentOffset += 1

				if currentOffset >= compressedSize {
					panic("Err(LzfError::DataCorrupted) 3")
				}
			}

			if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
				panic(err)
			}

			refOffset += int32(v)
			currentOffset += 1

			refPos := int32(output.Len()) - refOffset
			if refPos < 0 {
				panic(fmt.Errorf("Err(LzfError::DataCorrupted) 4 at %06x: %d", currentOffset-1, refPos))
			}

			c := output.Bytes()[refPos]
			output.WriteByte(c)
			refPos += 1

			c = output.Bytes()[refPos]
			output.WriteByte(c)
			refPos += 1

			for val > 0 {
				c = output.Bytes()[refPos]
				output.WriteByte(c)
				refPos += 1
				val -= 1
			}
		}
	}

	return output.Bytes(), nil
}
