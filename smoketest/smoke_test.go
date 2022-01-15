package smoketest

import (
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/martinlindhe/feng/mapper"
	"github.com/martinlindhe/feng/template"
	"github.com/stretchr/testify/assert"
)

func TestCompareWithReferenceParses(t *testing.T) {
	data, err := ioutil.ReadFile("./smoketest.yml")
	assert.Nil(t, err)

	smoketests, err := UnmarshalData(data)
	assert.Nil(t, err)

	templates, err := template.GetAllFilenames("../templates/")
	assert.Nil(t, err)

	for _, entry := range smoketests.GenerateFilenames() {

		for _, tpl := range templates {
			templateData, err := ioutil.ReadFile(tpl)
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
				continue
			}

			//fmt.Printf("Parsed %s as %s\n\n", entry.In, tpl)

			data := fl.Present(false)

			actual, err := ioutil.ReadFile(entry.Out)
			if err != nil {
				log.Fatal(err)
			}
			assert.Equal(t, data, string(actual))
			break
		}
	}
}
