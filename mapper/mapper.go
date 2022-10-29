package mapper

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/martinlindhe/feng"
	"github.com/martinlindhe/feng/template"
	"github.com/martinlindhe/feng/value"
)

const (
	DEBUG        = false
	DEBUG_MAPPER = false
	DEBUG_OFFSET = false
	DEBUG_LABEL  = false
)

var (
	ParseStopError = errors.New("manual parse stop")
)

func (fl *FileLayout) mapLayout(rr *bytes.Reader, fs *Struct, ds *template.DataStructure, df *value.DataField) error {

	if df.Kind == "offset" {
		// evaluate offset directive in top-level layout (needed by ps3_pkg)
		v, err := fl.EvaluateExpression(df.Label, df)
		if err != nil {
			return err
		}
		fl.offset = v
		_, err = rr.Seek(int64(v), io.SeekStart)
		if err != nil {
			return err
		}
		return nil
	}

	es, err := ds.FindStructure(df.Kind)
	if err != nil {
		return err
	}
	if DEBUG_MAPPER {
		log.Printf("mapping ds %s, df '%s' (kind %s)", ds.BaseName, df.Label, df.Kind)
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
			if err := fl.expandStruct(rr, df, ds, es.Expressions); err != nil {
				if DEBUG_LABEL {
					log.Printf("--- used Label %s, restoring to %s", df.Label, baseLabel)
				}
				df.Label = baseLabel

				if errors.Is(err, ParseStopError) {
					if DEBUG_MAPPER {
						log.Print("reached ParseStop")
					}
					break
				}
				if err == io.EOF {
					if DEBUG_MAPPER {
						log.Print("reached EOF")
					}
					break
				}
				return err
			}
			if DEBUG_LABEL {
				log.Printf("--- used Label %s, restoring to %s", df.Label, baseLabel)
			}
			df.Label = baseLabel
		}
		return nil
	}
	if df.Range != "" {
		rangeQ := df.Range
		if fs != nil {
			rangeQ = strings.ReplaceAll(rangeQ, "self.", fs.Name+".")
		}
		parsedRange, err := fl.EvaluateExpression(rangeQ, df)
		if err != nil {
			return err
		}

		if DEBUG {
			log.Printf("appending ranged %s[%d]", df.Kind, parsedRange)
		}

		baseLabel := df.Label
		for i := uint64(0); i < parsedRange; i++ {
			df.Index = int(i)
			df.Label = fmt.Sprintf("%s_%d", baseLabel, i)
			if err := fl.expandStruct(rr, df, ds, es.Expressions); err != nil {
				df.Label = baseLabel
				return err
			}
		}
		return nil
	}

	if err := fl.expandStruct(rr, df, ds, es.Expressions); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			// accept eof errors as valid parse for otherwise valid mapping
			return nil
		}
		feng.Yellow("%s errors out: %s\n", ds.BaseName, err.Error())
		return err
	}
	return nil
}

// produces a list of fields with offsets and sizes from input reader based on data structure
func MapReader(r io.Reader, ds *template.DataStructure) (*FileLayout, error) {

	ext := ""
	if len(ds.Extensions) > 0 {
		ext = ds.Extensions[0]
	}

	fileLayout := FileLayout{DS: ds, BaseName: ds.BaseName, endian: ds.Endian, Extension: ext}

	// read all data to get the total length
	b, _ := io.ReadAll(r)
	fileLayout.rawData = b
	fileLayout.size = uint64(len(b))
	rr := bytes.NewReader(b)

	if DEBUG {
		log.Printf("mapping ds '%s'", ds.BaseName)
	}

	for _, df := range ds.Layout {
		err := fileLayout.mapLayout(rr, nil, ds, &df)
		if err != nil {
			if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
				feng.Red("mapLayout error processing %s: %s\n", df.Label, err.Error())
			}
			return &fileLayout, nil
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
		ds, err := template.UnmarshalTemplateIntoDataStructure(rawTemplate, tpl)
		if err != nil {
			return fmt.Errorf("%s: %s", tpl, err.Error())
		}

		if ds.NoMagic {
			if DEBUG {
				log.Print("skip no_magic template", tpl)
			}
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
			if DEBUG {
				log.Printf("%s magic bytes don't match", tpl)
			}
			return nil
		}

		r.Reset(data)

		fl, err = MapReader(r, ds)
		fl.DataFileName = filename
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

func (fl *FileLayout) expandStruct(r *bytes.Reader, dfParent *value.DataField, ds *template.DataStructure, expressions []template.Expression) error {

	if DEBUG_MAPPER {
		log.Printf("expandStruct: adding struct %s", dfParent.Label)
	}

	fl.Structs = append(fl.Structs, Struct{Name: dfParent.Label})

	idx := len(fl.Structs) - 1
	fs := &fl.Structs[idx]

	err := fl.expandChildren(r, fs, dfParent, ds, expressions)
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		log.Error().Msgf("expandStruct error: [%08x] failed reading data for '%s' (err:%v)", fl.offset, dfParent.Label, err)

		if len(fl.Structs) < 1 || len(fl.Structs[0].Fields) == 0 {
			return fmt.Errorf("eof and no structs mapped")
		}
	}

	return err
}

func presentStringValue(v string) string {
	v = strings.TrimRight(v, "·")
	v = strings.TrimRight(v, " ")
	return v
}

func (fl *FileLayout) expandChildren(r *bytes.Reader, fs *Struct, dfParent *value.DataField, ds *template.DataStructure, expressions []template.Expression) error {
	var err error

	log.Debug().Msgf("expandChildren: %06x expanding struct %s", fl.offset, dfParent.Label)

	// track iterator index while parsing
	fs.Index = dfParent.Index

	lastIf := ""

	for _, es := range expressions {
		if DEBUG_MAPPER {
			log.Printf("expandChildren: working with %s field '%s %s': %v", dfParent.Label, es.Field.Kind, es.Field.Label, es)
		}
		switch es.Field.Kind {
		case "label":
			// "label: Foo Bar". augment the node with extra info
			// if it is a numeric field with patterns, return the string for the matched pattern,
			// else evaluate expression as strings
			if fs.Label != "" {
				log.Warn().Msg("overwriting label " + fs.Label + " with '" + es.Pattern.Value + "'")
			}

			if fl.isPatternVariableName(es.Pattern.Value, dfParent) {
				val, err := fl.MatchedValue(es.Pattern.Value, dfParent)
				if err != nil {
					panic(err)
				}
				fs.Label = presentStringValue(val)
			} else {
				val, err := fl.EvaluateStringExpression(es.Pattern.Value, dfParent)
				if err != nil {
					fs.Label = es.Pattern.Value
				} else {
					fs.Label = strings.TrimSpace(val)
				}
			}

		case "parse":
			// break parser
			if es.Pattern.Value != "stop" {
				log.Fatal().Msgf("invalid parse value '%s'", es.Pattern.Value)
			}
			//log.Println("-- PARSE STOP --")
			return ParseStopError

		case "endian":
			// change endian
			fl.endian = es.Pattern.Value
			log.Debug().Msgf("Endian set to '%s' at %06x", fl.endian, fl.offset)

		case "filename":
			// record filename to use for the next data output operation
			fl.filename, err = fl.EvaluateStringExpression(es.Pattern.Value, dfParent)
			if err != nil {
				return err
			}
			fl.filename = presentStringValue(fl.filename)
			log.Info().Msgf("Output filename set to '%s' at %06x", fl.filename, fl.offset)

		case "offset":
			// set/restore current offset
			if es.Pattern.Value == "restore" {
				previousOffset := fl.popLastOffset()
				if DEBUG_OFFSET {
					log.Printf("--- RESTORED OFFSET FROM %04x TO %04x", fl.offset, previousOffset)
				}
				fl.offset = previousOffset
				_, err := r.Seek(int64(fl.offset), io.SeekStart)
				if err != nil {
					return err
				}
				continue
			}

			fl.offsetChanges++
			if fl.offsetChanges > 2000 {
				panic("debug recursion: too many offset changes from template")
				//return fmt.Errorf("too many offset changes from template")
			}
			previousOffset := fl.pushOffset()

			fl.offset, err = fl.EvaluateExpression(es.Pattern.Value, dfParent)
			log.Debug().Msgf("--- CHANGED OFFSET FROM %04x TO %04x (%s)", previousOffset, fl.offset, es.Pattern.Value)
			if err != nil {
				return err
			}
			_, err = r.Seek(int64(fl.offset), io.SeekStart)
			if err != nil {
				return err
			}

		case "data":
			switch es.Pattern.Value {
			case "invalid":
				return fmt.Errorf("file invalidated by template")
			case "unseen":
				fl.unseen = true
			default:
				log.Fatal().Msgf("unhandled data value '%s'", es.Pattern.Value)
			}

		case "until":
			// syntax: "until: u8 scanData ff d9"
			// creates a variable named scanData with all data up until terminating hex string
			parts := strings.SplitN(es.Pattern.Value, " ", 3)
			if len(parts) != 3 {
				panic("invalid input: " + es.Pattern.Value)
			}

			kind := parts[0]
			label := parts[1]
			if kind != "u8" && kind != "ascii" {
				panic(fmt.Sprintf("until directive don't support type '%s'", kind))
			}

			needle, err := value.ParseHexString(parts[2])
			if err != nil {
				panic(err)
			}

			feng.Yellow("Reading until marker from %06x: marker % 02x\n", fl.offset, needle)

			val, err := readBytesUntilMarker(r, 4096, needle)
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", label, fl.offset)
			}
			len := uint64(len(val))
			if len > 0 {
				es.Field.Kind = kind
				es.Field.Range = fmt.Sprintf("%d", len)
				es.Field.Label = label
				fs.Fields = append(fs.Fields, Field{
					Offset:   fl.offset,
					Length:   len,
					Value:    val,
					Format:   es.Field,
					Endian:   fl.endian,
					Filename: fl.filename,
				})
				fl.offset += len
			}

		case "u8", "i8", "u16", "i16", "u32", "i32", "u64", "i64",
			"f32",
			"xyzm32",
			"ascii", "utf16",
			"rgb8",
			"time_t_32", "filetime", "dostime", "dosdate", "dostimedate",
			"compressed:deflate", "compressed:lzo1x", "compressed:lzss", "compressed:lz4", "compressed:zlib",
			"raw:u8":
			// internal data types
			log.Debug().Msgf("expandChildren type %s: %s", es.Field.Kind, dfParent.Label)
			es.Field.Range = strings.ReplaceAll(es.Field.Range, "self.", dfParent.Label+".")
			unitLength, totalLength := fl.GetAddressLengthPair(&es.Field)

			if totalLength == 0 {
				if DEBUG {
					log.Printf("SKIPPING ZERO-LENGTH FIELD '%s' %s", es.Field.Label, fl.PresentType(&es.Field))
				}
				continue
			}

			endian := fl.endian
			if es.Field.Endian != "" {
				if DEBUG {
					feng.Yellow("-- endian override on field %s to %s\n", es.Field.Label, es.Field.Endian)
				}
				endian = es.Field.Endian
			}

			val, err := readBytes(r, totalLength, unitLength, endian)
			if DEBUG {
				log.Printf("[%08x] reading %d bytes for '%s.%s': %02x (err:%v)", fl.offset, totalLength, dfParent.Label, es.Field.Label, val, err)
			}
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
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

			matchPatterns, err := es.EvaluateMatchPatterns(val, endian)
			if err != nil {
				return err
			}

			fs.Fields = append(fs.Fields, Field{
				Offset:          fl.offset,
				Length:          totalLength,
				Value:           val,
				Format:          es.Field,
				Endian:          endian,
				Filename:        fl.filename,
				MatchedPatterns: matchPatterns,
			})
			fl.offset += totalLength

		case "vu32":
			// variable-length u32
			_, raw, len, err := value.ReadVariableLengthU32(r)
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}
			fs.Fields = append(fs.Fields, Field{
				Offset:   fl.offset,
				Length:   len,
				Value:    raw,
				Format:   es.Field,
				Endian:   fl.endian,
				Filename: fl.filename,
			})
			fl.offset += len

		case "vu64":
			// variable-length u64
			_, raw, len, err := value.ReadVariableLengthU64(r)
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}
			fs.Fields = append(fs.Fields, Field{
				Offset:   fl.offset,
				Length:   len,
				Value:    raw,
				Format:   es.Field,
				Endian:   fl.endian,
				Filename: fl.filename,
			})
			fl.offset += len

		case "vs64":
			// variable-length u64
			_, raw, len, err := value.ReadVariableLengthS64(r)
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}
			fs.Fields = append(fs.Fields, Field{
				Offset:   fl.offset,
				Length:   len,
				Value:    raw,
				Format:   es.Field,
				Endian:   fl.endian,
				Filename: fl.filename,
			})
			fl.offset += len

		case "asciiz":
			val, err := readBytesUntilZero(r)
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}
			len := uint64(len(val))
			fs.Fields = append(fs.Fields, Field{
				Offset:   fl.offset,
				Length:   len,
				Value:    val,
				Format:   es.Field,
				Endian:   fl.endian,
				Filename: fl.filename,
			})
			fl.offset += len

		case "utf16z":
			val, err := readBytesUntilMarker(r, 2, []byte{0, 0})
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}
			// append terminator marker since readBytesUntilMarker() excludes it
			val = append(val, []byte{0, 0}...)

			len := uint64(len(val))
			fs.Fields = append(fs.Fields, Field{
				Offset:   fl.offset,
				Length:   len,
				Value:    val,
				Format:   es.Field,
				Endian:   fl.endian,
				Filename: fl.filename,
			})
			fl.offset += len

		case "if":
			q := es.Field.Label
			a, err := fl.EvaluateExpression(q, dfParent)
			if err != nil {
				return err
			}
			lastIf = q
			if DEBUG {
				if a != 0 {
					log.Print("IF EVALUATED TRUE: q=", q, ", a=", a)
				} else {
					log.Print("IF EVALUATED FALSE: q=", q, ", a=", a)
				}
			}
			if a != 0 {
				err := fl.expandChildren(r, fs, dfParent, ds, es.Children)
				if err != nil {
					return err
				}
			}

		case "else":
			if DEBUG {
				log.Print("ELSE: evaluating", lastIf)
			}
			a, err := fl.EvaluateExpression(lastIf, dfParent)
			if err != nil {
				return err
			}
			if DEBUG {
				if a == 0 {
					log.Print("ELSE EVALUATED TRUE: lastIf=", lastIf, ", a=", a)
				} else {
					log.Print("ELSE EVALUATED FALSE: lastIf=", lastIf, ", a=", a)
				}
			}
			if a == 0 {
				err := fl.expandChildren(r, fs, dfParent, ds, es.Children)
				if err != nil {
					return err
				}
			}

		default:
			// find custom struct with given name
			customStruct, err := fl.GetStruct(es.Field.Kind)
			if err != nil {
				log.Error().Err(err).Msg("custom struct")
				found := false
				for _, ev := range fl.DS.EvaluatedStructs {
					if ev.Name == es.Field.Kind {
						found = true

						subEs, err := ds.FindStructure(es.Field.Kind)
						if err != nil {
							return err
						}

						log.Debug().Msgf("expanding custom struct '%s %s'", es.Field.Kind, es.Field.Label)

						es.Field.Range = strings.ReplaceAll(es.Field.Range, "self.", dfParent.Label+".")
						if es.Field.Range != "" {

							parsedRange, err := fl.EvaluateExpression(es.Field.Range, &es.Field)
							if err != nil {
								return err
							}

							log.Info().Msgf("appending ranged %s[%d]", es.Field.Kind, parsedRange)

							for i := uint64(0); i < parsedRange; i++ {

								// add this as child node to current struct (fs)

								name := fmt.Sprintf("%s_%d", es.Field.Label, i)
								parent := es.Field
								parent.Label = name
								log.Info().Msgf("-- Appending %s", name)

								// XXX issue happens when child node uses self.VARIABLE and it is expanded, when self node is not yet added to fs.Structs

								child := Struct{Name: name, Index: int(i)}
								err = fl.expandChildren(r, &child, &parent, ds, subEs.Expressions)
								if err != nil {
									if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
										panic(err)
										log.Error().Err(err).Msgf("expanding custom struct '%s %s'", es.Field.Kind, es.Field.Label)
									}
								}
								fs.Children = append(fs.Children, child)
							}
						} else {

							// add this as child node to current struct (fs)
							child := Struct{Name: es.Field.Label}
							err = fl.expandChildren(r, &child, &es.Field, ds, subEs.Expressions)
							if err != nil {
								if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
									log.Error().Err(err).Msgf("expanding custom struct '%s %s'", es.Field.Kind, es.Field.Label)
								}
							}
							fs.Children = append(fs.Children, child)
						}
						break
					}
				}
				if !found {
					// this error is critical. it means the parsed template is not working.
					log.Fatal().Msgf("error fetching struct '%s': %v", es.Field.Kind, err)
				}
				continue
			}

			log.Printf("%#v", customStruct)

			log.Error().Msgf("unhandled field '%#v'", es.Field)
			return fmt.Errorf("unhandled field kind '%s'", es.Field.Kind)
		}
	}

	return nil
}

// reads bytes from reader and returns them in network byte order (big endian)
func readBytes(r io.ReadSeeker, totalLength, unitLength uint64, endian string) ([]byte, error) {
	if unitLength > 1 && endian == "" {
		return nil, fmt.Errorf("endian is not set in file format template, don't know how to read data")
	}

	if totalLength > 1024*1024*1024 {
		return nil, fmt.Errorf("readBytes: attempt to read unexpected amount of data %d", totalLength)
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
// FIXME: won't find patterns overlapping chunks
func readBytesUntilMarker(r *bytes.Reader, chunkSize int64, search []byte) ([]byte, error) {

	if int(chunkSize) < len(search) {
		panic("unlikely")
	}

	chunk := make([]byte, int(chunkSize)+len(search))
	n, err := r.Read(chunk[:chunkSize])
	res := []byte{}

	var offset int64
	idx := bytes.Index(chunk[:chunkSize], search)
	for {
		//log.Printf("Read a slice of len %d, Index %d: % 02x", n, idx, chunk[:4])
		if idx >= 0 {
			res = append(res, chunk[:idx]...)

			// rewind to before marker
			_, err = r.Seek(int64(-(n - idx)), io.SeekCurrent)
			return res, err
		} else {
			//log.Printf("appended %d bytes: % 02x, res is %d len", len(chunk[:chunkSize]), chunk[:4], len(res))
			res = append(res, chunk[:chunkSize]...)
		}
		if err == io.EOF {
			return nil, nil
		} else if err != nil {
			return nil, err
		}

		offset += chunkSize

		n, err = r.Read(chunk[:chunkSize])

		idx = bytes.Index(chunk[:chunkSize], search)
	}
}
