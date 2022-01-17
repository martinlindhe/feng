package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/martinlindhe/feng"
	"github.com/martinlindhe/feng/mapper"
	"github.com/martinlindhe/feng/smoketest"
	"github.com/martinlindhe/feng/template"
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
		panic(err)
	}

	err = os.RemoveAll("../smoketest/reference")
	if err != nil {
		log.Fatal(err)
	}

	smoketests, err := smoketest.UnmarshalData(data)
	if err != nil {
		panic(err)
	}

	for _, entry := range smoketests.GenerateFilenames() {
		fs.WalkDir(feng.Templates, ".", func(tpl string, d fs.DirEntry, err2 error) error {
			// cannot happen
			if err != nil {
				panic(err)
			}
			if d.IsDir() {
				return nil
			}

			if filepath.Ext(tpl) != ".yml" {
				return nil
			}

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
					log.Println(tpl, ":", err)
				}
				return nil
			}

			if len(fl.Structs) == 0 {
				fmt.Println("MapReader failure, skipping")
				return nil
			}

			fmt.Printf("Parsed %s as %s\n\n", entry.In, tpl)

			data := fl.Present(false)

			path := filepath.Dir(entry.Out)

			fmt.Println("MKDIR", path)
			err = os.MkdirAll(path, os.ModePerm)
			if err != nil {
				log.Fatal(err)
			}

			err = ioutil.WriteFile(entry.Out, []byte(data), 0644)
			if err != nil {
				panic(err)
			}
			return nil
		})
	}

}
