package smoketest

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/martinlindhe/feng/mapper"
	"github.com/stretchr/testify/assert"
)

func TestCompareWithReferenceParses(t *testing.T) {
	smoketestFile := "./smoketest.yml"
	data, err := ioutil.ReadFile(smoketestFile)
	assert.Nil(t, err)

	smoketests, err := UnmarshalData(data)
	assert.Nil(t, err)

	filenames := smoketests.GenerateFilenames(filepath.Dir(smoketestFile))
	log.Println(len(filenames))

	for idx, entry := range filenames {
		assert.NotEqual(t, "", entry.In)
		fl, err := mapper.MapFileToTemplate(entry.In)
		assert.Nil(t, err, entry.In)

		//log.Printf("Parsed %s with template %v", entry.In, fl.BaseName)
		if !fileOrDirExists(entry.Out) {
			continue
		}

		data := fl.Present(false)
		expected, err := ioutil.ReadFile(entry.Out)
		if err != nil {
			//		log.Fatal(err)
			assert.FailNow(t, err.Error())
			continue
		}

		assert.Equal(t, string(expected), data, entry.In)
		//break
		if idx >= 9 { // XXX if >= 9, hang forever. also missing .out files here...
			break
		}
	}

}

// reports whether the named file or directory exists.
func fileOrDirExists(path string) bool {
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}
