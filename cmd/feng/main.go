package main

import (
	"fmt"
	"path/filepath"
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
	Filename string `kong:"arg" name:"filename" type:"existingfile" help:"Input file."`
	Template string `type:"existingfile" help:"Parse file using this template."`
	Extract  bool   `help:"Extract data streams from input file." short:"x"`
	OutDir   string `help:"Write data streams to this directory. Requires --extract" short:"d"`
	//PackDir     string `help:"Packs directory of data streams according to filename layout." short:"p"`
	//OutFile     string `help:"Output filename. Requires --pack-dir" short:"o"`
	Offset      int64  `help:"Starting offset (default is 0)."`
	Raw         bool   `help:"Show raw values"`
	LocalTime   bool   `help:"Show timestamps in local timezone (default is UTC)."`
	Brief       bool   `help:"Show brief file information."`
	Tree        bool   `help:"Show parsed file structure tree."`
	Decimal     bool   `help:"Show offsets in decimal (default is hex)."`
	Unmapped    bool   `help:"[Dev] Print a report on unmapped bytes."`
	Overlapping bool   `help:"[Dev] Print a report on overlapping bytes."`
	Debug       bool   `help:"[Dev] Enable debug logging"`
	Time        bool   `help:"[Dev] Measure where processing time is spent."`
	CPUProfile  string `name:"cpu-profile" help:"[Dev] Create CPU profile."`
	MemProfile  string `name:"mem-profile" help:"[Dev] Create memory profile."`
}

var DEBUG_SLOWNESS = true

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

	/*
		if args.Extract && args.PackDir != "" {
			log.Fatal().Msg("--extract and --pack-file are mutually exclusive arguments")
		}
	*/

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
		MeasureTime:      args.Time || DEBUG_SLOWNESS,
		Brief:            args.Brief,
	}

	if args.Template != "" {
		fl, err = mapper.MapFileToGivenTemplate(cfg)
	} else {
		fl, err = mapper.MapFileToMatchingTemplate(cfg)
	}
	if err != nil {
		if args.Brief {
			size := mapper.FileSize(f)
			feng.Printf("%s: %s (%s)\n", err, args.Filename, mapper.ByteCountSI(size))
			return
		}
		log.Fatal().Err(err).Msgf("Failed to map %s.", args.Filename)
		return
	}

	if args.OutDir != "" {
		// --out-dir implies --extract mode
		args.Extract = true
	}

	if args.Extract {
		outDir := args.OutDir
		if outDir == "" {
			outDir = basenameWithoutExt(args.Filename)
		}

		err = fl.Extract(outDir)
		if err != nil {
			log.Fatal().Err(err).Msgf("Extraction failed.")
			return
		}
	} else {
		if args.Tree {
			fl.PresentStructureTree(fl.Structs)
		} else if args.Brief {
			size := mapper.FileSize(f)
			feng.Printf("%s: %s (%s)\n", args.Filename, fl.BaseName, mapper.ByteCountSI(size))
		} else {

			fmt.Printf("# %s (%s)\n", args.Filename, fl.DS.Name)

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

// returns the filename without path (basename) and without extension
func basenameWithoutExt(fileName string) string {
	s := filepath.Base(fileName)
	return s[:len(s)-len(filepath.Ext(s))]
}
