package prometheus

import (
	"log"
	"strings"
	"os/exec"
	"github.com/spf13/afero"
)

var logPrintf = log.Printf
var FS = afero.NewOsFs()

func getArgFromEnv(env, prefix string) (key, value string) {
	if strings.HasPrefix(env, prefix + "_") {
		values := strings.Split(env, "=")
		key = values[0]
		key = strings.TrimLeft(key, prefix + "_")
		key = strings.ToLower(key)
		value = values[1]
	}
	return key, value
}

var cmdRun = func(cmd *exec.Cmd) error {
	return cmd.Run()
}
