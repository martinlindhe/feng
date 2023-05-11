package mapper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/martinlindhe/feng"
	"github.com/martinlindhe/feng/compression"

	"github.com/rs/zerolog/log"
)

// Write data streams to outDir
func (fl *FileLayout) Extract(outDir string) error {

	err := os.MkdirAll(outDir, os.ModePerm)
	if err != nil {
		return err
	}

	for _, layout := range fl.Structs {
		for _, field := range layout.Fields {
			if err := fl.extractField(&field, layout, outDir); err != nil {
				log.Error().Err(err).Msgf("Extract failed.")
			}
		}

		for _, child := range layout.Children {
			for _, field := range child.Fields {
				if err := fl.extractField(&field, layout, outDir); err != nil {
					log.Error().Err(err).Msgf("Extract failed.")
				}
			}
		}
	}

	return nil
}

func (fl *FileLayout) extractField(field *Field, layout *Struct, outDir string) error {

	filename := ""
	if field.Outfile != "" {
		filename = field.Outfile

		// handle windows path separators
		filename = strings.ReplaceAll(filename, "\\", string(os.PathSeparator))
	}

	if filename == "" {
		filename = fmt.Sprintf("stream_%08x", field.Offset)
	} else {
		// remove "res://" prefix
		filename = strings.Replace(filename, "res://", "", 1)
	}

	fullName := filepath.Join(outDir, filename)

	// TODO security: make sure that dirname is inside extract dir

	fullDirName := filepath.Dir(fullName)

	parts := strings.SplitN(field.Format.Kind, ":", 2)
	if len(parts) != 2 {
		return nil
	}

	err := os.MkdirAll(fullDirName, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory '%s': %s", fullDirName, err)
	}

	switch parts[0] {
	case "compressed", "raw", "encrypted":
	default:
		return nil
	}

	feng.Printf("<%s.%s> Extracting %s from %08x to %s:", layout.Name, field.Format.Label, fl.PresentType(&field.Format), field.Offset, fullName)

	f, err := fs1.Create(fullName)
	if err != nil {
		return err
	}
	defer f.Close()

	switch parts[0] {
	case "compressed":
		extractor, err := compression.ExtractorFactory(parts[1])
		if err != nil {
			return err
		}

		r, err := fl.readerToField(field)
		if err != nil {
			log.Error().Err(err).Msgf("Read failed")
			return nil
		}

		var expanded []byte
		if t, ok := extractor.(compression.Lzf); ok {
			t.CompressedSize = uint(field.Length)
			expanded, err = t.Extract(r)
		} else {
			expanded, err = extractor.Extract(r)
		}

		if err != nil {
			log.Error().Err(err).Msgf("Extraction failed")
			return nil
		}
		_, err = f.Write(expanded)
		if err != nil {
			return err
		}

		feng.Printf(" Extracted %d bytes -> %d\n", field.Length, FileSize(f))

	case "raw":
		if parts[1] != "u8" {
			log.Fatal().Msgf("invalid raw type '%s'", parts[1])
		}
		if field.Length <= 1 {
			return nil
		}
		data, err := fl.peekBytes(field)
		if err != nil {
			log.Error().Err(err).Msgf("Read failed")
			return nil
		}
		_, err = f.Write(data)
		if err != nil {
			return err
		}
		feng.Printf(" OK\n")

	case "encrypted":
		if parts[1] != "u8" {
			log.Fatal().Msgf("invalid raw type '%s'", parts[1])
		}
		data, err := fl.peekBytes(field)
		if err != nil {
			log.Error().Err(err).Msgf("Read failed")
			return nil
		}
		dec, err := fl.DecryptData(data)
		if err != nil {
			log.Error().Err(err).Msgf("decryption failed")
		}
		_, err = f.Write(dec)
		if err != nil {
			return err
		}
		feng.Printf(" Decrypted %d bytes\n", field.Length)
	}

	return nil
}
