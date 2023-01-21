package grmgr

import (
	"fmt"
	"log"
	"strings"
)

type LogLvl int

const (
	Alert LogLvl = iota
	Debug
	NoLog
)

var (
	logr    *log.Logger
	logLvl  LogLvl // Alert, Warning, High
	errlogr func(string, error)
)

func SetLogger(lg *log.Logger, level ...LogLvl) {
	var lvl LogLvl
	if len(level) > 1 {
		panic(fmt.Errorf("SetLogger: not maore than one level can be specified"))
	}
	if len(level) == 0 {
		// default level
		lvl = Alert
	} else {
		lvl = level[0]
	}
	logr = lg
	logLvl = lvl
}

func SetErrLogger(el func(l string, e error)) {
	errlogr = el
}

func TestErrLogger() {
	err := fmt.Errorf("Error: grmgr TestErrLoggerErr")
	logAlert("TestErrLogger 1")
	logErr(err)
	logAlert("TestErrLogger 2")
	logErr(err)
	logAlert("TestErrLogger 3")
	logErr(err)
	logAlert("TestErrLogger 4")
	logErr(err)
	logAlert("TestErrLogger 5")
	logErr(err)
	logAlert("TestErrLogger 6")
	logErr(err)
}

func logErr(e error) {
	if logr == nil {
		panic(fmt.Errorf("logAlert: no logger set. Use SetLogger()"))
	}

	if errlogr != nil {
		errlogr(logr.Prefix(), e)
	}

	var out strings.Builder

	out.WriteString("|error|")
	out.WriteString(e.Error())

	logr.Print(out.String())

}

func logFatal(e error) {
	LogFail(e)
}
func LogFail(e error) {
	if logr == nil {
		panic(fmt.Errorf("logAlert: no logger set. Use SetLogger()"))
	}

	if errlogr != nil {
		errlogr(logr.Prefix(), e)
	}
	var out strings.Builder

	out.WriteString("|fatal|")
	out.WriteString(e.Error())

	logr.Fatal(out.String())

}

func logDebug(s string) {
	if logr == nil {
		panic(fmt.Errorf("logAlert: no logger set. Use SetLogger()"))
	}
	if logLvl == Debug {
		var out strings.Builder

		out.WriteString("|info|")
		out.WriteString(s)
		logr.Print(out.String())
	}
}

func logAlert(s string) {

	if logr == nil {
		panic(fmt.Errorf("logAlert: no logger set. Use SetLogger()"))
	}
	if logLvl <= Debug {
		var out strings.Builder
		out.WriteString("|alert|")
		out.WriteString(s)
		logr.Print(out.String())
	}
}
