package mapper

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	lzss "github.com/fbonhomm/LZSS/source"
	"github.com/pierrec/lz4/v4"
	"github.com/rasky/go-lzo"
	lzf "github.com/zhuyie/golzf"

	"github.com/rs/zerolog/log"
)

// Write data streams to outDir
// FIXME: dont evaluate if/else-blocks, so child values of them will not be extracted
func (fl *FileLayout) Extract(outDir string) error {

	err := os.MkdirAll(outDir, os.ModePerm)
	if err != nil {
		return err
	}

	for _, layout := range fl.Structs {

		for _, field := range layout.Fields {
			filename := ""
			if field.Outfile != "" {
				filename = field.Outfile
			}

			switch field.Format.Kind {
			case "compressed:lzo1x", "compressed:lzss", "compressed:lz4", "compressed:lzf", "compressed:zlib", "compressed:gzip", "compressed:deflate", "raw:u8", "encrypted:u8":
				if filename == "" {
					filename = fmt.Sprintf("stream_%08x", field.Offset)
				} else {
					// remove "res://" prefix
					filename = strings.Replace(filename, "res://", "", 1)
				}

				// handle windows path separators
				filename = strings.ReplaceAll(filename, "\\", string(os.PathSeparator))

				fullName := filepath.Join(outDir, filename)

				// TODO security: make sure that dirname is inside extract dir

				fullDirName := filepath.Dir(fullName)

				err = os.MkdirAll(fullDirName, 0755)
				if err != nil {
					return fmt.Errorf("failed to create directory '%s': %s", fullDirName, err)
				}

				log.Info().Msgf("%s.%s %s: Extracting data stream from %08x to %s", layout.Name, field.Format.Label, fl.PresentType(&field.Format), field.Offset, fullName)

				var b bytes.Buffer

				data, err := fl.peekBytes(&field)

				switch field.Format.Kind {
				case "compressed:lzo1x":
					expanded, err := lzo.Decompress1X(bytes.NewReader(data), 0, 0)
					if err != nil {
						log.Error().Err(err).Msgf("Extraction failed")
						continue
					}
					b.Write(expanded)

				case "compressed:lzss":
					lzssMode0 := lzss.LZSS{}
					expanded, err := lzssMode0.Decompress(data)
					if err != nil {
						log.Error().Err(err).Msgf("Extraction failed")
						continue
					}
					b.Write(expanded)

				case "compressed:lz4":
					expanded := make([]byte, 1024*1024) // XXX have a "known" expanded size value ready from format parsing
					n, err := lz4.UncompressBlock(data, expanded)
					if err != nil {
						log.Error().Err(err).Msgf("Extraction failed")
						continue
					}
					b.Write(expanded[0:n])

				case "compressed:lzf":
					expanded := make([]byte, 1024*1024) // XXX have a "known" expanded size value ready from format parsing
					n, err := lzf.Decompress(data, expanded)
					if err != nil {
						log.Error().Err(err).Msgf("Extraction failed")
						continue
					}
					b.Write(expanded[0:n])

				case "compressed:zlib":
					reader, err := zlib.NewReader(bytes.NewReader(data))
					if err != nil {
						log.Error().Err(err).Msgf("Extraction failed, writing untouched stream")
						b.Write(data) // write uncompressed stream
					} else {
						defer reader.Close()
						if _, err = io.Copy(&b, reader); err != nil {
							log.Error().Err(err).Msgf("Extraction failed")
							continue
						}
					}

				case "compressed:gzip":
					reader, err := gzip.NewReader(bytes.NewReader(data))
					if err != nil {
						log.Error().Err(err).Msgf("Extraction failed1")
						continue
					}
					defer reader.Close()

					if n, err := io.Copy(&b, reader); err != nil {
						// IMPORTANT: gzip decompressor errors out when input buffer is not exactly proper size, while extraction succeeds.
						if n == 0 {
							log.Error().Err(err).Msgf("Extraction failed, only %d written", n)
							continue
						}
					}

				case "compressed:deflate":
					reader := flate.NewReader(bytes.NewReader(data))
					defer reader.Close()

					var b bytes.Buffer
					if _, err = io.Copy(&b, reader); err != nil {
						log.Error().Err(err).Msgf("Extraction failed")
						continue
					}

				case "raw:u8":
					if field.Length <= 1 {
						continue
					}
					b.Write(data)

				case "encrypted:u8":
					dec, err := fl.DecryptData(data)
					if err != nil {
						log.Error().Err(err).Msgf("decryption failed")
					}
					b.Write(dec)

				default:
					log.Fatal().Msgf("unhandled type '%T'", field.Format.Kind) // unreachable
				}

				log.Debug().Msgf("Extracted %d bytes to %s", b.Len(), fullName)

				err = os.WriteFile(fullName, b.Bytes(), 0644)
				if err != nil {
					return err
				}

			}
		}
	}
	return nil
}
