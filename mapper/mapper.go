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
	"regexp"
	"strings"
	"time"

	"github.com/maja42/goval"
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
	ErrParseStop     = errors.New("manual parse stop")
	ErrParseContinue = errors.New("manual parse continue")
)

type MapReaderConfig struct {
	F afero.File

	DS *template.DataStructure

	// initial endianness
	Endian string

	// base offset
	StartOffset int64

	Brief bool

	MeasureTime bool
}

// produces a list of fields with offsets and sizes from input reader based on data structure
func MapReader(cfg *MapReaderConfig) (*FileLayout, error) {

	ext := ""
	if len(cfg.DS.Extensions) > 0 {
		ext = cfg.DS.Extensions[0]
	}

	if cfg.Endian == "" {
		cfg.Endian = cfg.DS.Endian
	}

	fl := FileLayout{DS: cfg.DS, BaseName: cfg.DS.BaseName, startOffset: cfg.StartOffset, endian: cfg.Endian, measureTime: cfg.MeasureTime, Extension: ext, _f: cfg.F, eval: goval.NewEvaluator()}
	fl.size = FileSize(cfg.F)
	if cfg.Brief {
		return &fl, nil
	}

	functions := make(map[string]goval.ExpressionFunction)
	functions["peek_i32"] = fl.evalPeekI32
	functions["peek_i16"] = fl.evalPeekI16
	functions["peek_i8"] = fl.evalPeekI8
	functions["atoi"] = fl.evalAtoi
	functions["otoi"] = fl.evalOtoi
	functions["ceil"] = fl.evalCeil
	functions["abs"] = fl.evalAbs
	functions["alignment"] = evalAlignment
	functions["offset"] = fl.evalOffset
	functions["len"] = fl.evalLen
	functions["not"] = fl.evalNot
	functions["either"] = fl.evalEither
	functions["sevenbitstring"] = fl.evalSevenBitString
	functions["bitset"] = fl.evalBitSet
	functions["cleanstring"] = fl.evalCleanString
	functions["ext"] = fl.evalExt
	functions["no_ext"] = fl.evalNoExt
	functions["basename"] = fl.evalBasename
	functions["struct"] = fl.evalStruct
	fl.scriptFunctions = functions

	log.Debug().Msgf("mapping ds '%s'", cfg.DS.BaseName)

	for _, df := range cfg.DS.Layout {
		err := fl.mapLayout(nil, cfg.DS, &df)
		if err != nil {
			if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
				log.Error().Err(err).Msgf("mapLayout error processing %s at %06x", df.Label, fl.offset)
			}
			return &fl, nil
		}
	}

	return &fl, nil
}

func (fl *FileLayout) mapLayout(fs *Struct, ds *template.DataStructure, df *value.DataField) error {

	if df.Kind == "offset" {

		if df.Label == "restore" {
			previousOffset := fl.popLastOffset()
			if DEBUG_OFFSET {
				log.Printf("--- RESTORED OFFSET FROM %04x TO %04x", fl.offset, previousOffset)
			}
			fl.offset = previousOffset
			_, err := fl._f.Seek(int64(fl.offset), io.SeekStart)
			if err != nil {
				return err
			}
			return nil
		}

		// offset directive in top-level layout
		v, err := fl.EvaluateExpression(df.Label, df)
		if err != nil {
			return err
		}
		previousOffset := fl.pushOffset()

		fl.offset = v
		log.Debug().Msgf("--- CHANGED OFFSET FROM %04x TO %04x (%s)", previousOffset, fl.offset, df.Label)
		_, err = fl._f.Seek(int64(v), io.SeekStart)
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
		log.Debug().Msgf("appending sliced %s[] %s", df.Kind, df.Label)

		baseLabel := df.Label
		for i := uint64(0); i < math.MaxUint64; i++ {
			df.Index = int(i)
			df.Label = fmt.Sprintf("%s_%d", baseLabel, i)
			if err := fl.expandStruct(df, ds, es.Expressions, true); err != nil {
				if DEBUG_LABEL {
					log.Printf("--- used Label %s, restoring to %s", df.Label, baseLabel)
				}
				df.Label = baseLabel

				if errors.Is(err, ErrParseStop) {
					if DEBUG_MAPPER {
						log.Info().Msgf("reached ParseStop")
					}
					break
				}

				if errors.Is(err, ErrParseContinue) {
					continue
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
		//parsedRange, err := fl.evaluateExpressionWithExistingVariables(rangeQ, df)
		if err != nil {
			return err
		}

		log.Debug().Msgf("appending ranged %s[%d]", df.Kind, parsedRange)

		baseLabel := df.Label
		for i := int64(0); i < parsedRange; i++ {
			df.Index = int(i)
			df.Label = fmt.Sprintf("%s_%d", baseLabel, i)
			if err := fl.expandStruct(df, ds, es.Expressions, false); err != nil {
				if errors.Is(err, ErrParseContinue) {
					continue
				}
				df.Label = baseLabel
				return err
			}
		}
		return nil
	}

	if err := fl.expandStruct(df, ds, es.Expressions, false); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			// accept eof errors as valid parse for otherwise valid mapping
			log.Error().Msgf("reached EOF")
			return nil
		}
		log.Error().Msgf("%s errors out: %s\n", ds.BaseName, err.Error())
		return err
	}
	return nil
}

func FileSize(f afero.File) int64 {
	fi, err := f.Stat()
	if err != nil {
		log.Fatal().Err(err).Msg("stat failed")
	}
	return fi.Size()
}

var (
	errMapFileMatched = errors.New("matched file")
)

type MapperConfig struct {
	F                afero.File
	StartOffset      int64
	TemplateFilename string
	MeasureTime      bool

	// only detect by magic, don't map
	Brief bool
}

// maps input file to template specified in cfg.TemplateFilename
func MapFileToGivenTemplate(cfg *MapperConfig) (fl *FileLayout, err error) {

	rawTemplate, err := os.ReadFile(cfg.TemplateFilename)
	if err != nil {
		return nil, err
	}
	ds, err := template.UnmarshalTemplateIntoDataStructure(rawTemplate, cfg.TemplateFilename)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", cfg.TemplateFilename, err.Error())
	}

	_, err = cfg.F.Seek(cfg.StartOffset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	fl, err = MapReader(&MapReaderConfig{
		F:           cfg.F,
		DS:          ds,
		StartOffset: cfg.StartOffset,
		Brief:       cfg.Brief,
	})
	if err != nil {
		log.Error().Msgf("MapReader: %s: %s\n", cfg.TemplateFilename, err.Error())

	}
	if len(fl.Structs) > 0 {
		log.Info().Msgf("Parsed %s as %s", cfg.F.Name(), cfg.TemplateFilename)
	}
	return fl, nil
}

// returns true if magic bytes and optionally file extension matches
func (cfg *MapperConfig) MatchesMagic(ds *template.DataStructure) (bool, string) {

	if ds.NoMagic && len(ds.Magic) > 0 {
		log.Fatal().Msgf("error in template: no_magic and magic is set")
	}

	if ds.NoMagic {
		log.Debug().Msgf("MatchesMagic skip no_magic, template %s", ds.BaseName)
		return false, ""
	}

	for _, m := range ds.Magic {
		_, err := cfg.F.Seek(cfg.StartOffset+int64(m.Offset), io.SeekStart)
		if err != nil {
			log.Error().Err(err).Msgf("seek failed")
			return false, ""
		}
		weakMagic := false
		if len(m.Match) < 4 {
			weakMagic = true
		}
		b := make([]byte, len(m.Match))
		_, _ = cfg.F.Read(b)
		if bytes.Equal(m.Match, b) {
			extensions := m.Extensions
			if len(extensions) == 0 {
				extensions = ds.Extensions
			}

			if len(extensions) > 0 {

				found := false
				actualExtension := strings.ToLower(filepath.Ext(cfg.F.Name()))

				for _, knownExt := range extensions {
					if knownExt == actualExtension {
						found = true
					}
				}
				if !found {
					if weakMagic {
						log.Debug().Msgf("MatchesMagic skip match for %s, wrong extension '%s', expected '%s", ds.BaseName, actualExtension, extensions)
					} else {
						log.Warn().Msgf("MatchesMagic skip match for %s, wrong extension '%s', expected '%s", ds.BaseName, actualExtension, extensions)
					}
					return false, ""
				}
			}

			return true, m.Endian
		}
	}
	return false, ""
}

func mapTemplateIntoDataStructure(templateFilename string) (*template.DataStructure, error) {
	rawTemplate, err := fs.ReadFile(feng.Templates, templateFilename)
	if err != nil {
		return nil, err
	}
	ds, err := template.UnmarshalTemplateIntoDataStructure(rawTemplate, templateFilename)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", templateFilename, err.Error())
	}
	return ds, nil
}

func (cfg *MapperConfig) mapFileToReader(ds *template.DataStructure, endian string) (*FileLayout, error) {
	_, _ = cfg.F.Seek(cfg.StartOffset, io.SeekStart)

	fl, err := MapReader(&MapReaderConfig{
		F:           cfg.F,
		DS:          ds,
		StartOffset: cfg.StartOffset,
		Endian:      endian,
		Brief:       cfg.Brief,
		MeasureTime: cfg.MeasureTime,
	})
	if err != nil {
		// template don't match, try another
		if _, ok := err.(EvaluateError); ok {
			log.Error().Msgf("MapReader EvaluateError: %s: %s\n", fl.BaseName, err.Error())
		} else {
			return nil, nil
		}
	}
	if len(fl.Structs) > 0 {
		log.Printf("Parsed %s as %s", cfg.F.Name(), fl.BaseName)
		return fl, errMapFileMatched
	}
	return fl, err
}

// maps input file to a matching template
func MapFileToMatchingTemplate(cfg *MapperConfig) (fl *FileLayout, err error) {

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

		ds, err := mapTemplateIntoDataStructure(tpl)
		if err != nil {
			return err
		}
		processed++

		matched, endian := cfg.MatchesMagic(ds)
		if !matched {
			log.Debug().Msgf("%s magic bytes don't match", tpl)
			return nil
		}

		parseStart := time.Now()
		fl, err = cfg.mapFileToReader(ds, endian)

		fl.totalEvaluationTimeUntilMatch = time.Since(started)
		fl.evaluationTime = time.Since(parseStart)

		return err
	})
	if errors.Is(err, errMapFileMatched) {
		return fl, nil
	}
	if err != nil {
		return fl, err
	}

	if fl == nil {
		// if no magic match, try to find a filename extension match on any template with no_magic = true
		fl, err = mapFileToNoMagicMatchingExtension(cfg)
		if err != nil {
			// dump hex of first bytes for unknown files
			_, _ = cfg.F.Seek(0, io.SeekStart)
			buf := make([]byte, 10)
			n, _ := cfg.F.Read(buf)
			buf = buf[:n]

			s, _ := value.AsciiPrintableString(buf, len(buf))
			size := FileSize(cfg.F)
			return nil, fmt.Errorf("no match '%s' %s (%d bytes)", hex.EncodeToString(buf[:n]), s, size)
		}
	}

	return fl, nil
}

// try to find a filename extension match on any template with no_magic = true
func mapFileToNoMagicMatchingExtension(cfg *MapperConfig) (fl *FileLayout, err2 error) {

	err2 = fs.WalkDir(feng.Templates, ".", func(tpl string, d fs.DirEntry, err error) error {
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

		ds, err := mapTemplateIntoDataStructure(tpl)
		if err != nil {
			return err
		}

		if !ds.NoMagic {
			// only process no_magic templates
			return nil
		}

		// match on filenames
		matched := 0
		if len(ds.Filenames) > 0 {
			actualName := filepath.Base(cfg.F.Name())

			for _, wantedName := range ds.Filenames {
				re := regexp.MustCompile("(?i)" + wantedName)
				matches := re.MatchString(actualName)

				if matches {
					log.Debug().Msgf("%s: no_magic filename match %s == %v", tpl, wantedName, actualName)
					matched++
				}
			}
		}
		if matched == 0 {
			// match on extension
			actualExtension := strings.ToLower(filepath.Ext(cfg.F.Name()))
			for _, wantedExt := range ds.Extensions {
				if wantedExt == actualExtension {
					matched++
				}
			}
		}
		if matched == 0 || matched != 1 {
			log.Debug().Msgf("%s: no_magic extension don't match (count %d)", tpl, matched)
			return nil
		}

		log.Warn().Msgf("%s: WEAK MATCH", tpl) // TODO: improve presentation in 1-line --brief output

		fl, err = cfg.mapFileToReader(ds, ds.Endian)
		return err
	})
	if errors.Is(err2, errMapFileMatched) {
		return fl, nil
	}

	if fl == nil {
		return nil, fmt.Errorf("no no_magic match")
	}

	return fl, err2
}

func (fl *FileLayout) expandStruct(dfParent *value.DataField, ds *template.DataStructure, expressions []template.Expression, isSlice bool) error {

	if DEBUG_MAPPER {
		log.Printf("expandStruct: adding struct %s", dfParent.Label)
	}

	fs := &Struct{Name: dfParent.Label}
	fl.Structs = append(fl.Structs, fs)

	err := fl.expandChildren(fs, dfParent, ds, expressions)
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {

		if len(fl.Structs) < 1 || len(fl.Structs[0].Fields) == 0 {
			log.Error().Msgf("expandStruct error: [%08x] failed reading data for '%s' (err:%v)", fl.offset, dfParent.Label, err)

			return fmt.Errorf("eof and no structs mapped")
		}

		// NOTE: if we try to expand a slice of chunks, reaching EOF is expected and not an error
		if !isSlice {
			log.Error().Err(err).Msgf("reached EOF at %08x", fl.offset)
		}

	}

	return err
}

func presentStringValue(v string) string {
	v = strings.TrimRight(v, "·")
	v = strings.TrimRight(v, " ")
	return v
}

func (fl *FileLayout) expandChildren(fs *Struct, dfParent *value.DataField, ds *template.DataStructure, expressions []template.Expression) error {
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
			switch es.Pattern.Value {
			case "stop":
				// break parser
				//log.Println("-- PARSE STOP --")
				return ErrParseStop

			case "continue":
				// iterate to next expression
				//log.Info().Msgf("-- PARSE CONTINUE --")
				return ErrParseContinue
			}
			log.Fatal().Msgf("invalid parse value '%s'", es.Pattern.Value)

		case "endian":
			// change endian
			fl.endian = es.Pattern.Value
			log.Debug().Msgf("Endian set to '%s' at %06x", fl.endian, fl.offset)

		case "encryption":
			matches := strings.SplitN(es.Pattern.Value, ",", 2)
			if len(matches) != 2 {
				log.Fatal().Msgf("encryption: invalid value '%s'", es.Pattern.Value)
			}

			key, err := value.ParseHexString(strings.TrimSpace(matches[1]))
			if err != nil {
				log.Fatal().Err(err).Msgf("can't parse encryption key hex string '%s'", matches[1])
			}

			hashSize := map[string]byte{
				"aes_128_cbc": 16,
			}

			fl.encryptionMethod = strings.TrimSpace(matches[0])
			fl.encryptionKey = key

			if val, ok := hashSize[fl.encryptionMethod]; ok {
				if len(key) != int(val) {
					log.Fatal().Err(err).Msgf("encryption: key for %s must be %d bytes long, %d found", fl.encryptionMethod, val, len(key))
				}
			} else {
				log.Fatal().Msgf("encryption: unknown method '%s'", fl.encryptionMethod)
			}

			log.Info().Msgf("Encryption set to '%s' at %06x", fl.encryptionMethod, fl.offset)

		case "import":
			// import data block from another file
			// syntax: import: type, offset, size, filename
			matches := strings.SplitN(es.Pattern.Value, ",", 4)
			kind := matches[0]
			if kind != "raw:u8" { // TODO: handle more formats / refactor "import" with standard data types
				log.Fatal().Msgf("import: unknown type '%s'", kind)
			}

			offset, err := fl.EvaluateExpression(matches[1], dfParent)
			if err != nil {
				return err
			}
			size, err := fl.EvaluateExpression(matches[2], dfParent)
			if err != nil {
				return err
			}
			filename, err := fl.EvaluateStringExpression(matches[3], dfParent)
			if err != nil {
				return err
			}

			es.Field.Kind = kind
			es.Field.Range = fmt.Sprintf("%d", size)
			es.Field.Label = "import"

			fs.Fields = append(fs.Fields, Field{
				Offset:     offset,
				Length:     size,
				Format:     es.Field,
				Endian:     fl.endian,
				Outfile:    fl.filename,
				ImportFile: filename,
			})

		case "filename":
			// record filename to use for the next data output operation
			if strings.Contains(es.Pattern.Value, ".png") || strings.Contains(es.Pattern.Value, ".jpg") || strings.Contains(es.Pattern.Value, ".bin") {
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
				_, err := fl._f.Seek(int64(fl.offset), io.SeekStart)
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
			//fl.offset, err = fl.evaluateExpressionWithExistingVariables(es.Pattern.Value, dfParent)
			log.Debug().Msgf("--- CHANGED OFFSET FROM %04x TO %04x (%s)", previousOffset, fl.offset, es.Pattern.Value)
			if err != nil {
				return err
			}
			_, err = fl._f.Seek(int64(fl.offset), io.SeekStart)
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

			log.Info().Msgf("Reading until marker from %06x: marker % 02x\n", fl.offset, needle)

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
					Offset:  fl.offset,
					Length:  len,
					Format:  es.Field,
					Endian:  fl.endian,
					Outfile: fl.filename,
				})
				fl.offset += len
			}

		case "u8", "i8", "u16", "i16", "u24", "u32", "i32", "u64", "i64",
			"f32",
			"xyzm32",
			"ascii", "utf16", "sjis",
			"rgb8", "rgba32",
			"time_t_32", "filetime", "dostime", "dosdate", "dostimedate",
			"compressed:deflate", "compressed:lzo1x", "compressed:lzss", "compressed:lz4",
			"compressed:lzf", "compressed:zlib", "compressed:zlib_loose", "compressed:gzip",
			"compressed:lzma", "compressed:lzma2",
			"raw:u8", "encrypted:u8":
			// internal data types
			log.Debug().Msgf("expandChildren type %s: %s (child of %s)", es.Field.Kind, es.Field.Label, dfParent.Label)
			es.Field.Range = strings.ReplaceAll(es.Field.Range, "self.", dfParent.Label+".")
			unitLength, totalLength := fl.GetAddressLengthPair(&es.Field)

			if totalLength == 0 {
				log.Debug().Msgf("SKIPPING ZERO-LENGTH FIELD '%s' %s", es.Field.Label, fl.PresentType(&es.Field))
				continue
			}

			if es.Pattern.Known {
				log.Debug().Msgf("KNOWN PATTERN FOR '%s' %s", es.Field.Label, fl.PresentType(&es.Field))
			}

			endian := fl.endian
			if es.Field.Endian != "" {
				log.Debug().Msgf("-- endian override on field %s to %s\n", es.Field.Label, es.Field.Endian)
				endian = es.Field.Endian
			}

			val, err := fl.readBytes(totalLength, unitLength, endian)
			log.Debug().Msgf("[%08x] reading %d bytes for '%s.%s': %02x (err:%v)", fl.offset, totalLength, dfParent.Label, es.Field.Label, val, err)

			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}

			var matchPatterns []value.MatchedPattern
			// if known data pattern, see if it matches file data
			if es.Pattern.Known {

				if !bytes.Equal(es.Pattern.Pattern, val) {
					log.Debug().Msgf("[%08x] pattern '%s' does not match. expected '% 02x', got '% 02x'", fl.offset, es.Field.Label, es.Pattern.Pattern, val)
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
				Outfile:         fl.filename,
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
				Offset:  fl.offset,
				Length:  len,
				Format:  es.Field,
				Endian:  fl.endian,
				Outfile: fl.filename,
			})
			fl.offset += len

		case "vu64":
			// variable-length u64
			_, _, len, err := fl.ReadVariableLengthU64()
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}
			fs.Fields = append(fs.Fields, Field{
				Offset:  fl.offset,
				Length:  len,
				Format:  es.Field,
				Endian:  fl.endian,
				Outfile: fl.filename,
			})
			fl.offset += len

		case "vs64":
			// variable-length u64
			_, _, len, err := fl.ReadVariableLengthS64()
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}
			fs.Fields = append(fs.Fields, Field{
				Offset:  fl.offset,
				Length:  len,
				Format:  es.Field,
				Endian:  fl.endian,
				Outfile: fl.filename,
			})
			fl.offset += len

		case "asciiz", "utf8z":
			val, err := fl.readBytesUntilMarkerByte(0)
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}
			len := int64(len(val))
			fs.Fields = append(fs.Fields, Field{
				Offset:  fl.offset,
				Length:  len,
				Format:  es.Field,
				Endian:  fl.endian,
				Outfile: fl.filename,
			})
			fl.offset += len

		case "asciinl":
			val, err := fl.readBytesUntilMarkerByte('\n')
			if err != nil {
				return errors.Wrapf(err, "%s at %06x", es.Field.Label, fl.offset)
			}
			len := int64(len(val))
			fs.Fields = append(fs.Fields, Field{
				Offset:  fl.offset,
				Length:  len,
				Format:  es.Field,
				Endian:  fl.endian,
				Outfile: fl.filename,
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
				Offset:  fl.offset,
				Length:  len,
				Format:  es.Field,
				Endian:  fl.endian,
				Outfile: fl.filename,
			})
			fl.offset += len

		case "if":
			q := es.Field.Label
			a, err := fl.EvaluateExpression(q, dfParent)
			if err != nil {
				return err
			}
			lastIf = q
			if a != 0 {
				log.Debug().Msgf("IF EVALUATED TRUE: q=%s, a=%d", q, a)
			} else {
				log.Debug().Msgf("IF EVALUATED FALSE: q=%s, a=%d", q, a)
			}
			if a != 0 {
				err := fl.expandChildren(fs, dfParent, ds, es.Children)
				if err != nil {
					return err
				}
			}

		case "else":
			log.Debug().Msgf("ELSE: evaluating %s", lastIf)
			a, err := fl.EvaluateExpression(lastIf, dfParent)
			if err != nil {
				return err
			}
			if a == 0 {
				log.Debug().Msgf("ELSE EVALUATED TRUE: lastIf=%s, a=%d", lastIf, a)
			} else {
				log.Debug().Msgf("ELSE EVALUATED FALSE: lastIf=%s, a=%d", lastIf, a)
			}
			if a == 0 {
				err := fl.expandChildren(fs, dfParent, ds, es.Children)
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

						if es.Field.Slice {
							// like ranged layout but keep reading until EOF
							log.Debug().Msgf("appending sliced %s[] %s", es.Field.Kind, es.Field.Label)

							baseLabel := es.Field.Label
							for i := uint64(0); i < math.MaxUint64; i++ {

								es.Field.Index = int(i)
								es.Field.Label = fmt.Sprintf("%s_%d", baseLabel, i)
								if err := fl.expandStruct(&es.Field, ds, subEs.Expressions, true); err != nil {
									if DEBUG_LABEL {
										log.Printf("--- used Label %s, restoring to %s", es.Field.Label, baseLabel)
									}
									es.Field.Label = baseLabel

									if errors.Is(err, ErrParseStop) {
										if DEBUG_MAPPER {
											log.Info().Msgf("reached ParseStop")
										}
										break
									}

									if errors.Is(err, ErrParseContinue) {
										continue
									}

									if err == io.EOF {
										log.Error().Msgf("reached EOF")
										break
									}
									return err
								}
								if DEBUG_LABEL {
									log.Printf("--- used Label %s, restoring to %s", es.Field.Label, baseLabel)
								}
								es.Field.Label = baseLabel
							}
							continue
						}

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
								err = fl.expandChildren(child, &parent, ds, subEs.Expressions)
								if err != nil {
									//if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
									log.Error().Err(err).Msgf("expanding custom struct '%s %s'", es.Field.Kind, es.Field.Label)
									//}
									break
								}

							}
						} else {

							// add this as child node to current struct (fs)
							child := &Struct{Name: es.Field.Label}
							err = fl.expandChildren(child, &es.Field, ds, subEs.Expressions)
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
