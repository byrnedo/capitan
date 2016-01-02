package logger

import (
	"log"
	"bytes"
	"github.com/mgutz/ansi"
"io"
)

type ContainerLogWriter struct{
	*log.Logger
	colorCode []byte
}

var resetCode = []byte(ansi.ColorCode("reset"))

func NewContainerLogWriter(out io.Writer, containerName string, color string) *ContainerLogWriter {

	conOut := log.New(out,
		ansi.Color(containerName+" | ", color),
		0)
	return &ContainerLogWriter{
		Logger: conOut,
		colorCode: []byte(ansi.ColorCode(color)),
	}
}

func (w *ContainerLogWriter) Write(b []byte) (int, error) {
	for _, row := range bytes.Split(b, []byte{'\r','\n'}) {
		if len(row) > 0 {
			w.Printf("%s%s%s", w.colorCode, bytes.Trim(row, " "), resetCode)
		}
	}
	return len(b), nil
}


