package main

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/martinlindhe/feng/mapper"
)

var args struct {
	Filename   string `kong:"arg" name:"filename" type:"existingfile" help:"Input file."`
	ExtractDir string `help:"Extract files to this directory."`
	Verbose    bool   `short:"v" help:"Be more verbose."`
	HideRaw    bool   `help:"Hide raw values"`
}

func main() {

	_ = kong.Parse(&args,
		kong.Name("feng"),
		kong.Description("A binary template reader and data presenter."))

	if args.Verbose {
		//template.DEBUG = true
	}

	fl, err := mapper.MapFileToTemplate(args.Filename)
	if err != nil {
		log.Fatal(err)
	}
	if args.ExtractDir != "" {
		// write data streams to specified dir
		err = os.MkdirAll(args.ExtractDir, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}

		for _, layout := range fl.Structs {
			log.Println("---", layout.Label)

			for _, field := range layout.Fields {
				switch field.Format.Kind {
				case "compressed:zlib":
					log.Printf("%s.%s %s: extracting zlib stream from %08x", layout.Label, field.Format.Label, fl.PresentType(&field.Format), field.Offset)

					reader, err := zlib.NewReader(bytes.NewReader(field.Value))
					if err != nil {
						log.Fatal(err)
					}
					defer reader.Close()

					var b bytes.Buffer
					if _, err = io.Copy(&b, reader); err != nil {
						log.Fatal(err)
					}

					filename := filepath.Join(args.ExtractDir, fmt.Sprintf("stream_zlib_%08x", field.Offset))

					log.Printf("extracted %d bytes to %s", b.Len(), filename)

					err = ioutil.WriteFile(filename, b.Bytes(), 0644)
					if err != nil {
						log.Fatal(err)
					}

				default:
					//log.Println("skipping", field.Format.Kind)
				}
			}
		}

	} else {
		fmt.Print(fl.Present(args.HideRaw))
	}
}
