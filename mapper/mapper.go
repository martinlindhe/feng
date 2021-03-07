package mapper

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"strconv"

	"github.com/martinlindhe/feng/template"
	"github.com/martinlindhe/feng/value"
)

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
func (ff *fileField) PresentValue() string {

	if ff.Format.Slice || ff.Format.Range != "" {
		return ""
	}

	switch ff.Format.Kind {
	case "u32":
		v := binary.BigEndian.Uint32(ff.Value)
		return fmt.Sprintf("%d", v)
	}
	log.Fatalf("PresentValue unhandled kind %s", ff.Format.Kind)
	return ""
}

const (
	DEBUG = false
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

// produces a list of fields with offsets and sizes from input reader based on data structure
func MapReader(r io.Reader, ds *template.DataStructure) (*FileLayout, error) {

	endian := ""

	offset := uint64(0)

	fileLayout := FileLayout{}

	for _, layout := range ds.Layout {
		if layout.Slice {
			log.Fatalf("TODO handle sliced layout %#v", layout)
		}
		if layout.Range != "" {
			log.Fatalf("TODO handle ranged layout %#v", layout)
		}
		if DEBUG {
			fmt.Printf("appending struct '%s'\n", layout.PresentType())
		}

		struct_, err := ds.FindStructure(&layout)
		if err != nil {
			panic(err)
		}

		fs := FileStruct{Label: layout.Label}

		for _, es := range struct_.Expressions {
			var field fileField
			switch es.Field.Kind {
			case "endian":
				// special form
				endian = es.Pattern.Value
				if DEBUG {
					fmt.Printf("endian changed to '%s'\n", endian)
				}

			case "u8", "u16", "u32", "u64":
				if es.Field.IsRangeUnit() {
					log.Fatalf("XXX invalid u8[%s] form", es.Field.Range)
				}

				unitLength := es.Field.SingleUnitSize()
				rangeLength := uint64(1)

				if es.Field.Range != "" {
					rangeLength, err = strconv.ParseUint(es.Field.Range, 10, 64) // XXX evaluate range
					if err != nil {
						log.Fatalf("cant parse uint '%s': %v", es.Field.Range, err)
					}
				}

				totalLength := unitLength * rangeLength

				val := make([]byte, totalLength)
				if DEBUG {
					log.Printf("[%08x] reading %d bytes", offset, totalLength)
				}
				if _, err := io.ReadFull(r, val); err != nil {
					return nil, err
				}

				if unitLength > 1 && endian == "" {
					return nil, fmt.Errorf("endian is not set in file format template, don't know how to read data")
				}

				// always convert read value to network byte order (big) before comparisions
				if unitLength > 1 && endian == "little" {
					val = value.ReverseBytes(val, int(unitLength))
				}

				// if known value, see if value is in file data
				if es.Pattern.Known {
					if !bytes.Equal(es.Pattern.Pattern, val) {
						return nil, fmt.Errorf("[%08x] pattern '%s' does not match. expected '% 02x', got '% 02x'", offset, es.Field.Label, es.Pattern.Pattern, val)
					}
				}

				field = fileField{Offset: offset, Length: totalLength, Value: val, Format: es.Field, Endian: endian}
				fs.Fields = append(fs.Fields, field)

			default:
				return nil, fmt.Errorf("MapReader: unhandled field kind '%s'", es.Field.Kind)
			}
			offset += field.Length
		}

		fileLayout.Structs = append(fileLayout.Structs, fs)
	}

	return &fileLayout, nil
}
