package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/martinlindhe/feng"
	"github.com/martinlindhe/feng/mapper"
	"github.com/martinlindhe/feng/smoketest"
)

var args struct {
	Filename string `kong:"arg" name:"filename" type:"existingfile" help:"Input yaml with file listing."`
}

func main() {

	_ = kong.Parse(&args,
		kong.Name("feng"),
		kong.Description("A binary template reader and data presenter."))

	data, err := ioutil.ReadFile(args.Filename)
	if err != nil {
		log.Fatal(err)
	}

	referenceRoot := "./smoketest/reference"

	err = os.RemoveAll(referenceRoot)
	if err != nil {
		log.Fatal(err)
	}

	smoketests, err := smoketest.UnmarshalData(data)
	if err != nil {
		log.Fatal(err)
	}
	filenames := smoketests.GenerateFilenames(filepath.Dir(args.Filename))

	for _, entry := range filenames {
		feng.Green("Start entry %s\n", entry.In)

		fl, err := mapper.MapFileToTemplate(entry.In)
		if err != nil {
			// template don't match, try another
			if _, ok := err.(mapper.EvaluateError); ok {
				log.Println(" failed to evaluate:", err)
			}

			log.Println("MapReader returned err:", err)

			continue
		}
		/*
			fs.WalkDir(feng.Templates, ".", func(tpl string, d fs.DirEntry, err2 error) error {
				// cannot happen
				if err != nil {
					log.Fatal(err)
				}
				if d.IsDir() {
					return nil
				}

				if filepath.Ext(tpl) != ".yml" {
					return nil
				}

				feng.Yellow("Parsing %s with %s ...\n", entry.In, tpl)
				//return nil

				templateData, err := fs.ReadFile(feng.Templates, tpl)
				if err != nil {
					log.Fatal(err)
				}
				ds, err := template.UnmarshalTemplateIntoDataStructure(templateData, tpl)
				if err != nil {
					log.Fatal(err)
				}

				r, err := os.Open(entry.In)
				if err != nil {
					log.Fatal(err)
				}

				fl, err := mapper.MapReader(r, ds)
				if err != nil {
					// template don't match, try another
					if _, ok := err.(mapper.EvaluateError); ok {
						log.Fatal(tpl, " failed to evaluate:", err)
					}

					log.Println("MapReader returned err:", err)

					if _, ok := err.(template.ValidationError); ok {
						return nil
					}
					return nil
				}
		*/
		if len(fl.Structs) == 0 {
			fmt.Println("MapReader failure, skipping")
			continue
		}

		feng.Green("Parsed %s as %s\n\n", entry.In, fl.BaseName)

		data := fl.Present(false)

		//base, _ := os.Getwd()
		//root := filepath.Join(base, referenceRoot)
		filename, _ := filepath.Abs(filepath.Join(referenceRoot, entry.Out)) // XXX invalid result
		path := filepath.Dir(filename)

		err = os.MkdirAll(path, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
		feng.Green("WRITE ROOT %s, out %s, full %s\n", referenceRoot, entry.Out, filename)
		err = ioutil.WriteFile(filename, []byte(data), 0644)
		if err != nil {
			log.Fatal(err)
		}

	}

}
