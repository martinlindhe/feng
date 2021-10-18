package mapper

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"regexp"

	"github.com/martinlindhe/feng/template"
	"github.com/martinlindhe/feng/value"
)

const (
	DEBUG = false
)

var (
	ifMatchExpressionRE = regexp.MustCompile(`([\w .]+) (in|notin) \(([\w, ]+)\)`)
)

func init() {
	log.SetFlags(log.Lshortfile)
}

// produces a list of fields with offsets and sizes from input reader based on data structure
func MapReader(r io.Reader, ds *template.DataStructure) (*FileLayout, error) {

	ext := ""
	if len(ds.Extensions) > 0 {
		ext = ds.Extensions[0]
	}

	fileLayout := FileLayout{endian: ds.Endian, Extension: ext}

	// read all data to get the total length
	b, _ := ioutil.ReadAll(r)
	fileLayout.size = uint64(len(b))
	rr := bytes.NewReader(b)

	for _, df := range ds.Layout {
		es, err := ds.FindStructure(&df)
		if err != nil {
			log.Fatal(err)
		}
		if DEBUG {
			log.Printf("mapping struct '%s' (kind %s) to %+v\n", df.Label, df.Kind, es)
		}

		if df.Slice {
			// like ranged layout but keep reading until EOF
			if DEBUG {
				log.Printf("appending sliced %s[]", df.Kind)
			}

			baseLabel := df.Label
			for i := uint64(0); i < math.MaxUint64; i++ {
				df.Label = fmt.Sprintf("%s[%d]", baseLabel, i)
				if err := fileLayout.expandStruct(rr, &df, ds, es.Expressions); err != nil {
					if err == io.EOF {
						break
					}
					return &fileLayout, err
				}
				df.Label = baseLabel
			}
			continue

		}
		if df.Range != "" {
			kind, val, err := fileLayout.GetValue(df.Range, &df)
			if err != nil {
				return nil, err
			}

			parsedRange := value.AsUint64(kind, val)
			if DEBUG {
				log.Printf("appending ranged %s[%d]", df.Kind, parsedRange)
			}

			baseLabel := df.Label
			for i := uint64(0); i < parsedRange; i++ {
				df.Label = fmt.Sprintf("%s[%d]", baseLabel, i)
				if err := fileLayout.expandStruct(rr, &df, ds, es.Expressions); err != nil {
					return &fileLayout, err
				}
				df.Label = baseLabel
			}
			continue
		}

		if err := fileLayout.expandStruct(rr, &df, ds, es.Expressions); err != nil {
			return &fileLayout, err
		}
	}

	return &fileLayout, nil
}

func (fl *FileLayout) expandStruct(r *bytes.Reader, df *value.DataField, ds *template.DataStructure, expressions []template.Expression) error {

	if DEBUG {
		log.Printf("expandStruct: adding struct %s", df.Label)
	}

	fl.Structs = append(fl.Structs, Struct{Label: df.Label})

	idx := len(fl.Structs) - 1
	fs := &fl.Structs[idx]

	err := fl.expandChildren(r, fs, df, ds, expressions)
	if err != nil {
		// remove last struct in case of error
		fl.Structs = append(fl.Structs[:idx], fl.Structs[idx+1:]...)
	}
	return err
}

func (fl *FileLayout) expandChildren(r *bytes.Reader, fs *Struct, df *value.DataField, ds *template.DataStructure, expressions []template.Expression) error {

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

		case "u8", "i8", "u16", "i16", "u32", "i32", "u64", "i64", "ascii", "utf16le", "time_t_32", "filetime":
			if es.Field.Range != "" {
				var err error
				es.Field.Range, err = fl.ExpandVariables(es.Field.Range, df)
				if err != nil {
					return err
				}
			}

			unitLength, totalLength := fl.GetLength(&es.Field)
			if totalLength == 0 {
				if DEBUG {
					log.Printf("SKIPPING ZERO-LENGTH FIELD '%s' %s", es.Field.Label, fl.PresentType(&es.Field))
				}
				continue
			}

			prevOffset := fl.offset
			if es.Field.IsAbsoluteAddress() {
				// if range = start:len, first move to given offset
				rangeStart, _, err := es.Field.GetAbsoluteAddress()
				if err != nil {
					log.Fatal(err)
				}

				log.Printf("--- SEEKING TO ABSOLUTE OFFSET %08x", rangeStart)
				_, err = r.Seek(int64(rangeStart), io.SeekStart)
				if err != nil {
					return err
				}

				fl.offset = rangeStart
			}

			val, err := readBytes(r, totalLength, unitLength, fl.endian)
			if DEBUG {
				log.Printf("[%08x] reading %d bytes for '%s.%s' %s: %02x", fl.offset, totalLength, df.Label, es.Field.Label, fl.PresentType(&es.Field), val)
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
			if es.Field.IsAbsoluteAddress() {
				fl.offset = prevOffset
				log.Printf("--- RESTORING FILE POSITION TO ABSOLUTE OFFSET %08x", fl.offset)
				_, err := r.Seek(int64(fl.offset), io.SeekStart)
				if err != nil {
					return err
				}
			} else {
				fl.offset += field.Length
			}

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

				switch operation {
				case "in", "notin":
					if DEBUG {
						log.Printf("-- matching IF key=%s, operation=%s, pattern=%s", key, operation, pattern)
					}

					kind, val, err := fl.GetValue(key, df)
					if err != nil {
						return err
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

				if DEBUG {
					log.Printf("-- matching IF NOTZERO key=%s", key)
				}

				_, val, err := fl.GetValue(key, df)
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
				return fmt.Errorf("error fetching struct '%s': %v", es.Field.Kind, err)
			}

			log.Printf("%#v", customStruct)

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
