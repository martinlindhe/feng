package smoketest

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog/log"

	"github.com/martinlindhe/feng/mapper"
	"github.com/stretchr/testify/assert"
)

var (
	referenceRoot = "./reference"
	smoketestFile = "./smoketest.yml"
)

func TestaaaCompareWithReferenceParses(t *testing.T) {

	data, err := ioutil.ReadFile(smoketestFile)
	assert.Nil(t, err)

	smoketests, err := UnmarshalData(data)
	assert.Nil(t, err)

	filenames := smoketests.GenerateFilenames(filepath.Dir(smoketestFile))

	for _, entry := range filenames {
		assert.NotEqual(t, "", entry.In)
		fl, err := mapper.MapFileToTemplate(entry.In)
		assert.Nil(t, err, entry.In)

		log.Printf("Parsed %s with template %v", entry.In, fl.BaseName)
		expectedOutputFilename, _ := filepath.Abs(filepath.Join(referenceRoot, entry.Out))

		if !fileOrDirExists(expectedOutputFilename) {
			assert.Fail(t, "missing file "+expectedOutputFilename)
			continue
		}

		data := fl.Present(&mapper.PresentFileLayoutConfig{
			ShowRaw:           true,
			ReportOverlapping: true,
			InUTC:             true,
		})
		expected, err := ioutil.ReadFile(expectedOutputFilename)
		if err != nil {
			assert.Fail(t, err.Error())
			continue
		}

		assert.Equal(t, string(expected), data, entry.In)
	}
}

// reports whether the named file or directory exists.
func fileOrDirExists(path string) bool {
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}
