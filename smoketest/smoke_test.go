package smoketest

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/martinlindhe/feng/mapper"
	"github.com/stretchr/testify/assert"
)

func TestCompareWithReferenceParses(t *testing.T) {
	data, err := ioutil.ReadFile("./smoketest.yml")
	assert.Nil(t, err)

	smoketests, err := UnmarshalData(data)
	assert.Nil(t, err)

	for _, entry := range smoketests.GenerateFilenames() {

		fl, err := mapper.MapFileToTemplate(entry.In)
		assert.Nil(t, err, entry.In)

		//fmt.Printf("Parsed %s as %s\n\n", entry.In, tpl)

		data := fl.Present(false)

		expected, err := ioutil.ReadFile(entry.Out)
		if err != nil {
			log.Fatal(err)
		}
		assert.Equal(t, string(expected), data, entry.In)
		break
	}
}
