package main

import (
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/davecgh/go-spew/spew"
	"github.com/martinlindhe/feng/mapper"
	"github.com/martinlindhe/feng/template"
)

func init() {
	log.SetFlags(log.Lshortfile)
}

var (
	filename     = kingpin.Arg("filename", "Input file.").Required().String()
	templateName = kingpin.Flag("template", "Enforce specific template.").Required().String()
)

func main() {

	kingpin.Parse()

	templateData, err := ioutil.ReadFile(*templateName)
	if err != nil {
		log.Fatal(err)
	}

	ds, err := template.UnmarshalTemplateIntoDataStructure(templateData)
	if err != nil {
		log.Fatal(err)
	}

	r, err := os.Open(*filename)
	if err != nil {
		log.Fatal(err)
	}

	ff, err := mapper.MapReader(r, ds)
	if err != nil {
		log.Fatal(err)
	}

	// XXX TODO 2. cli - file structure listing, meaning + actual data

	spew.Dump(ff)
}
