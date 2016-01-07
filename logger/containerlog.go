package logger

import (
	"bytes"
	"fmt"
	"github.com/mgutz/ansi"
	"io"
	"log"
	"strconv"
)

type ContainerLogWriter struct {
	*log.Logger
	colorCode []byte
}

var (
	resetCode            = []byte(ansi.ColorCode("reset"))
	LongestContainerName int
)

func NewContainerLogWriter(out io.Writer, containerName string, color string) *ContainerLogWriter {

	conOut := log.New(out,
		ansi.Color(fmt.Sprintf("%-"+strconv.Itoa(LongestContainerName)+"s| ", containerName), color),
		0)
	return &ContainerLogWriter{
		Logger:    conOut,
		colorCode: []byte(ansi.ColorCode(color)),
	}
}

func (w *ContainerLogWriter) Write(b []byte) (int, error) {
	toPrint := bytes.Split(bytes.Trim(b, "\n"), []byte{'\n'})
	for _, line := range toPrint {
		w.Printf("%s%s%s", w.colorCode, line, resetCode)
	}
	return len(b), nil
}
