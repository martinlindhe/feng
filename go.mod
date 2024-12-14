module github.com/martinlindhe/feng

go 1.18

require (
	github.com/JoshVarga/blast v0.0.0-20210808061142-eadad17358e8
	github.com/alecthomas/kong v1.6.0
	github.com/davecgh/go-spew v1.1.1
	github.com/maja42/goval v1.4.0
	github.com/pierrec/lz4/v4 v4.1.22
	github.com/pkg/errors v0.9.1
	github.com/rasky/go-lzo v0.0.0-20200203143853-96a758eda86e
	github.com/rs/zerolog v1.33.0
	github.com/spf13/afero v1.11.0
	github.com/stretchr/testify v1.8.1
	github.com/ulikunitz/xz v0.5.12
	golang.org/x/text v0.21.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/kr/pretty v0.3.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/fbonhomm/LZSS => github.com/martinlindhe/LZSS v0.0.0-20221025204446-acc47c959dfe
