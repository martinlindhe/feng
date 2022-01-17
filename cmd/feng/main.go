package main

import (
	"fmt"
	"log"

	"github.com/alecthomas/kong"
	"github.com/martinlindhe/feng/mapper"
)

var args struct {
	Filename   string `kong:"arg" name:"filename" type:"existingfile" help:"Input file."`
	ExtractDir string `help:"Extract files to this directory."`
	Verbose    bool   `short:"v" help:"Be more verbose."`
	HideRaw    bool   `help:"Hide raw values"`
}

func main() {

	_ = kong.Parse(&args,
		kong.Name("feng"),
		kong.Description("A binary template reader and data presenter."))

	if args.Verbose {
		//template.DEBUG = true
	}

	fl, err := mapper.MapFileToTemplate(args.Filename)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(fl.Present(args.HideRaw))
}
