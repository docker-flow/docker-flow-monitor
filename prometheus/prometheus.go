package prometheus

import (
	"log"
	"os"
	"strings"
	"fmt"
	"os/exec"
)

var logPrintf = log.Printf
var cmdRun = func(cmd *exec.Cmd) error {
	return cmd.Run()
}

type Prometheus struct {
}

func (s *Prometheus) RunPrometheus() error {
	logPrintf("Starting Prometheus")
	cmdString := "prometheus"
	for _, e := range os.Environ() {
		if key, value := s.getArgFromEnv(e, "ARG"); len(key) > 0 {
			key = strings.Replace(key, "_", ".", -1)
			cmdString = fmt.Sprintf("%s -%s=%s", cmdString, key, value)
		}
	}
	cmd := exec.Command("/bin/sh", "-c", cmdString)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmdRun(cmd)
}

func (s *Prometheus) getArgFromEnv(env, prefix string) (key, value string) {
	if strings.HasPrefix(env, prefix + "_") {
		values := strings.Split(env, "=")
		key = values[0]
		key = strings.TrimLeft(key, prefix)
		key = strings.ToLower(key)
		key = strings.Replace(key, "_", "", 1)
		value = values[1]
	}
	return key, value
}
