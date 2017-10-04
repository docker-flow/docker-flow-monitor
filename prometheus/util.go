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
		println(env)
		values := strings.Split(env, "=")
		for i, v := range values {
			if i == 0 {
				key = v
				key = strings.Replace(key, prefix+"_", "", 1)
				key = strings.ToLower(key)
			} else {
				if len(value) > 0 {
					value += "="
				}
				value += v
			}
		}
	}
	return key, value
}

var cmdRun = func(cmd *exec.Cmd) error {
	logPrintf(strings.Join(cmd.Args, " "))
	return cmd.Run()
}
