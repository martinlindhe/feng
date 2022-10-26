module github.com/martinlindhe/feng

go 1.18

require (
	github.com/alecthomas/kong v0.6.1
	github.com/davecgh/go-spew v1.1.1
	github.com/fatih/color v1.13.0
	github.com/fbonhomm/LZSS v0.0.0-20200907090355-ba1a01a92989
	github.com/maja42/goval v1.2.1
	github.com/pierrec/lz4/v4 v4.1.15
	github.com/pkg/errors v0.9.1
	github.com/rasky/go-lzo v0.0.0-20200203143853-96a758eda86e
	github.com/rs/zerolog v1.28.0
	github.com/stretchr/testify v1.7.2
	golang.org/x/text v0.3.7
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.16 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.1.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// https://github.com/fbonhomm/LZSS/pull/1
replace github.com/fbonhomm/LZSS v0.0.0-20200907090355-ba1a01a92989 => github.com/martinlindhe/LZSS v0.0.0-20221025204446-acc47c959dfe
