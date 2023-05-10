package feng

import (
	"bufio"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var STDOUT = bufio.NewWriterSize(os.Stdout, 4096)

func InitLogging() *bufio.Writer {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()

	return STDOUT
}

func Fprintf(format string, a ...any) {
	fmt.Fprintf(STDOUT, format, a...)
}
