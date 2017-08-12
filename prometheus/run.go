package prometheus

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Run starts `prometheus` process
func Run() error {
	LogPrintf("Starting Prometheus")
	cmdString := "prometheus"
	for _, e := range os.Environ() {
		if key, value := getArgFromEnv(e, "ARG"); len(key) > 0 {
			key = strings.Replace(key, "_", ".", -1)
			cmdString = fmt.Sprintf("%s -%s=%s", cmdString, key, value)
		}
	}
	cmd := exec.Command("/bin/sh", "-c", cmdString)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmdRun(cmd)
}
