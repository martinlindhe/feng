package feng

import (
	"os"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	Red    = color.New(color.FgRed).PrintfFunc()
	Green  = color.New(color.FgGreen).PrintfFunc()
	Yellow = color.New(color.FgYellow).PrintfFunc()
)

func InitLogging() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
}
