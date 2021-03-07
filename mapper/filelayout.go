package mapper

import (
	"encoding/binary"
	"fmt"
	"log"

	"github.com/martinlindhe/feng/value"
)

// parsed file data from a template "layout"
type FileLayout struct {
	Structs []FileStruct
}

// parsed file data section from a template "struct"
type FileStruct struct {
	Label string

	Fields []fileField
}

type fileField struct {
	Offset uint64
	Length uint64

	// value in network byte order (big)
	Value []byte

	// on-disk endianness
	Endian string

	// underlying data structure
	Format value.DataField
}

// decodes simple value types for presentation
func (ff *fileField) Present() string {

	if ff.Format.Slice || ff.Format.Range != "" {
		return ""
	}

	switch ff.Format.Kind {
	case "u32":
		v := binary.BigEndian.Uint32(ff.Value)
		return fmt.Sprintf("%d", v)
	}
	log.Fatalf("Present unhandled kind %s", ff.Format.Kind)
	return ""
}

func (fl *FileLayout) Present() {
	for _, layout := range fl.Structs {
		fmt.Printf("%s\n", layout.Label)

		for _, field := range layout.Fields {
			kind := field.Format.PresentType()
			if field.Format.SingleUnitSize() > 1 {
				if field.Endian == "little" {
					kind += " le"
				} else {
					kind += " be"
				}
			}

			fmt.Printf("  [%06x] %-30s %-10s %-10s %-20s\n",
				field.Offset, field.Format.Label, kind, field.Present(), fmt.Sprintf("% 02x", field.Value))
		}
	}
}
