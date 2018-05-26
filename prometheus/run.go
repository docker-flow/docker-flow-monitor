package prometheus

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Run starts `prometheus` process
var Run = func() error {
	logPrintf("Starting Prometheus")
	cmdString := "prometheus"
	flags := EnvToPrometheusFlags("ARG")
	if len(flags) > 0 {
		allFlags := strings.Join(flags, " ")
		cmdString = fmt.Sprintf("%s %s", cmdString, allFlags)
	}
	cmd := exec.Command("/bin/sh", "-c", cmdString)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmdRun(cmd)
}
