package main

import (
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/martinlindhe/feng/mapper"
	"github.com/martinlindhe/feng/template"
)

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

	fl, err := mapper.MapReader(r, ds)
	if err != nil {
		log.Fatal(err)
	}

	fl.Present()
}
