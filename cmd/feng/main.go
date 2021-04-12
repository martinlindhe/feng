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
			//log.Println(tpl, ":", err)
			continue
		}

		fmt.Println(tpl)

		fl.Present()
		break
	}

}
