package logger

import (
	"io/ioutil"
	"log"
	"os"
)

var (
	Trace   *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger

	//logFormat int = log.Ldate | log.Ltime | log.Lshortfile
	logFormat int  = 0
)

type LogLevel int

const (
	TraceLevel LogLevel = 0
	InfoLevel  LogLevel = 1
	WarnLevel  LogLevel = 2
	ErrorLevel LogLevel = 3
)

func init() {

	Trace = log.New(ioutil.Discard,
		"DEBUG: ",
		logFormat)

	Info = log.New(os.Stdout,
		"",
		logFormat)

	Warning = log.New(os.Stdout,
		"WARN: ",
		logFormat)

	Error = log.New(os.Stderr,
		"ERR: ",
		logFormat)
}

func SetDebug() {
	Trace = log.New(os.Stdout,
		"DEBUG: ",
		logFormat)
}
