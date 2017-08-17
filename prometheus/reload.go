package prometheus

import (
	"os"
	"os/exec"
	"sync"
)

var mu = &sync.Mutex{}

// Reload sends `pkill -HUP` signal to `prometheus` process.
var Reload = func() error {
	mu.Lock()
	defer mu.Unlock()
	logPrintf("Reloading Prometheus")
	cmd := exec.Command("pkill", "-HUP", "prometheus")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmdRun(cmd)
	if err != nil {
		logPrintf(err.Error())
	}
	logPrintf("Prometheus was reloaded")
	return err
}
