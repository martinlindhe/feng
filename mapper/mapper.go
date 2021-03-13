package mapper

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"regexp"

	"github.com/martinlindhe/feng/template"
	"github.com/martinlindhe/feng/value"
)

const (
	DEBUG = true
)

var (
	ifMatchExpressionRE = regexp.MustCompile(`([\w .]+) (in|notin) \(([\w]+)\)`)
)

func init() {
	log.SetFlags(log.Lshortfile)
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
			log.Printf("mapping struct '%s'\n", layout.PresentType())
		}

		struct_, err := ds.FindStructure(&layout)
		if err != nil {
			panic(err)
		}
		if DEBUG {
			log.Printf("mapped '%s' to %+v\n", layout.PresentType(), struct_)
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
					log.Fatalf("invalid %s form: %s", es.Field.Kind, es.Field.PresentType())
				}

				if es.Field.Range != "" {
					es.Field.Range = fs.ExpandVariables(es.Field.Range)
				}

				unitLength, totalLength := es.Field.GetLength()

				if DEBUG {
					log.Printf("[%08x] reading %d bytes for '%s' %s", offset, totalLength, es.Field.Label, es.Field.PresentType())
				}
				val, err := readBytes(r, totalLength, unitLength, endian)
				if err != nil {
					return nil, err
				}

				// if known value, see if value is in file data
				if es.Pattern.Known {
					if !bytes.Equal(es.Pattern.Pattern, val) {
						return nil, fmt.Errorf("[%08x] pattern '%s' does not match. expected '% 02x', got '% 02x'",
							offset, es.Field.Label, es.Pattern.Pattern, val)
					}
				}

				matchPatterns, err := es.EvaluateMatchPatterns(val)
				if err != nil {
					return nil, err
				}

				field = fileField{Offset: offset, Length: totalLength, Value: val, Format: es.Field, Endian: endian, MatchedPatterns: matchPatterns}
				fs.Fields = append(fs.Fields, field)
				offset += field.Length

			case "if":
				matches := ifMatchExpressionRE.FindStringSubmatch(es.Field.Label)
				if len(matches) > 0 {

					key := matches[1]
					operation := matches[2] // "in" or "notin"
					pattern := matches[3]

					if DEBUG {
						log.Printf("-- matching IF key=%s, operation=%s, pattern=%s", key, operation, pattern)
					}

					switch operation {
					case "in", "notin":
						// find value of "key" in current struct
						kind, val, err := fs.GetValue(key)
						if err != nil {
							log.Fatal(err)
						}

						if DEBUG {
							log.Printf("if-match: %s %02x", kind, val)
						}

						patternValues, err := ds.ParsePattern(pattern, kind)
						if err != nil {
							log.Fatal(err)
						}
						matched := false
						for _, patternVal := range patternValues {
							if DEBUG {
								log.Printf("if-match: comparing '%v' %s '%v'", val, operation, patternVal)
							}
							if operation == "in" && bytes.Equal(val, patternVal) {
								matched = true
							}
							if operation == "notin" && !bytes.Equal(val, patternVal) {
								matched = true
							}
						}

						if matched {
							// if evaluation is true, append all child nodes to fileLayout
							for _, child := range es.Children {
								if DEBUG {
									log.Printf("[%08x] if-match: adding child %s", offset, child.Field.Label)
								}

								if child.Field.Range != "" {
									child.Field.Range = fs.ExpandVariables(child.Field.Range)
								}

								unitLength, totalLength := child.Field.GetLength()

								childVal, err := readBytes(r, totalLength, unitLength, endian)
								if err != nil {
									log.Fatal(err)
								}

								length := child.Field.SingleUnitSize()

								field = fileField{Offset: offset, Length: length, Value: childVal, Format: child.Field, Endian: endian}
								fs.Fields = append(fs.Fields, field)

								offset += length
							}
						}

					default:
						log.Fatalf("unhandled if-match operation '%s'", operation)
					}
				}

			default:
				log.Printf("unhandled field '%#v'", es.Field)
				return nil, fmt.Errorf("unhandled field kind '%s'", es.Field.Kind)
			}
		}

		fileLayout.Structs = append(fileLayout.Structs, fs)
	}

	return &fileLayout, nil
}

// reads bytes from reader and returns them in network byte order (big endian)
func readBytes(r io.Reader, totalLength, unitLength uint64, endian string) ([]byte, error) {

	val := make([]byte, totalLength)
	if _, err := io.ReadFull(r, val); err != nil {
		return nil, err
	}

	if unitLength > 1 && endian == "" {
		return nil, fmt.Errorf("endian is not set in file format template, don't know how to read data")
	}

	// convert to network byte order
	if unitLength > 1 && endian == "little" {
		val = value.ReverseBytes(val, int(unitLength))
	}

	return val, nil
}
