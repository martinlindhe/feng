package mapper

import (
	"strings"
	"testing"

	"github.com/martinlindhe/feng/template"
	"github.com/stretchr/testify/assert"
)

var testPResentConfig = PresentFileLayoutConfig{
	ShowRaw: true,
}

func TestPresentData1(t *testing.T) {
	templateData := `
structs:
  header:
    u8 Flags:
      bit b0001: B0
      bit b0110: Rest
      default: invalid
    #if Flags == 6:   		# FIXME should error out on "unknown variable name 'Flags'"
    if self.Flags == 6:
      u8 Child: ??

layout:
  - header Header
`
	in := []byte{0x06, 0x03}
	expected := `
Header
  [000000] Flags                          u8               6                              06
           - B0                           bit 0:1          0
           - Rest                         bit 1:2          3
  [000001] Child                          u8               3                              03
`

	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData), "")
	assert.Equal(t, nil, err)

	f := mockFile(t, "in", in)

	fl, err := MapReader(&MapReaderConfig{
		F:  f,
		DS: ds,
	})
	assert.Equal(t, nil, err)

	data := fl.PresentFullString(&testPResentConfig)
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(data))
}
