package prometheus

import (
	"github.com/spf13/afero"
	"log"
	"os/exec"
	"strings"
)

// FS defines file system used to read and write configuration files
var FS = afero.NewOsFs()

var logPrintf = log.Printf
func getArgFromEnv(env, prefix string) (key, value string) {
	if strings.HasPrefix(env, prefix+"_") {
		values := strings.Split(env, "=")
		key = values[0]
		key = strings.Replace(key, prefix+"_", "", 1)
		key = strings.ToLower(key)
		value = values[1]
	}
	return key, value
}

var cmdRun = func(cmd *exec.Cmd) error {
	logPrintf(strings.Join(cmd.Args, " "))
	return cmd.Run()
}
