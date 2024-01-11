package mapper

import (
	"strings"
	"testing"

	"github.com/martinlindhe/feng"
	"github.com/martinlindhe/feng/template"
	"github.com/stretchr/testify/assert"
)

var testPResentConfig = PresentFileLayoutConfig{
	ShowRaw: true,
}

func init() {
	feng.InitLogging()
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

func TestBitMatching(t *testing.T) {
	templateData := `
endian: big
structs:
  header:
    u8 Flags8:
      bit b0000_1111: Lo8
      bit b1111_0000: Hi8
    u16 Flags16:
      bit b0000_0000_0000_1111: Lo16
      bit b1111_1111_1111_0000: Hi16
    u32 Flags32:
      bit b0000_0000_0000_0000_0000_0000_0000_1111: Lo32
      bit b1111_1111_1111_1111_1111_1111_1111_0000: Hi32

layout:
  - header Header
`
	in := []byte{
		0x11,
		0x00, 0x11,
		0x00, 0x00, 0x00, 0x11,
	}
	expected := `
Header
  [000000] Flags8                         u8               17                             11
           - Lo8                          bit 0:4          1
           - Hi8                          bit 4:4          1
  [000001] Flags16                        u16 be           17                             00 11
           - Lo16                         bit 0:4          1
           - Hi16                         bit 4:12         1
  [000003] Flags32                        u32 be           17                             00 00 00 11
           - Lo32                         bit 0:4          1
           - Hi32                         bit 4:28         1
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
