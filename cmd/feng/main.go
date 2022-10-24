package main

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/alecthomas/kong"
	"github.com/martinlindhe/feng"
	"github.com/martinlindhe/feng/mapper"
	"github.com/pierrec/lz4/v4"
)

var args struct {
	Filename    string `kong:"arg" name:"filename" type:"existingfile" help:"Input file."`
	ExtractDir  string `help:"Extract files to this directory."`
	Raw         bool   `help:"Show raw values"`
	Unmapped    bool   `help:"Print a report on unmapped bytes."`
	Overlapping bool   `help:"Print a report on overlapping bytes."`
	LocalTime   bool   `help:"Show timestamps in local timezone. Default is UTC."`
	Brief       bool   `help:"Show brief file information."`
	Tree        bool   `help:"Show parsed structure tree."`
	CPUProfile  string `name:"cpu-profile" help:"Create CPU profile."`
	MemProfile  string `name:"mem-profile" help:"Create memory profile."`
	Debug       bool   `help:"Enable debug logging"`
}

func main() {

	_ = kong.Parse(&args,
		kong.Name("feng"),
		kong.Description("A binary template reader and data presenter."))

	feng.InitLogging()
	if args.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if args.CPUProfile != "" {
		f, err := os.Create(args.CPUProfile)
		if err != nil {
			log.Fatal().Err(err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal().Err(err).Msg("could not start CPU profile")
		}
		defer pprof.StopCPUProfile()
	}

	fl, err := mapper.MapFileToTemplate(args.Filename)
	if err != nil {
		log.Fatal().Err(err)
	}
	if args.ExtractDir != "" {
		// write data streams to specified dir
		err = os.MkdirAll(args.ExtractDir, os.ModePerm)
		if err != nil {
			log.Fatal().Err(err)
		}

		for _, layout := range fl.Structs {
			for _, field := range layout.Fields {
				switch field.Format.Kind {
				case "compressed:lz4":
					log.Info().Msgf("%s.%s %s: extracting lz4 stream from %08x", layout.Name, field.Format.Label, fl.PresentType(&field.Format), field.Offset)

					expanded := make([]byte, 1024*1024) // XXX have a "known" expanded size value ready from format parsing
					n, err := lz4.UncompressBlock(field.Value, expanded)
					if err != nil {
						panic(err)
					}
					expanded = expanded[0:n]

					filename := filepath.Join(args.ExtractDir, fmt.Sprintf("stream_%08x", field.Offset))
					log.Info().Msgf("Extracted %d bytes to %s", len(expanded), filename)

					err = ioutil.WriteFile(filename, expanded, 0644)
					if err != nil {
						log.Fatal().Err(err)
					}

				case "compressed:zlib":
					log.Info().Msgf("%s.%s %s: extracting zlib stream from %08x", layout.Name, field.Format.Label, fl.PresentType(&field.Format), field.Offset)

					reader, err := zlib.NewReader(bytes.NewReader(field.Value))
					if err != nil {
						log.Fatal().Err(err)
					}
					defer reader.Close()

					var b bytes.Buffer
					if _, err = io.Copy(&b, reader); err != nil {
						log.Fatal().Err(err)
					}

					filename := filepath.Join(args.ExtractDir, fmt.Sprintf("stream_%08x", field.Offset))
					log.Info().Msgf("Extracted %d bytes to %s", b.Len(), filename)

					err = ioutil.WriteFile(filename, b.Bytes(), 0644)
					if err != nil {
						log.Fatal().Err(err)
					}

				case "compressed:deflate":
					log.Info().Msgf("%s.%s %s: extracting DEFLATE stream from %08x", layout.Name, field.Format.Label, fl.PresentType(&field.Format), field.Offset)

					reader := flate.NewReader(bytes.NewReader(field.Value))
					defer reader.Close()

					var b bytes.Buffer
					if _, err = io.Copy(&b, reader); err != nil {
						log.Fatal().Err(err)
					}

					filename := filepath.Join(args.ExtractDir, fmt.Sprintf("stream_%08x", field.Offset))
					log.Info().Msgf("Extracted %d bytes to %s", b.Len(), filename)

					err = ioutil.WriteFile(filename, b.Bytes(), 0644)
					if err != nil {
						log.Fatal().Err(err)
					}

				case "raw:u8":
					if len(field.Value) <= 1 {
						continue
					}
					log.Info().Msgf("%s.%s %s: extracting raw data stream from %08x", layout.Name, field.Format.Label, fl.PresentType(&field.Format), field.Offset)

					filename := filepath.Join(args.ExtractDir, fmt.Sprintf("stream_%08x", field.Offset))
					log.Info().Msgf("Extracted %d bytes to %s", len(field.Value), filename)

					err = ioutil.WriteFile(filename, field.Value, 0644)
					if err != nil {
						log.Fatal().Err(err)
					}
				}
			}
		}

	} else {
		if args.Tree {
			fmt.Print(fl.PresentStructureTree(fl.Structs))
		} else if args.Brief {
			// TODO: if brief, only do magic match + if no match do attempted fuzzy match.
			//       don't evaluate full struct (fast mode for scanning many files)
			fmt.Println(args.Filename+":", fl.BaseName)
		} else {
			fmt.Print(fl.Present(&mapper.PresentFileLayoutConfig{
				ShowRaw:           args.Raw,
				ReportUnmapped:    args.Unmapped,
				ReportOverlapping: args.Overlapping,
				InUTC:             !args.LocalTime,
			}))
		}
	}

	if args.MemProfile != "" {
		f, err := os.Create(args.MemProfile)
		if err != nil {
			log.Fatal().Err(err).Msg("could not create memory profile")
		}
		defer f.Close()
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal().Err(err).Msg("could not write memory profile")
		}
	}
}
