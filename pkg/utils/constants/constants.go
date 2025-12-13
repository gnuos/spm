// Package constants
package constants

import (
	"fmt"
	"os"
)

const (
	DefaultLogLevel   = "debug"
	DefaultDaemonName = "spm"
)

var SpmHome = getHome()

var DaemonLogFilePath = getDaemonPath("log")
var DaemonPidFilePath = getDaemonPath("pid")
var DaemonSockFilePath = getDaemonPath("sock")
var DaemonDumpFilePath = getDaemonPath("dump")

func getHome() string {
	return fmt.Sprintf("%s/.spm", os.Getenv("HOME"))
}

func getDaemonPath(suffix string) string {
	return fmt.Sprintf("%s/%s.%s", SpmHome, DefaultDaemonName, suffix)
}
