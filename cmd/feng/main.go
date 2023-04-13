package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/alecthomas/kong"
	"github.com/martinlindhe/feng"
	"github.com/martinlindhe/feng/mapper"
)

var args struct {
	Filename    string `kong:"arg" name:"filename" type:"existingfile" help:"Input file."`
	Template    string `type:"existingfile" help:"Parse file using this template."`
	OutDir      string `help:"Write files to this directory."`
	Offset      int64  `help:"Starting offset (default is 0)."`
	Raw         bool   `help:"Show raw values"`
	LocalTime   bool   `help:"Show timestamps in local timezone (default is UTC)."`
	Brief       bool   `help:"Show brief file information."`
	Tree        bool   `help:"Show parsed file structure tree."`
	Decimal     bool   `help:"Show offsets in decimal (default is hex)."`
	Unmapped    bool   `help:"Print a report on unmapped bytes."`
	Overlapping bool   `help:"Print a report on overlapping bytes."`
	Debug       bool   `help:"[Dev] Enable debug logging"`
	CPUProfile  string `name:"cpu-profile" help:"[Dev] Create CPU profile."`
	MemProfile  string `name:"mem-profile" help:"[Dev] Create memory profile."`
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
			log.Fatal().Err(err).Msgf("failed")
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal().Err(err).Msg("could not start CPU profile")
		}
		defer pprof.StopCPUProfile()
	}

	var fl *mapper.FileLayout
	var err error

	f, err := os.Open(args.Filename)
	defer f.Close()
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to open %s.", args.Filename)
	}

	if args.Template != "" {
		fl, err = mapper.MapFileToGivenTemplate(f, args.Offset, args.Filename, args.Template)
	} else {
		fl, err = mapper.MapFileToMatchingTemplate(f, args.Offset, args.Filename)
	}
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to map %s.", args.Filename)
		return
	}

	if args.OutDir != "" {
		err = fl.Extract(args.OutDir)
		if err != nil {
			log.Fatal().Err(err).Msgf("Extraction failed.")
			return
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
				ShowInDecimal:     args.Decimal,
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
