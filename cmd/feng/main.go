package main

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"

	lzss "github.com/fbonhomm/LZSS/source"
	"github.com/pierrec/lz4/v4"
	"github.com/rasky/go-lzo"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/alecthomas/kong"
	"github.com/martinlindhe/feng"
	"github.com/martinlindhe/feng/mapper"
)

var args struct {
	Filename    string `kong:"arg" name:"filename" type:"existingfile" help:"Input file."`
	ExtractDir  string `help:"Extract files to this directory."`
	Raw         bool   `help:"Show raw values"`
	Unmapped    bool   `help:"Dev: Print a report on unmapped bytes."`
	Overlapping bool   `help:"Dev: Print a report on overlapping bytes."`
	LocalTime   bool   `help:"Show timestamps in local timezone. Default is UTC."`
	Brief       bool   `help:"Show brief file information."`
	Tree        bool   `help:"Show parsed file structure tree."`
	CPUProfile  string `name:"cpu-profile" help:"Dev: Create CPU profile."`
	MemProfile  string `name:"mem-profile" help:"Dev: Create memory profile."`
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
		log.Fatal().Err(err).Msgf("Failed to map %s.", args.Filename)

		return
	}
	if args.ExtractDir != "" {
		// write data streams to specified dir
		err = os.MkdirAll(args.ExtractDir, os.ModePerm)
		if err != nil {
			log.Fatal().Err(err)
		}

		for _, layout := range fl.Structs {

			for _, field := range layout.Fields {
				filename := ""
				if field.Filename != "" {
					filename = field.Filename
				}
				switch field.Format.Kind {
				case "compressed:lzo1x", "compressed:lzss", "compressed:lz4", "compressed:zlib", "compressed:gzip", "compressed:deflate", "raw:u8":
					if filename == "" {
						filename = fmt.Sprintf("stream_%08x", field.Offset)
					} else {
						// remove "res://" prefix
						filename = strings.Replace(filename, "res://", "", 1)
					}
					fullName := filepath.Join(args.ExtractDir, filename)

					// TODO security: make sure that dirname is inside extractdir

					fullDirName := filepath.Dir(fullName)
					err = os.MkdirAll(fullDirName, 0755)
					if err != nil {
						log.Fatal().Err(err)
					}

					log.Info().Msgf("%s.%s %s: Extracting data stream from %08x to %s", layout.Name, field.Format.Label, fl.PresentType(&field.Format), field.Offset, fullName)

					var b bytes.Buffer

					switch field.Format.Kind {
					case "compressed:lzo1x":
						expanded, err := lzo.Decompress1X(bytes.NewReader(field.Value), 0, 0)
						if err != nil {
							log.Error().Err(err).Msgf("Extraction failed")
							continue
						}
						b.Write(expanded)

					case "compressed:lzss":
						lzssMode0 := lzss.LZSS{}
						expanded, err := lzssMode0.Decompress(field.Value)
						if err != nil {
							log.Error().Err(err).Msgf("Extraction failed")
							continue
						}
						b.Write(expanded)

					case "compressed:lz4":
						expanded := make([]byte, 1024*1024) // XXX have a "known" expanded size value ready from format parsing
						n, err := lz4.UncompressBlock(field.Value, expanded)
						if err != nil {
							log.Error().Err(err).Msgf("Extraction failed")
							continue
						}
						b.Write(expanded[0:n])

					case "compressed:zlib":
						reader, err := zlib.NewReader(bytes.NewReader(field.Value))
						if err != nil {
							log.Error().Err(err).Msgf("Extraction failed")
							continue
						}
						defer reader.Close()

						if _, err = io.Copy(&b, reader); err != nil {
							log.Error().Err(err).Msgf("Extraction failed")
							continue
						}

					case "compressed:gzip":
						reader, err := gzip.NewReader(bytes.NewReader(field.Value))
						if err != nil {
							log.Error().Err(err).Msgf("Extraction failed1")
							continue
						}
						defer reader.Close()

						if n, err := io.Copy(&b, reader); err != nil {
							// IMPORTANT: gzip decompressor errors out when input buffer is not exactly proper size, while extraction succeeds.
							if n == 0 {
								log.Error().Err(err).Msgf("Extraction failed, only %d written", n)
								continue
							}
						}

					case "compressed:deflate":
						reader := flate.NewReader(bytes.NewReader(field.Value))
						defer reader.Close()

						var b bytes.Buffer
						if _, err = io.Copy(&b, reader); err != nil {
							log.Error().Err(err).Msgf("Extraction failed")
							continue
						}

					case "raw:u8":
						if len(field.Value) <= 1 {
							continue
						}
						b.Write(field.Value)

					default:
						panic(field.Format.Kind) // unreachable
					}

					log.Debug().Msgf("Extracted %d bytes to %s", b.Len(), fullName)

					err = os.WriteFile(fullName, b.Bytes(), 0644)
					if err != nil {
						log.Error().Err(err).Msgf("Write failed")
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
