package mapper

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"

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
	ErrParseStop = errors.New("manual parse stop")
)

func (fl *FileLayout) mapLayout(rr afero.File, fs *Struct, ds *template.DataStructure, df *value.DataField) error {

	if df.Kind == "offset" {
		// offset directive in top-level layout
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

				if errors.Is(err, ErrParseStop) {
					if DEBUG_MAPPER {
						log.Print("reached ParseStop")
					}
					break
				}
				if err == io.EOF {
					log.Error().Msgf("reached EOF")
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
		for i := int64(0); i < parsedRange; i++ {
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
			log.Error().Msgf("reached EOF")
			return nil
		}
		feng.Yellow("%s errors out: %s\n", ds.BaseName, err.Error())
		return err
	}
	return nil
}

func fileSize(f afero.File) int64 {
	fi, err := f.Stat()
	if err != nil {
		log.Fatal().Err(err).Msg("stat failed")
	}
	return fi.Size()
}

// produces a list of fields with offsets and sizes from input reader based on data structure
func MapReader(f afero.File, ds *template.DataStructure, endian string) (*FileLayout, error) {

	ext := ""
	if len(ds.Extensions) > 0 {
		ext = ds.Extensions[0]
	}

	if endian == "" {
		endian = ds.Endian
	}

	fileLayout := FileLayout{DS: ds, BaseName: ds.BaseName, endian: endian, Extension: ext, _f: f}
	fileLayout.size = fileSize(f)

	if DEBUG {
		log.Printf("mapping ds '%s'", ds.BaseName)
	}

	for _, df := range ds.Layout {
		err := fileLayout.mapLayout(f, nil, ds, &df)
		if err != nil {
			if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
				log.Error().Err(err).Msgf("mapLayout error processing %s", df.Label)
			}
			return &fileLayout, nil
		}
	}

	return &fileLayout, nil
}

var (
	errMapFileMatched = errors.New("matched file")
)

func MapFileToGivenTemplate(f afero.File, startOffset int64, filename string, templateFileName string) (fl *FileLayout, err error) {

	rawTemplate, err := os.ReadFile(templateFileName)
	if err != nil {
		return nil, err
	}
	ds, err := template.UnmarshalTemplateIntoDataStructure(rawTemplate, templateFileName)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", templateFileName, err.Error())
	}

	f.Seek(startOffset, os.SEEK_SET)

	fl, err = MapReader(f, ds, "")
	fl.DataFileName = filename
	if err != nil {
		feng.Red("MapReader: %s: %s\n", templateFileName, err.Error())

	}
	if len(fl.Structs) > 0 {
		log.Printf("Parsed %s as %s", filename, templateFileName)
		return fl, nil
	}
	return nil, nil
}

// maps input file to a matching template
func MapFileToMatchingTemplate(f afero.File, startOffset int64, filename string, measureTime bool) (fl *FileLayout, err error) {

	started := time.Now()
	processed := 0

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
		processed++

		if ds.NoMagic {
			if DEBUG {
				log.Print("skip no_magic template", tpl)
			}
			return nil
		}

		// skip if no magic bytes matches
		found := false
		endian := ""
		for _, m := range ds.Magic {
			_, err = f.Seek(startOffset+int64(m.Offset), io.SeekStart)
			if err != nil {
				return err
			}
			b := make([]byte, len(m.Match))
			_, _ = f.Read(b)
			if bytes.Equal(m.Match, b) {
				found = true
				endian = m.Endian
				break
			}
		}
		if !found {
			if DEBUG {
				log.Info().Msgf("%s magic bytes don't match", tpl)
			}
			return nil
		}

		_, _ = f.Seek(startOffset, io.SeekStart)

		parseStart := time.Now()
		fl, err = MapReader(f, ds, endian)
		parseTime := time.Since(parseStart)
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
			if measureTime {
				passed := time.Since(started)
				log.Warn().Msgf("MEASURE: evaluation of %d templates until a match was found: %v, template parsed in %v", processed, passed, parseTime)
			}

			log.Printf("Parsed %s as %s", filename, tpl)
			return errMapFileMatched
		}
		return nil
	})
	if errors.Is(err, errMapFileMatched) {
		return fl, nil
	}
	if err != nil {
		return fl, err
	}

	if fl == nil {
		// dump hex of first bytes for unknown files
		_, _ = f.Seek(0, io.SeekStart)
		buf := make([]byte, 10)
		n, _ := f.Read(buf)
		buf = buf[:n]

		s, _ := value.AsciiPrintableString(buf, len(buf))

		return nil, fmt.Errorf("no match '%s' %s", hex.EncodeToString(buf[:n]), s)
	}
	return fl, nil
}

func (fl *FileLayout) expandStruct(r afero.File, dfParent *value.DataField, ds *template.DataStructure, expressions []template.Expression) error {

	if DEBUG_MAPPER {
		log.Printf("expandStruct: adding struct %s", dfParent.Label)
	}

	fs := &Struct{Name: dfParent.Label}
	fl.Structs = append(fl.Structs, fs)

	err := fl.expandChildren(r, fs, dfParent, ds, expressions)
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {

		// NOTE: if we try to expand a slice of chunks, reaching EOF is expected and not an error

		if len(fl.Structs) < 1 || len(fl.Structs[0].Fields) == 0 {
			log.Error().Msgf("expandStruct error: [%08x] failed reading data for '%s' (err:%v)", fl.offset, dfParent.Label, err)

			return fmt.Errorf("eof and no structs mapped")
		}

		log.Error().Msgf("reached EOF at %08x", fl.offset)
	}

	return err
}

func presentStringValue(v string) string {
	v = strings.TrimRight(v, "·")
	v = strings.TrimRight(v, " ")
	return v
}

func (fl *FileLayout) expandChildren(r afero.File, fs *Struct, dfParent *value.DataField, ds *template.DataStructure, expressions []template.Expression) error {
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
			return ErrParseStop

		case "endian":
			// change endian
			fl.endian = es.Pattern.Value
			log.Debug().Msgf("Endian set to '%s' at %06x", fl.endian, fl.offset)

		case "encryption":
			matches := strings.SplitN(es.Pattern.Value, " ", 2)
			if len(matches) != 2 {
				log.Fatal().Msgf("encryption: invalid value '%s'", es.Pattern.Value)
			}

			key, err := value.ParseHexString(matches[1])
			if err != nil {
				log.Fatal().Err(err).Msgf("can't parse encryption key hex string '%s'", matches[1])
			}

			hashSize := map[string]byte{
				"aes_128_cbc": 16,
			}

			fl.encryptionMethod = matches[0]
			fl.encryptionKey = key

			if val, ok := hashSize[fl.encryptionMethod]; ok {
				if len(key) != int(val) {
					log.Fatal().Err(err).Msgf("encryption: key for %s must be %d bytes long, %d found", fl.encryptionMethod, val, len(key))
				}
			} else {
				log.Fatal().Msgf("encryption: unknown method '%s'", fl.encryptionMethod)
			}

			log.Info().Msgf("Encryption set to '%s' at %06x", fl.encryptionMethod, fl.offset)

		case "filename":
			// record filename to use for the next data output operation
			if strings.Contains(es.Pattern.Value, ".png") {
				// don't evaluate plain filenames
				fl.filename = es.Pattern.Value
			} else {
				fl.filename, err = fl.EvaluateStringExpression(es.Pattern.Value, dfParent)
				if err != nil {
					return err
				}
			}
			fl.filename = presentStringValue(fl.filename)
			log.Debug().Msgf("Output filename set to '%s' at %06x", fl.filename, fl.offset)

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
			if fl.offsetChanges > 100_000 {
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

			val, err := fl.readBytesUntilMarkerSequence(4096, needle)
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", label, fl.offset)
			}
			len := int64(len(val))
			if len > 0 {
				es.Field.Kind = kind
				es.Field.Range = fmt.Sprintf("%d", len)
				es.Field.Label = label
				fs.Fields = append(fs.Fields, Field{
					Offset:   fl.offset,
					Length:   len,
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
			"compressed:deflate", "compressed:lzo1x", "compressed:lzss", "compressed:lz4",
			"compressed:lzf", "compressed:zlib", "compressed:gzip",
			"raw:u8", "encrypted:u8":
			// internal data types
			log.Debug().Msgf("expandChildren type %s: %s (child of %s)", es.Field.Kind, es.Field.Label, dfParent.Label)
			es.Field.Range = strings.ReplaceAll(es.Field.Range, "self.", dfParent.Label+".")
			unitLength, totalLength := fl.GetAddressLengthPair(&es.Field)

			if totalLength == 0 {
				if DEBUG {
					log.Printf("SKIPPING ZERO-LENGTH FIELD '%s' %s", es.Field.Label, fl.PresentType(&es.Field))
				}
				continue
			}

			if es.Pattern.Known {
				log.Info().Msgf("KNOWN PATTERN FOR '%s' %s", es.Field.Label, fl.PresentType(&es.Field))
			}

			endian := fl.endian
			if es.Field.Endian != "" {
				if DEBUG {
					feng.Yellow("-- endian override on field %s to %s\n", es.Field.Label, es.Field.Endian)
				}
				endian = es.Field.Endian
			}

			val, err := fl.readBytes(totalLength, unitLength, endian)
			if DEBUG {
				log.Printf("[%08x] reading %d bytes for '%s.%s': %02x (err:%v)", fl.offset, totalLength, dfParent.Label, es.Field.Label, val, err)
			}
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}

			var matchPatterns []value.MatchedPattern
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

			matchPatterns, err = es.EvaluateMatchPatterns(val, endian)
			if err != nil {
				return err
			}

			fs.Fields = append(fs.Fields, Field{
				Offset:          fl.offset,
				Length:          totalLength,
				Format:          es.Field,
				Endian:          endian,
				Filename:        fl.filename,
				MatchedPatterns: matchPatterns,
			})
			fl.offset += totalLength

		case "vu32":
			// variable-length u32
			_, _, len, err := fl.ReadVariableLengthU32()
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}
			fs.Fields = append(fs.Fields, Field{
				Offset:   fl.offset,
				Length:   len,
				Format:   es.Field,
				Endian:   fl.endian,
				Filename: fl.filename,
			})
			fl.offset += len

		case "vu64":
			// variable-length u64
			_, _, len, err := fl.ReadVariableLengthU64()
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}
			fs.Fields = append(fs.Fields, Field{
				Offset:   fl.offset,
				Length:   len,
				Format:   es.Field,
				Endian:   fl.endian,
				Filename: fl.filename,
			})
			fl.offset += len

		case "vs64":
			// variable-length u64
			_, _, len, err := fl.ReadVariableLengthS64()
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}
			fs.Fields = append(fs.Fields, Field{
				Offset:   fl.offset,
				Length:   len,
				Format:   es.Field,
				Endian:   fl.endian,
				Filename: fl.filename,
			})
			fl.offset += len

		case "asciiz":
			val, err := fl.readBytesUntilMarkerByte(0)
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}
			len := int64(len(val))
			fs.Fields = append(fs.Fields, Field{
				Offset:   fl.offset,
				Length:   len,
				Format:   es.Field,
				Endian:   fl.endian,
				Filename: fl.filename,
			})
			fl.offset += len

		case "asciinl":
			val, err := fl.readBytesUntilMarkerByte('\n')
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}
			len := int64(len(val))
			fs.Fields = append(fs.Fields, Field{
				Offset:   fl.offset,
				Length:   len,
				Format:   es.Field,
				Endian:   fl.endian,
				Filename: fl.filename,
			})
			fl.offset += len

		case "utf16z":
			val, err := fl.readBytesUntilMarkerSequence(2, []byte{0, 0})
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}
			// append terminator marker since readBytesUntilMarker() excludes it
			val = append(val, []byte{0, 0}...)

			len := int64(len(val))
			fs.Fields = append(fs.Fields, Field{
				Offset:   fl.offset,
				Length:   len,
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

							for i := int64(0); i < parsedRange; i++ {

								// add this as child node to current struct (fs)

								name := fmt.Sprintf("%s_%d", es.Field.Label, i)
								parent := es.Field
								parent.Label = name
								log.Info().Msgf("-- Appending %s", name)

								// XXX issue happens when child node uses self.VARIABLE and it is expanded,
								//     when self node is not yet added to fs.Structs

								child := &Struct{Name: name, Index: int(i)}
								fs.Children = append(fs.Children, child)
								err = fl.expandChildren(r, child, &parent, ds, subEs.Expressions)
								if err != nil {
									//if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
									log.Error().Err(err).Msgf("expanding custom struct '%s %s'", es.Field.Kind, es.Field.Label)
									//}
								}

							}
						} else {

							// add this as child node to current struct (fs)
							child := &Struct{Name: es.Field.Label}
							err = fl.expandChildren(r, child, &es.Field, ds, subEs.Expressions)
							if err != nil {
								//if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
								log.Error().Err(err).Msgf("expanding custom struct '%s %s'", es.Field.Kind, es.Field.Label)
								//}
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
