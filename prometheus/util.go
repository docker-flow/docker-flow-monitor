package prometheus

import (
	"log"
	"strings"
	"os/exec"
	"github.com/spf13/afero"
)

var LogPrintf = log.Printf
var FS = afero.NewOsFs()

func getArgFromEnv(env, prefix string) (key, value string) {
	if strings.HasPrefix(env, prefix + "_") {
		values := strings.Split(env, "=")
		key = values[0]
		key = strings.Replace(key, prefix + "_", "", 1)
		key = strings.ToLower(key)
		value = values[1]
	}
	return key, value
}

func GetScrapeFromEnv(env string, prefix []string) (key, value string) {
	for _, p := range prefix {
		if strings.HasPrefix(env, p + "_") {
			values := strings.Split(env, "=")
			key = values[0]
			value = values[1]
		}
	}

	return key, value
}

var cmdRun = func(cmd *exec.Cmd) error {
	LogPrintf(strings.Join(cmd.Args, " "))
	return cmd.Run()
}
