package util

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/util/wait"
)

type GlogWriter struct{}

func init() {
	flag.Set("logtostderr", "true")
}

func (writer GlogWriter) Write(data []byte) (n int, err error) {
	glog.Info(string(data))
	return len(data), nil
}

func InitLogs() {
	log.SetOutput(GlogWriter{})
	log.SetFlags(0)
	flushFreq := 5 * time.Second
	go wait.Until(glog.Flush, flushFreq, wait.NeverStop)
}

func FlushLogs() {
	glog.Flush()
}

// All bootkube printing to stdout should go through this fmt.Printf wrapper.
// The stdout of bootkube should convey information useful to a human sitting
// at a terminal watching their cluster bootstrap itself. Otherwise the message
// should go to stderr.
func UserOutput(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}
