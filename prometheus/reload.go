package prometheus

import (
	"os/exec"
	"os"
)

var Reload = func() error {
	LogPrintf("Reloading Prometheus")
	cmd := exec.Command("pkill", "-HUP", "prometheus")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmdRun(cmd)
	if err != nil {
		LogPrintf(err.Error())
	}
	return err
}

