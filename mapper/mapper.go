package mapper

import (
	"bytes"
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
	Label  string

	// value in network byte order (big)
	Value []byte
}

const (
	DEBUG = true
)

// produces a list of fields with offsets and sizes from input reader based on data structure
func MapReader(r io.Reader, ds *template.DataStructure) ([]fileField, error) {

	endian := ""

	fileStruct := []fileField{}
	offset := uint64(0)

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

				field = fileField{Offset: offset, Length: totalLength, Value: val, Label: es.Field.Label}
				fileStruct = append(fileStruct, field)

			default:
				log.Printf("MapReader: unhandled es.Field.Kind '%s'", es.Field.Kind)
			}
			offset += field.Length
		}
	}

	return fileStruct, nil
}
