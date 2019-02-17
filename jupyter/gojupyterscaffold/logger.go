package gojupyterscaffold

import (
	"fmt"
	"log"
)

var logger = loggerWrapper{ll: &stdLogger{}}

// SetLogger sets the new LeveledLogger to record internal logs of gojupyterscaffold.
// By default, the logs are recorded with the standard log library.
func SetLogger(ll LeveledLogger) {
	logger = loggerWrapper{ll}
}

// LeveledLogger is the interface that wraps leveled logging like golang/glog.

// LeveledLogger is used to record logs in gojupyterscaffold internally.
// This exists to remove the direct dependency to golang/glog from gojupyterscaffold (See https://github.com/yunabe/lgo/issues/74 for details)
type LeveledLogger interface {
	Info(msg string)
	Warning(msg string)
	Error(msg string)
	Fatal(msg string)
}

type loggerWrapper struct{ ll LeveledLogger }

func (w loggerWrapper) Info(args ...interface{}) {
	w.ll.Info(fmt.Sprint(args...))
}
func (w loggerWrapper) Infof(format string, args ...interface{}) {
	w.ll.Info(fmt.Sprintf(format, args...))
}
func (w loggerWrapper) Warning(args ...interface{}) {
	w.ll.Warning(fmt.Sprint(args...))
}
func (w loggerWrapper) Warningf(format string, args ...interface{}) {
	w.ll.Warning(fmt.Sprintf(format, args...))
}
func (w loggerWrapper) Error(args ...interface{}) {
	w.ll.Error(fmt.Sprint(args...))
}
func (w loggerWrapper) Errorf(format string, args ...interface{}) {
	w.ll.Error(fmt.Sprintf(format, args...))
}
func (w loggerWrapper) Fatal(args ...interface{}) {
	w.ll.Fatal(fmt.Sprint(args...))
}
func (w loggerWrapper) Fatalf(format string, args ...interface{}) {
	w.ll.Fatal(fmt.Sprintf(format, args...))
}

type stdLogger struct{}

func (*stdLogger) Info(msg string)    { log.Print(msg) }
func (*stdLogger) Warning(msg string) { log.Print(msg) }
func (*stdLogger) Error(msg string)   { log.Print(msg) }
func (*stdLogger) Fatal(msg string)   { log.Fatal(msg) }
