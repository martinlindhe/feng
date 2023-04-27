package mapper

import (
	"testing"

	"github.com/martinlindhe/feng/template"
	"github.com/stretchr/testify/assert"
)

func TestIfFieldValueEqualsInt(t *testing.T) {
	templateData := `
endian: little
structs:
  header:
    u16 Type: ??
    if self.Type == 1:
      u8 TypeOne: ??

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData), "")
	assert.Equal(t, nil, err)

	f := mockFile(t, "in", []byte{
		0x01, 0x00, // Type
		0xff, // TypeOne
	})

	fl, err := MapReader(&MapReaderConfig{
		F:  f,
		DS: ds,
	})
	assert.Equal(t, nil, err)

	assert.Equal(t, `Header
  [000000] Type                           u16 le           1
  [000002] TypeOne                        u8               255

EOF
`, fl.Present(&PresentFileLayoutConfig{}))
}

func TestIfFieldValueEqualsIntNoMatch(t *testing.T) {
	templateData := `
endian: little
structs:
  header:
    u16 Type: ??
    if self.Type == 1:
      u8 TypeOne: ??

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData), "")
	assert.Equal(t, nil, err)

	f := mockFile(t, "in", []byte{
		0x02, 0x00, // Type
		0xff, // data
	})

	fl, err := MapReader(&MapReaderConfig{
		F:  f,
		DS: ds,
	})
	assert.Equal(t, nil, err)

	assert.Equal(t, `Header
  [000000] Type                           u16 le           2

0x0001 (1) unmapped bytes (33.3%)
Total file size 0x0003 (3)
`, fl.Present(&PresentFileLayoutConfig{}))
}

func TestIfFieldValueEqualsConstant(t *testing.T) {
	templateData := `
endian: little
constants:
  u16 TYPE_ONE: 00 01
structs:
  header:
    u16 Type: ??
    if self.Type == TYPE_ONE:
      u8 TypeOne: ??

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData), "")
	assert.Equal(t, nil, err)

	f := mockFile(t, "in", []byte{
		0x01, 0x00, // Type
		0xff, // TypeOne
	})

	fl, err := MapReader(&MapReaderConfig{
		F:  f,
		DS: ds,
	})
	assert.Equal(t, nil, err)

	assert.Equal(t, `Header
  [000000] Type                           u16 le           1
  [000002] TypeOne                        u8               255

EOF
`, fl.Present(&PresentFileLayoutConfig{}))
}

func TestIfFieldValueEqualsFieldConstant(t *testing.T) {
	templateData := `
endian: little
structs:
  header:
    u16 Type:
      eq 0001: TYPE_ONE
    if self.Type == TYPE_ONE:
      u8 TypeOne: ??

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData), "")
	assert.Equal(t, nil, err)

	f := mockFile(t, "in", []byte{
		0x01, 0x00, // Type
		0xff, // TypeOne
	})

	fl, err := MapReader(&MapReaderConfig{
		F:  f,
		DS: ds,
	})
	assert.Equal(t, nil, err)

	assert.Equal(t, `Header
  [000000] Type                           u16 le           1                     00 01
           - TYPE_ONE                     eq               1
  [000002] TypeOne                        u8               255                   ff

EOF
`, fl.Present(&PresentFileLayoutConfig{}))
}

func TestIfFieldValueEqualsFieldNestedConstant(t *testing.T) {
	templateData := `
endian: little
structs:
  header:
    u16 Type:
      eq 0001: TYPE_ONE
    if self.Type == TYPE_ONE:
      u8 Ext:
        eq f0: EXT_F0
      if self.Ext == EXT_F0:
        u8 TypeOne: ??

layout:
  - header Header
`
	ds, err := template.UnmarshalTemplateIntoDataStructure([]byte(templateData), "")
	assert.Equal(t, nil, err)

	f := mockFile(t, "in", []byte{
		0x01, 0x00, // Type
		0xf0, // EXT_F0
		0xff, // TypeOne
	})

	fl, err := MapReader(&MapReaderConfig{
		F:  f,
		DS: ds,
	})
	assert.Equal(t, nil, err)

	assert.Equal(t, `Header
  [000000] Type                           u16 le           1                     00 01
           - TYPE_ONE                     eq               1
  [000002] Ext                            u8               240                   f0
           - EXT_F0                       eq               240
  [000003] TypeOne                        u8               255                   ff

EOF
`, fl.Present(&PresentFileLayoutConfig{}))
}
