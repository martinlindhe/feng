package mapper

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/martinlindhe/feng"
	"github.com/martinlindhe/feng/template"
	"github.com/martinlindhe/feng/value"
)

const (
	DEBUG = false
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

	fileLayout := FileLayout{DS: ds, BaseName: ds.BaseName, endian: ds.Endian, Extension: ext}

	// read all data to get the total length
	b, _ := ioutil.ReadAll(r)
	fileLayout.size = uint64(len(b))
	rr := bytes.NewReader(b)

	if DEBUG {
		log.Printf("mapping ds '%s'", ds.BaseName)
	}

	for _, df := range ds.Layout {
		if df.Kind == "offset" {
			// evaluate label in top-level layout (used by ps4_cnt)
			v, err := fileLayout.EvaluateExpression(df.Label)
			if err != nil {
				panic(err)
			}
			fileLayout.offset = v
			_, err = rr.Seek(int64(v), io.SeekStart)
			if err != nil {
				log.Fatal(err)
			}
			continue
		}

		es, err := ds.FindStructure(&df)
		if err != nil {
			log.Fatal(err)
		}
		if DEBUG {
			log.Printf("mapping struct '%s' (kind %s) to %+v", df.Label, df.Kind, es)
		}

		if df.Slice {
			// like ranged layout but keep reading until EOF
			if DEBUG {
				log.Printf("appending sliced %s[]", df.Kind)
			}

			baseLabel := df.Label
			for i := uint64(0); i < math.MaxUint64; i++ {
				df.Index = int(i)
				df.Label = fmt.Sprintf("%s_%d", baseLabel, i)
				if err := fileLayout.expandStruct(rr, &df, ds, es.Expressions); err != nil {
					if err == io.EOF {
						log.Println("reached EOF")
						break
					}
					/*
						// do not propagate error, so that trailing data after slices will not count as parse error
						if err != nil {
							if _, ok := err.(template.ValidationError); ok {
								log.Println("invalidating file due to no matching pattern", err)
								//return &fileLayout, nil
								break
							}
							// XXX must respect "TYPE IS NOT VALID" error
							log.Printf("error (ignored): %#v   ", err)
						}
					*/
					return &fileLayout, err
				}
				df.Label = baseLabel
			}
			continue
		}
		if df.Range != "" {
			parsedRange, err := fileLayout.EvaluateExpression(df.Range)
			if err != nil {
				panic(err)
			}

			if DEBUG {
				log.Printf("appending ranged %s[%d]", df.Kind, parsedRange)
			}

			baseLabel := df.Label
			for i := uint64(0); i < parsedRange; i++ {
				df.Index = int(i)
				df.Label = fmt.Sprintf("%s_%d", baseLabel, i)
				if err := fileLayout.expandStruct(rr, &df, ds, es.Expressions); err != nil {
					return &fileLayout, err
				}
				df.Label = baseLabel
			}
			continue
		}

		if err := fileLayout.expandStruct(rr, &df, ds, es.Expressions); err != nil {
			if DEBUG {
				log.Println("errors out:", err)
			}
			return &fileLayout, err
		}
	}

	return &fileLayout, nil
}

func MapFileToTemplate(filename string) (fl *FileLayout, err error) {

	fs.WalkDir(feng.Templates, ".", func(tpl string, d fs.DirEntry, err error) error {
		// cannot happen
		if err != nil {
			panic(err)
		}
		if d.IsDir() {
			return nil
		}

		if filepath.Ext(tpl) != ".yml" {
			return nil
		}

		b, err := fs.ReadFile(feng.Templates, tpl)
		if err != nil {
			return err // or panic or ignore
		}

		ds, err := template.UnmarshalTemplateIntoDataStructure(b, tpl)
		if err != nil {
			return err
		}

		r, err := os.Open(filename)
		if err != nil {
			log.Fatal(err)
		}

		fl, err = MapReader(r, ds)
		if err != nil {
			// template don't match, try another
			if _, ok := err.(EvaluateError); ok {
				log.Println(red(tpl + ": " + err.Error()))
			}
			return nil
		}
		if len(fl.Structs) > 0 {
			fmt.Printf("Parsed %s as %s\n\n", filename, tpl)
			return fmt.Errorf("break WalkDir")
		}
		return nil
	})

	if fl == nil {
		return nil, fmt.Errorf("no match")
	}
	return fl, nil
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
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			//log.Printf("error: unexpected eof at %s %s in %s. %d structs",
			//	df.Kind, df.Label, ds.BaseName, len(fl.Structs))
			if len(fl.Structs) < 1 || len(fl.Structs[0].Fields) == 0 {
				return fmt.Errorf("eof and no structs mapped")
			}
			return nil
		}
		return err

		// remove the added struct in case of error
		log.Print(red("removing struct '%s.%s' due to error: %v", fl.BaseName, fs.Label, err))
		fl.Structs = append(fl.Structs[:idx], fl.Structs[idx+1:]...)
	}
	return err
}

func (fl *FileLayout) expandChildren(r *bytes.Reader, fs *Struct, df *value.DataField, ds *template.DataStructure, expressions []template.Expression) error {

	if DEBUG {
		log.Printf("expandChildren: working with struct %s", df.Label)
	}

	// track iterator index while parsing
	fs.Index = df.Index

	lastIf := ""

	for _, es := range expressions {
		switch es.Field.Kind {
		case "label":
			// "label: APP0". augment node with extra info

			// XXX eval expression such as "self.Type", get evaluated match???
			fs.decoration = es.Pattern.Value

		case "endian":
			// special form
			fl.endian = es.Pattern.Value
			if DEBUG {
				fmt.Printf("endian changed to '%s'\n", fl.endian)
			}

		case "offset":
			// set/restore current offset

			if es.Pattern.Value == "restore" {
				log.Printf("--- RESTORED OFFSET FROM %04x TO %04x", fl.offset, fl.previousOffset)
				fl.offset = fl.previousOffset
				_, err := r.Seek(int64(fl.offset), io.SeekStart)
				return err
			}

			var err error
			fl.offsetChanges++
			if fl.offsetChanges > 50 {
				return fmt.Errorf("too many offset changes from template")
			}
			fl.previousOffset = fl.offset
			fl.offset, err = fl.GetInt(es.Pattern.Value, df) // XXX eval with goval!!!
			log.Printf("--- CHANGED OFFSET FROM %04x TO %04x (%s)", fl.previousOffset, fl.offset, es.Pattern.Value)
			if err != nil {
				return err
			}
			_, err = r.Seek(int64(fl.offset), io.SeekStart)
			if err != nil {
				return err
			}

		case "data":
			// the template directive "data:invalid" marks the data stream invalid
			if es.Pattern.Value != "invalid" {
				log.Fatalf("unhandled file value '%s", es.Pattern.Value)
			}
			return fmt.Errorf("file invalidated by template")

		case "u8", "i8", "u16", "i16", "u32", "i32", "u64", "i64",
			"ascii", "utf16",
			"time_t_32", "filetime", "dostime", "dosdate",
			"compressed:zlib", "raw:u8":
			// internal data types
			if es.Field.Range != "" {
				var err error
				es.Field.Range, err = fl.ExpandVariables(es.Field.Range, df)
				if err != nil {
					return err
				}
			}

			unitLength, totalLength := fl.GetAddressLengthPair(&es.Field)
			if totalLength == 0 {
				if DEBUG {
					log.Printf("SKIPPING ZERO-LENGTH FIELD '%s' %s", es.Field.Label, fl.PresentType(&es.Field))
				}
				continue
			}

			prevOffset := fl.offset
			if fl.IsAbsoluteAddress(&es.Field) {
				// if range = start:len, first move to given offset
				rangeStart, _, err := fl.GetAbsoluteAddressRange(&es.Field)
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
				log.Printf("[%08x] reading %d bytes for '%s.%s' %s: %02x (err:%v)", fl.offset, totalLength, df.Label, es.Field.Label, fl.PresentType(&es.Field), val, err)
			}
			if err != nil {
				if errors.Is(err, io.ErrUnexpectedEOF) {
					if DEBUG {
						log.Printf("error: [%08x] failed reading %d bytes for '%s.%s' %s: %02x (err:%v)", fl.offset, totalLength, df.Label, es.Field.Label, fl.PresentType(&es.Field), val, err)
					}
					continue
				}
				return err
			}

			// if known value, see if value is in file data
			if es.Pattern.Known {
				if !bytes.Equal(es.Pattern.Pattern, val) {
					if DEBUG {
						log.Printf("[%08x] pattern '%s' does not match. expected '% 02x', got '% 02x'", fl.offset, es.Field.Label, es.Pattern.Pattern, val)
					}
					return fmt.Errorf("[%08x] pattern '%s' does not match. expected '% 02x', got '% 02x'",
						fl.offset, es.Field.Label, es.Pattern.Pattern, val)
				}
			}

			matchPatterns, err := es.EvaluateMatchPatterns(val)
			if err != nil {
				return err
			}
			field := Field{Offset: fl.offset, Length: totalLength, Value: val, Format: es.Field, Endian: fl.endian, MatchedPatterns: matchPatterns}
			fs.Fields = append(fs.Fields, field)
			if fl.IsAbsoluteAddress(&es.Field) {
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

			field := Field{Offset: fl.offset, Length: uint64(len(val)), Value: val, Format: es.Field, Endian: fl.endian}
			fs.Fields = append(fs.Fields, field)
			fl.offset += field.Length

		case "if":
			q := es.Field.Label
			q = strings.ReplaceAll(q, "self.", df.Label+".")

			// workaround for yaml limitation of not allowing [] in keys:
			q = strings.ReplaceAll(q, "{", "[")
			q = strings.ReplaceAll(q, "}", "]")

			a, err := fl.EvaluateExpression(q)
			if err != nil {
				return err
			}
			lastIf = q
			//log.Println("EVAL EXPRESSION", q, ":", a)
			if a != 0 {
				err := fl.expandChildren(r, fs, df, ds, es.Children)
				if err != nil {
					return err
				}
			}

		case "else":
			log.Println("ELSE: evaluating", lastIf)
			a, err := fl.EvaluateExpression(lastIf)
			if err != nil {
				return err
			}
			if a == 0 {
				err := fl.expandChildren(r, fs, df, ds, es.Children)
				if err != nil {
					return err
				}
			}

		default:
			// find custom struct with given name
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
