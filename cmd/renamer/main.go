package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/fatih/color"
	"github.com/martinlindhe/feng"
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

	err := filepath.Walk(args.Folder, func(tpl string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Println(err)
			return nil
		}
		if fi.IsDir() {
			return nil
		}

		extensions := []string{}

		err = fs.WalkDir(feng.Templates, ".", func(tpl string, d fs.DirEntry, err2 error) error {
			templateData, err := ioutil.ReadFile(tpl)
			if err != nil {
				return err
			}
			ds, err := template.UnmarshalTemplateIntoDataStructure(templateData, tpl)
			if err != nil {
				return err
			}

			r, err := os.Open(tpl)
			if err != nil {
				return err
			}

			fl, err := mapper.MapReader(r, ds)
			r.Close()
			if err != nil {
				// template don't match, try another
				if args.Verbose {
					log.Println(tpl, ":", err)
				}
				return nil
			}

			extensions = append(extensions, fl.Extension)
			return nil
		})
		if err != nil {
			log.Fatal(err)
		}

		ext := filepath.Ext(tpl)
		if len(extensions) == 1 && ext == extensions[0] {
			if args.Verbose {
				fmt.Println(green("OK %s: %v", tpl, extensions))
			}
		} else if len(extensions) == 1 {
			if args.Fix {
				newName := strings.TrimSuffix(tpl, filepath.Ext(tpl)) + extensions[0]
				fmt.Println(red("RENAMING %s => %s", tpl, newName))
				oldName, err := filepath.Abs(tpl)
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
				fmt.Println(red("WRONG EXT %s: %s", tpl, extensions[0]))
			}

		} else if len(extensions) == 0 {
			fmt.Println(yellow("NO MATCH %s", tpl))
		} else {
			fmt.Println(red("MULTI MATCH %s: %v", tpl, extensions))
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
