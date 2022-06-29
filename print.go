package feng

import "github.com/fatih/color"

var (
	Red    = color.New(color.FgRed).PrintfFunc()
	Green  = color.New(color.FgGreen).PrintfFunc()
	Yellow = color.New(color.FgYellow).PrintfFunc()
)
