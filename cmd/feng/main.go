package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

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

	fl, err := mapper.MapReader(r, ds)
	if err != nil {
		log.Fatal(err)
	}

	for _, layout := range fl.Structs {
		fmt.Printf("%s\n", layout.Label)

		for _, field := range layout.Fields {
			fmt.Printf("  [%06x] %-30s % 02x\n", field.Offset, field.Label, field.Value)
		}
	}
}
