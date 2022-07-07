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

	"github.com/k0kubun/pp/v3"
	"github.com/martinlindhe/feng"
	"github.com/martinlindhe/feng/template"
	"github.com/martinlindhe/feng/value"
)

const (
	DEBUG = false
)

var (
	ParseStopError = errors.New("manual parse stop")
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

	fileLayout.DS.Constants = append(fileLayout.DS.Constants, template.EvaluatedConstant{
		Field: value.DataField{Label: "FILE_SIZE", Kind: "u64"},
		Value: int64(fileLayout.size),
	})

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
				log.Printf("appending sliced %s[] %s", df.Kind, df.Label)
			}

			baseLabel := df.Label
			for i := uint64(0); i < math.MaxUint64; i++ {
				df.Index = int(i)
				df.Label = fmt.Sprintf("%s_%d", baseLabel, i)
				if err := fileLayout.expandStruct(rr, &df, ds, es.Expressions); err != nil {
					if errors.Is(err, ParseStopError) {
						log.Println("reached ParseStop")
						break
					}
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

			if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
				// accept eof errors as valid parse for otherwise valid mapping
				return &fileLayout, nil
			}
			//if DEBUG {
			feng.Yellow("%s errors out: %s\n", ds.BaseName, err.Error())
			//}
			return &fileLayout, err
		}
	}

	return &fileLayout, nil
}

var (
	mapFileMatchedError = errors.New("matched file")
)

func MapFileToTemplate(filename string) (fl *FileLayout, err error) {

	data, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	r := bytes.NewReader(data)

	err = fs.WalkDir(feng.Templates, ".", func(tpl string, d fs.DirEntry, err error) error {
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

		rawTemplate, err := fs.ReadFile(feng.Templates, tpl)
		if err != nil {
			return err // or panic or ignore
		}
		log.Println(tpl)
		ds, err := template.UnmarshalTemplateIntoDataStructure(rawTemplate, tpl)
		if err != nil {
			return err
		}

		if ds.NoMagic {
			log.Println("skip no_magic template", tpl)
			return nil
		}

		// skip if no magic bytes matches
		found := false
		for _, m := range ds.Magic {
			_, err = r.Seek(int64(m.Offset), io.SeekStart)
			if err != nil {
				return err
			}
			b := make([]byte, len(m.Match))
			_, _ = r.Read(b)
			if bytes.Equal(m.Match, b) {
				found = true
			}
		}
		if !found {
			feng.Red("%s magic bytes don't match\n", tpl)
			return nil
		}

		r.Reset(data)

		fl, err = MapReader(r, ds)
		if err != nil {
			// template don't match, try another
			if _, ok := err.(EvaluateError); ok {
				feng.Red("MapReader EvaluateError: %s: %s\n", tpl, err.Error())
			} else {
				return nil
			}
		}
		if len(fl.Structs) > 0 {
			log.Printf("Parsed %s as %s", filename, tpl)
			return mapFileMatchedError
		}
		return nil
	})
	if errors.Is(err, mapFileMatchedError) {
		return fl, nil
	}
	if err != nil {
		return fl, err
	}

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
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		//if DEBUG {
		feng.Red("expandStruct error: [%08x] failed reading data for '%s' (err:%v)\n", fl.offset, df.Label, err)
		//}

		if len(fl.Structs) < 1 || len(fl.Structs[0].Fields) == 0 {
			return fmt.Errorf("eof and no structs mapped")
		}
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
		if DEBUG {
			log.Printf("expandChildren: working with field %s %s: %v", es.Field.Kind, es.Field.Label, es)
		}
		switch es.Field.Kind {
		case "label":
			// "label: APP0". augment node with extra info
			val, err := fl.MatchedValue(es.Pattern.Value, df)
			if err != nil {
				panic(err)
			}
			fs.decoration = strings.TrimSpace(val)

		case "parse":
			// break parser
			if es.Pattern.Value != "stop" {
				log.Fatalf("invalid parse value '%s'", es.Pattern.Value)
			}
			//log.Println("-- PARSE STOP --")
			return ParseStopError

		case "endian":
			// change endian
			fl.endian = es.Pattern.Value
			//if DEBUG {
			feng.Yellow("endian set to '%s' at %06x\n", fl.endian, fl.offset)
			//}

		case "offset":
			// set/restore current offset
			if es.Pattern.Value == "restore" {
				log.Printf("--- RESTORED OFFSET FROM %04x TO %04x", fl.offset, fl.previousOffset)
				fl.offset = fl.previousOffset
				_, err := r.Seek(int64(fl.offset), io.SeekStart)
				if err != nil {
					return err
				}
				continue
			}

			var err error
			fl.offsetChanges++
			if fl.offsetChanges > 100 {
				panic("debug recursion: too many offset changes from template")
				return fmt.Errorf("too many offset changes from template")
			}
			fl.previousOffset = fl.offset
			fl.offset, err = fl.GetInt(es.Pattern.Value, df)
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

		case "until":
			// syntax: "until: u8 scanData ff d9"
			// creates a variable named scanData with all data up until terminating hex string

			parts := strings.SplitN(es.Pattern.Value, " ", 3)
			if len(parts) != 3 {
				panic("invalid input: " + es.Pattern.Value)
			}

			if parts[0] != "u8" {
				panic(fmt.Sprintf("until directive don't support type '%s'", parts[0]))
			}

			needle, err := value.ParseHexString(parts[2])
			if err != nil {
				panic(err)
			}

			val, err := readBytesUntilMarker(r, needle)
			if err != nil {
				return err
			}
			len := uint64(len(val))
			es.Field.Kind = "u8"
			es.Field.Range = fmt.Sprintf("%d", len)
			es.Field.Label = parts[1]
			fs.Fields = append(fs.Fields, Field{
				Offset: fl.offset,
				Length: len,
				Value:  val,
				Format: es.Field,
				Endian: fl.endian})
			fl.offset += len

		case "u8", "i8", "u16", "i16", "u32", "i32", "u64", "i64",
			"ascii", "utf16",
			"time_t_32", "filetime", "dostime", "dosdate",
			"compressed:lz4", "compressed:zlib",
			"raw:u8":
			// internal data types
			es.Field.Range = strings.ReplaceAll(es.Field.Range, "self.", df.Label+".")
			unitLength, totalLength := fl.GetAddressLengthPair(&es.Field)
			if totalLength == 0 {
				if DEBUG {
					log.Printf("SKIPPING ZERO-LENGTH FIELD '%s' %s", es.Field.Label, fl.PresentType(&es.Field))
				}
				continue
			}

			val, err := readBytes(r, totalLength, unitLength, fl.endian)
			if DEBUG {
				log.Printf("[%08x] reading %d bytes for '%s.%s': %02x (err:%v)", fl.offset, totalLength, df.Label, es.Field.Label, val, err)
			}
			if err != nil {
				return err
			}

			// if known data pattern, see if it matches file data
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

			fs.Fields = append(fs.Fields, Field{
				Offset:          fl.offset,
				Length:          totalLength,
				Value:           val,
				Format:          es.Field,
				Endian:          fl.endian,
				MatchedPatterns: matchPatterns})
			fl.offset += totalLength

		case "asciiz":
			val, err := readBytesUntilZero(r)
			if err != nil {
				return err
			}
			len := uint64(len(val))
			fs.Fields = append(fs.Fields, Field{
				Offset: fl.offset,
				Length: len,
				Value:  val,
				Format: es.Field,
				Endian: fl.endian})
			fl.offset += len

		case "utf16z":
			val, err := readBytesUntilMarker(r, []byte{0, 0})
			if err != nil {
				return err
			}
			// append terminator marker since readBytesUntilMarker() excludes it
			val = append(val, []byte{0, 0}...)

			len := uint64(len(val))
			fs.Fields = append(fs.Fields, Field{
				Offset: fl.offset,
				Length: len,
				Value:  val,
				Format: es.Field,
				Endian: fl.endian})
			fl.offset += len

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
			if DEBUG {
				if a != 0 {
					log.Println("IF EVALUATED TRUE: q=", q, ", a=", a)
				} else {
					log.Println("IF EVALUATED FALSE: q=", q, ", a=", a)
				}
			}
			if a != 0 {
				err := fl.expandChildren(r, fs, df, ds, es.Children)
				if err != nil {
					return err
				}
			}

		case "else":
			if DEBUG {
				log.Println("ELSE: evaluating", lastIf)
			}
			a, err := fl.EvaluateExpression(lastIf)
			if err != nil {
				return err
			}
			if DEBUG {
				if a == 0 {
					log.Println("ELSE EVALUATED TRUE: lastIf=", lastIf, ", a=", a)
				} else {
					log.Println("ELSE EVALUATED FALSE: lastIf=", lastIf, ", a=", a)
				}
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
				// this error is always critical. it means the parsed template is not working
				pp.Print(fl.DS)
				log.Fatalf("error fetching struct '%s': %v", es.Field.Kind, err)
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
	if unitLength > 1 && endian == "" {
		return nil, fmt.Errorf("endian is not set in file format template, don't know how to read data")
	}

	val := make([]byte, totalLength)
	if _, err := io.ReadFull(r, val); err != nil {
		return nil, err
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

// reads bytes from reader until the marker byte sequence is found. returned data excludes the marker
// NOTE: only looks for marker on every N bytes, where N is the length of marker
func readBytesUntilMarker(r *bytes.Reader, mark []byte) ([]byte, error) {

	b := make([]byte, len(mark))

	res := []byte{}

	for {
		if _, err := io.ReadFull(r, b); err != nil {
			return nil, err
		}
		if bytes.Equal(b, mark) {
			// rewind N bytes
			r.Seek(int64(-len(mark)), io.SeekCurrent)
			break
		}
		res = append(res, b...)
	}
	return res, nil
}
