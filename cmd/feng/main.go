package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/alecthomas/kong"
	"github.com/martinlindhe/feng/mapper"
	"github.com/martinlindhe/feng/template"
)

var args struct {
	Filename string `kong:"arg" name:"filename" type:"existingfile" help:"Input file."`
	Verbose  bool   `short:"v" help:"Be more verbose."`
	HideRaw  bool   `help:"Hide raw values"`
}

func main() {

	_ = kong.Parse(&args,
		kong.Name("feng"),
		kong.Description("A binary template reader and data presenter."))

	templates, err := template.GetAllFilenames("./templates/")
	if err != nil {
		log.Fatal(err)
	}

	for _, tpl := range templates {
		templateData, err := ioutil.ReadFile(tpl)
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
			// template don't match, try another
			if _, ok := err.(mapper.EvaluateError); ok {
				log.Println(tpl, ":", err)
			} else if args.Verbose {
				log.Println(tpl, ":", err)
			}
			continue
		}

		fmt.Println("Parsed as", tpl)
		fl.Present(args.HideRaw)
		break
	}
}
