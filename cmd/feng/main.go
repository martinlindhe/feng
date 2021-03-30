package main

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/alecthomas/kong"
	"github.com/martinlindhe/feng/mapper"
	"github.com/martinlindhe/feng/template"
)

var args struct {
	Filename string `kong:"arg" name:"filename" type:"existingfile" help:"Input file."`
	Template string `kong:"required"  type:"existingfile" help:"Enforce specific template." placeholder:"FILE"`
}

func main() {

	_ = kong.Parse(&args,
		kong.Name("feng"),
		kong.Description("A binary template reader and data presenter."))

	templateData, err := ioutil.ReadFile(args.Template)
	if err != nil {
		log.Fatal(err)
	}

	ds, err := template.UnmarshalTemplateIntoDataStructure(templateData)
	if err != nil {
		log.Fatal(err)
	}

	r, err := os.Open(args.Filename)
	if err != nil {
		log.Fatal(err)
	}

	fl, err := mapper.MapReader(r, ds)
	if err != nil {
		log.Fatal(err)
	}

	fl.Present()
}
