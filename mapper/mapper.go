package mapper

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/martinlindhe/feng/template"
	"github.com/martinlindhe/feng/value"
)

const (
	DEBUG = true
)

var (
	ifMatchExpressionRE = regexp.MustCompile(`([\w .]+) (in|notin) \(([\w, ]+)\)`)
)

func init() {
	log.SetFlags(log.Lshortfile)
}

// produces a list of fields with offsets and sizes from input reader based on data structure
func MapReader(r io.Reader, ds *template.DataStructure) (*FileLayout, error) {

	fileLayout := FileLayout{}

	for _, df := range ds.Layout {
		es, err := ds.FindStructure(&df)
		if err != nil {
			log.Fatal(err)
		}
		if DEBUG {
			log.Printf("mapping struct '%s' (kind %s) to %+v\n", df.Label, df.Kind, es)
		}

		if df.Slice {
			log.Fatalf("TODO handle sliced layout %#v", df)
		}
		if df.Range != "" {
			kind, val, err := fileLayout.GetValue(df.Range)
			if err != nil {
				log.Fatal(err)
			}

			parsedRange := value.AsUint64(kind, val)
			log.Printf("appending ranged %s[%d]", df.Kind, parsedRange)

			baseLabel := df.Label
			for i := uint64(0); i < parsedRange; i++ {
				oldOffset := fileLayout.offset
				df.Label = fmt.Sprintf("%s #%d", baseLabel, i+1)
				if err := fileLayout.expandStruct(r, &df, ds, es.Expressions); err != nil {
					return nil, err
				}
				log.Printf(" -- OFFSET AFTER %s FROM %08x to %08x", df.Label, oldOffset, fileLayout.offset)
			}
			continue
		}

		oldOffset := fileLayout.offset
		if err := fileLayout.expandStruct(r, &df, ds, es.Expressions); err != nil {
			return nil, err
		}
		log.Printf(" -- OFFSET AFTER %s FROM %08x to %08x", df.Label, oldOffset, fileLayout.offset)
	}

	return &fileLayout, nil
}

func (fl *FileLayout) expandStruct(r io.Reader, df *value.DataField, ds *template.DataStructure, expressions []template.Expression) error {

	if DEBUG {
		log.Printf("expandStruct: adding struct %s", df.Label)
	}

	fl.Structs = append(fl.Structs, Struct{Label: df.Label})
	fs := &fl.Structs[len(fl.Structs)-1]

	return fl.expandChildren(r, fs, df, ds, expressions)
}

func (fl *FileLayout) expandChildren(r io.Reader, fs *Struct, df *value.DataField, ds *template.DataStructure, expressions []template.Expression) error {

	if DEBUG {
		log.Printf("expandChildren: working with struct %s", df.Label)
	}

	for _, es := range expressions {
		var field Field
		switch es.Field.Kind {
		case "endian":
			// special form
			fl.endian = es.Pattern.Value
			if DEBUG {
				fmt.Printf("endian changed to '%s'\n", fl.endian)
			}
		case "data":
			if es.Pattern.Value != "invalid" {
				log.Fatalf("unhandled file value '%s", es.Pattern.Value)
			}
			return fmt.Errorf("file invalidated by template")

		case "u8", "u16", "u32", "u64", "ascii":
			if es.Field.IsRangeUnit() {
				log.Fatalf("invalid %s form: %s", es.Field.Kind, es.Field.PresentType())
			}

			if es.Field.Range != "" {
				es.Field.Range = strings.Replace(es.Field.Range, "self.", df.Label+".", 1)
				es.Field.Range = fl.ExpandVariables(es.Field.Range)
			}

			unitLength, totalLength := es.Field.GetLength()

			val, err := readBytes(r, totalLength, unitLength, fl.endian)
			if DEBUG {
				log.Printf("[%08x] reading %d bytes for '%s' %s: %02x", fl.offset, totalLength, es.Field.Label, es.Field.PresentType(), val)
			}
			if err != nil {
				return err
			}

			// if known value, see if value is in file data
			if es.Pattern.Known {
				if !bytes.Equal(es.Pattern.Pattern, val) {
					return fmt.Errorf("[%08x] pattern '%s' does not match. expected '% 02x', got '% 02x'",
						fl.offset, es.Field.Label, es.Pattern.Pattern, val)
				}
			}

			matchPatterns, err := es.EvaluateMatchPatterns(val)
			if err != nil {
				return err
			}

			field = Field{Offset: fl.offset, Length: totalLength, Value: val, Format: es.Field, Endian: fl.endian, MatchedPatterns: matchPatterns}
			fs.Fields = append(fs.Fields, field)
			fl.offset += field.Length

		case "asciiz":
			val, err := readBytesUntilZero(r)
			if err != nil {
				return err
			}

			field = Field{Offset: fl.offset, Length: uint64(len(val)), Value: val, Format: es.Field, Endian: fl.endian}
			fs.Fields = append(fs.Fields, field)
			fl.offset += field.Length

		case "if":
			matches := ifMatchExpressionRE.FindStringSubmatch(es.Field.Label)
			if len(matches) > 0 {

				key := matches[1]
				operation := matches[2] // "in" or "notin"
				pattern := matches[3]

				key = strings.Replace(key, "self.", df.Label+".", 1)

				if DEBUG {
					log.Printf("-- matching IF key=%s, operation=%s, pattern=%s", key, operation, pattern)
				}

				switch operation {
				case "in", "notin":
					kind, val, err := fl.GetValue(key)
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
						if bytes.Equal(val, patternVal) {
							// op "in": if match in any of values, count as true
							// op "notin": if match in NONE of values, count as true
							matched = true
						}
					}
					if DEBUG {
						log.Printf("if-match: compared '%v' %s '%v' to %v", val, operation, patternValues, matched)
					}

					if (operation == "in" && matched) || (operation == "notin" && !matched) {
						err := fl.expandChildren(r, fs, df, ds, es.Children)
						if err != nil {
							return err
						}
					}

				default:
					log.Fatalf("unhandled if-match operation '%s'", operation)
				}
			} else {
				key := es.Field.Label
				key = strings.Replace(key, "self.", df.Label+".", 1)

				if DEBUG {
					log.Printf("-- matching IF NOTZERO key=%s", key)
				}

				_, val, err := fl.GetValue(key)
				if err == nil {
					matched := false
					for _, b := range val {
						if b != 0 {
							matched = true
						}
					}

					if matched {
						err := fl.expandChildren(r, fs, df, ds, es.Children)
						if err != nil {
							log.Fatal(err)
						}
					}
				}
			}

		default:
			// XXX find custom struct with given name
			customStruct, err := fl.GetStruct(es.Field.Kind)
			if err != nil {
				log.Fatalf("error fetching struct '%s': %v", es.Field.Kind, err)
			}

			spew.Dump(customStruct)

			log.Printf("unhandled field '%#v'", es.Field)
			return fmt.Errorf("unhandled field kind '%s'", es.Field.Kind)
		}
	}

	return nil
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

// reads bytes from reader until 0x00 is found. returned data includes the terminating 0x00
func readBytesUntilZero(r io.Reader) ([]byte, error) {

	b := make([]byte, 1)

	res := []byte{}

	for {
		if _, err := io.ReadFull(r, b); err != nil {
			return nil, err
		}
		res = append(res, b[0])
		if b[0] == 0x00 {
			break
		}
	}
	return res, nil
}
