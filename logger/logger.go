package logger

import (
	"io/ioutil"
	"log"
	"os"
)

var (
	Debug   *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger

	//logFormat int = log.Ldate | log.Ltime | log.Lshortfile
	logFormat int = 0
	level         = InfoLevel
)

type LogLevel int

const (
	DebugLevel LogLevel = 0
	InfoLevel  LogLevel = 1
	WarnLevel  LogLevel = 2
	ErrorLevel LogLevel = 3
)

func init() {

	Debug = log.New(ioutil.Discard,
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

func GetLevel() LogLevel {
	return level
}

func SetDebug() {
	level = DebugLevel
	Debug = log.New(os.Stdout,
		"DEBUG: ",
		logFormat)
}
