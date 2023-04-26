package main

import (
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"

	"github.com/alecthomas/kong"
	"github.com/martinlindhe/feng"
	"github.com/martinlindhe/feng/mapper"
	"github.com/martinlindhe/feng/template"
)

var args struct {
	Folder  string `kong:"arg" name:"folder" type:"existingdir" help:"Input folder."`
	Fix     bool   `help:"Rename incorrect extensions."`
	Verbose bool   `help:"Be more verbose."`
}

func main() {

	var fs1 = afero.NewOsFs()

	_ = kong.Parse(&args,
		kong.Name("feng"),
		kong.Description("Batch rename recognized files in input folder."))

	err := filepath.Walk(args.Folder, func(tpl string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Print(err)
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

			r, err := fs1.Open(tpl)
			if err != nil {
				return err
			}

			cfg := &mapper.MapReaderConfig{
				F:  r,
				DS: ds,
			}

			fl, err := mapper.MapReader(cfg)
			r.Close()
			if err != nil {
				// template don't match, try another
				if args.Verbose {
					log.Print(tpl, ":", err)
				}
				return nil
			}

			extensions = append(extensions, fl.Extension)
			return nil
		})
		if err != nil {
			log.Fatal().Err(err).Msgf("failed")
		}

		ext := filepath.Ext(tpl)
		if len(extensions) == 1 && ext == extensions[0] {
			if args.Verbose {
				log.Info().Msgf("OK %s: %v\n", tpl, extensions)
			}
		} else if len(extensions) == 1 {
			if args.Fix {
				newName := strings.TrimSuffix(tpl, filepath.Ext(tpl)) + extensions[0]
				log.Warn().Msgf("RENAMING %s => %s\n", tpl, newName)
				oldName, err := filepath.Abs(tpl)
				if err != nil {
					log.Fatal().Err(err).Msgf("failed")
				}
				newName, err = filepath.Abs(newName)
				if err != nil {
					log.Fatal().Err(err).Msgf("failed")
				}
				if err := os.Rename(oldName, newName); err != nil {
					log.Fatal().Err(err).Msgf("failed")
				}
			} else {
				log.Error().Msgf("WRONG EXT %s: %s\n", tpl, extensions[0])
			}

		} else if len(extensions) == 0 {
			log.Warn().Msgf("NO MATCH %s\n", tpl)
		} else {
			log.Warn().Msgf("MULTI MATCH %s: %v\n", tpl, extensions)
		}

		return nil
	})

	if err != nil {
		log.Fatal().Err(err).Msgf("failed")
	}
}
