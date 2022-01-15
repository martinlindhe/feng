package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/fatih/color"
	"github.com/martinlindhe/feng/mapper"
	"github.com/martinlindhe/feng/template"
)

var args struct {
	Folder  string `kong:"arg" name:"folder" type:"existingdir" help:"Input folder."`
	Fix     bool   `help:"Rename incorrect extensions."`
	Verbose bool   `help:"Be more verbose."`
}

var (
	red    = color.New(color.FgRed).SprintfFunc()
	green  = color.New(color.FgGreen).SprintfFunc()
	yellow = color.New(color.FgYellow).SprintfFunc()
)

func main() {

	_ = kong.Parse(&args,
		kong.Name("feng"),
		kong.Description("Batch rename recognized files in input folder."))

	templates, err := template.GetAllFilenames("./templates/")
	if err != nil {
		log.Fatal(err)
	}

	err = filepath.Walk(args.Folder, func(fp string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Println(err)
			return nil
		}
		if fi.IsDir() {
			return nil
		}

		extensions := []string{}
		for _, tpl := range templates {
			templateData, err := ioutil.ReadFile(tpl)
			if err != nil {
				log.Fatal(err)
			}
			ds, err := template.UnmarshalTemplateIntoDataStructure(templateData, tpl)
			if err != nil {
				log.Fatal(err)
			}

			r, err := os.Open(fp)
			if err != nil {
				log.Fatal(err)
			}

			fl, err := mapper.MapReader(r, ds)
			r.Close()
			if err != nil {
				// template don't match, try another
				if args.Verbose {
					log.Println(tpl, ":", err)
				}
				continue
			}

			extensions = append(extensions, fl.Extension)
		}

		ext := filepath.Ext(fp)
		if len(extensions) == 1 && ext == extensions[0] {
			if args.Verbose {
				fmt.Println(green("OK %s: %v", fp, extensions))
			}
		} else if len(extensions) == 1 {
			if args.Fix {
				newName := strings.TrimSuffix(fp, filepath.Ext(fp)) + extensions[0]
				fmt.Println(red("RENAMING %s => %s", fp, newName))
				oldName, err := filepath.Abs(fp)
				if err != nil {
					log.Fatal(err)
				}
				newName, err = filepath.Abs(newName)
				if err != nil {
					log.Fatal(err)
				}
				if err := os.Rename(oldName, newName); err != nil {
					log.Fatal(err)
				}
			} else {
				fmt.Println(red("WRONG EXT %s: %s", fp, extensions[0]))
			}

		} else if len(extensions) == 0 {
			fmt.Println(yellow("NO MATCH %s", fp))
		} else {
			fmt.Println(red("MULTI MATCH %s: %v", fp, extensions))
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
