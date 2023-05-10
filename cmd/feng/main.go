package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"

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
	Time        bool   `help:"[Dev] Measure where processing time is spent."`
	CPUProfile  string `name:"cpu-profile" help:"[Dev] Create CPU profile."`
	MemProfile  string `name:"mem-profile" help:"[Dev] Create memory profile."`
}

func main() {

	var fs1 = afero.NewOsFs()

	_ = kong.Parse(&args,
		kong.Name("feng"),
		kong.Description("A binary template reader and data presenter."))

	stdOut := feng.InitLogging()
	defer stdOut.Flush()

	if args.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if args.CPUProfile != "" {
		f, err := fs1.Create(args.CPUProfile)
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

	f, err := fs1.Open(args.Filename)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to open %s.", args.Filename)
	}
	defer f.Close()

	cfg := &mapper.MapperConfig{
		F:                f,
		StartOffset:      args.Offset,
		TemplateFilename: args.Template,
		MeasureTime:      args.Time,
		Brief:            args.Brief,
	}

	if args.Template != "" {
		fl, err = mapper.MapFileToGivenTemplate(cfg)
	} else {
		fl, err = mapper.MapFileToMatchingTemplate(cfg)
	}
	if err != nil {
		if args.Brief {
			fmt.Printf("%s: %s\n", err, args.Filename)
			os.Exit(1)
		}
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
			fl.PresentStructureTree(fl.Structs)
		} else if args.Brief {
			fmt.Println(args.Filename+":", fl.BaseName)
		} else {
			fl.Present(&mapper.PresentFileLayoutConfig{
				ShowRaw:           args.Raw,
				ShowInDecimal:     args.Decimal,
				ReportUnmapped:    args.Unmapped,
				ReportOverlapping: args.Overlapping,
				InUTC:             !args.LocalTime,
			})
		}
	}

	if args.MemProfile != "" {
		f, err := fs1.Create(args.MemProfile)
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
