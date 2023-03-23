module github.com/martinlindhe/feng

go 1.18

require (
	github.com/alecthomas/kong v0.7.1
	github.com/davecgh/go-spew v1.1.1
	github.com/fatih/color v1.15.0
	github.com/fbonhomm/LZSS v0.0.0-20200907090355-ba1a01a92989
	github.com/maja42/goval v1.3.1
	github.com/pierrec/lz4/v4 v4.1.17
	github.com/pkg/errors v0.9.1
	github.com/rasky/go-lzo v0.0.0-20200203143853-96a758eda86e
	github.com/rs/zerolog v1.29.0
	github.com/stretchr/testify v1.8.1
	github.com/zhuyie/golzf v0.0.0-20161112031142-8387b0307ade
	golang.org/x/text v0.8.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/kr/pretty v0.1.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.18 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// https://github.com/fbonhomm/LZSS/pull/1
replace github.com/fbonhomm/LZSS v0.0.0-20200907090355-ba1a01a92989 => github.com/martinlindhe/LZSS v0.0.0-20221025204446-acc47c959dfe

replace github.com/zhuyie/golzf v0.0.0-20161112031142-8387b0307ade => ../golzf
